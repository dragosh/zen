// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import type { LintResult } from "../api";
import { alert } from "../lint/test-util";
import { pageFeature } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

let env: Env;
let api: FakeApi;
let unmount: (() => void) | undefined;

const SOURCES = { "README.md": "# Home\n\nSome teh text.\n", "b.md": "# B\n\nA teh b.\n" };
const TEH = alert({ line: 3, col: 6, endCol: 8, match: "teh" });

beforeEach(() => {
  document.head.replaceChildren();
  document.body.replaceChildren();
  api = createFakeApi({ sources: { ...SOURCES }, lint: { available: true, alerts: [TEH] } });
  env = setup(api);
});
afterEach(() => { unmount?.(); unmount = undefined; vi.useRealTimers(); });

const mount = () => { unmount = pageFeature.mount(env.slots, env.ctx); };
const article = () => env.slots.main.querySelector("article.zen-page") as HTMLElement;
const countButton = () => env.slots.actions.querySelector<HTMLButtonElement>(".zen-lint-count")!;
const settle = async () => { for (let i = 0; i < 5; i++) await tick(); };

test("lints the page from disk after it renders and marks the article", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(api.callsTo("lint")).toEqual([["README.md", ""]]);
  const m = article().querySelector("mark.zen-lint")!;
  expect(m.textContent).toBe("teh");
  expect(countButton().hidden).toBe(false);
  expect(countButton().textContent).toBe("1 issue");
});

test("shows nothing when Vale is unavailable or the lint call fails", async () => {
  api.lintResult = { available: false, alerts: [TEH] };
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(article().querySelector("mark")).toBeNull();
  expect(countButton().hidden).toBe(true);

  api.lintResult = { available: true, alerts: [TEH] };
  api.failNext("lint", new Error("boom"));
  const err = vi.spyOn(console, "warn").mockImplementation(() => {});
  api.emit("page:changed", { path: "README.md" });
  await settle();
  expect(countButton().hidden).toBe(true);
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
  expect(countButton().textContent).toBe("2 issues");
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
  expect(countButton().textContent).toBe("1 issue");
  expect(article().querySelector("h1")!.textContent).toBe("B");
});

test("clicking an issue scrolls to its mark and highlights it briefly", async () => {
  const spy = vi.fn();
  (window.HTMLElement.prototype as unknown as { scrollIntoView: unknown }).scrollIntoView = spy;
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  vi.useFakeTimers();
  countButton().click();
  const item = env.slots.actions.querySelector<HTMLButtonElement>(".zen-lint-panel li button")!;
  item.click();
  const m = article().querySelector("mark.zen-lint")!;
  expect(spy).toHaveBeenCalled();
  expect(spy.mock.contexts[0]).toBe(m);
  expect(m.classList.contains("zen-lint-flash")).toBe(true);
  vi.advanceTimersByTime(2000);
  expect(m.classList.contains("zen-lint-flash")).toBe(false);
});

test("unmount removes the count button", async () => {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  unmount!();
  unmount = undefined;
  expect(env.slots.actions.querySelector(".zen-lint")).toBeNull();
});
