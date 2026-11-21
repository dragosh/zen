import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { start, staticApi, staticHooks } from "./static";

/** A Storage over a Map, with the key order of insertion. */
function memoryStorage(): Storage {
  const m = new Map<string, string>();
  return {
    get length() { return m.size; },
    clear: () => m.clear(),
    getItem: (k) => (m.has(k) ? m.get(k)! : null),
    key: (i) => Array.from(m.keys())[i] ?? null,
    removeItem: (k) => void m.delete(k),
    setItem: (k, v) => void m.set(k, String(v)),
  };
}

beforeEach(() => {
  document.head.replaceChildren();
  document.body.replaceChildren();
  staticHooks.importModule = undefined;
  staticHooks.navigate = undefined;
  staticHooks.protocol = undefined;
});
afterEach(() => { vi.restoreAllMocks(); });

// ---- storage ----
test("storage: keys are namespaced by site id and kind, values are JSON text", async () => {
  const store = memoryStorage();
  const a = staticApi("notes", store);
  const b = staticApi("other", store);
  await a.pluginStorageSet("quiz", "score", "42");
  await a.layoutStorageSet("course", "step", '"two"');
  await b.pluginStorageSet("quiz", "score", "7");
  expect(store.getItem("zen.site.notes.plugin.quiz.score")).toBe("42");
  expect(store.getItem("zen.site.notes.layout.course.step")).toBe('"two"');
  expect(store.getItem("zen.site.other.plugin.quiz.score")).toBe("7");
  expect(await a.pluginStorageGet("quiz", "score")).toBe("42");
  expect(await b.pluginStorageGet("quiz", "score")).toBe("7");
  expect(await a.pluginStorageGet("quiz", "nothing")).toBe(""); // like the app: no value is ""
  expect(await a.layoutStorageGet("course", "step")).toBe('"two"');
  await a.pluginStorageSet("quiz", "q.1", "1");
  await a.pluginStorageSet("quiz", "r", "1");
  expect(await a.pluginStorageKeys("quiz", "")).toEqual(["q.1", "r", "score"]);
  expect(await a.pluginStorageKeys("quiz", "q.")).toEqual(["q.1"]);
  expect(await a.layoutStorageKeys("course", "")).toEqual(["step"]);
});

test("storage: invalid keys are refused, as in the app", async () => {
  const api = staticApi("notes", memoryStorage());
  await expect(api.pluginStorageSet("p", "bad key!", "1")).rejects.toThrow();
  await expect(api.pluginStorageSet("p", "", "1")).rejects.toThrow();
  await expect(api.pluginStorageSet("p", "x".repeat(65), "1")).rejects.toThrow();
});

test("storage: without localStorage it works for the session only", async () => {
  const broken: Storage = new Proxy({} as Storage, { get() { throw new Error("SecurityError"); } });
  const api = staticApi("notes", broken);
  await api.pluginStorageSet("p", "k", '"v"');
  expect(await api.pluginStorageGet("p", "k")).toBe('"v"');
  expect(await api.pluginStorageKeys("p", "")).toEqual(["k"]);
  // A write that fails halfway (quota) falls back too.
  const full = memoryStorage();
  full.setItem = () => { throw new Error("QuotaExceededError"); };
  const api2 = staticApi("notes", full);
  await api2.pluginStorageSet("p", "k", "1");
  expect(await api2.pluginStorageGet("p", "k")).toBe("1");
  // The default store is window.localStorage, and an unreadable one is not fatal.
  vi.spyOn(window, "localStorage", "get").mockImplementation(() => { throw new Error("denied"); });
  const api3 = staticApi("notes");
  await api3.pluginStorageSet("p", "k", "2");
  expect(await api3.pluginStorageGet("p", "k")).toBe("2");
});

// ---- invoke and stream ----
test("invoke and stream reject with the usual text", async () => {
  const api = staticApi("notes", memoryStorage());
  await expect(api.pluginInvoke("p", "a.md", "{}")).rejects.toThrow(new Error("runs in the zen app only"));
  await expect(api.pluginStream("p", "a.md", "{}")).rejects.toThrow(new Error("runs in the zen app only"));
});

// ---- mounting ----
type Page = { path: string; siteId?: string; root?: string; assets?: { css: string[]; js: string[] }; portable?: boolean };
function setup(page: Page, body: string, modules = ""): void {
  document.body.innerHTML =
    `<article class="zen-page">${body}</article>` +
    `<script type="application/json" id="zen-page">${JSON.stringify({ siteId: "notes", root: "", assets: { css: [], js: [] }, portable: false, ...page })}</script>` +
    modules;
}
const settle = async () => { for (let i = 0; i < 10; i++) await Promise.resolve(); await new Promise((r) => setTimeout(r, 0)); };

test("mounts like the app: plugin and layout scripts get the app's ctx", async () => {
  const imported: string[] = [];
  const seen: Record<string, any> = {};
  staticHooks.importModule = async (url: string) => {
    imported.push(url);
    const id = url.includes("/plugins/") ? "plugin" : "layout";
    return { default: { mount(el: HTMLElement, ctx: any) { seen[id] = ctx; el.dataset.mounted = id; } } };
  };
  setup(
    { path: "docs/a.md", root: "../", assets: { css: [], js: ["/_plugins/hello/hello.js", "/_layouts/memo/m.js"] } },
    `<div data-zen-layout="memo" data-zen-attrs='{"reading":"true"}'><div data-zen-plugin="hello" data-zen-name="hello">x</div></div>`,
  );
  start(document);
  await settle();
  expect(imported.sort()).toEqual(["../_zen/layouts/memo/m.js", "../_zen/plugins/hello/hello.js"].map((u) => new URL(u, document.baseURI).href).sort());
  expect(document.querySelector("[data-zen-plugin]")!.getAttribute("data-mounted")).toBe("plugin");
  expect(document.querySelector("[data-zen-layout]")!.getAttribute("data-mounted")).toBe("layout");
  expect(seen.plugin.id).toBe("hello");
  expect(seen.plugin.kind).toBe("block");
  expect(seen.plugin.page.path).toBe("docs/a.md");
  expect(seen.layout.kind).toBe("layout");
  expect(seen.layout.attrs).toEqual({ reading: "true" });
  expect(typeof seen.plugin.invoke).toBe("function");
  expect(seen.layout.invoke).toBeUndefined(); // layouts have no backend, as in the app
  // Storage goes to localStorage under the site id.
  await seen.plugin.storage.set("n", 3);
  expect(localStorage.getItem("zen.site.notes.plugin.hello.n")).toBe("3");
  await seen.layout.storage.set("step", "b");
  expect(localStorage.getItem("zen.site.notes.layout.memo.step")).toBe('"b"');
  expect(await seen.plugin.storage.get("n")).toBe(3);
  expect(await seen.plugin.storage.keys("")).toEqual(["n"]);
  localStorage.clear();
});

test("mounts like the app: a plugin that needs the app shows the standard error box", async () => {
  staticHooks.importModule = async () => ({
    default: { async mount(_el: HTMLElement, ctx: any) { await ctx.invoke({ a: 1 }); } },
  });
  vi.spyOn(console, "error").mockImplementation(() => {});
  setup({ path: "a.md", assets: { css: [], js: ["/_plugins/calc/calc.js"] } }, `<div data-zen-plugin="calc" data-zen-name="calc">x</div>`);
  start(document);
  await settle();
  const box = document.querySelector("[data-zen-plugin] .zen-plugin-error");
  expect(box?.textContent).toBe("calc: runs in the zen app only");
});

test("mounts like the app: stream rejects and a failing script leaves the rest alone", async () => {
  staticHooks.importModule = async () => ({
    default: { mount(el: HTMLElement, ctx: any) { ctx.stream({}, () => {}).done.catch((e: Error) => { el.dataset.err = e.message; }); } },
  });
  setup({ path: "a.md", assets: { css: [], js: ["/_plugins/s/s.js"] } }, `<p id="keep">kept</p><div data-zen-plugin="s" data-zen-name="s">x</div>`);
  start(document);
  await settle();
  expect(document.querySelector("[data-zen-plugin]")!.getAttribute("data-err")).toBe("runs in the zen app only");
  expect(document.getElementById("keep")!.textContent).toBe("kept");
});

test("navigate, anchor and onAnchor follow the static links; badge does nothing", async () => {
  let ctx: any;
  const urls: string[] = [];
  staticHooks.navigate = (u) => void urls.push(u);
  staticHooks.importModule = async () => ({ default: { mount(_el: HTMLElement, c: any) { ctx = c; } } });
  setup({ path: "docs/a.md", root: "../", assets: { css: [], js: ["/_plugins/n/n.js"] } }, `<div data-zen-plugin="n" data-zen-name="n">x</div>`);
  start(document);
  await settle();
  ctx.navigate("docs/b c.md", "sec");
  ctx.navigate("README.md");
  expect(urls).toEqual(["../docs/b%20c.html#sec", "../README.html"]);
  expect(ctx.anchor()).toBe("");
  const heard: string[] = [];
  const off = ctx.onAnchor((a: string) => heard.push(a));
  window.location.hash = "#two";
  window.dispatchEvent(new HashChangeEvent("hashchange"));
  expect(ctx.anchor()).toBe("two");
  expect(heard).toEqual(["two"]);
  off();
  window.dispatchEvent(new HashChangeEvent("hashchange"));
  expect(heard).toEqual(["two"]);
  expect(() => ctx.badge("a.md", "new")).not.toThrow();
  expect(ctx.assetUrl("mermaid.min.js")).toBe(new URL("../_zen/plugins/n/mermaid.min.js", document.baseURI).href);
  window.location.hash = "";
});

test("print links print the page", async () => {
  const print = vi.fn();
  (window as unknown as { print: () => void }).print = print;
  setup({ path: "a.md" }, `<a data-zen-action="print" href="#" role="button">Print</a>`);
  start(document);
  document.querySelector<HTMLElement>("[data-zen-action=print]")!.click();
  expect(print).toHaveBeenCalledTimes(1);
});

// ---- portable ----
test("portable: scripts come from the page as text and load through Blob URLs, never import() of a file", async () => {
  const text = "export default { mount(el) { el.dataset.p = '1' } }";
  const blobs: Blob[] = [];
  vi.spyOn(URL, "createObjectURL").mockImplementation((b: Blob | MediaSource) => { blobs.push(b as Blob); return `blob:null/${blobs.length}`; });
  const imported: string[] = [];
  staticHooks.importModule = async (url: string) => {
    imported.push(url);
    return { default: { mount(el: HTMLElement) { el.dataset.p = "1"; } } };
  };
  setup(
    { path: "a.md", portable: true, assets: { css: [], js: ["/_plugins/hello/hello.js"] } },
    `<div data-zen-plugin="hello" data-zen-name="hello">x</div>`,
    `<script type="application/json" data-zen-module="plugin:hello">${JSON.stringify({ "hello.js": text })}</script>`,
  );
  start(document);
  await settle();
  expect(imported).toEqual(["blob:null/1"]);
  expect(blobs).toHaveLength(1);
  expect(await blobs[0].text()).toBe(text);
  expect(blobs[0].type).toBe("text/javascript");
  expect(document.querySelector("[data-zen-plugin]")!.getAttribute("data-p")).toBe("1");
  expect(imported.some((u) => u.includes("_zen/"))).toBe(false);
});

test("portable: a script that is not in the page is an error in its block, not a request", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const imported: string[] = [];
  staticHooks.importModule = async (url: string) => { imported.push(url); return {}; };
  setup({ path: "a.md", portable: true, assets: { css: [], js: ["/_plugins/hello/hello.js"] } }, `<div data-zen-plugin="hello" data-zen-name="hello">x</div>`);
  start(document);
  await settle();
  expect(imported).toEqual([]);
  expect(document.querySelector("[data-zen-plugin] .zen-plugin-error")).not.toBeNull();
});

test("start does nothing on a page without data", () => {
  expect(() => start(document)).not.toThrow();
  document.body.innerHTML = `<script type="application/json" id="zen-page">{not json</script>`;
  expect(() => start(document)).not.toThrow();
});

test("hosted output opened from disk says why its scripts do not run, without trying to load them", async () => {
  const { HOSTED_FROM_DISK } = await import("./static");
  const loaded: string[] = [];
  staticHooks.protocol = "file:";
  staticHooks.importModule = async (u) => { loaded.push(u); return { default: { mount() {} } }; };
  setup({ path: "a.md", root: "", assets: { css: [], js: ["/_plugins/n/n.js"] } }, `<div data-zen-plugin="n" data-zen-name="n">x</div>`);
  start(document);
  await settle();
  expect(loaded).toEqual([]);
  expect(document.querySelector(".zen-plugin-error")?.textContent).toContain(HOSTED_FROM_DISK);
});

// ---- the site sidebar's disclosure arrows (CSS only)
test("group summaries get a chevron and no native marker", () => {
  const css = readFileSync(resolve(import.meta.dirname, "static.css"), "utf8");
  expect(css).toMatch(/\.zen-site-nav summary\s*\{[^}]*list-style:\s*none/);
  expect(css).toContain(".zen-site-nav summary::-webkit-details-marker");
  expect(css).toMatch(/\.zen-site-nav summary::before\s*\{[^}]*border-right/);
  expect(css).toMatch(/\.zen-site-nav details\[open\] > summary::before\s*\{[^}]*rotate\(45deg\)/);
  expect(css).toMatch(/prefers-reduced-motion[^{]*\{[^}]*summary::before/);
});

// ---- the top bar (ui/site-topbar) ----
const BAR = `<div class="zen-site-frame" data-test>
<aside class="zen-site-side" id="zen-site-side"><details class="zen-site-menu"><summary>Contents</summary><nav><ul><li><a href="a.html">A</a></li></ul></nav></details></aside>
<div class="zen-site-body"><header class="zen-top zen-site-top"><div class="zen-top-controls">
<button type="button" class="ghost zen-site-sidebar-toggle" aria-label="Hide sidebar" title="Hide sidebar (⌘\\)" aria-expanded="true" aria-controls="zen-site-side" hidden></button>
<button type="button" class="ghost zen-site-back" aria-label="Back" title="Back (⌘[)" disabled hidden>‹</button>
<button type="button" class="ghost zen-site-forward" aria-label="Forward" title="Forward (⌘])" disabled hidden>›</button>
</div><nav class="zen-site-crumbs"><ol><li>Home</li></ol></nav></header>
<main><article class="zen-page">x</article></main></div></div>
<script type="application/json" id="zen-page">{"path":"a.md","siteId":"notes","root":"","assets":{"css":[],"js":[]},"portable":false}</script>`;
const q = <T extends HTMLElement>(s: string) => document.querySelector<T>(s)!;
function bar(): void {
  document.body.innerHTML = BAR;
  start(document);
}
const key = (k: string, mods: KeyboardEventInit = { metaKey: true }) => document.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...mods }));

beforeEach(() => {
  try { localStorage.clear(); sessionStorage.clear(); history.replaceState(null, ""); } catch { /* none */ }
});

test("top bar: the script shows the buttons and puts the sidebar icon in the toggle", () => {
  document.body.innerHTML = BAR;
  expect(q(".zen-site-sidebar-toggle").hidden).toBe(true); // without the script
  start(document);
  for (const s of [".zen-site-sidebar-toggle", ".zen-site-back", ".zen-site-forward"]) expect(q(s).hidden, s).toBe(false);
  const svg = q(".zen-site-sidebar-toggle").querySelector("svg")!;
  expect(svg.getAttribute("viewBox")).toBe("0 0 24 24");
  expect(svg.getAttribute("width")).toBe("16");
  expect(svg.querySelector("[stroke-width='1.5']")).not.toBeNull();
  expect(q(".zen-site-back").textContent).toBe("‹");
});

test("top bar: the toggle hides and shows the sidebar, follows aria, and is remembered per site", () => {
  bar();
  const t = q(".zen-site-sidebar-toggle");
  expect(t.getAttribute("aria-expanded")).toBe("true");
  t.click();
  expect(t.getAttribute("aria-expanded")).toBe("false");
  expect(t.getAttribute("aria-label")).toBe("Hide sidebar".replace("Hide", "Show"));
  expect(q(".zen-site-frame").dataset.sidebar).toBe("closed");
  expect(q(".zen-site-side").hidden).toBe(true);
  expect(localStorage.getItem("zen.site.notes.sidebar")).toBe("closed");
  bar(); // the next page of the site starts the way it was left
  expect(q(".zen-site-sidebar-toggle").getAttribute("aria-expanded")).toBe("false");
  expect(q(".zen-site-side").hidden).toBe(true);
  q(".zen-site-sidebar-toggle").click();
  expect(q(".zen-site-sidebar-toggle").getAttribute("aria-label")).toBe("Hide sidebar");
  expect(q(".zen-site-side").hidden).toBe(false);
  expect(localStorage.getItem("zen.site.notes.sidebar")).toBe("open");
});

test("top bar: another site's choice is not read", () => {
  localStorage.setItem("zen.site.other.sidebar", "closed");
  bar();
  expect(q(".zen-site-side").hidden).toBe(false);
});

test("top bar: ⌘\\ toggles the sidebar, as in the app", () => {
  bar();
  key("\\");
  expect(q(".zen-site-sidebar-toggle").getAttribute("aria-expanded")).toBe("false");
  key("\\");
  expect(q(".zen-site-sidebar-toggle").getAttribute("aria-expanded")).toBe("true");
  key("\\", {}); // no modifier: a plain backslash is not a shortcut
  expect(q(".zen-site-sidebar-toggle").getAttribute("aria-expanded")).toBe("true");
});

test("top bar: with the script on, the sidebar details stay open and the Contents summary is not needed", () => {
  bar();
  expect((q(".zen-site-menu") as HTMLDetailsElement).open).toBe(true);
  expect(q(".zen-site-frame").hasAttribute("data-js")).toBe(true);
});

test("top bar: Back and Forward use history and are disabled when there is nowhere to go", () => {
  const back = vi.spyOn(history, "back").mockImplementation(() => {});
  const fwd = vi.spyOn(history, "forward").mockImplementation(() => {});
  bar(); // the first page of the visit
  expect((q(".zen-site-back") as HTMLButtonElement).disabled).toBe(true);
  expect((q(".zen-site-forward") as HTMLButtonElement).disabled).toBe(true);
  q(".zen-site-back").click();
  key("[");
  expect(back).not.toHaveBeenCalled();
  history.replaceState(null, ""); // a link to the next page: a new entry, with no state yet
  bar();
  expect((q(".zen-site-back") as HTMLButtonElement).disabled).toBe(false);
  expect((q(".zen-site-forward") as HTMLButtonElement).disabled).toBe(true);
  q(".zen-site-back").click();
  expect(back).toHaveBeenCalledTimes(1);
  key("[");
  expect(back).toHaveBeenCalledTimes(2);
  key("]");
  expect(fwd).not.toHaveBeenCalled();
  // Back was done: the first entry shows again (its state comes back with it), so Forward is open.
  history.replaceState({ zenIdx: 0 }, "");
  window.dispatchEvent(new PopStateEvent("popstate", { state: { zenIdx: 0 } }));
  expect((q(".zen-site-back") as HTMLButtonElement).disabled).toBe(true);
  expect((q(".zen-site-forward") as HTMLButtonElement).disabled).toBe(false);
  q(".zen-site-forward").click();
  key("]");
  expect(fwd).toHaveBeenCalledTimes(2);
});

test("top bar: tooltips name the shortcuts", () => {
  bar();
  expect(q(".zen-site-back").title).toBe("Back (⌘[)");
  expect(q(".zen-site-forward").title).toBe("Forward (⌘])");
  expect(q(".zen-site-sidebar-toggle").title).toBe("Hide sidebar (⌘\\)");
  q(".zen-site-sidebar-toggle").click();
  expect(q(".zen-site-sidebar-toggle").title).toBe("Show sidebar (⌘\\)");
});

test("top bar: a page without a sidebar has no toggle to wire and ⌘\\ does nothing", () => {
  document.body.innerHTML = BAR.replace(/<aside[\s\S]*?<\/aside>/, "").replace(/<button[^>]*zen-site-sidebar-toggle[\s\S]*?<\/button>/, "");
  start(document);
  expect(() => key("\\")).not.toThrow();
  expect(q(".zen-site-back").hidden).toBe(false);
});

test("top bar: works when localStorage is blocked", () => {
  vi.spyOn(window, "localStorage", "get").mockImplementation(() => { throw new Error("denied"); });
  bar();
  q(".zen-site-sidebar-toggle").click();
  expect(q(".zen-site-sidebar-toggle").getAttribute("aria-expanded")).toBe("false");
});

// ---- the top bar's CSS (no DOM) ----
const css = () => readFileSync(resolve(import.meta.dirname, "static.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
const rule = (raw: string, sel: string): string => {
  const src = raw.replace(/\/\*[\s\S]*?\*\//g, "");
  for (const m of Array.from(src.matchAll(/([^{}]+)\{([^{}]*)\}/g))) if (m[1].split(",").map((s) => s.trim()).includes(sel)) return m[2];
  return "";
};

test("top bar css: fixed in place, the app's height, tokens only", () => {
  const top = rule(css(), ".zen-site-top");
  expect(top).toMatch(/position:\s*(sticky|fixed)/);
  expect(top).toMatch(/top:\s*0/);
  const shared = readFileSync(resolve(import.meta.dirname, "../css/top-bar.css"), "utf8");
  expect(rule(shared, ".zen-top")).toMatch(/height:\s*44px/);
  expect(rule(shared, ".zen-top")).toMatch(/border-bottom:\s*1px solid var\(--rule\)/);
  expect(top).toMatch(/background:\s*var\(--bg\)/);
  expect(top + shared).not.toMatch(/#[0-9a-fA-F]{3,6}\b|rgb\(/);
});

test("top bar css: the app's top bar and icon styles come from one shared stylesheet", () => {
  const dir = resolve(import.meta.dirname, "..");
  expect(readFileSync(resolve(dir, "static/static.css"), "utf8")).toContain('@import "../css/top-bar.css"');
  const kit = readFileSync(resolve(dir, "../../kit/ui/src/styles/shell.css"), "utf8");
  expect(kit).toContain('@import "@zenmd/runtime/css/top-bar.css"');
  expect(kit).not.toMatch(/^\.zen-top\s*\{/m);
  expect(kit).not.toMatch(/^\.zen-icon\s*\{/m);
  const shared = readFileSync(resolve(dir, "css/top-bar.css"), "utf8");
  expect(rule(shared, ".zen-icon")).toMatch(/display:\s*block/);
  expect(rule(shared, ".zen-top-controls button")).toMatch(/min-width:\s*28px/);
});

test("top bar css: narrow windows put the sidebar over the page, the bar stays", () => {
  const narrow = css().slice(css().indexOf("@media (max-width: 719px)"));
  expect(narrow).toMatch(/\.zen-site-frame\[data-js\][^{]*\.zen-site-side\s*\{[^}]*position:\s*fixed/);
  expect(narrow).toMatch(/data-sidebar="open"/);
  expect(narrow).not.toMatch(/\.zen-site-top\s*\{[^}]*position:\s*static/);
});

test("top bar css: without scripts the buttons stay hidden and Contents stays", () => {
  const c = css();
  // the Contents summary is only dropped when the script has run (data-js)
  expect(c).toMatch(/\.zen-site-frame\[data-js\][^{]*\.zen-site-menu > summary\s*\{[^}]*display:\s*none/);
  expect(c).not.toMatch(/\.zen-site-sidebar-toggle\s*\{[^}]*display/); // [hidden] in the HTML does it
});

test("top bar css: left out when printing", () => {
  const print = readFileSync(resolve(import.meta.dirname, "../css/print.css"), "utf8");
  expect(print).toMatch(/\.zen-site-top[^{]*\{[^}]*display:\s*none/);
  expect(css()).toMatch(/@media print[^]*\.zen-site-top/);
});
