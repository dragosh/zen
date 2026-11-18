import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../../apps/zenmark/frontend/src/api/fake";
import { installRuntime, type PluginCtx } from "./plugins";

/** Plugin scripts by URL: what import() would hand back (happy-dom cannot fetch modules). */
const modules = new Map<string, unknown>();
const imported: string[] = [];
const importModule = async (url: string): Promise<unknown> => {
  imported.push(url);
  if (!modules.has(url)) throw new Error(`404 ${url}`);
  return modules.get(url);
};
const urlOf = (id: string) => `/_plugins/${id}/${id}.js?v=abc123abc123`;
const layoutUrlOf = (id: string) => `/_layouts/${id}/${id}.js?v=abc123abc123`;
let registered: string[] = [];
/** Publishes plugin `id`'s script as a module with this default export and names it in the page's assets. */
function register(id: string, handler: unknown): void {
  modules.set(urlOf(id), { default: handler });
  if (!registered.includes(urlOf(id))) registered.push(urlOf(id));
}
/** The same for a layout: its script lives under /_layouts/<id>/ and binds to the layout wrapper. */
function registerLayout(id: string, handler: unknown): void {
  modules.set(layoutUrlOf(id), { default: handler });
  if (!registered.includes(layoutUrlOf(id))) registered.push(layoutUrlOf(id));
}
const assets = () => ({ js: registered.slice(), css: [] as string[] });
/** What the reader does after every render. */
const mount = () => rt.mountAll(root, assets());

let api: FakeApi;
let rt: ReturnType<typeof installRuntime>;
let navigations: [string, string | undefined][];
let root: HTMLElement;
let n = 0;
const fresh = () => `p${++n}`;
let refreshes = 0;

beforeEach(() => {
  document.body.replaceChildren();
  api = createFakeApi();
  navigations = [];
  modules.clear();
  imported.length = 0;
  registered = [];
  refreshes = 0;
  rt = installRuntime({ api, navigate: (p, a) => void navigations.push([p, a]), page: () => ({ path: "guides/a.md", title: "A" }), importModule, refresh: () => { refreshes++; } });
  root = document.body.appendChild(document.createElement("article"));
});
afterEach(() => { rt.destroy(); vi.restoreAllMocks(); });

function wrapper(id: string, opts: { tag?: "div" | "span"; name?: string; attrs?: string; layout?: string; text?: string } = {}): HTMLElement {
  const w = document.createElement(opts.tag ?? "div");
  if (opts.layout) {
    w.setAttribute("data-zen-layout", opts.layout);
    w.setAttribute("data-zen-layout-source", "builtin");
  } else {
    w.setAttribute("data-zen-plugin", id);
  }
  if (opts.name) w.setAttribute("data-zen-name", opts.name);
  if (opts.attrs !== undefined) w.setAttribute("data-zen-attrs", opts.attrs);
  w.textContent = opts.text ?? "content";
  root.append(w);
  return w;
}
const settle = async () => { for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0)); };

test("window.zen is { version: 2 }, frozen, non-writable and non-configurable; there is no register", async () => {
  const z = (window as unknown as { zen: { version: number; register?: unknown } }).zen;
  expect(z.version).toBe(2);
  expect(z.register).toBeUndefined();
  expect(Object.isFrozen(z)).toBe(true);
  const d = Object.getOwnPropertyDescriptor(window, "zen")!;
  expect(d.writable).toBe(false);
  expect(d.configurable).toBe(false);
  expect(() => { (window as unknown as { zen: unknown }).zen = { version: 99 }; }).toThrow();
  expect(() => Object.defineProperty(window, "zen", { value: {} })).toThrow();
  installRuntime({ api, navigate: () => {}, page: () => ({ path: "a.md", title: "A" }), importModule }).destroy();
  expect((window as unknown as { zen: { version: number } }).zen.version).toBe(2);
});

test("mounts every wrapper of a registered plugin with its ctx", async () => {
  const id = fresh();
  const block = wrapper(id, { name: "quiz", attrs: "{\"id\":\"q1\",\"title\":\"Two words\"}" });
  const inline = wrapper(id, { tag: "span", name: "badge" });
  const seen: { el: HTMLElement; ctx: PluginCtx }[] = [];
  register(id, { mount(el, ctx) { seen.push({ el, ctx }); } });
  expect(seen).toHaveLength(0); // nothing rendered yet
  await mount();
  expect(seen.map((s) => s.el)).toEqual([block, inline]);
  expect(seen.map((s) => s.ctx.kind)).toEqual(["block", "inline"]);
  expect(seen.map((s) => s.ctx.name)).toEqual(["quiz", "badge"]);
  expect(seen[0].ctx.id).toBe(id);
  expect(seen[0].ctx.attrs).toEqual({ id: "q1", title: "Two words" });
  expect(seen[1].ctx.attrs).toEqual({});
  expect(seen[0].ctx.page).toEqual({ path: "guides/a.md", title: "A" });
  seen[0].ctx.navigate("b.md", "part");
  seen[0].ctx.navigate("c.md");
  expect(navigations).toEqual([["b.md", "part"], ["c.md", undefined]]);
});

test("attrs are {} for invalid JSON or anything that is not an object of strings", async () => {
  const id = fresh();
  wrapper(id, { attrs: "{not json" });
  wrapper(id, { attrs: "[1,2]" });
  wrapper(id, { attrs: "\"text\"" });
  wrapper(id, { attrs: "{\"a\":\"x\",\"n\":3,\"o\":{}}" });
  const attrs: unknown[] = [];
  register(id, { mount(_el, ctx) { attrs.push(ctx.attrs); } });
  await mount();
  expect(attrs).toEqual([{}, {}, {}, { a: "x" }]);
});

test("wrappers nobody registered for (built-ins, unknown ids) are left alone", async () => {
  const id = fresh();
  const builtin = wrapper("highlight", { text: "code" });
  const mine = wrapper(id);
  register(id, { mount(el) { el.textContent = "mounted"; } });
  await mount();
  expect(builtin.textContent).toBe("code");
  expect(builtin.querySelector(".zen-plugin-error")).toBeNull();
  expect(mine.textContent).toBe("mounted");
});

test("scripts are imported with their versioned URL and bound to the id in that URL", async () => {
  const a = fresh();
  const b = fresh();
  const wa = wrapper(a);
  const wb = wrapper(b);
  register(a, { mount(el) { el.textContent = "A"; } });
  register(b, { mount(el) { el.textContent = "B"; } });
  await mount();
  expect(imported).toEqual([urlOf(a), urlOf(b)]);
  expect(imported[0]).toContain("?v=");
  expect(wa.textContent).toBe("A");
  expect(wb.textContent).toBe("B");
});

test("a script cannot take over another plugin's blocks: handlers belong to the id of their own URL", async () => {
  const mine = fresh();
  const other = fresh();
  const wo = wrapper(other, { text: "other's" });
  wrapper(mine);
  const seen: string[] = [];
  register(mine, { mount(_el, ctx) { seen.push(ctx.id); } });
  // the other plugin has no script on this page
  await mount();
  expect(seen).toEqual([mine]);
  expect(wo.textContent).toBe("other's");
  expect(wo.querySelector(".zen-plugin-error")).toBeNull();
});

test("a script that is imported once keeps its module for later renders; mounting needs no new import", async () => {
  const id = fresh();
  const log: string[] = [];
  register(id, { mount() { log.push("mount"); return () => log.push("clean"); } });
  wrapper(id);
  await mount();
  root.replaceChildren();
  wrapper(id);
  await mount();
  expect(imported).toEqual([urlOf(id)]);
  expect(log).toEqual(["mount", "clean", "mount"]);
});

test("a module without a default mount shows an error naming the plugin; others mount", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const bad = fresh();
  const bad2 = fresh();
  const good = fresh();
  const w1 = wrapper(bad);
  const w2 = wrapper(bad2);
  const w3 = wrapper(good);
  modules.set(urlOf(bad), { mount() {} }); // named export, no default
  registered.push(urlOf(bad));
  register(bad2, { notMount: 1 });
  register(good, { mount(el) { el.textContent = "fine"; } });
  await mount();
  expect(w1.querySelector(".zen-plugin-error")!.textContent).toContain(bad);
  expect(w2.querySelector(".zen-plugin-error")!.textContent).toContain(bad2);
  expect(w3.textContent).toBe("fine");
  expect(w3.querySelector(".zen-plugin-error")).toBeNull();
});

test("a script that fails to load shows an error in its wrappers only", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const id = fresh();
  const w = wrapper(id);
  registered.push(urlOf(id)); // no module behind the URL: import() rejects
  await mount();
  expect(w.querySelector(".zen-plugin-error")!.textContent).toContain(id);
  expect(w.querySelector(".zen-plugin-error")!.textContent).toContain("404");
});

test("a script with no wrapper on the page (diagram-like, no default) is quiet", async () => {
  const err = vi.spyOn(console, "error").mockImplementation(() => {});
  modules.set(urlOf("quiet"), { other: 1 });
  registered.push(urlOf("quiet"));
  await mount();
  expect(err).not.toHaveBeenCalled();
  expect(root.querySelector(".zen-plugin-error")).toBeNull();
});

test("only /_plugins/<id>/ script URLs are imported", async () => {
  await rt.load({ js: ["https://evil.test/x.js", "/_ws/x.js", "/_plugins/../x.js", "//evil.test/_plugins/a/a.js"] });
  expect(imported).toEqual([]);
});

test("plugins:changed unmounts everything, forgets the modules, removes plugin stylesheets and asks the host to refresh", async () => {
  const id = fresh();
  const w = wrapper(id);
  const log: string[] = [];
  register(id, { mount() { log.push("mount"); return () => log.push("clean"); } });
  const mk = (href: string) => {
    const l = document.createElement("link");
    l.rel = "stylesheet";
    l.href = href;
    l.dataset.zenAsset = href;
    document.head.append(l);
  };
  mk(`/_plugins/${id}/a.css?v=1`);
  const other = document.createElement("link");
  other.dataset.zenAsset = "https://x.test/a.css";
  document.head.append(other);
  await mount();
  expect(log).toEqual(["mount"]);

  api.emit("plugins:changed", undefined);
  expect(log).toEqual(["mount", "clean"]);
  expect(document.head.querySelector("link[data-zen-asset^='/_plugins/']")).toBeNull();
  expect(document.head.contains(other)).toBe(true);
  expect(refreshes).toBe(1);

  // the host re-renders with new URLs: the module is imported again
  modules.set(`/_plugins/${id}/${id}.js?v=newhash00000`, { default: { mount(el: HTMLElement) { el.textContent = "v2"; } } });
  await rt.mountAll(root, { js: [`/_plugins/${id}/${id}.js?v=newhash00000`], css: [] });
  expect(imported).toEqual([urlOf(id), `/_plugins/${id}/${id}.js?v=newhash00000`]);
  expect(w.textContent).toBe("v2");
  other.remove();
});

test("workspace:opened and workspace:changed reset the same way; destroy stops listening", async () => {
  const id = fresh();
  wrapper(id);
  const log: string[] = [];
  register(id, { mount() { log.push("mount"); return () => log.push("clean"); } });
  await mount();
  api.emit("workspace:opened", { workspace: { root: "/x", name: "x", nav: [], tags: [] } as never, page: "", mode: "read" });
  expect(log).toEqual(["mount", "clean"]);
  expect(refreshes).toBe(1);
  await mount();
  api.emit("workspace:changed", { root: "/x", name: "x", nav: [], tags: [] } as never);
  expect(log).toEqual(["mount", "clean", "mount", "clean"]);
  expect(refreshes).toBe(2);
  rt.destroy();
  api.emit("plugins:changed", undefined);
  expect(refreshes).toBe(2);
});

test("a late import of an older generation does not mount into a reset runtime", async () => {
  const id = fresh();
  const w = wrapper(id);
  let release!: () => void;
  const slow = new Promise<void>((r) => { release = r; });
  const mod = { default: { mount(el: HTMLElement) { el.textContent = "mounted"; } } };
  rt.destroy();
  rt = installRuntime({ api, navigate: () => {}, page: () => ({ path: "a.md", title: "A" }), importModule: async () => { await slow; return mod; } });
  const p = rt.mountAll(root, { js: [urlOf(id)] });
  rt.reset();
  release();
  await p;
  await settle();
  expect(w.textContent).toBe("content");
});

test("unmountAll runs every clean-up once, last mounted first, sync and async alike", async () => {
  const a = fresh();
  const b = fresh();
  wrapper(a, { name: "one" });
  wrapper(b, { name: "two" });
  wrapper(a, { name: "three" });
  const log: string[] = [];
  register(a, { mount(_el, ctx) { return () => log.push(`clean ${ctx.name}`); } });
  register(b, { async mount(_el, ctx) { await Promise.resolve(); return () => log.push(`clean ${ctx.name}`); } });
  await mount();
  await settle();
  rt.unmountAll();
  expect(log).toEqual(["clean three", "clean two", "clean one"]);
  rt.unmountAll();
  expect(log).toHaveLength(3);
});

test("an async mount that finishes after the unmount is cleaned up at once", async () => {
  const id = fresh();
  wrapper(id);
  let finish!: (clean: () => void) => void;
  const cleaned = vi.fn();
  register(id, { mount: () => new Promise<() => void>((r) => { finish = r; }) });
  await mount();
  rt.unmountAll();
  expect(cleaned).not.toHaveBeenCalled();
  finish(cleaned);
  await settle();
  expect(cleaned).toHaveBeenCalledTimes(1);
});

test("mountAll on a new render cleans up what was mounted before", async () => {
  const id = fresh();
  wrapper(id);
  const log: string[] = [];
  register(id, { mount() { log.push("mount"); return () => log.push("clean"); } });
  await mount();
  root.replaceChildren();
  wrapper(id);
  await mount();
  expect(log).toEqual(["mount", "clean", "mount"]);
});

test("a mount that throws or rejects shows an error in its own wrapper only", async () => {
  const err = vi.spyOn(console, "error").mockImplementation(() => {});
  const bad = fresh();
  const slow = fresh();
  const good = fresh();
  const w1 = wrapper(bad);
  const w2 = wrapper(slow);
  const w3 = wrapper(good);
  register(bad, { mount() { throw new Error("<b>boom</b>"); } });
  register(slow, { async mount() { throw new Error("later"); } });
  register(good, { mount(el) { el.textContent = "fine"; } });
  await mount();
  await settle();
  const box = w1.querySelector(".zen-plugin-error")!;
  expect(box.textContent).toContain(bad);
  expect(box.textContent).toContain("<b>boom</b>");
  expect(box.querySelector("b")).toBeNull();
  expect(w2.querySelector(".zen-plugin-error")!.textContent).toContain("later");
  expect(w3.textContent).toBe("fine");
  expect(root.querySelectorAll(".zen-plugin-error")).toHaveLength(2);
  expect(err).toHaveBeenCalledTimes(2);
});

test("a clean-up that throws does not stop the others", async () => {
  const err = vi.spyOn(console, "error").mockImplementation(() => {});
  const a = fresh();
  const b = fresh();
  const w1 = wrapper(a);
  wrapper(b);
  const cleaned = vi.fn();
  register(a, { mount: () => cleaned });
  register(b, { mount: () => () => { throw new Error("stuck"); } });
  await mount();
  expect(() => rt.unmountAll()).not.toThrow();
  expect(cleaned).toHaveBeenCalledTimes(1);
  expect(err).toHaveBeenCalledTimes(1);
  expect(root.querySelectorAll(".zen-plugin-error")).toHaveLength(1);
  expect(w1.querySelector(".zen-plugin-error")).toBeNull();
});

test("storage round trips JSON values under the plugin's own id; missing keys are null", async () => {
  const a = fresh();
  const b = fresh();
  wrapper(a);
  wrapper(b);
  const ctxs: Record<string, PluginCtx> = {};
  for (const id of [a, b]) register(id, { mount(_el, ctx) { ctxs[id] = ctx; } });
  await mount();

  expect(await ctxs[a].storage.get("count")).toBeNull();
  await ctxs[a].storage.set("count", 3);
  await ctxs[a].storage.set("state", { done: ["x"], ok: true });
  expect(await ctxs[a].storage.get("count")).toBe(3);
  expect(await ctxs[a].storage.get("state")).toEqual({ done: ["x"], ok: true });
  // another plugin does not see it, and no call ever names another id
  expect(await ctxs[b].storage.get("count")).toBeNull();
  expect(api.callsTo("pluginStorageSet")).toEqual([[a, "count", "3"], [a, "state", "{\"done\":[\"x\"],\"ok\":true}"]]);
  expect(api.callsTo("pluginStorageGet").map((c) => c[0])).toEqual([a, a, a, b]);
  // the functions take a key and a value only: an extra "id" argument changes nothing
  await (ctxs[b].storage.set as (...x: unknown[]) => Promise<void>)("k", 1, a);
  expect(api.callsTo("pluginStorageSet").at(-1)).toEqual([b, "k", "1"]);
});

test("storage.set rejects values that are not JSON; API failures reject", async () => {
  const id = fresh();
  wrapper(id);
  let ctx!: PluginCtx;
  register(id, { mount(_el, c) { ctx = c; } });
  await mount();
  const loop: Record<string, unknown> = {};
  loop.self = loop;
  await expect(ctx.storage.set("k", loop)).rejects.toThrow();
  await expect(ctx.storage.set("k", undefined)).rejects.toThrow("JSON");
  await expect(ctx.storage.set("k", () => 1)).rejects.toThrow("JSON");
  expect(api.callsTo("pluginStorageSet")).toEqual([]);
  api.failNext("pluginStorageGet", new Error("storage is full"));
  await expect(ctx.storage.get("k")).rejects.toThrow("storage is full");
});

test("invoke sends the payload as JSON for the plugin's own id and resolves the parsed answer", async () => {
  const id = fresh();
  wrapper(id);
  let ctx!: PluginCtx;
  register(id, { mount(_el, c) { ctx = c; } });
  await mount();
  api.invokeResult = (pid: string, payload: unknown) => ({ pid, payload });
  expect(await ctx.invoke({ answer: 2 })).toEqual({ pid: id, payload: { answer: 2 } });
  expect(await ctx.invoke(undefined)).toEqual({ pid: id, payload: null });
  // the page the block is on goes along (core/plugin-hardening M4)
  expect(api.callsTo("pluginInvoke")).toEqual([[id, "guides/a.md", "{\"answer\":2}"], [id, "guides/a.md", "null"]]);
  api.failNext("pluginInvoke", new Error("no action hook"));
  await expect(ctx.invoke(1)).rejects.toThrow("no action hook");
});

test("destroy cleans up and forgets its modules; a new runtime imports again", async () => {
  const id = fresh();
  const w = wrapper(id);
  const cleaned = vi.fn();
  register(id, { mount(el) { el.textContent = "on"; return cleaned; } });
  await mount();
  rt.destroy();
  expect(cleaned).toHaveBeenCalledTimes(1);
  w.textContent = "content";
  rt = installRuntime({ api, navigate: () => {}, page: () => ({ path: "x.md", title: "X" }), importModule });
  await mount();
  expect(w.textContent).toBe("on");
  expect(imported).toEqual([urlOf(id), urlOf(id)]);
});

// ---- core/plugin-extras: stream, storage.keys, anchor, badge ----

type Extras = { anchor?: () => string; onAnchor?: (fn: (a: string) => void) => () => void; badge?: (path: string, text: string | null) => void };
/** Replaces the runtime of beforeEach with one that has the extras wired. */
function withExtras(extras: Extras = {}) {
  rt.destroy();
  rt = installRuntime({ api, navigate: () => {}, page: () => ({ path: "guides/a.md", title: "A" }), importModule, ...extras });
}
/** Mounts one block of a fresh plugin and returns its ctx. */
async function ctxOf(tag: string = "one"): Promise<PluginCtx> {
  const id = fresh();
  wrapper(id, { name: tag });
  let got!: PluginCtx;
  register(id, { mount(_el, ctx) { got = ctx; } });
  await mount();
  return got;
}
const chunk = (call: string, text: string) => api.emit("plugin:stream", { call, chunk: text, done: false, error: "" });
const end = (call: string, error = "") => api.emit("plugin:stream", { call, chunk: "", done: true, error });

test("storage.keys lists the plugin's keys, optionally by prefix", async () => {
  const id = fresh();
  wrapper(id);
  let ctx!: PluginCtx;
  register(id, { mount(_el, c) { ctx = c; } });
  await mount();
  await ctx.storage.set("quiz.b", 1);
  await ctx.storage.set("quiz.a", 2);
  await ctx.storage.set("other", 3);
  expect(await ctx.storage.keys("quiz.")).toEqual(["quiz.a", "quiz.b"]);
  expect(await ctx.storage.keys()).toEqual(["other", "quiz.a", "quiz.b"]);
  expect(api.callsTo("pluginStorageKeys")).toEqual([[id, "quiz."], [id, ""]]);
});

test("stream delivers chunks in order and done resolves when Go says so", async () => {
  const ctx = await ctxOf();
  const got: string[] = [];
  const s = ctx.stream({ q: 1 }, (c) => got.push(c));
  await settle();
  expect(api.callsTo("pluginStream")).toEqual([[ctx.id, "guides/a.md", "{\"q\":1}"]]);
  chunk("call-1", "a");
  chunk("call-1", "b");
  let resolved = false;
  void s.done.then(() => { resolved = true; });
  await settle();
  expect(resolved).toBe(false);
  end("call-1");
  await s.done;
  expect(got).toEqual(["a", "b"]);
  chunk("call-1", "after the end");
  expect(got).toEqual(["a", "b"]);
});

test("a batched chunk (lines joined by Go) reaches the plugin one line at a time, empty lines kept", async () => {
  const ctx = await ctxOf();
  const got: string[] = [];
  const s = ctx.stream(null, (c) => got.push(c));
  await settle();
  chunk("call-1", '{"s":"out","t":"one"}\n\n{"s":"out","t":"three"}');
  end("call-1");
  await s.done;
  expect(got).toEqual(['{"s":"out","t":"one"}', "", '{"s":"out","t":"three"}']);
});

test("chunks that arrive before the call id is known are not lost", async () => {
  const ctx = await ctxOf();
  const got: string[] = [];
  const s = ctx.stream(null, (c) => got.push(c));
  chunk("call-1", "early");   // pluginStream() has not resolved yet
  end("call-1");
  await s.done;
  expect(got).toEqual(["early"]);
});

test("done rejects with the stream's error text", async () => {
  const ctx = await ctxOf();
  const s = ctx.stream("x", () => {});
  await settle();
  end("call-1", "exit code 3: boom");
  await expect(s.done).rejects.toThrow("exit code 3: boom");
  // a start that fails rejects too
  api.failNext("pluginStream", new Error("too many streams"));
  await expect(ctx.stream("y", () => {}).done).rejects.toThrow("too many streams");
});

test("a ctx only sees the chunks of its own calls", async () => {
  const got: PluginCtx[] = [];
  for (const name of ["a", "b"]) {
    const id = fresh();
    wrapper(id, { name });
    register(id, { mount(_el, c) { got.push(c); } });
  }
  await mount();
  const [a, b] = got;
  const ga: string[] = [];
  const gb: string[] = [];
  const sa = a.stream(1, (c) => ga.push(c));
  const sb = b.stream(2, (c) => gb.push(c));
  await settle();
  chunk("call-2", "for b");
  chunk("call-1", "for a");
  chunk("call-99", "somebody else's");
  end("call-1");
  end("call-2");
  await Promise.all([sa.done, sb.done]);
  expect(ga).toEqual(["for a"]);
  expect(gb).toEqual(["for b"]);
});

test("cancel tells Go, stops delivery and rejects done with cancelled", async () => {
  const ctx = await ctxOf();
  const got: string[] = [];
  const s = ctx.stream(1, (c) => got.push(c));
  await settle();
  chunk("call-1", "one");
  s.cancel();
  expect(api.callsTo("pluginCancel")).toEqual([["call-1"]]);
  chunk("call-1", "two");
  await expect(s.done).rejects.toThrow("cancelled");
  expect(got).toEqual(["one"]);
  s.cancel();
  expect(api.callsTo("pluginCancel")).toHaveLength(1);
});

test("cancel before the call id is known cancels as soon as it is", async () => {
  const ctx = await ctxOf();
  const s = ctx.stream(1, () => {});
  s.cancel();
  await expect(s.done).rejects.toThrow("cancelled");
  await settle();
  expect(api.callsTo("pluginCancel")).toEqual([["call-1"]]);
});

test("streams started by a block are cancelled when it unmounts", async () => {
  const id = fresh();
  wrapper(id);
  let ctx!: PluginCtx;
  register(id, { mount(_el, c) { ctx = c; } });
  await mount();
  const s1 = ctx.stream(1, () => {});
  const s2 = ctx.stream(2, () => {});
  s1.done.catch(() => {});
  s2.done.catch(() => {});
  await settle();
  end("call-2");
  await s2.done;
  rt.unmountAll();
  expect(api.callsTo("pluginCancel")).toEqual([["call-1"]]); // the finished one is not cancelled again
  await expect(s1.done).rejects.toThrow("cancelled");
  // nothing is left listening after the page is gone
  expect(() => chunk("call-1", "late")).not.toThrow();
});

test("an onChunk that throws does not break the stream", async () => {
  const ctx = await ctxOf();
  vi.spyOn(console, "error").mockImplementation(() => {});
  const got: string[] = [];
  const s = ctx.stream(1, (c) => { got.push(c); if (c === "a") throw new Error("handler bug"); });
  await settle();
  chunk("call-1", "a");
  chunk("call-1", "b");
  end("call-1");
  await s.done;
  expect(got).toEqual(["a", "b"]);
});

test("anchor() and onAnchor(): current anchor, changes, removal on unmount", async () => {
  let current = "intro";
  const subs = new Set<(a: string) => void>();
  withExtras({ anchor: () => current, onAnchor: (fn) => { subs.add(fn); return () => void subs.delete(fn); } });
  const ctx = await ctxOf();
  expect(ctx.anchor()).toBe("intro");
  current = "next";
  expect(ctx.anchor()).toBe("next");
  const seen: string[] = [];
  const off = ctx.onAnchor((a) => seen.push(a));
  const off2 = ctx.onAnchor(() => {});
  expect(subs.size).toBe(2);
  subs.forEach((fn) => fn("part-2"));
  expect(seen).toEqual(["part-2"]);
  off2();
  expect(subs.size).toBe(1);
  expect(typeof off).toBe("function");
  rt.unmountAll();
  expect(subs.size).toBe(0);
});

test("anchor() is empty and onAnchor harmless when the host does not provide them", async () => {
  const ctx = await ctxOf();
  expect(ctx.anchor()).toBe("");
  expect(() => ctx.onAnchor(() => {})()).not.toThrow();
  expect(() => ctx.badge("a.md", "x")).not.toThrow();
});

test("assetUrl(file) names another file of the plugin, inside its folder", async () => {
  const ctx = await ctxOf();
  expect(ctx.assetUrl("mermaid.min.js")).toBe(`/_plugins/${ctx.id}/mermaid.min.js`);
  expect(ctx.assetUrl("../../x/a b.js")).toBe(`/_plugins/${ctx.id}/x/a%20b.js`);
});

test("badge(path, text) sets and clears a label, at most 12 characters", async () => {
  const calls: [string, string | null][] = [];
  withExtras({ badge: (p, t) => void calls.push([p, t]) });
  const ctx = await ctxOf();
  ctx.badge("guides/a.md", "Done");
  ctx.badge("guides/a.md", "a very long label indeed");
  ctx.badge("guides/a.md", null);
  ctx.badge("guides/a.md", "");
  ctx.badge("guides/a.md", "  ");
  expect(calls).toEqual([["guides/a.md", "Done"], ["guides/a.md", "a very long "], ["guides/a.md", null], ["guides/a.md", null], ["guides/a.md", null]]);
  // a badge outlives the block that set it
  rt.unmountAll();
  expect(calls).toHaveLength(5);
});

test("badge ignores a path that is not a string", async () => {
  const calls: unknown[] = [];
  withExtras({ badge: (p, t) => void calls.push([p, t]) });
  const ctx = await ctxOf();
  (ctx.badge as (p: unknown, t: unknown) => void)(42, "x");
  (ctx.badge as (p: unknown, t: unknown) => void)("", "x");
  expect(calls).toEqual([]);
});

// ---- layouts: bound to the layout's own module, no backend

test("a layout wrapper mounts the layout's module from /_layouts/<id>/ with a layout ctx", async () => {
  const seen: { el: HTMLElement; ctx: Record<string, unknown> }[] = [];
  registerLayout("post", { mount(el: HTMLElement, ctx: Record<string, unknown>) { seen.push({ el, ctx }); } });
  const w = wrapper("post", { layout: "post", attrs: "{\"cover\":\"true\"}" });
  await mount();
  expect(seen.map((s) => s.el)).toEqual([w]);
  const ctx = seen[0].ctx as unknown as PluginCtx;
  expect(ctx.kind).toBe("layout");
  expect(ctx.id).toBe("post");
  expect(ctx.name).toBe("post");
  expect(ctx.attrs).toEqual({ cover: "true" });
  expect(ctx.page).toEqual({ path: "guides/a.md", title: "A" });
  expect(Object.isFrozen(ctx)).toBe(true);
});

test("a layout ctx has no invoke and no stream", async () => {
  const keys: string[][] = [];
  registerLayout("post", { mount(_el: HTMLElement, ctx: object) { keys.push(Object.keys(ctx).sort()); } });
  wrapper("post", { layout: "post" });
  await mount();
  expect(keys[0]).toEqual(["anchor", "assetUrl", "attrs", "badge", "id", "kind", "name", "navigate", "onAnchor", "page", "storage"]);
  expect(keys[0]).not.toContain("invoke");
  expect(keys[0]).not.toContain("stream");
  expect(api.callsTo("pluginInvoke")).toEqual([]);
  expect(api.callsTo("pluginStream")).toEqual([]);
});

test("a layout ctx's storage goes to the layout services, not the plugin ones", async () => {
  let ctx!: PluginCtx;
  registerLayout("post", { mount(_el: HTMLElement, c: PluginCtx) { ctx = c; } });
  wrapper("post", { layout: "post" });
  await mount();
  await ctx.storage.set("seen", { n: 2 });
  expect(await ctx.storage.get("seen")).toEqual({ n: 2 });
  expect(await ctx.storage.get("absent")).toBeNull();
  await ctx.storage.set("seen-b", 1);
  expect(await ctx.storage.keys("seen")).toEqual(["seen", "seen-b"]);
  expect(await ctx.storage.keys()).toEqual(["seen", "seen-b"]);
  expect(api.callsTo("layoutStorageSet")).toEqual([["post", "seen", "{\"n\":2}"], ["post", "seen-b", "1"]]);
  expect(api.callsTo("layoutStorageGet")[0]).toEqual(["post", "seen"]);
  expect(api.callsTo("layoutStorageKeys")).toEqual([["post", "seen"], ["post", ""]]);
  expect(api.callsTo("pluginStorageSet")).toEqual([]);
  expect(api.callsTo("pluginStorageGet")).toEqual([]);
  expect(api.callsTo("pluginStorageKeys")).toEqual([]);
  await expect(ctx.storage.set("x", undefined)).rejects.toThrow("JSON");
});

test("a plugin and a layout with the same id keep separate modules and storage", async () => {
  const log: string[] = [];
  register("same", { mount(_el: HTMLElement, c: PluginCtx) { log.push(`plugin:${c.kind}`); return c.storage.set("k", 1); } });
  registerLayout("same", { mount(_el: HTMLElement, c: PluginCtx) { log.push(`layout:${c.kind}`); return c.storage.set("k", 2); } });
  wrapper("same");
  wrapper("same", { layout: "same" });
  await mount();
  await settle();
  expect(log).toEqual(["plugin:block", "layout:layout"]);
  expect(api.callsTo("pluginStorageSet")).toEqual([["same", "k", "1"]]);
  expect(api.callsTo("layoutStorageSet")).toEqual([["same", "k", "2"]]);
});

test("a layout wrapper is not mounted by a plugin script of the same id, and a layout script does not mount plugin blocks", async () => {
  const log: string[] = [];
  register("only", { mount() { log.push("plugin"); } });
  wrapper("only", { layout: "only" }); // no layout script registered
  await mount();
  expect(log).toEqual([]);
  rt.reset();
  root.replaceChildren();
  registered = [];
  modules.clear();
  registerLayout("only", { mount() { log.push("layout"); } });
  wrapper("only"); // a plugin block
  await mount();
  expect(log).toEqual([]);
});

test("layouts and blocks mount in document order and clean up in reverse", async () => {
  const log: string[] = [];
  register("quiz", { mount() { log.push("mount block"); return () => log.push("clean block"); } });
  registerLayout("post", { mount() { log.push("mount layout"); return () => log.push("clean layout"); } });
  const layout = wrapper("post", { layout: "post" });
  const block = document.createElement("div");
  block.setAttribute("data-zen-plugin", "quiz");
  layout.append(block);
  await mount();
  expect(log).toEqual(["mount layout", "mount block"]);
  rt.unmountAll();
  expect(log).toEqual(["mount layout", "mount block", "clean block", "clean layout"]);
});

test("a layout whose script fails to load or has no mount shows an error in its wrapper", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const w = wrapper("gone", { layout: "gone" });
  registered.push(layoutUrlOf("gone")); // named in the assets, 404 on import
  const w2 = wrapper("empty", { layout: "empty" });
  registerLayout("empty", {});
  await mount();
  expect(w.querySelector(".zen-plugin-error")!.textContent).toContain("could not be loaded");
  expect(w2.querySelector(".zen-plugin-error")!.textContent).toContain("no default export");
});

test("a layout script URL is bound to the layout id it names; `..` URLs are ignored", async () => {
  registerLayout("post", { mount() {} });
  const evil = "/_layouts/post/../x/x.js?v=1";
  modules.set(evil, { default: { mount() { throw new Error("must not load"); } } });
  await rt.mountAll(root, { js: [layoutUrlOf("post"), evil, "/_other/post/a.js"], css: [] });
  expect(imported).toEqual([layoutUrlOf("post")]);
});

test("plugins:changed drops layout modules too and asks the host to render again", async () => {
  const log: string[] = [];
  registerLayout("post", { mount() { log.push("mount"); return () => log.push("clean"); } });
  wrapper("post", { layout: "post" });
  await mount();
  api.emit("layouts:changed", undefined);
  expect(log).toEqual(["mount", "clean"]);
  expect(refreshes).toBe(1);
  await mount();
  expect(imported.filter((u) => u === layoutUrlOf("post"))).toHaveLength(2);
});

test("layouts:changed removes layout stylesheets and keeps none of the old modules", async () => {
  for (const href of ["/_layouts/post/post.css?v=1", "/_plugins/p/p.css?v=1"]) {
    const l = document.createElement("link");
    l.dataset.zenAsset = href;
    document.head.append(l);
  }
  api.emit("layouts:changed", undefined);
  expect(Array.from(document.head.querySelectorAll("link[data-zen-asset]")).map((l) => (l as HTMLElement).dataset.zenAsset)).toEqual([]);
  document.head.replaceChildren();
});
