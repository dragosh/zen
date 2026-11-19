// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import { alert } from "../lint/test-util";
import { INSPECTOR_STATE, INSPECTOR_TOGGLE, OPEN_KEY, TAB_KEY } from "../inspector";
import { pageFeature } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

let env: Env;
let api: FakeApi;
let unmount: (() => void) | undefined;

const SOURCES = { "README.md": "# Home\n\nSome teh text.\n", "b.md": "# B\n\nA b page with [home](README.md).\n" };
const TEH = alert({ line: 3, col: 6, endCol: 8, match: "teh" });

beforeEach(() => {
  localStorage.clear();
  document.head.replaceChildren();
  document.body.replaceChildren();
  api = createFakeApi({ sources: { ...SOURCES }, lint: { available: true, alerts: [TEH] } });
  env = setup(api);
});
afterEach(() => { unmount?.(); unmount = undefined; vi.useRealTimers(); });

const mount = () => { unmount = pageFeature.mount(env.slots, env.ctx); };
const settle = async () => { for (let i = 0; i < 6; i++) await tick(); };
let states: { available: boolean; open: boolean; count: number }[] = [];
const onState = (e: Event) => states.push((e as CustomEvent).detail);
beforeEach(() => { states = []; document.addEventListener(INSPECTOR_STATE, onState); });
afterEach(() => document.removeEventListener(INSPECTOR_STATE, onState));
const toggleEvent = () => document.dispatchEvent(new CustomEvent(INSPECTOR_TOGGLE));
const panel = () => document.querySelector<HTMLElement>(".zen-inspector")!;
const field = (name: string) => panel().querySelector(`[data-field="${name}"]`)?.textContent;
const altCmdI = () => document.dispatchEvent(new KeyboardEvent("keydown", { key: "ˆ", code: "KeyI", metaKey: true, altKey: true, bubbles: true, cancelable: true }));

test("no Inspector button in the actions; the sidebar button is told, and toggles the panel", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(env.slots.actions.querySelector(".zen-inspector-toggle")).toBeNull();
  expect(Array.from(env.slots.actions.children).map((c) => c.textContent)).toEqual(["Edit"]);
  expect(panel().hidden).toBe(true);
  expect(panel().compareDocumentPosition(env.slots.main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(states.at(-1)).toEqual({ available: true, open: false, count: 1 });
  toggleEvent();
  expect(panel().hidden).toBe(false);
  expect(states.at(-1)).toEqual({ available: true, open: true, count: 1 });
});

test("nothing shows while no page is open", async () => {
  mount();
  await settle();
  expect(states.at(-1)?.available ?? false).toBe(false);
  expect(panel().hidden).toBe(true);
  altCmdI();
  toggleEvent();
  expect(panel().hidden).toBe(true);
});

test("⌥⌘I toggles it and the choice is remembered", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  altCmdI();
  expect(panel().hidden).toBe(false);
  expect(localStorage.getItem(OPEN_KEY)).toBe("1");
  altCmdI();
  expect(panel().hidden).toBe(true);
  expect(localStorage.getItem(OPEN_KEY)).toBe("0");
});

test("the Page and File sections follow navigation, live reload and drafts", async () => {
  env.ctx.store.set({ path: "README.md" });
  localStorage.setItem(OPEN_KEY, "1");
  mount();
  await settle();
  expect(panel().hidden).toBe(false);
  expect(field("title")).toBe("Home");
  expect(field("path")).toBe("README.md");
  expect(field("size")).toBe("23 B");
  expect(api.callsTo("pageInfo")).toEqual([["README.md"]]);

  env.ctx.navigate("b.md");
  await settle();
  expect(field("title")).toBe("B");
  expect(field("path")).toBe("b.md");
  expect(field("links")).toBe("1 internal · 0 external");
  expect(api.callsTo("pageInfo").at(-1)).toEqual(["b.md"]);

  const calls = api.callsTo("pageInfo").length;
  api.emit("page:changed", { path: "b.md" });
  await settle();
  expect(api.callsTo("pageInfo").length).toBeGreaterThan(calls);

  api.emit("page:draft", { path: "b.md", content: "# Draft title\n\nOne two three four.\n" });
  await settle();
  expect(field("title")).toBe("Draft title");
  expect(field("words")).toBe("6");
});

test("issues live in the panel: listed, selected, and cleared on navigation", async () => {
  const spy = vi.fn();
  (window.HTMLElement.prototype as unknown as { scrollIntoView: unknown }).scrollIntoView = spy;
  env.ctx.store.set({ path: "README.md" });
  localStorage.setItem(OPEN_KEY, "1");
  mount();
  await settle();
  const item = panel().querySelector<HTMLButtonElement>(".zen-lint-item")!;
  expect(item.textContent).toContain("teh");
  vi.useFakeTimers();
  item.click();
  const m = env.slots.main.querySelector("mark.zen-lint")!;
  expect(spy.mock.contexts[0]).toBe(m);
  expect(m.classList.contains("zen-lint-flash")).toBe(true);
  vi.useRealTimers();

  api.lintResult = { available: false, alerts: [] };
  env.ctx.navigate("b.md");
  await settle();
  expect(panel().textContent).toContain("Linting off");
  expect(states.at(-1)?.count).toBe(0);
});

test("a page that fails to load hides the inspector; unmount removes everything", async () => {
  env.ctx.store.set({ path: "missing.md" });
  const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
  mount();
  await settle();
  expect(states.at(-1)?.available).toBe(false);
  unmount!();
  unmount = undefined;
  expect(states.at(-1)).toEqual({ available: false, open: false, count: 0 });
  expect(document.querySelector(".zen-inspector")).toBeNull();
  warn.mockRestore();
});

test("the panel is tabbed and remembers the tab", async () => {
  env.ctx.store.set({ path: "README.md" });
  localStorage.setItem(OPEN_KEY, "1");
  localStorage.setItem(TAB_KEY, "page");
  mount();
  await settle();
  const tabs = Array.from(panel().querySelectorAll('[role="tab"]'));
  expect(tabs.map((t) => t.getAttribute("aria-selected"))).toEqual(["false", "false", "true"]);
});
