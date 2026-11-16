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
