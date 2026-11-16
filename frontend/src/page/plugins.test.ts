// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import type { PluginCtx } from "../plugins/runtime";
import { mountPage, pageFeature } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

type Zen = { register(id: string, h: { mount(el: HTMLElement, ctx: PluginCtx): unknown }): void };
const zen = () => (window as unknown as { zen?: Zen }).zen;

let env: Env;
let api: FakeApi;
let unmount: (() => void) | undefined;
let n = 0;
const fresh = () => `pg${++n}`;

beforeEach(() => {
  document.head.replaceChildren();
  document.body.replaceChildren();
  api = createFakeApi({ sources: { "README.md": "# Home\n\nText.", "b.md": "# B\n\nBee." } });
  env = setup(api);
});
afterEach(() => { unmount?.(); unmount = undefined; });

const mount = () => { unmount = pageFeature.mount(env.slots, env.ctx); };
const article = () => env.slots.main.querySelector("article.zen-page") as HTMLElement;
const settle = async () => { for (let i = 0; i < 5; i++) await tick(); };
/** Every page gets one block of plugin `id` after its own content. */
const withBlock = (id: string) => {
  api.decorate = (p) => ({ ...p, html: `${p.html}<div data-zen-plugin="${id}" data-zen-name="box" data-zen-attrs='{"k":"v"}'>raw</div>` });
};

test("the zen object exists once the reader is mounted, before any page or script loads", () => {
  mount();
  expect(typeof zen()?.register).toBe("function");
});

test("mounts plugin blocks after a render, with the page being shown and the app's navigate", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md" });
  mount();
  const seen: PluginCtx[] = [];
  zen()!.register(id, { mount(el, ctx) { seen.push(ctx); el.textContent = "mounted"; } });
  await settle();
  expect(seen).toHaveLength(1);
  expect(seen[0].page).toEqual({ path: "README.md", title: "Home" });
  expect(seen[0].attrs).toEqual({ k: "v" });
  expect(article().querySelector(`[data-zen-plugin="${id}"]`)!.textContent).toBe("mounted");
  seen[0].navigate("b.md", "top");
  expect(env.navigations).toEqual([["b.md", "top"]]);
});

test("cleans up before every re-render: navigation, live reload and draft updates", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md" });
  mount();
  const log: string[] = [];
  zen()!.register(id, {
    mount(el, ctx) {
      log.push(`mount ${ctx.page.path}`);
      return () => log.push(`clean ${ctx.page.path} ${el.isConnected ? "connected" : "gone"}`);
    },
  });
  await settle();
  expect(log).toEqual(["mount README.md"]);

  api.emit("page:changed", { path: "README.md" });
  await settle();
  expect(log.slice(1)).toEqual(["clean README.md connected", "mount README.md"]);

  api.emit("page:draft", { path: "README.md", content: "# Draft\n\nNew." });
  await settle();
  expect(log.slice(3)).toEqual(["clean README.md connected", "mount README.md"]);
  expect(article().querySelector("h1")!.textContent).toBe("Draft");

  env.ctx.navigate("b.md");
  await settle();
  expect(log.slice(5)).toEqual(["clean README.md connected", "mount b.md"]);

  unmount!();
  unmount = undefined;
  expect(log.slice(7)).toEqual(["clean b.md connected"]);
});

test("a failed load leaves nothing mounted twice", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md" });
  mount();
  const log: string[] = [];
  zen()!.register(id, { mount() { log.push("mount"); return () => log.push("clean"); } });
  await settle();
  api.failNext("page", new Error("gone"));
  api.emit("page:changed", { path: "README.md" });
  await settle();
  api.emit("page:changed", { path: "README.md" });
  await settle();
  expect(log).toEqual(["mount", "clean", "mount"]);
});

test("a layout page marks the article so the plugin gets the whole pane", async () => {
  const id = fresh();
  api.decorate = (p) => p.path === "README.md"
    ? { ...p, layout: "slides", html: `<div data-zen-plugin="${id}" data-zen-layout="slides">${p.html}</div>` }
    : p;
  env.ctx.store.set({ path: "README.md" });
  mount();
  const kinds: string[] = [];
  zen()!.register(id, { mount(_el, ctx) { kinds.push(`${ctx.kind}:${ctx.name}`); } });
  await settle();
  expect(kinds).toEqual(["layout:slides"]);
  expect(article().dataset.layout).toBe("slides");
  expect(article().classList.contains("zen-layout-slides")).toBe(true);

  env.ctx.navigate("b.md");
  await settle();
  expect(article().dataset.layout).toBeUndefined();
  expect(article().className).toBe("zen-page");
});

test("mountPage ignores a layout name that is not a plain name", () => {
  const el = document.createElement("article");
  el.className = "zen-page";
  const page = { path: "a.md", title: "A", html: "<p>a</p>", frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [] };
  mountPage(el, { ...page, layout: "x y\"><b>" });
  expect(el.dataset.layout).toBeUndefined();
  expect(el.className).toBe("zen-page");
  mountPage(el, { ...page, layout: "course" });
  expect(el.className).toBe("zen-page zen-layout-course");
  mountPage(el, { ...page, layout: "slides" });
  expect(el.className).toBe("zen-page zen-layout-slides");
});

test("a page without plugin blocks logs no errors", async () => {
  const err = vi.spyOn(console, "error").mockImplementation(() => {});
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(err).not.toHaveBeenCalled();
  err.mockRestore();
});

// ---- core/plugin-extras wiring: anchor, badge, print ----

test("ctx.anchor() is the current anchor; onAnchor fires when only the anchor changes, not on a reload", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md", anchor: "intro" });
  mount();
  let ctx!: PluginCtx;
  zen()!.register(id, { mount(_el, c) { ctx = c; } });
  await settle();
  expect(ctx.anchor()).toBe("intro");
  const seen: string[] = [];
  ctx.onAnchor((a) => seen.push(a));
  env.ctx.store.set({ anchor: "part-2" });
  expect(seen).toEqual(["part-2"]);
  expect(ctx.anchor()).toBe("part-2");
  env.ctx.store.set({ anchor: "" });
  expect(seen).toEqual(["part-2", ""]);
  // another page is a new render: handlers of the old one are gone
  env.ctx.navigate("b.md", "x");
  await settle();
  expect(seen).toEqual(["part-2", ""]);
});

test("ctx.badge puts a label in the store for the window's lifetime", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md" });
  mount();
  let ctx!: PluginCtx;
  zen()!.register(id, { mount(_el, c) { ctx = c; } });
  await settle();
  ctx.badge("b.md", "2/5");
  expect(env.ctx.store.get().badges).toEqual({ "b.md": "2/5" });
  ctx.badge("README.md", "x".repeat(30));
  expect(env.ctx.store.get().badges).toEqual({ "b.md": "2/5", "README.md": "x".repeat(12) });
  ctx.badge("b.md", null);
  expect(env.ctx.store.get().badges).toEqual({ "README.md": "x".repeat(12) });
  unmount!(); unmount = undefined;
  expect(env.ctx.store.get().badges).toEqual({ "README.md": "x".repeat(12) });
});

test("⌘P prints the page (not in an editor window); other keys do not", async () => {
  const print = vi.spyOn(window, "print").mockImplementation(() => {});
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  const e = new KeyboardEvent("keydown", { key: "p", metaKey: true, cancelable: true, bubbles: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(true);
  expect(print).toHaveBeenCalledTimes(1);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "p", ctrlKey: true, bubbles: true }));
  expect(print).toHaveBeenCalledTimes(2);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "p", bubbles: true }));
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "p", metaKey: true, shiftKey: true, bubbles: true }));
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "o", metaKey: true, bubbles: true }));
  expect(print).toHaveBeenCalledTimes(2);
  unmount!(); unmount = undefined;
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "p", metaKey: true, bubbles: true }));
  expect(print).toHaveBeenCalledTimes(2);
});
