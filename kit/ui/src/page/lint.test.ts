// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { INSPECTOR_STATE, INSPECTOR_TOGGLE } from "../inspector";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import type { LintResult } from "../host";
import { alert } from "../lint/test-util";
import { pageFeature } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

let env: Env;
let api: FakeApi;
let unmount: (() => void) | undefined;

const SOURCES = { "README.md": "# Home\n\nSome teh text.\n", "b.md": "# B\n\nA teh b.\n" };
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
const article = () => env.slots.main.querySelector("article.zen-page") as HTMLElement;
// The issue count is the badge on the Inspector button (ui/inspector); the list is in the panel.
// It reaches the sidebar's button as an event; `badge` mirrors what that button shows (hidden at 0).
let last = { available: false, open: false, count: 0 };
const onState = (e: Event) => { last = (e as CustomEvent).detail; };
beforeEach(() => { last = { available: false, open: false, count: 0 }; document.addEventListener(INSPECTOR_STATE, onState); });
afterEach(() => document.removeEventListener(INSPECTOR_STATE, onState));
const badge = () => ({ hidden: last.count === 0, textContent: String(last.count) });
const settle = async () => { for (let i = 0; i < 5; i++) await tick(); };

test("lints the page from disk after it renders and marks the article", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(api.callsTo("lint")).toEqual([["README.md", ""]]);
  const m = article().querySelector("mark.zen-lint")!;
  expect(m.textContent).toBe("teh");
  expect(badge().hidden).toBe(false);
  expect(badge().textContent).toBe("1");
});

test("shows nothing when Vale is unavailable or the lint call fails", async () => {
  api.lintResult = { available: false, alerts: [TEH] };
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(article().querySelector("mark")).toBeNull();
  expect(badge().hidden).toBe(true);

  api.lintResult = { available: true, alerts: [TEH] };
  api.failNext("lint", new Error("boom"));
  const err = vi.spyOn(console, "warn").mockImplementation(() => {});
  api.emit("page:changed", { path: "README.md" });
  await settle();
  expect(badge().hidden).toBe(true);
  err.mockRestore();
});

test("page:changed re-lints the open page", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  api.lintResult = { available: true, alerts: [TEH, alert({ line: 3, col: 1, endCol: 4, match: "Some", severity: "warning" })] };
  api.emit("page:changed", { path: "README.md" });
  await settle();
  expect(api.callsTo("lint")).toHaveLength(2);
  expect(article().querySelectorAll("mark.zen-lint")).toHaveLength(2);
  expect(badge().textContent).toBe("2");
});

test("a late result for the previous page is dropped", async () => {
  let release!: (r: LintResult) => void;
  const late = new Promise<LintResult>((r) => (release = r));
  const real = api.lint.bind(api);
  let first = true;
  api.lint = (p, c) => {
    if (first) { first = false; return late; }
    return real(p, c);
  };
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  env.ctx.navigate("b.md");
  await settle();
  release({ available: true, alerts: [TEH, TEH, TEH] });
  await settle();
  expect(badge().textContent).toBe("1");
  expect(article().querySelector("h1")!.textContent).toBe("B");
});

test("clicking an issue scrolls to its mark and highlights it briefly", async () => {
  const spy = vi.fn();
  (window.HTMLElement.prototype as unknown as { scrollIntoView: unknown }).scrollIntoView = spy;
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  vi.useFakeTimers();
  document.dispatchEvent(new CustomEvent(INSPECTOR_TOGGLE));
  const item = document.querySelector<HTMLButtonElement>(".zen-inspector .zen-lint-item")!;
  item.click();
  const m = article().querySelector("mark.zen-lint")!;
  expect(spy).toHaveBeenCalled();
  expect(spy.mock.contexts[0]).toBe(m);
  expect(m.classList.contains("zen-lint-flash")).toBe(true);
  vi.advanceTimersByTime(2000);
  expect(m.classList.contains("zen-lint-flash")).toBe(false);
});

test("unmount tells the sidebar button the inspector is gone", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  unmount!();
  unmount = undefined;
  expect(last.available).toBe(false);
});

test("follows an editor's unsaved draft live, keeping scroll, then returns to disk", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  env.slots.main.scrollTop = 40;
  api.emit("page:draft", { path: "README.md", content: "# Draft\n\nNew teh words.\n" });
  await settle();
  expect(api.callsTo("preview").at(-1)).toEqual(["README.md", "# Draft\n\nNew teh words.\n"]);
  expect(article().querySelector("h1")!.textContent).toBe("Draft");
  expect(api.callsTo("lint").at(-1)).toEqual(["README.md", "# Draft\n\nNew teh words.\n"]);
  expect(env.slots.main.scrollTop).toBe(40);

  // While a draft is shown, a disk change does not overwrite it.
  const pages = api.callsTo("page").length;
  api.emit("page:changed", { path: "README.md" });
  await settle();
  expect(api.callsTo("page").length).toBe(pages);

  api.emit("page:draft", { path: "README.md", content: null });
  await settle();
  expect(api.callsTo("page").length).toBe(pages + 1);
  expect(article().querySelector("h1")!.textContent).toBe("Home");
});

test("ignores drafts for other pages", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  api.emit("page:draft", { path: "b.md", content: "# Other" });
  await settle();
  expect(api.callsTo("preview")).toEqual([]);
  expect(article().querySelector("h1")!.textContent).toBe("Home");
});
