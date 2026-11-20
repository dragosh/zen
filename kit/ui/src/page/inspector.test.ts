// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import { alert } from "../lint/test-util";
import { INSPECTOR_STATE, INSPECTOR_TOGGLE, OPEN_KEY, TAB_KEY } from "../inspector";
import { INSPECTOR_MARK, type ProblemTarget } from "../inspector/events";
import { pageFeature } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

let env: Env;
let api: FakeApi;
let unmount: (() => void) | undefined;

const SOURCES = { "README.md": "# Home\n\nSome teh text.\n", "b.md": "# B\n\nA b page with [home](README.md).\n" };
const TEH = alert({ line: 3, col: 6, endCol: 8, match: "teh" });

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
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
  expect(sessionStorage.getItem(OPEN_KEY)).toBe("1");
  altCmdI();
  expect(panel().hidden).toBe(true);
  expect(sessionStorage.getItem(OPEN_KEY)).toBe("0");
});

test("the Page and File sections follow navigation, live reload and drafts", async () => {
  env.ctx.store.set({ path: "README.md" });
  sessionStorage.setItem(OPEN_KEY, "1");
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
  sessionStorage.setItem(OPEN_KEY, "1");
  mount();
  await settle();
  const item = panel().querySelector<HTMLButtonElement>('.zen-problem[data-kind="spelling"]')!;
  expect(item.textContent).toContain("teh");
  item.click(); // one click selects it and scrolls to its mark
  const m = env.slots.main.querySelector("mark.zen-lint")!;
  expect(spy.mock.contexts[0]).toBe(m);

  api.lintResult = { available: false, alerts: [] };
  env.ctx.navigate("b.md");
  await settle();
  expect(panel().textContent).toContain("No problems on this page.");
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
  sessionStorage.setItem(OPEN_KEY, "1");
  localStorage.setItem(TAB_KEY, "problems");
  mount();
  await settle();
  const tabs = Array.from(panel().querySelectorAll('[role="tab"]'));
  expect(tabs.map((t) => t.getAttribute("aria-selected"))).toEqual(["false", "true", "false", "false"]); // Page Info, Problems, Console and Build (hidden while its switch is off)
});

// ---- ui/inspector-polish: warnings go to Problems

const withWarnings = (warnings: string[]) => {
  const real = env.api.page;
  env.api.page = async (p) => ({ ...(await real(p)), warnings });
};
const bar = () => env.slots.main.querySelector<HTMLElement>(".zen-page-warnings:not(.zen-dialect-hint)")!;
const problemRows = () => Array.from(panel().querySelectorAll<HTMLElement>('.zen-problem[data-kind="warning"]')).map((r) => r.textContent);

test("warnings go to Problems: the bar stays empty with the inspector feature, and the count includes them", async () => {
  withWarnings(['unknown layout "slides"', "dialect myst is not in this build"]);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(bar().hidden).toBe(true);
  expect(bar().querySelector(".zen-page-warnings-text")!.textContent).toBe("");
  expect(problemRows()).toHaveLength(2);
  expect(problemRows()[0]).toContain('unknown layout "slides"');
  expect(states.at(-1)?.count).toBe(2 + 1); // two warnings and the spelling alert
});

test("warnings go to Problems: they follow a live reload", async () => {
  withWarnings(["dialect myst is not in this build"]);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(problemRows()).toHaveLength(1);
  withWarnings([]);
  api.emit("page:changed", { path: "README.md" });
  await settle();
  expect(problemRows()).toHaveLength(0);
});

test("warnings go to Problems: a build without the inspector feature keeps the bar", async () => {
  const bare = createFakeApi({ sources: { ...SOURCES }, build: { features: ["reader"] } });
  env = setup(bare);
  withWarnings(['unknown layout "slides"']);
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  expect(bar().hidden).toBe(false);
  expect(bar().textContent).toContain('unknown layout "slides"');
});

// ---- ui/inspector-details: a problem is marked in the page

let registry: Map<string, { ranges: Range[] }>;
let scrolled: { el: unknown; opts: unknown }[] = [];
const stubHighlights = () => {
  registry = new Map();
  vi.stubGlobal("CSS", { highlights: registry });
  vi.stubGlobal("Highlight", class { ranges: Range[]; priority = 0; constructor(...r: Range[]) { this.ranges = r; } });
};
const article = () => document.querySelector<HTMLElement>("article.zen-page")!;
const mark = (t: Partial<ProblemTarget> & Pick<ProblemTarget, "kind">) =>
  document.dispatchEvent(new CustomEvent(INSPECTOR_MARK, { detail: { severity: "warning", ...t } }));
const marked = (name: string) => registry.get(name)?.ranges.map((r) => r.toString());
async function shown(): Promise<void> {
  env.ctx.store.set({ path: "README.md" });
  mount();
  await settle();
  article().insertAdjacentHTML("beforeend", "<p>See <a data-zen-link='broken' href='gone.md'>Gone page</a> and <a data-zen-link='broken' href='gone.md'>Gone page</a> <img src='/_ws/m.png' alt='M'></p>");
  scrolled = [];
  (window.HTMLElement.prototype as unknown as { scrollIntoView: unknown }).scrollIntoView = function (this: unknown, opts: unknown) { scrolled.push({ el: this, opts }); };
}
beforeEach(() => stubHighlights());
afterEach(() => vi.unstubAllGlobals());

test("problem marks: a broken link marks its text with the Vale colour of its severity and scrolls to it", async () => {
  await shown();
  mark({ kind: "link", href: "gone.md", text: "Gone page" });
  expect(marked("zen-problem")).toEqual(["Gone page"]);
  expect(marked("zen-problem-warning")).toEqual(["Gone page"]);
  expect(scrolled).toHaveLength(1);
  expect(scrolled[0].el).toBe(article().querySelector("a[data-zen-link=broken]"));
  expect(scrolled[0].opts).toMatchObject({ behavior: "smooth", block: "center" });
  expect(article().querySelector("mark.zen-problem")).toBeNull(); // the DOM is not changed
});

test("problem marks: the second of two equal links is found by its offset", async () => {
  await shown();
  mark({ kind: "link", href: "gone.md", text: "Gone page", offset: 1 });
  expect(scrolled[0].el).toBe(article().querySelectorAll("a[data-zen-link=broken]")[1]);
});

test("problem marks: a missing image gets the outline, not a text range", async () => {
  await shown();
  mark({ kind: "image", src: "/_ws/m.png", text: "M" });
  const img = article().querySelector("img")!;
  expect(img.getAttribute("data-zen-problem")).toBe("warning");
  expect(registry.size).toBe(0);
  expect(scrolled[0].el).toBe(img);
  mark({ kind: "image", src: "/_ws/other.png" }); // another problem: the first outline is gone
  expect(img.hasAttribute("data-zen-problem")).toBe(false);
});

test("problem marks: a spelling problem marks the word from the Vale mark, in its colour", async () => {
  await shown();
  mark({ kind: "spelling", severity: "error", text: "teh", offset: 0, length: 3 });
  expect(marked("zen-problem")).toEqual(["teh"]);
  expect(marked("zen-problem-error")).toEqual(["teh"]);
  expect(scrolled).toHaveLength(1);
});

test("problem marks: a spelling problem with no Vale mark falls back to the first matching text", async () => {
  await shown();
  mark({ kind: "spelling", severity: "suggestion", text: "Some", offset: 99, length: 4 });
  expect(marked("zen-problem-suggestion")).toEqual(["Some"]);
});

test("problem marks: only one at a time; null, a page warning and an unknown target clear it", async () => {
  await shown();
  mark({ kind: "link", href: "gone.md", text: "Gone page" });
  mark({ kind: "spelling", severity: "error", text: "teh", offset: 0 });
  expect(Array.from(registry.keys()).sort()).toEqual(["zen-problem", "zen-problem-error"]);
  document.dispatchEvent(new CustomEvent(INSPECTOR_MARK, { detail: null }));
  expect(registry.size).toBe(0);
  mark({ kind: "link", href: "gone.md", text: "Gone page" });
  mark({ kind: "warning" });
  expect(registry.size).toBe(0);
  mark({ kind: "link", href: "gone.md", text: "Gone page" });
  mark({ kind: "link", href: "nothing.md", text: "Nope" });
  expect(registry.size).toBe(0);
  expect(scrolled).toHaveLength(4); // the warning and the unknown link scrolled nowhere
});

test("problem marks: the page changing clears it", async () => {
  await shown();
  mark({ kind: "link", href: "gone.md", text: "Gone page" });
  env.ctx.store.set({ path: "b.md" });
  await settle();
  expect(registry.size).toBe(0);
});

test("problem marks: the Inspector closing clears it", async () => {
  await shown();
  toggleEvent();
  mark({ kind: "link", href: "gone.md", text: "Gone page" });
  expect(registry.size).toBeGreaterThan(0);
  toggleEvent();
  expect(registry.size).toBe(0);
});

test("problem marks: the background flash fades, and stays still when Reduce motion is on", async () => {
  await shown();
  vi.useFakeTimers();
  mark({ kind: "link", href: "gone.md", text: "Gone page" });
  expect(registry.has("zen-problem")).toBe(true);
  vi.advanceTimersByTime(1500);
  expect(registry.has("zen-problem")).toBe(false);
  expect(registry.has("zen-problem-warning")).toBe(true); // the underline stays
  document.documentElement.dataset.reduceMotion = "on";
  try {
    mark({ kind: "link", href: "gone.md", text: "Gone page" });
    expect(scrolled.at(-1)!.opts).toMatchObject({ behavior: "auto" });
    vi.advanceTimersByTime(1500);
    expect(registry.has("zen-problem")).toBe(true);
  } finally {
    delete document.documentElement.dataset.reduceMotion;
  }
});

test("problem marks: without the Highlight API the element is still scrolled to and focused", async () => {
  vi.unstubAllGlobals();
  vi.stubGlobal("CSS", {});
  await shown();
  mark({ kind: "link", href: "gone.md", text: "Gone page" });
  const a = article().querySelector("a[data-zen-link=broken]")!;
  expect(scrolled[0].el).toBe(a);
  expect(document.activeElement).toBe(a);
});

test("problem marks: a hostile href or text is compared, never turned into a selector", async () => {
  await shown();
  expect(() => mark({ kind: "link", href: `"]'),x[y='`, text: "\\" })).not.toThrow();
  expect(registry.size).toBe(0);
});

test("problem marks: choosing a problem in the Inspector marks it in the page", async () => {
  await shown();
  toggleEvent();
  panel().querySelector<HTMLButtonElement>('[data-tab="problems"]')!.click();
  const row = Array.from(panel().querySelectorAll<HTMLElement>(".zen-problem")).find((r) => r.dataset.kind === "spelling")!;
  row.click();
  expect(marked("zen-problem")).toEqual(["teh"]);
});
