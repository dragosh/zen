// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmd-app/frontend/src/api/fake";
import type { PluginCtx } from "@zenmd/runtime/plugins";
import { mountPage, pageFeature, pageHooks } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

/** Plugin scripts by URL: what import() would hand back. */
const modules = new Map<string, unknown>();
const imported: string[] = [];
pageHooks.importModule = async (url) => {
  imported.push(url);
  if (!modules.has(url)) throw new Error(`404 ${url}`);
  return modules.get(url);
};
const urlOf = (id: string, v = "aaaaaaaaaaaa") => `/_plugins/${id}/${id}.js?v=${v}`;
let version = "aaaaaaaaaaaa";
const layoutUrl = (id: string) => `/_layouts/${id}/${id}.js?v=aaaaaaaaaaaa`;
const registerLayout = (id: string, h: { mount(el: HTMLElement, ctx: PluginCtx): unknown }) => { modules.set(layoutUrl(id), { default: h }); };
const register = (id: string, h: { mount(el: HTMLElement, ctx: PluginCtx): unknown }) => { modules.set(urlOf(id, version), { default: h }); };

let env: Env;
let api: FakeApi;
let unmount: (() => void) | undefined;
let n = 0;
const fresh = () => `pg${++n}`;

beforeEach(() => {
  document.head.replaceChildren();
  document.body.replaceChildren();
  modules.clear();
  imported.length = 0;
  version = "aaaaaaaaaaaa";
  api = createFakeApi({ sources: { "README.md": "# Home\n\nText.", "b.md": "# B\n\nBee." } });
  env = setup(api);
});
afterEach(() => { unmount?.(); unmount = undefined; });

const mount = () => { unmount = pageFeature.mount(env.slots, env.ctx); };
const article = () => env.slots.main.querySelector("article.zen-page") as HTMLElement;
const settle = async () => { for (let i = 0; i < 5; i++) await tick(); };
/** Every page gets one block of plugin `id` after its own content. */
const withBlock = (id: string) => {
  api.decorate = (p) => ({
    ...p, html: `${p.html}<div data-zen-plugin="${id}" data-zen-name="box" data-zen-attrs='{"k":"v"}'>raw</div>`,
    assets: { css: [`/_plugins/${id}/${id}.css?v=${version}`], js: [urlOf(id, version)] },
  });
};

test("the zen object exists once the reader is mounted: a version only", () => {
  mount();
  const z = (window as unknown as { zen?: { version: number; register?: unknown } }).zen;
  expect(z?.version).toBe(2);
  expect(z?.register).toBeUndefined();
});

test("mounts plugin blocks after a render, with the page being shown and the app's navigate", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md" });
  mount();
  const seen: PluginCtx[] = [];
  register(id, { mount(el, ctx) { seen.push(ctx); el.textContent = "mounted"; } });
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
  register(id, {
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

test("plugins:changed re-renders the page with the new asset URLs: old script and stylesheet gone, new ones loaded", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md" });
  mount();
  const log: string[] = [];
  register(id, { mount(el) { log.push("mount v1"); el.textContent = "v1"; return () => log.push("clean v1"); } });
  await settle();
  expect(Array.from(document.head.querySelectorAll("link"), (l) => l.getAttribute("href"))).toEqual([`/_plugins/${id}/${id}.css?v=aaaaaaaaaaaa`]);

  // Go reports a new plugin set; the plugin's files changed, so its URLs carry a new version
  version = "bbbbbbbbbbbb";
  register(id, { mount(el) { log.push("mount v2"); el.textContent = "v2"; } });
  api.emit("plugins:changed", undefined);
  await settle();
  expect(log).toEqual(["mount v1", "clean v1", "mount v2"]);
  expect(imported).toEqual([urlOf(id, "aaaaaaaaaaaa"), urlOf(id, "bbbbbbbbbbbb")]);
  expect(Array.from(document.head.querySelectorAll("link"), (l) => l.getAttribute("href"))).toEqual([`/_plugins/${id}/${id}.css?v=bbbbbbbbbbbb`]);
  expect(article().querySelector(`[data-zen-plugin="${id}"]`)!.textContent).toBe("v2");
  expect(document.head.querySelector("script")).toBeNull();
});

test("a plugin that is no longer on the page after plugins:changed is unmounted and not run again", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md" });
  mount();
  const log: string[] = [];
  register(id, { mount() { log.push("mount"); return () => log.push("clean"); } });
  await settle();
  api.decorate = undefined; // the plugin was blocked: the page comes back without its assets
  api.emit("plugins:changed", undefined);
  await settle();
  expect(log).toEqual(["mount", "clean"]);
  expect(document.head.querySelector("link")).toBeNull();
});

test("ctx.invoke and ctx.stream name the page being shown", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "b.md" });
  mount();
  let ctx!: PluginCtx;
  register(id, { mount(_el, c) { ctx = c; } });
  await settle();
  await ctx.invoke({ a: 1 });
  ctx.stream({ s: 1 }, () => {});
  await settle();
  expect(api.callsTo("pluginInvoke")).toEqual([[id, "b.md", "{\"a\":1}"]]);
  expect(api.callsTo("pluginStream")).toEqual([[id, "b.md", "{\"s\":1}"]]);
});

test("a failed load leaves nothing mounted twice", async () => {
  const id = fresh();
  withBlock(id);
  env.ctx.store.set({ path: "README.md" });
  mount();
  const log: string[] = [];
  register(id, { mount() { log.push("mount"); return () => log.push("clean"); } });
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
    ? { ...p, layout: { name: "slides", source: "builtin" }, assets: { css: [], js: [layoutUrl(id)] }, html: `<div data-zen-layout="${id}" data-zen-layout-source="builtin">${p.html}</div>` }
    : p;
  env.ctx.store.set({ path: "README.md" });
  mount();
  const kinds: string[] = [];
  registerLayout(id, { mount(_el, ctx) { kinds.push(`${ctx.kind}:${ctx.name}`); } });
  await settle();
  expect(kinds).toEqual([`layout:${id}`]);
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
  mountPage(el, { ...page, layout: { name: "x y\"><b>", source: "" } });
  expect(el.dataset.layout).toBeUndefined();
  expect(el.className).toBe("zen-page");
  mountPage(el, { ...page, layout: { name: "course", source: "builtin" } });
  expect(el.className).toBe("zen-page zen-layout-course");
  mountPage(el, { ...page, layout: { name: "slides", source: "builtin" } });
  expect(el.className).toBe("zen-page zen-layout-slides");
  mountPage(el, { ...page, layout: { name: "article", source: "" } });
  expect(el.className).toBe("zen-page"); // the reading view keeps the reading column
  expect(el.dataset.layout).toBeUndefined();
  mountPage(el, { ...page, layout: { name: "docs", source: "builtin" } });
  expect(el.className).toBe("zen-page zen-layout-docs");
  expect(el.dataset.layout).toBe("docs");
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
  register(id, { mount(_el, c) { ctx = c; } });
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
  register(id, { mount(_el, c) { ctx = c; } });
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

test("the page leaves ⌘P to the print flow (print/page.ts)", async () => {
  const print = vi.fn();
  (window as unknown as { print: () => void }).print = print; // happy-dom has none
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  const e = new KeyboardEvent("keydown", { key: "p", metaKey: true, cancelable: true, bubbles: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(false);
  expect(print).not.toHaveBeenCalled();
});

test("layouts:changed renders the page again and mounts the new layout script", async () => {
  const id = fresh();
  let v = 1;
  api.decorate = (p) => p.path === "README.md"
    ? { ...p, layout: { name: "post", source: "folder" }, assets: { css: [], js: [layoutUrl(id).replace("aaaaaaaaaaaa", `v${v}`)] }, html: `<div data-zen-layout="${id}" data-zen-layout-source="folder">${p.html}</div>` }
    : p;
  env.ctx.store.set({ path: "README.md" });
  mount();
  const log: string[] = [];
  modules.set(layoutUrl(id).replace("aaaaaaaaaaaa", "v1"), { default: { mount() { log.push("v1"); return () => log.push("clean v1"); } } });
  modules.set(layoutUrl(id).replace("aaaaaaaaaaaa", "v2"), { default: { mount() { log.push("v2"); } } });
  await settle();
  expect(log).toEqual(["v1"]);
  expect(article().dataset.layout).toBe("post");
  v = 2;
  api.emit("layouts:changed", undefined);
  await settle();
  expect(log).toEqual(["v1", "clean v1", "v2"]);
  expect(api.callsTo("page").length).toBeGreaterThanOrEqual(2);
});

test("layouts:changed re-renders an editor's unsaved draft, not the file", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  api.emit("page:draft", { path: "README.md", content: "# Draft" });
  await settle();
  const pages = api.callsTo("page").length;
  api.emit("layouts:changed", undefined);
  await settle();
  expect(api.callsTo("page").length).toBe(pages);
  expect(api.callsTo("preview").length).toBeGreaterThanOrEqual(2);
});

test("a layout page links its stylesheet from /_layouts/", async () => {
  api.decorate = (p) => ({ ...p, layout: { name: "post", source: "builtin" }, assets: { css: ["/_layouts/post/post.css?v=abc123abc123"], js: [] } });
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(document.head.querySelector("link[data-zen-asset^='/_layouts/post/']")).not.toBeNull();
});
