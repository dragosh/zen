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
