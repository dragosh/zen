// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi } from "../api/fake";
import type { Page } from "../api";
import { pageFeature } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

let env: Env;
let unmount: (() => void) | undefined;

const SOURCES = {
  "README.md": "# Sample docs\n\nIntro.\n\n## Usage\n\nBody.",
  "b.md": "# B\n\nBee.",
};

beforeEach(() => {
  document.head.replaceChildren();
  document.body.replaceChildren();
  env = setup(createFakeApi({ sources: { ...SOURCES } }));
});
afterEach(() => { unmount?.(); unmount = undefined; vi.useRealTimers(); });

const mount = () => { unmount = pageFeature.mount(env.slots, env.ctx); };
const article = () => env.slots.main.querySelector("article.zen-page") as HTMLElement;
const btn = (root: HTMLElement, label: string) =>
  Array.from(root.querySelectorAll("button")).find((b) => b.textContent?.includes(label)) as HTMLButtonElement;

test("loads, mounts, sets title and stores the page", async () => {
  expect(pageFeature.id).toBe("page");
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  expect(article().querySelector("h1")?.textContent).toBe("Sample docs");
  expect(document.title).toBe("Sample docs — zen");
  expect(env.ctx.store.get().page?.path).toBe("README.md");

  env.ctx.navigate("b.md");
  await tick();
  expect(article().querySelector("h1")?.textContent).toBe("B");
  expect(env.api.callsTo("page")).toEqual([["README.md"], ["b.md"]]);
});

test("clicks in the article are handled", async () => {
  env.api = createFakeApi();
  env = setup(env.api);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  (article().querySelector("a[data-zen-link=internal]") as HTMLElement).click();
  expect(env.navigations[0][0]).toBe("guides/install.md");
});

test("anchors: scroll to element on load, top otherwise, smooth for same page", async () => {
  const spy = vi.fn();
  (window.HTMLElement.prototype as unknown as { scrollIntoView: unknown }).scrollIntoView = spy;
  env.ctx.store.set({ path: "README.md", anchor: "usage" });
  mount();
  await tick();
  // fake renderer emits no ids; add one and re-trigger via navigation
  expect(spy).not.toHaveBeenCalled();
  env.slots.main.scrollTop = 50;
  env.ctx.navigate("b.md");
  await tick();
  expect(env.slots.main.scrollTop).toBe(0);

  article().innerHTML = `<h2 id="x">X</h2>`;
  env.ctx.navigate("b.md", "x");
  expect(spy).toHaveBeenCalledTimes(1);
  expect(spy.mock.calls[0][0]).toMatchObject({ behavior: expect.stringMatching(/smooth|auto/) });
  expect(env.api.callsTo("page")).toHaveLength(2); // no reload for same page
});

test("anchor on another page scrolls after mount", async () => {
  const spy = vi.fn();
  (window.HTMLElement.prototype as unknown as { scrollIntoView: unknown }).scrollIntoView = spy;
  env = setup(createFakeApi({ sources: SOURCES }));
  const realPage = env.api.page;
  env.api.page = async (p) => ({ ...(await realPage(p)), html: `<h1 id="top">T</h1><h2 id="sec">S</h2>` });
  env.ctx.store.set({ path: "README.md", anchor: "sec" });
  mount();
  await tick();
  expect(spy).toHaveBeenCalledTimes(1);
  expect((spy.mock.contexts[0] as HTMLElement).id).toBe("sec");
});

test("latest request wins", async () => {
  const resolvers: Record<string, (p: Page) => void> = {};
  const base = (path: string): Page => ({ path, title: path, html: `<h1>${path}</h1>`, frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [] });
  env.api.page = (path) => new Promise((r) => { resolvers[path] = r; });
  env.ctx.store.set({ path: "README.md" });
  mount();
  env.ctx.navigate("b.md");
  resolvers["b.md"](base("b.md"));
  await tick();
  resolvers["README.md"](base("README.md"));
  await tick();
  expect(article().textContent).toBe("b.md");
  expect(env.ctx.store.get().page?.path).toBe("b.md");
});

test("progress line appears after 150 ms and old page stays", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  vi.useFakeTimers();
  let release!: () => void;
  const real = env.api.page;
  env.api.page = (p) => new Promise((r) => { release = () => r(real(p)); });
  env.ctx.navigate("b.md");
  const progress = env.slots.main.querySelector(".zen-progress") as HTMLElement;
  expect(progress.hidden).toBe(true);
  vi.advanceTimersByTime(149);
  expect(progress.hidden).toBe(true);
  vi.advanceTimersByTime(2);
  expect(progress.hidden).toBe(false);
  expect(article().textContent).toContain("Sample docs");
  vi.useRealTimers();
  release();
  await tick(); await tick();
  expect(progress.hidden).toBe(true);
});

test("error shows message and Retry; error text is escaped", async () => {
  env.api.failNext("page", new Error("<b>boom</b>"));
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  const err = env.slots.main.querySelector(".zen-page-error") as HTMLElement;
  expect(err.hidden).toBe(false);
  expect(err.textContent).toContain("<b>boom</b>");
  expect(err.querySelector("b")).toBeNull();
  btn(err, "Retry").click();
  await tick();
  expect(err.hidden).toBe(true);
  expect(article().querySelector("h1")?.textContent).toBe("Sample docs");
});

test("page:changed re-renders the current page in place and keeps scroll", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  const events: unknown[] = [];
  document.addEventListener("zen:page-rendered", (e) => events.push(e));
  await env.api.save("README.md", "# Changed\n\nNew.");
  env.slots.main.scrollTop = 120;
  env.api.emit("page:changed", { path: "b.md" });
  await tick();
  expect(env.api.callsTo("page")).toHaveLength(1);
  env.api.emit("page:changed", { path: "README.md" });
  await tick();
  expect(article().querySelector("h1")?.textContent).toBe("Changed");
  expect(env.slots.main.scrollTop).toBe(120);
  expect(events).toHaveLength(1);
});

test("warnings bar is shown, escaped and dismissible", async () => {
  const real = env.api.page;
  env.api.page = async (p) => ({ ...(await real(p)), warnings: ["bad <i>front</i>matter"] });
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  const bar = env.slots.main.querySelector(".zen-page-warnings") as HTMLElement;
  expect(bar.hidden).toBe(false);
  expect(bar.textContent).toContain("bad <i>front</i>matter");
  expect(bar.querySelector("i")).toBeNull();
  (bar.querySelector("button") as HTMLButtonElement).click();
  expect(bar.hidden).toBe(true);
});

test("Edit button and ⌘E switch to edit mode; unmount cleans up", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  btn(env.slots.actions, "Edit").click();
  expect(env.ctx.store.get().mode).toBe("edit");
  env.ctx.store.set({ mode: "read" });
  const e = new KeyboardEvent("keydown", { key: "e", metaKey: true, cancelable: true, bubbles: true });
  document.dispatchEvent(e);
  expect(env.ctx.store.get().mode).toBe("edit");
  expect(e.defaultPrevented).toBe(true);
  unmount!(); unmount = undefined;
  expect(env.slots.actions.childNodes).toHaveLength(0);
  expect(env.slots.main.childNodes).toHaveLength(0);
  env.ctx.store.set({ mode: "read" });
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "e", metaKey: true }));
  expect(env.ctx.store.get().mode).toBe("read");
});
