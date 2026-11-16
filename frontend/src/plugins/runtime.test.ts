// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import { installRuntime, type PluginCtx } from "./runtime";

type Zen = { version: number; register(id: string, h: { mount(el: HTMLElement, ctx: PluginCtx): unknown }): void };
const zen = () => (window as unknown as { zen: Zen }).zen;

let api: FakeApi;
let rt: ReturnType<typeof installRuntime>;
let navigations: [string, string | undefined][];
let root: HTMLElement;
let n = 0;
/** A fresh plugin id per test: handlers outlive a runtime (scripts load once). */
const fresh = () => `p${++n}`;

beforeEach(() => {
  document.body.replaceChildren();
  api = createFakeApi();
  navigations = [];
  rt = installRuntime({ api, navigate: (p, a) => void navigations.push([p, a]), page: () => ({ path: "guides/a.md", title: "A" }) });
  root = document.body.appendChild(document.createElement("article"));
});
afterEach(() => { rt.destroy(); vi.restoreAllMocks(); });

function wrapper(id: string, opts: { tag?: "div" | "span"; name?: string; attrs?: string; layout?: string; text?: string } = {}): HTMLElement {
  const w = document.createElement(opts.tag ?? "div");
  w.setAttribute("data-zen-plugin", id);
  if (opts.name) w.setAttribute("data-zen-name", opts.name);
  if (opts.attrs !== undefined) w.setAttribute("data-zen-attrs", opts.attrs);
  if (opts.layout) w.setAttribute("data-zen-layout", opts.layout);
  w.textContent = opts.text ?? "content";
  root.append(w);
  return w;
}
const settle = async () => { for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0)); };

test("window.zen exists with version 1 as soon as the runtime is installed", () => {
  expect(zen().version).toBe(1);
  expect(typeof zen().register).toBe("function");
});

test("mounts every wrapper of a registered plugin with its ctx", () => {
  const id = fresh();
  const block = wrapper(id, { name: "quiz", attrs: "{\"id\":\"q1\",\"title\":\"Two words\"}" });
  const inline = wrapper(id, { tag: "span", name: "badge" });
  const layout = wrapper(id, { layout: "slides", attrs: "{\"transition\":\"fade\"}" });
  const seen: { el: HTMLElement; ctx: PluginCtx }[] = [];
  zen().register(id, { mount(el, ctx) { seen.push({ el, ctx }); } });
  expect(seen).toHaveLength(0); // nothing rendered yet
  rt.mountAll(root);
  expect(seen.map((s) => s.el)).toEqual([block, inline, layout]);
  expect(seen.map((s) => s.ctx.kind)).toEqual(["block", "inline", "layout"]);
  expect(seen.map((s) => s.ctx.name)).toEqual(["quiz", "badge", "slides"]);
  expect(seen[0].ctx.id).toBe(id);
  expect(seen[0].ctx.attrs).toEqual({ id: "q1", title: "Two words" });
  expect(seen[1].ctx.attrs).toEqual({});
  expect(seen[2].ctx.attrs).toEqual({ transition: "fade" });
  expect(seen[0].ctx.page).toEqual({ path: "guides/a.md", title: "A" });
  seen[0].ctx.navigate("b.md", "part");
  seen[0].ctx.navigate("c.md");
  expect(navigations).toEqual([["b.md", "part"], ["c.md", undefined]]);
});

test("attrs are {} for invalid JSON or anything that is not an object of strings", () => {
  const id = fresh();
  wrapper(id, { attrs: "{not json" });
  wrapper(id, { attrs: "[1,2]" });
  wrapper(id, { attrs: "\"text\"" });
  wrapper(id, { attrs: "{\"a\":\"x\",\"n\":3,\"o\":{}}" });
  const attrs: unknown[] = [];
  zen().register(id, { mount(_el, ctx) { attrs.push(ctx.attrs); } });
  rt.mountAll(root);
  expect(attrs).toEqual([{}, {}, {}, { a: "x" }]);
});

test("wrappers nobody registered for (built-ins, unknown ids) are left alone", () => {
  const id = fresh();
  const builtin = wrapper("highlight", { text: "code" });
  const mine = wrapper(id);
  zen().register(id, { mount(el) { el.textContent = "mounted"; } });
  rt.mountAll(root);
  expect(builtin.textContent).toBe("code");
  expect(builtin.querySelector(".zen-plugin-error")).toBeNull();
  expect(mine.textContent).toBe("mounted");
});

test("a handler registering late mounts the wrappers already on the page", () => {
  const id = fresh();
  const w = wrapper(id);
  rt.mountAll(root);
  expect(w.textContent).toBe("content");
  zen().register(id, { mount(el) { el.textContent = "late"; } });
  expect(w.textContent).toBe("late");
});

test("registering twice replaces the handler and remounts after the old clean-up", () => {
  const id = fresh();
  const w = wrapper(id);
  const log: string[] = [];
  zen().register(id, { mount() { log.push("mount 1"); return () => log.push("clean 1"); } });
  rt.mountAll(root);
  zen().register(id, { mount(el) { log.push("mount 2"); el.textContent = "second"; } });
  expect(log).toEqual(["mount 1", "clean 1", "mount 2"]);
  expect(w.textContent).toBe("second");
  rt.unmountAll();
  rt.mountAll(root);
  expect(log).toEqual(["mount 1", "clean 1", "mount 2", "mount 2"]);
});

test("unmountAll runs every clean-up once, last mounted first, sync and async alike", async () => {
  const a = fresh();
  const b = fresh();
  wrapper(a, { name: "one" });
  wrapper(b, { name: "two" });
  wrapper(a, { name: "three" });
  const log: string[] = [];
  zen().register(a, { mount(_el, ctx) { return () => log.push(`clean ${ctx.name}`); } });
  zen().register(b, { async mount(_el, ctx) { await Promise.resolve(); return () => log.push(`clean ${ctx.name}`); } });
  rt.mountAll(root);
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
  zen().register(id, { mount: () => new Promise<() => void>((r) => { finish = r; }) });
  rt.mountAll(root);
  rt.unmountAll();
  expect(cleaned).not.toHaveBeenCalled();
  finish(cleaned);
  await settle();
  expect(cleaned).toHaveBeenCalledTimes(1);
});

test("mountAll on a new render cleans up what was mounted before", () => {
  const id = fresh();
  wrapper(id);
  const log: string[] = [];
  zen().register(id, { mount() { log.push("mount"); return () => log.push("clean"); } });
  rt.mountAll(root);
  root.replaceChildren();
  wrapper(id);
  rt.mountAll(root);
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
  zen().register(bad, { mount() { throw new Error("<b>boom</b>"); } });
  zen().register(slow, { async mount() { throw new Error("later"); } });
  zen().register(good, { mount(el) { el.textContent = "fine"; } });
  rt.mountAll(root);
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

test("a handler without a mount function is reported, not thrown", () => {
  const err = vi.spyOn(console, "error").mockImplementation(() => {});
  const id = fresh();
  const w = wrapper(id);
  rt.mountAll(root);
  expect(() => zen().register(id, {} as never)).not.toThrow();
  expect(() => zen().register("", { mount() {} })).not.toThrow();
  expect(w.querySelector(".zen-plugin-error")).toBeNull();
  expect(err).toHaveBeenCalled();
});

test("a clean-up that throws does not stop the others", () => {
  const err = vi.spyOn(console, "error").mockImplementation(() => {});
  const a = fresh();
  const b = fresh();
  const w1 = wrapper(a);
  wrapper(b);
  const cleaned = vi.fn();
  zen().register(a, { mount: () => cleaned });
  zen().register(b, { mount: () => () => { throw new Error("stuck"); } });
  rt.mountAll(root);
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
  for (const id of [a, b]) zen().register(id, { mount(_el, ctx) { ctxs[id] = ctx; } });
  rt.mountAll(root);

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
  zen().register(id, { mount(_el, c) { ctx = c; } });
  rt.mountAll(root);
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
  zen().register(id, { mount(_el, c) { ctx = c; } });
  rt.mountAll(root);
  api.invokeResult = (pid: string, payload: unknown) => ({ pid, payload });
  expect(await ctx.invoke({ answer: 2 })).toEqual({ pid: id, payload: { answer: 2 } });
  expect(await ctx.invoke(undefined)).toEqual({ pid: id, payload: null });
  expect(api.callsTo("pluginInvoke")).toEqual([[id, "{\"answer\":2}"], [id, "null"]]);
  api.failNext("pluginInvoke", new Error("no action hook"));
  await expect(ctx.invoke(1)).rejects.toThrow("no action hook");
});

test("destroy cleans up; handlers stay registered for the next runtime", () => {
  const id = fresh();
  const w = wrapper(id);
  const cleaned = vi.fn();
  zen().register(id, { mount(el) { el.textContent = "on"; return cleaned; } });
  rt.mountAll(root);
  rt.destroy();
  expect(cleaned).toHaveBeenCalledTimes(1);
  w.textContent = "content";
  zen().register(id, { mount(el) { el.textContent = "again"; } }); // no page: only recorded
  expect(w.textContent).toBe("content");
  rt = installRuntime({ api, navigate: () => {}, page: () => ({ path: "x.md", title: "X" }) });
  rt.mountAll(root);
  expect(w.textContent).toBe("again");
});

// ---- core/plugin-extras: stream, storage.keys, anchor, badge ----

type Extras = { anchor?: () => string; onAnchor?: (fn: (a: string) => void) => () => void; badge?: (path: string, text: string | null) => void };
/** Replaces the runtime of beforeEach with one that has the extras wired. */
function withExtras(extras: Extras = {}) {
  rt.destroy();
  rt = installRuntime({ api, navigate: () => {}, page: () => ({ path: "guides/a.md", title: "A" }), ...extras });
}
/** Mounts one block of a fresh plugin and returns its ctx. */
function ctxOf(tag: string = "one"): PluginCtx {
  const id = fresh();
  wrapper(id, { name: tag });
  let got!: PluginCtx;
  zen().register(id, { mount(_el, ctx) { got = ctx; } });
  rt.mountAll(root);
  return got;
}
const chunk = (call: string, text: string) => api.emit("plugin:stream", { call, chunk: text, done: false, error: "" });
const end = (call: string, error = "") => api.emit("plugin:stream", { call, chunk: "", done: true, error });

test("storage.keys lists the plugin's keys, optionally by prefix", async () => {
  const id = fresh();
  wrapper(id);
  let ctx!: PluginCtx;
  zen().register(id, { mount(_el, c) { ctx = c; } });
  rt.mountAll(root);
  await ctx.storage.set("quiz.b", 1);
  await ctx.storage.set("quiz.a", 2);
  await ctx.storage.set("other", 3);
  expect(await ctx.storage.keys("quiz.")).toEqual(["quiz.a", "quiz.b"]);
  expect(await ctx.storage.keys()).toEqual(["other", "quiz.a", "quiz.b"]);
  expect(api.callsTo("pluginStorageKeys")).toEqual([[id, "quiz."], [id, ""]]);
});

test("stream delivers chunks in order and done resolves when Go says so", async () => {
  const ctx = ctxOf();
  const got: string[] = [];
  const s = ctx.stream({ q: 1 }, (c) => got.push(c));
  await settle();
  expect(api.callsTo("pluginStream")).toEqual([[ctx.id, "{\"q\":1}"]]);
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

test("chunks that arrive before the call id is known are not lost", async () => {
  const ctx = ctxOf();
  const got: string[] = [];
  const s = ctx.stream(null, (c) => got.push(c));
  chunk("call-1", "early");   // pluginStream() has not resolved yet
  end("call-1");
  await s.done;
  expect(got).toEqual(["early"]);
});

test("done rejects with the stream's error text", async () => {
  const ctx = ctxOf();
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
    zen().register(id, { mount(_el, c) { got.push(c); } });
  }
  rt.mountAll(root);
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
  const ctx = ctxOf();
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
  const ctx = ctxOf();
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
  zen().register(id, { mount(_el, c) { ctx = c; } });
  rt.mountAll(root);
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
  const ctx = ctxOf();
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

test("anchor() and onAnchor(): current anchor, changes, removal on unmount", () => {
  let current = "intro";
  const subs = new Set<(a: string) => void>();
  withExtras({ anchor: () => current, onAnchor: (fn) => { subs.add(fn); return () => void subs.delete(fn); } });
  const ctx = ctxOf();
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

test("anchor() is empty and onAnchor harmless when the host does not provide them", () => {
  const ctx = ctxOf();
  expect(ctx.anchor()).toBe("");
  expect(() => ctx.onAnchor(() => {})()).not.toThrow();
  expect(() => ctx.badge("a.md", "x")).not.toThrow();
});

test("badge(path, text) sets and clears a label, at most 12 characters", () => {
  const calls: [string, string | null][] = [];
  withExtras({ badge: (p, t) => void calls.push([p, t]) });
  const ctx = ctxOf();
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

test("badge ignores a path that is not a string", () => {
  const calls: unknown[] = [];
  withExtras({ badge: (p, t) => void calls.push([p, t]) });
  const ctx = ctxOf();
  (ctx.badge as (p: unknown, t: unknown) => void)(42, "x");
  (ctx.badge as (p: unknown, t: unknown) => void)("", "x");
  expect(calls).toEqual([]);
});
