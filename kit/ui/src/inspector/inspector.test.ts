// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { AddonInfo, Page, PageInfo } from "../host";
import { alert } from "../lint/test-util";
import { createInspector, INSPECTOR_CLOSE, INSPECTOR_STATE, INSPECTOR_TOGGLE, inspectorKey, OPEN_KEY, TAB_KEY, type Inspector } from "./index";

const tick = () => new Promise((r) => setTimeout(r, 0));
const settle = async () => { for (let i = 0; i < 4; i++) await tick(); };

const INFO: PageInfo = { path: "docs/a.md", absPath: "/Users/me/site/docs/a.md", size: 1536, modTime: Date.UTC(2026, 2, 4, 12), birthTime: Date.UTC(2026, 1, 1, 9) };
const page = (p: Partial<Page> = {}): Page => ({
  path: "docs/a.md", title: "Alpha", html: "", frontmatter: { title: "Alpha", tags: [] }, breadcrumb: [], assets: { css: [], js: [] }, warnings: [],
  layout: { name: "article", source: "" }, ...p,
});
const article = (html = "<h1>Alpha</h1><p>one two three <a href='b.md'>b</a> <a href='https://x.org'>x</a></p><img src='a.png' alt=''>") => {
  const a = document.createElement("article");
  a.innerHTML = html;
  return a;
};

let made: Inspector[] = [];
const VALE: AddonInfo = { id: "vale", name: "Vale linting", summary: "Spelling and style checks." };
let host: { pageInfo: ReturnType<typeof vi.fn>; revealInFinder?: ReturnType<typeof vi.fn>; addons?: readonly AddonInfo[] };
const select = vi.fn();

function make(h = host): Inspector {
  const i = createInspector({ doc: document, host: h as never, onSelectIssue: select });
  made.push(i);
  document.body.append(i.button, i.el);
  return i;
}
const field = (i: Inspector, name: string) => i.el.querySelector<HTMLElement>(`[data-field="${name}"]`)?.textContent;
const section = (i: Inspector, id: string) => i.el.querySelector<HTMLElement>(`[data-section="${id}"]`)!;

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  document.body.replaceChildren();
  select.mockReset();
  host = { pageInfo: vi.fn(async () => ({ ...INFO })), revealInFinder: vi.fn(async () => undefined), addons: [VALE] };
});
afterEach(() => { for (const i of made.splice(0)) i.destroy(); vi.restoreAllMocks(); });

test("starts closed; the button toggles the panel and says so", () => {
  const i = make();
  i.setPage("docs/a.md", page(), article());
  expect(i.isOpen).toBe(false);
  expect(i.el.hidden).toBe(true);
  expect(i.button.getAttribute("aria-expanded")).toBe("false");
  expect(i.button.getAttribute("aria-controls")).toBe(i.el.id);
  expect(i.el.getAttribute("aria-label")).toBe("Inspector");
  i.button.click();
  expect(i.isOpen).toBe(true);
  expect(i.el.hidden).toBe(false);
  expect(i.button.getAttribute("aria-expanded")).toBe("true");
  i.button.click();
  expect(i.isOpen).toBe(false);
});

test("closed by default: open lasts only while the app runs, a new launch starts closed", () => {
  localStorage.setItem(OPEN_KEY, "1"); // left by an older version
  expect(make().isOpen).toBe(false);
  const i = make();
  i.setPage("docs/a.md", page(), article());
  i.open();
  expect(sessionStorage.getItem(OPEN_KEY)).toBe("1");
  expect(localStorage.getItem(OPEN_KEY)).toBe("1"); // untouched: the open state is not kept across launches
  sessionStorage.clear(); // a new launch
  expect(make().isOpen).toBe(false);
});

test("open or closed is remembered while the app runs, and broken storage does not break it", () => {
  let i = make();
  i.setPage("docs/a.md", page(), article());
  i.open();
  expect(sessionStorage.getItem(OPEN_KEY)).toBe("1");
  i.destroy();
  i = make();
  expect(i.isOpen).toBe(true);
  i.close();
  expect(sessionStorage.getItem(OPEN_KEY)).toBe("0");

  const get = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("no storage"); });
  const set = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("no storage"); });
  const j = make();
  expect(j.isOpen).toBe(false);
  expect(() => { j.open(); j.close(); j.toggle(); }).not.toThrow();
  expect(j.isOpen).toBe(true);
  get.mockRestore();
  set.mockRestore();
});

test("neither button nor panel shows without a page, or when the host says the page is not shown", () => {
  const i = make();
  i.open();
  expect(i.button.hidden).toBe(true);
  expect(i.el.hidden).toBe(true);
  i.setPage("docs/a.md", page(), article());
  expect(i.button.hidden).toBe(false);
  expect(i.el.hidden).toBe(false);
  i.setActive(false);
  expect(i.button.hidden).toBe(true);
  expect(i.el.hidden).toBe(true);
  i.setActive(true);
  expect(i.el.hidden).toBe(false);
  i.setPage(null);
  expect(i.button.hidden).toBe(true);
  expect(i.el.hidden).toBe(true);
});

test("shortcut predicate: ⌥⌘I only", () => {
  const k = (init: KeyboardEventInit) => inspectorKey(new KeyboardEvent("keydown", init));
  expect(k({ key: "ˆ", code: "KeyI", metaKey: true, altKey: true })).toBe(true);
  expect(k({ key: "i", code: "KeyI", ctrlKey: true, altKey: true })).toBe(true);
  expect(k({ key: "i", code: "KeyI", metaKey: true })).toBe(false);
  expect(k({ key: "i", code: "KeyI", altKey: true })).toBe(false);
  expect(k({ key: "i", code: "KeyI", metaKey: true, altKey: true, shiftKey: true })).toBe(false);
  expect(k({ key: "j", code: "KeyJ", metaKey: true, altKey: true })).toBe(false);
});

test("Esc closes the panel from inside and returns focus to the button; outside it does nothing", () => {
  const i = make();
  i.setPage("docs/a.md", page(), article());
  i.open();
  const inside = i.el.querySelector<HTMLButtonElement>("button")!;
  inside.focus();
  inside.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  expect(i.isOpen).toBe(false);
  expect(document.activeElement).toBe(i.button);

  i.open();
  document.body.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(i.isOpen).toBe(true);
});

const filter = (i: Inspector) => i.el.querySelector<HTMLSelectElement>("select.zen-problems-filter")!;
const tabs = (i: Inspector) => Array.from(i.el.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
const key = (el: Element, k: string) => el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));

test("tabs: Page Info and Problems as a tablist with one tabpanel each; Page Info is first and selected", () => {
  const i = make();
  i.setPage("docs/a.md", page(), article());
  i.open();
  expect(i.el.querySelector('[role="tablist"]')).not.toBeNull();
  expect(tabs(i).map((t) => t.textContent)).toEqual(["Page Info", "Problems"]);
  expect(tabs(i).map((t) => t.dataset.tab)).toEqual(["info", "problems"]);
  expect(tabs(i).map((t) => t.getAttribute("aria-selected"))).toEqual(["true", "false"]);
  expect(tabs(i).map((t) => t.tabIndex)).toEqual([0, -1]);
  const panels = Array.from(i.el.querySelectorAll<HTMLElement>('[role="tabpanel"]')).filter((p) => !p.hidden || p.dataset.section !== "console");
  expect(panels.map((p) => p.hidden)).toEqual([false, true]);
  tabs(i).forEach((t, n) => {
    expect(t.getAttribute("aria-controls")).toBe(panels[n].id);
    expect(panels[n].getAttribute("aria-labelledby")).toBe(t.id);
  });
  tabs(i)[1].click();
  expect(tabs(i)[1].getAttribute("aria-selected")).toBe("true");
  expect(panels.map((p) => p.hidden)).toEqual([true, false]);
});

test("tabs: arrow keys move and wrap, Home and End jump, focus follows", () => {
  const i = make();
  i.setPage("docs/a.md", page(), article());
  i.open();
  const list = i.el.querySelector<HTMLElement>('[role="tablist"]')!;
  const sel = () => tabs(i).findIndex((t) => t.getAttribute("aria-selected") === "true");
  key(list, "ArrowRight");
  expect(sel()).toBe(1);
  expect(document.activeElement).toBe(tabs(i)[1]);
  key(list, "ArrowRight");
  expect(sel()).toBe(0);
  key(list, "ArrowLeft");
  expect(sel()).toBe(1);
  key(list, "Home");
  expect(sel()).toBe(0);
  key(list, "End");
  expect(sel()).toBe(1);
});

test("tabs: the last tab is remembered, and an unknown stored value (an old Linting, File or Page) falls back to Page Info", () => {
  let i = make();
  i.setPage("docs/a.md", page(), article());
  tabs(i)[1].click();
  expect(localStorage.getItem(TAB_KEY)).toBe("problems");
  i.destroy();
  i = make();
  i.setPage("docs/a.md", page(), article());
  expect(tabs(i).map((t) => t.getAttribute("aria-selected"))).toEqual(["false", "true"]);
  i.destroy();
  for (const old of ["nope", "issues", "file", "page"]) {
    localStorage.setItem(TAB_KEY, old);
    i = make();
    i.setPage("docs/a.md", page(), article());
    expect(tabs(i)[0].getAttribute("aria-selected")).toBe("true");
    i.destroy();
  }
});

test("the button is an icon with an accessible name; the badge is red text with the count and absent at 0", () => {
  const i = make();
  i.setPage("docs/a.md", page(), article());
  expect(i.button.querySelector("svg")).not.toBeNull();
  expect(i.button.getAttribute("aria-label")).toBe("Inspector");
  i.setIssues([alert({ line: 1, match: "x" }), alert({ line: 2, match: "x" }), alert({ line: 3, match: "x" })]);
  expect(i.button.getAttribute("aria-label")).toBe("Inspector, 3 issues");
  expect(i.button.querySelector<HTMLElement>(".zen-inspector-badge")!.textContent).toBe("3");
  i.setIssues([]);
  expect(i.button.querySelector<HTMLElement>(".zen-inspector-badge")!.hidden).toBe(true);
  expect(i.button.getAttribute("aria-label")).toBe("Inspector");
});

test("external: no own button; the state goes out as an event and a toggle event drives it", () => {
  const states: unknown[] = [];
  const on = (e: Event) => states.push((e as CustomEvent).detail);
  document.addEventListener(INSPECTOR_STATE, on);
  const i = createInspector({ doc: document, host: host as never, onSelectIssue: select, external: true });
  made.push(i);
  document.body.append(i.el);
  i.setPage("docs/a.md", page(), article());
  i.setIssues([alert({ line: 1, match: "x" }), alert({ line: 2, match: "x" })]);
  expect(states.at(-1)).toEqual({ available: true, open: false, count: 2 });
  document.dispatchEvent(new CustomEvent(INSPECTOR_TOGGLE));
  expect(i.isOpen).toBe(true);
  expect(states.at(-1)).toEqual({ available: true, open: true, count: 2 });
  document.dispatchEvent(new CustomEvent(INSPECTOR_CLOSE)); // the Developer panel opened
  expect(i.isOpen).toBe(false);
  document.dispatchEvent(new CustomEvent(INSPECTOR_TOGGLE));
  expect(i.isOpen).toBe(true);
  i.setActive(false);
  expect(states.at(-1)).toEqual({ available: false, open: true, count: 2 });
  document.dispatchEvent(new CustomEvent(INSPECTOR_TOGGLE));
  expect(i.isOpen).toBe(true); // not available: ignored
  i.destroy();
  expect(states.at(-1)).toEqual({ available: false, open: false, count: 0 });
  document.removeEventListener(INSPECTOR_STATE, on);
});

test("the badge counts the problems and stays with the panel closed", () => {
  const i = make();
  i.setPage("docs/a.md", page(), article("<p><a data-zen-link='broken'>x</a></p>"));
  const badge = () => i.button.querySelector<HTMLElement>(".zen-inspector-badge")!;
  expect(badge().textContent).toBe("1"); // the broken link
  i.setIssues([alert({ line: 3, match: "teh" }), alert({ line: 7, match: "very", severity: "warning" })]);
  expect(i.isOpen).toBe(false);
  expect(badge().hidden).toBe(false);
  expect(badge().textContent).toBe("3");
  expect(i.button.getAttribute("aria-label")).toBe("Inspector, 3 issues");
  i.setIssues([]);
  expect(i.button.getAttribute("aria-label")).toBe("Inspector, 1 issue");
  i.setPage("docs/a.md", page(), article("<p>fine</p>"));
  expect(badge().hidden).toBe(true);
  expect(i.button.getAttribute("aria-label")).toBe("Inspector");
});

test("file: details, formatted, with full path, and created only when known", async () => {
  const i = make();
  i.open();
  i.setPage("docs/a.md", page(), article());
  expect(field(i, "path")).toBe("docs/a.md");
  expect(field(i, "size")).toBe("…");
  await settle();
  expect(host.pageInfo).toHaveBeenCalledWith("docs/a.md");
  expect(field(i, "abs")).toBe("/Users/me/site/docs/a.md");
  expect(field(i, "size")).toBe("1.5 KB");
  expect(field(i, "modified")).toMatch(/2026/);
  expect(field(i, "created")).toMatch(/2026/);

  host.pageInfo.mockResolvedValueOnce({ ...INFO, birthTime: 0 });
  i.setPage("docs/a.md", page(), article());
  await settle();
  expect(i.el.querySelector('[data-field="created"]')).toBeNull();
});

test("file: nothing is fetched while closed; opening fetches; a failure says so", async () => {
  const i = make();
  i.setPage("docs/a.md", page(), article());
  await settle();
  expect(host.pageInfo).not.toHaveBeenCalled();
  host.pageInfo.mockRejectedValueOnce(new Error("gone"));
  i.open();
  await settle();
  expect(host.pageInfo).toHaveBeenCalledTimes(1);
  expect(field(i, "size")).toBe("Not available");
  expect(section(i, "info").querySelector('[data-action="reveal"]')).not.toBeNull();
});

test("file: a stale answer for the previous page is dropped", async () => {
  let release!: (v: PageInfo) => void;
  host.pageInfo.mockImplementationOnce(() => new Promise<PageInfo>((r) => (release = r)));
  const i = make();
  i.open();
  i.setPage("docs/a.md", page(), article());
  host.pageInfo.mockResolvedValueOnce({ ...INFO, path: "b.md", absPath: "/Users/me/site/b.md", size: 10 });
  i.setPage("b.md", page({ path: "b.md", title: "B" }), article());
  await settle();
  release({ ...INFO, size: 999999 });
  await settle();
  expect(field(i, "path")).toBe("b.md");
  expect(field(i, "size")).toBe("10 B");
});

test("copy path copies the full path and says so; reveal calls the host", async () => {
  const writeText = vi.fn(async () => undefined);
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  const i = make();
  i.open();
  i.setPage("docs/a.md", page(), article());
  await settle();
  const copy = i.el.querySelector<HTMLButtonElement>('[data-action="copy"]')!;
  copy.click();
  await settle();
  expect(writeText).toHaveBeenCalledWith("/Users/me/site/docs/a.md");
  expect(copy.textContent).toBe("Copied");
  i.el.querySelector<HTMLButtonElement>('[data-action="reveal"]')!.click();
  expect(host.revealInFinder).toHaveBeenCalledWith("docs/a.md");
});

test("a host without pageInfo or revealInFinder still shows the path and the page", () => {
  const i = make({ pageInfo: undefined as never });
  i.open();
  i.setPage("docs/a.md", page(), article());
  expect(field(i, "path")).toBe("docs/a.md");
  expect(i.el.querySelector('[data-field="size"]')).toBeNull();
  expect(i.el.querySelector('[data-action="copy"]')).toBeNull();
  expect(i.el.querySelector('[data-action="reveal"]')).toBeNull();
  expect(field(i, "title")).toBe("Alpha");

  host.revealInFinder = undefined;
  const j = make();
  j.open();
  j.setPage("docs/a.md", page(), article());
  expect(j.el.querySelector('[data-action="reveal"]')).toBeNull();
});

test("page: title, layout, counts, plugins and front matter", () => {
  const i = make();
  i.open();
  i.setPage("docs/a.md", page({ layout: { name: "slides", source: "builtin" } }),
    article("<h1>Alpha</h1><h2>x</h2><p>one two <a href='b.md'>b</a> <a href='https://x.org'>x</a></p><img src='a.png'><div data-zen-plugin='quiz'></div>"));
  expect(field(i, "title")).toBe("Alpha");
  expect(field(i, "layout")).toBe("slides · built-in");
  expect(field(i, "words")).toBe("6");
  expect(field(i, "reading")).toBe("~1 min");
  expect(field(i, "headings")).toBe("2");
  expect(field(i, "links")).toBe("1 internal · 1 external");
  expect(field(i, "images")).toBe("1");
  expect(field(i, "plugins")).toBe("quiz");
  expect(field(i, "frontmatter")).toBe("title, tags");

  i.setPage("docs/a.md", page({ frontmatter: {} }), article("<p></p>"));
  expect(field(i, "layout")).toBe("article");
  expect(field(i, "reading")).toBe("under a minute");
  expect(field(i, "plugins")).toBe("none");
  expect(field(i, "frontmatter")).toBe("none");
});

test("page data is text, never markup", () => {
  const i = make();
  i.open();
  i.setPage("a<img src=x onerror=alert(1)>.md", page({ title: "<b>x</b>", frontmatter: { "<i>k</i>": 1 } }), article("<div data-zen-plugin='p'></div>"));
  expect(i.el.querySelector("img, b, i")).toBeNull();
  expect(field(i, "title")).toBe("<b>x</b>");
});

test("onOpenChange is told when the panel opens or closes, not when nothing changes", () => {
  const seen: boolean[] = [];
  const i = createInspector({ doc: document, host: host as never, onSelectIssue: select, onOpenChange: (o) => seen.push(o) });
  made.push(i);
  document.body.append(i.button, i.el);
  i.setPage("docs/a.md", page(), article());
  i.open();
  i.open();
  i.toggle();
  i.close();
  expect(seen).toEqual([true, false]);
});

test("without the vale add-on the Problems tab has no Spelling filter, and spelling alerts are ignored", async () => {
  const i = make({ ...host, addons: [] });
  i.setPage("docs/a.md", page(), article());
  i.open();
  expect(tabs(i).map((t) => t.dataset.tab)).toEqual(["info", "problems"]);
  expect(Array.from(filter(i).options).map((o) => o.value)).toEqual(["all", "checks"]);
  i.setIssues([alert({ line: 1, match: "x" })]);
  expect(i.button.querySelector<HTMLElement>(".zen-inspector-badge")!.hidden).toBe(true);
  expect(i.button.getAttribute("aria-label")).toBe("Inspector");
  await settle();
  expect(field(i, "path")).toBe("docs/a.md");
});

test("external without vale: the state carries no count", () => {
  const states: { count: number }[] = [];
  const on = (e: Event) => states.push((e as CustomEvent).detail);
  document.addEventListener(INSPECTOR_STATE, on);
  const i = createInspector({ doc: document, host: { ...host, addons: [] } as never, onSelectIssue: select, external: true });
  made.push(i);
  document.body.append(i.el);
  i.setPage("docs/a.md", page(), article());
  i.setIssues([alert({ line: 1, match: "x" })]);
  expect(states.at(-1)).toMatchObject({ available: true, count: 0 });
  document.removeEventListener(INSPECTOR_STATE, on);
});
