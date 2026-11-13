// Tests for diagram.js with a stub mermaid and a tiny fake DOM (no deps).
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { MERMAID_URL, loadMermaid, renderAll, mermaidTheme } from "./diagram.js";

class El {
  constructor(tag, doc) { this.tagName = tag.toUpperCase(); this.ownerDocument = doc; this.children = []; this.dataset = {}; this.attrs = {}; this.className = ""; this.textContent = ""; this.innerHTML = ""; this.listeners = {}; this.parent = null; }
  append(...cs) { for (const c of cs) { c.parent = this; this.children.push(c); } }
  remove() { if (this.parent) this.parent.children = this.parent.children.filter((c) => c !== this); }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  getAttribute(k) { return this.attrs[k] ?? null; }
  addEventListener(t, f) { (this.listeners[t] ||= []).push(f); }
  fire(t) { for (const f of this.listeners[t] || []) f({ type: t }); }
  all() { return this.children.flatMap((c) => [c, ...c.all()]); }
  querySelector(s) { return this.querySelectorAll(s)[0] || null; }
  querySelectorAll(s) {
    const m = {
      ".zen-diagram:not([data-rendered])": (e) => e.className.split(" ").includes("zen-diagram") && !("rendered" in e.dataset),
      "pre": (e) => e.tagName === "PRE",
      ".zen-plugin-error": (e) => e.className === "zen-plugin-error",
      ".zen-diagram-svg": (e) => e.className === "zen-diagram-svg",
    }[s];
    if (s.startsWith("script")) return this.all().filter((e) => e.tagName === "SCRIPT");
    assert.ok(m, "unsupported selector " + s);
    return this.all().filter(m);
  }
}
function makeDoc(appTheme) {
  const doc = { listeners: {} };
  doc.createElement = (t) => new El(t, doc);
  doc.head = new El("head", doc);
  doc.body = new El("body", doc);
  doc.documentElement = new El("html", doc);
  if (appTheme) doc.documentElement.dataset.theme = appTheme;
  doc.defaultView = { matchMedia: () => ({ matches: false }) };
  return doc;
}
function diagram(doc, src, theme = "neutral") {
  const d = doc.createElement("div");
  d.className = "zen-diagram"; d.dataset.theme = theme; d.dataset.zenPlugin = "diagram";
  const pre = doc.createElement("pre"); pre.textContent = src; d.append(pre);
  doc.body.append(d);
  return d;
}
function stub(fail) {
  const calls = { init: [], render: [] };
  return {
    calls,
    initialize(c) { calls.init.push(c); },
    async render(id, src) {
      calls.render.push({ id, src });
      if (fail && src.includes(fail)) throw new Error("Parse error on line 2");
      return { svg: `<svg id="${id}"><g/></svg>` };
    },
  };
}

test("renders each pending diagram into an svg and marks it", async () => {
  const doc = makeDoc();
  const a = diagram(doc, "graph TD\nA-->B");
  const b = diagram(doc, "graph LR\nX-->Y");
  const m = stub();
  await renderAll(doc.body, m);
  for (const d of [a, b]) {
    assert.equal("rendered" in d.dataset, true);
    assert.equal(d.querySelector("pre"), null);
    assert.match(d.querySelector(".zen-diagram-svg").innerHTML, /^<svg/);
  }
  assert.deepEqual(m.calls.render.map((c) => c.src), ["graph TD\nA-->B", "graph LR\nX-->Y"]);
  assert.equal(new Set(m.calls.render.map((c) => c.id)).size, 2);
  for (const c of m.calls.init) {
    assert.equal(c.startOnLoad, false);
    assert.equal(c.securityLevel, "strict");
  }
  await renderAll(doc.body, m);
  assert.equal(m.calls.render.length, 2, "rendered diagrams are skipped");
});

test("syntax error keeps the source and shows the message", async () => {
  const doc = makeDoc();
  const d = diagram(doc, "graph TD\nBAD<b>");
  await renderAll(doc.body, stub("BAD"));
  assert.equal(d.querySelector("pre").textContent, "graph TD\nBAD<b>");
  const err = d.querySelector(".zen-plugin-error");
  assert.equal(err.textContent, "diagram: Parse error on line 2");
  assert.equal(err.innerHTML, "", "message set as text, not HTML");
  assert.equal("rendered" in d.dataset, true);
});

test("theme follows data-theme, forced by the app theme", () => {
  const el = (t) => ({ dataset: { theme: t } });
  assert.equal(mermaidTheme(el("neutral"), makeDoc()), "neutral");
  assert.equal(mermaidTheme(el("default"), makeDoc()), "default");
  assert.equal(mermaidTheme(el("dark"), makeDoc("light")), "neutral");
  assert.equal(mermaidTheme(el("neutral"), makeDoc("dark")), "dark");
  assert.equal(mermaidTheme(el("bogus"), makeDoc()), "neutral");
  const sys = makeDoc(); sys.defaultView.matchMedia = () => ({ matches: true });
  assert.equal(mermaidTheme(el("neutral"), sys), "dark");
});

test("mermaid is lazy-loaded once from /_plugins/diagram/ as a classic script", async () => {
  const doc = makeDoc();
  const g = {};
  const p1 = loadMermaid(doc, g);
  const p2 = loadMermaid(doc, g);
  const scripts = doc.head.querySelectorAll("script");
  assert.equal(scripts.length, 1);
  assert.equal(scripts[0].src, "/_plugins/diagram/mermaid.min.js");
  assert.equal(MERMAID_URL, "/_plugins/diagram/mermaid.min.js");
  assert.notEqual(scripts[0].type, "module");
  const m = stub();
  g.mermaid = m;
  scripts[0].fire("load");
  assert.equal(await p1, m);
  assert.equal(await p2, m);
});

test("failed load rejects and can be retried", async () => {
  const doc = makeDoc();
  const g = {};
  const p = loadMermaid(doc, g);
  doc.head.querySelectorAll("script")[0].fire("error");
  await assert.rejects(p);
  loadMermaid(doc, g);
  assert.equal(doc.head.querySelectorAll("script").length, 1, "old tag removed before retry");
});

test("no remote URLs, eval or innerHTML from source in diagram.js", () => {
  const src = readFileSync(new URL("./diagram.js", import.meta.url), "utf8");
  assert.doesNotMatch(src, /[a-z][a-z0-9+.-]*:\/\//i);
  assert.doesNotMatch(src, /\beval\s*\(|new Function/);
  assert.match(src, /securityLevel:\s*"strict"/);
});
