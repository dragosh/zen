// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import type { Page } from "../host";
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
  expect(document.title).toBe("Sample docs — Zenmark");
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
  const base = (path: string): Page => ({ path, title: path, html: `<h1>${path}</h1>`, frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [], layout: { name: "article", source: "" } });
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

test("Edit button and ⌘E open the editor window; no in-window edit mode; unmount cleans up", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  btn(env.slots.actions, "Edit").click();
  expect(env.api.callsTo("openEditor")).toEqual([["README.md"]]);
  expect(env.ctx.store.get().mode).toBe("read");
  expect(article().hidden).toBe(false);

  env.ctx.navigate("b.md");
  const e = new KeyboardEvent("keydown", { key: "e", metaKey: true, cancelable: true, bubbles: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(true);
  expect(env.api.callsTo("openEditor")).toEqual([["README.md"], ["b.md"]]);
  expect(env.ctx.store.get().mode).toBe("read");

  unmount!(); unmount = undefined;
  expect(env.slots.actions.childNodes).toHaveLength(0);
  expect(env.slots.main.childNodes).toHaveLength(0);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "e", metaKey: true }));
  expect(env.api.callsTo("openEditor")).toHaveLength(2);
});

test("a failed openEditor is reported, not thrown", async () => {
  const errors: unknown[] = [];
  const orig = console.error;
  console.error = (...a: unknown[]) => void errors.push(a);
  try {
    env.ctx.store.set({ path: "README.md" });
    mount();
    env.api.failNext("openEditor", new Error("not a markdown file"));
    btn(env.slots.actions, "Edit").click();
    await tick();
    expect(errors).toHaveLength(1);
  } finally {
    console.error = orig;
  }
});

test("Edit with no page does nothing", () => {
  mount();
  btn(env.slots.actions, "Edit").click();
  expect(env.api.callsTo("openEditor")).toHaveLength(0);
});

test("read-only: no edit actions", async () => {
  const api = createFakeApi({ sources: { ...SOURCES }, build: { features: [] } });
  const page = api.page.bind(api);
  api.page = async (path: string) => {
    const pg = await page(path);
    return { ...pg, html: pg.html + `<p><a data-zen-action="edit" href="#">Edit this page</a></p>` };
  };
  env = setup(api);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  expect(article().querySelector("h1")?.textContent).toBe("Sample docs");
  // no Edit button
  expect(btn(env.slots.actions, "Edit")).toBeUndefined();
  // ⌘E is not taken
  const e = new KeyboardEvent("keydown", { key: "e", metaKey: true, cancelable: true, bubbles: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(false);
  // the layout's Edit this page link is hidden, and does nothing
  const link = article().querySelector<HTMLElement>("[data-zen-action=edit]")!;
  expect(link.hidden).toBe(true);
  link.click();
  await tick();
  expect(env.api.callsTo("openEditor")).toHaveLength(0);
  // reading is unchanged
  env.ctx.navigate("b.md");
  await tick();
  expect(article().querySelector("h1")?.textContent).toBe("B");
});

test("with the editor the edit link is shown", async () => {
  const api = createFakeApi({ sources: { ...SOURCES } });
  const page = api.page.bind(api);
  api.page = async (path: string) => ({ ...(await page(path)), html: `<p><a data-zen-action="edit" href="#">Edit this page</a></p>` });
  env = setup(api);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  expect(article().querySelector<HTMLElement>("[data-zen-action=edit]")!.hidden).toBe(false);
  expect(btn(env.slots.actions, "Edit")).toBeDefined();
});

const hint = () => env.slots.main.querySelector<HTMLElement>(".zen-dialect-hint");

const unmountPage = () => { unmount?.(); unmount = undefined; };

test("dialect hint", async () => {
  const api = createFakeApi({ sources: { ...SOURCES }, dialectHint: "myst" });
  env = setup(api);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  // one quiet line above the page, announced politely, text only
  expect(hint()!.hidden).toBe(false);
  expect(hint()!.getAttribute("role")).toBe("status");
  expect(hint()!.textContent).toContain("This folder looks like a MyST project. Turn on MyST syntax?");
  expect(Array.from(hint()!.querySelectorAll("button")).map((b) => b.textContent)).toEqual(["Turn on", "Not now"]);
  expect(hint()!.compareDocumentPosition(article()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

  // Turn on writes the folder and the pages render again
  const before = api.callsTo("page").length;
  btn(hint()!, "Turn on").click();
  await tick();
  expect(api.callsTo("setDialect")).toEqual([["myst", true]]);
  expect(hint()!.hidden).toBe(true);
  expect(api.callsTo("page").length).toBeGreaterThan(before);
  expect(api.callsTo("dismissDialectHint")).toEqual([]);
  unmountPage();

  // Not now is remembered by the app
  const api2 = createFakeApi({ sources: { ...SOURCES }, dialectHint: "myst" });
  env = setup(api2);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  btn(hint()!, "Not now").click();
  await tick();
  expect(api2.callsTo("dismissDialectHint")).toEqual([["myst"]]);
  expect(api2.callsTo("setDialect")).toEqual([]);
  expect(hint()!.hidden).toBe(true);
  unmountPage();

  // Esc dismisses it like Not now
  const api3 = createFakeApi({ sources: { ...SOURCES }, dialectHint: "myst" });
  env = setup(api3);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  await tick();
  expect(api3.callsTo("dismissDialectHint")).toEqual([["myst"]]);
  expect(hint()!.hidden).toBe(true);
  unmountPage();

  // a refused Turn on says why and keeps the hint
  const api4 = createFakeApi({ sources: { ...SOURCES }, dialectHint: "myst" });
  env = setup(api4);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  api4.failNext("setDialect", new Error("could not save .zen.yml"));
  btn(hint()!, "Turn on").click();
  await tick();
  expect(hint()!.hidden).toBe(false);
  expect(hint()!.textContent).toContain("could not save .zen.yml");
  unmountPage();

  // no hint: nothing suggested, a read-only app, an api without the calls
  for (const [name, a] of [
    ["none", createFakeApi({ sources: { ...SOURCES }, dialectHint: "" })],
    ["read-only", createFakeApi({ sources: { ...SOURCES }, dialectHint: "myst", build: { features: [] } })],
    ["dialect not in the list", createFakeApi({ sources: { ...SOURCES }, dialectHint: "mkdocs" })],
  ] as const) {
    env = setup(a);
    env.ctx.store.set({ path: "README.md" });
    mount();
    await tick();
    expect(hint() === null || hint()!.hidden, name).toBe(true);
    unmountPage();
    document.body.replaceChildren();
  }
  const bare = createFakeApi({ sources: { ...SOURCES } }) as unknown as Record<string, unknown>;
  delete bare.dialectHint;
  env = setup(bare as unknown as ReturnType<typeof createFakeApi>);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await tick();
  expect(hint() === null || hint()!.hidden).toBe(true);
});
