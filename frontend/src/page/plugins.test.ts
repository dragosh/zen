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
