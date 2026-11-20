// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { BuildInfo, KitHost, LogEntry, Page } from "../host";
import { installConsoleCapture, type ConsoleCapture } from "../shell/console";
import { DEV_KEY, setDeveloperEnabled } from "../shell/developer";
import { BOTTOM_PANEL_HEIGHT, PANEL_COLLAPSED_KEY } from "../panel";
import { createInspector, INSPECTOR_CONSOLE, INSPECTOR_STATE, INSPECTOR_TOGGLE, OPEN_KEY, TAB_KEY, type Inspector } from "./index";

const tick = () => new Promise((r) => setTimeout(r, 0));
const settle = async () => { for (let i = 0; i < 4; i++) await tick(); };
const page = (): Page => ({ path: "a.md", title: "Alpha", html: "", frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [], layout: { name: "article", source: "" } });
const entry = (seq: number, source: LogEntry["source"], message: string, level: LogEntry["level"] = "info"): LogEntry => ({ seq, time: Date.UTC(2026, 10, 18, 10, 30, 5, 7) + seq, level, source, message });
const build = (...features: string[]): BuildInfo => ({ app: "t", features, plugins: [], layouts: [], templates: [], dialects: [] } as unknown as BuildInfo);

let made: Inspector[] = [];
let cap: ConsoleCapture;
let push: (e: LogEntry[]) => void = () => {};
const baseHost = (features?: string[], logs: LogEntry[] = [entry(1, "app", "started"), entry(2, "plugin", "plugin failed", "error")]): KitHost =>
  ({
    pageInfo: vi.fn(async () => ({ path: "a.md", absPath: "/x/a.md", size: 1, modTime: 1, birthTime: 0 })),
    logs: async () => logs,
    onLog: (fn: (e: LogEntry[]) => void) => { push = fn; return () => { push = () => {}; }; },
    ...(features ? { build: build(...features) } : {}),
  }) as unknown as KitHost;

function make(host: KitHost, withPage = true, external = false): Inspector {
  const i = createInspector({ doc: document, host, onSelectIssue: () => {}, external });
  made.push(i);
  document.body.append(i.el, i.button);
  if (withPage) {
    const a = document.createElement("article");
    a.innerHTML = "<p>hello</p>";
    i.setPage("a.md", page(), a);
  }
  return i;
}
const tabIds = (i: Inspector) => Array.from(i.el.querySelectorAll<HTMLElement>('[role="tab"]')).filter((t) => !t.hidden).map((t) => t.dataset.tab);
const allTabIds = (i: Inspector) => Array.from(i.el.querySelectorAll<HTMLElement>('[role="tab"]')).map((t) => t.dataset.tab);
const jKey = (init: KeyboardEventInit = {}) => document.dispatchEvent(new KeyboardEvent("keydown", { code: "KeyJ", key: "∆", altKey: true, metaKey: true, bubbles: true, cancelable: true, ...init }));
const rows = (i: Inspector) => Array.from(i.el.querySelectorAll<HTMLElement>(".zen-dev-row")).map((r) => r.textContent);
const consoleSource = (i: Inspector) => i.el.querySelector<HTMLSelectElement>("select.zen-console-source")!;

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  document.body.replaceChildren();
  vi.spyOn(console, "log").mockImplementation(() => {});
  cap = installConsoleCapture(window, 50);
});
afterEach(() => { for (const i of made.splice(0)) i.destroy(); cap.uninstall(); vi.restoreAllMocks(); });

// ---- Console: only with the build's developer feature and the Settings switch

test("the Console tab shows only while the switch is on, live, and the key of the switch is unchanged", () => {
  expect(DEV_KEY).toBe("zen.developer");
  const i = make(baseHost(["inspector", "developer"]));
  expect(tabIds(i)).toEqual(["info", "problems"]);
  setDeveloperEnabled(true);
  expect(tabIds(i)).toEqual(["info", "problems", "console"]);
  setDeveloperEnabled(false);
  expect(tabIds(i)).toEqual(["info", "problems"]);
});

test("a build without the developer feature has no Console tab at all, switch or not", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector"]));
  expect(allTabIds(i)).toEqual(["info", "problems"]);
  expect(i.el.querySelector('[data-section="console"]')).toBeNull();
});

test("the switch turned off while Console is selected moves to another tab", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector", "developer"]));
  i.showConsole();
  expect(i.el.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')!.dataset.tab).toBe("console");
  setDeveloperEnabled(false);
  expect(i.el.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')!.dataset.tab).toBe("info");
  expect(i.isOpen).toBe(true);
});

test("Console: the log on the left, the entry on the right, the filters in the header", async () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector", "developer"]));
  i.showConsole();
  await settle();
  const body = i.el.querySelector<HTMLElement>('[data-section="console"]')!;
  expect(body.querySelector(".zen-panel-col-main .zen-dev-list")).not.toBeNull();
  expect(body.querySelector(".zen-panel-col-side .zen-console-details")).not.toBeNull();
  expect(rows(i)).toHaveLength(2);
  expect(Array.from(consoleSource(i).options).map((o) => o.textContent)).toEqual(["All", "App", "Plugins", "Page"]);
  expect(i.el.querySelector(".zen-panel-head")!.contains(consoleSource(i))).toBe(true);
  expect(i.el.querySelector(".zen-panel-head .zen-dev-level-filter")).not.toBeNull();
  expect(i.el.querySelector(".zen-panel-head .zen-dev-text-filter")).not.toBeNull();
  expect(i.el.querySelector(".zen-panel-head .zen-dev-clear")).not.toBeNull();
  expect(i.el.querySelector(".zen-panel-head .zen-dev-copy")).not.toBeNull();
  consoleSource(i).value = "plugin";
  consoleSource(i).dispatchEvent(new Event("change"));
  expect(rows(i)).toHaveLength(1);
  i.el.querySelector<HTMLElement>(".zen-dev-row")!.click();
  expect(i.el.querySelector('.zen-console-details [data-field="message"]')!.textContent).toBe("plugin failed");
  push([entry(3, "app", "later")]);
  expect(rows(i)).toHaveLength(1); // filtered out
  i.el.querySelector<HTMLButtonElement>(".zen-dev-clear")!.click();
  expect(rows(i)).toHaveLength(0);
});

test("the Console's header controls are shown only on the Console tab", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector", "developer"]));
  const group = consoleSource(i).closest<HTMLElement>(".zen-panel-tools-group")!;
  expect(group.hidden).toBe(true);
  i.showConsole();
  expect(group.hidden).toBe(false);
});

// ---- ⌥⌘J and the Develop menu

test("⌥⌘J opens the Inspector on the Console tab and focuses it; again, it closes", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector", "developer"]), true, true);
  expect(i.isOpen).toBe(false);
  jKey();
  expect(i.isOpen).toBe(true);
  const tab = i.el.querySelector<HTMLElement>('[role="tab"][data-tab="console"]')!;
  expect(tab.getAttribute("aria-selected")).toBe("true");
  expect(i.el.contains(document.activeElement)).toBe(true);
  expect(localStorage.getItem(TAB_KEY)).toBe("console");
  jKey();
  expect(i.isOpen).toBe(false);
});

test("⌥⌘J from another tab or a collapsed panel brings the Console up", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector", "developer"]), true, true);
  i.open();
  i.setCollapsed(true);
  jKey();
  expect(i.isOpen).toBe(true);
  expect(i.collapsed).toBe(false);
  expect(i.el.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')!.dataset.tab).toBe("console");
});

test("the Develop menu's event does the same, and only opens", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector", "developer"]), true, true);
  document.dispatchEvent(new CustomEvent(INSPECTOR_CONSOLE));
  expect(i.isOpen).toBe(true);
  document.dispatchEvent(new CustomEvent(INSPECTOR_CONSOLE));
  expect(i.isOpen).toBe(true);
});

test("with the switch off ⌥⌘J does nothing, as before", () => {
  const i = make(baseHost(["inspector", "developer"]), true, true);
  const ev = new KeyboardEvent("keydown", { code: "KeyJ", key: "∆", altKey: true, metaKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(ev);
  expect(i.isOpen).toBe(false);
  expect(ev.defaultPrevented).toBe(false);
  document.dispatchEvent(new CustomEvent(INSPECTOR_CONSOLE));
  expect(i.isOpen).toBe(false);
});

test("⌘J alone or ⇧⌥⌘J does nothing", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector", "developer"]), true, true);
  jKey({ altKey: false });
  jKey({ shiftKey: true });
  expect(i.isOpen).toBe(false);
});

// ---- features

test("an inspector-only build: Page Info and Problems, no Console, and ⌥⌘J does nothing", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector"]), true, true);
  expect(tabIds(i)).toEqual(["info", "problems"]);
  jKey();
  expect(i.isOpen).toBe(false);
});

test("a developer-only build has a panel with the Console tab alone, toggled by ⌥⌘J, with no Inspector button", async () => {
  setDeveloperEnabled(true);
  const states: { available: boolean }[] = [];
  const on = (e: Event) => states.push((e as CustomEvent).detail);
  document.addEventListener(INSPECTOR_STATE, on);
  const i = make(baseHost(["developer"]), false, true);
  expect(allTabIds(i)).toEqual(["console"]);
  expect(i.el.querySelector('[data-section="info"], [data-section="problems"]')).toBeNull();
  expect(i.available).toBe(false); // the Inspector button (⌥⌘I) has nothing to inspect
  expect(i.button.hidden).toBe(true);
  jKey();
  expect(i.isOpen).toBe(true);
  expect(i.el.hidden).toBe(false);
  await settle();
  expect(rows(i)).toHaveLength(2);
  document.dispatchEvent(new CustomEvent(INSPECTOR_TOGGLE)); // the button's event: nothing to toggle
  expect(i.isOpen).toBe(true);
  expect(states.every((s) => !s.available)).toBe(true);
  jKey();
  expect(i.isOpen).toBe(false);
  expect(i.el.hidden).toBe(true);
  document.removeEventListener(INSPECTOR_STATE, on);
});

test("a developer-only build with the switch off has no visible panel, and an open state left behind does not reopen it", () => {
  sessionStorage.setItem(OPEN_KEY, "1");
  const i = make(baseHost(["developer"]), false, true);
  expect(i.isOpen).toBe(false);
  expect(i.el.hidden).toBe(true);
  jKey();
  expect(i.isOpen).toBe(false);
  setDeveloperEnabled(true);
  expect(i.isOpen).toBe(false);
  expect(i.el.hidden).toBe(true);
});

test("turning the switch off in a developer-only build closes the panel", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["developer"]), false, true);
  jKey();
  expect(i.isOpen).toBe(true);
  setDeveloperEnabled(false);
  expect(i.isOpen).toBe(false);
  expect(i.el.hidden).toBe(true);
});

test("a build with both, with a remembered open panel: it reopens with the tabs it has", () => {
  sessionStorage.setItem(OPEN_KEY, "1");
  const i = make(baseHost(["inspector", "developer"]));
  expect(i.isOpen).toBe(true);
  expect(tabIds(i)).toEqual(["info", "problems"]);
});

test("the panel is shown with only the Console while no page is open", () => {
  setDeveloperEnabled(true);
  const i = make(baseHost(["inspector", "developer"]), false, true);
  expect(tabIds(i)).toEqual(["console"]);
  jKey();
  expect(i.el.hidden).toBe(false);
  expect(i.available).toBe(false);
  const a = document.createElement("article");
  i.setPage("a.md", page(), a);
  expect(tabIds(i)).toEqual(["info", "problems", "console"]);
});

// ---- the panel itself: columns, collapse, height

test("Page Info has two columns: the page on the left, the file on the right", async () => {
  const i = make(baseHost(["inspector"]));
  i.open();
  await settle();
  const body = i.el.querySelector<HTMLElement>('[data-section="info"]')!;
  expect(body.querySelector('.zen-panel-col-main [data-field="title"]')).not.toBeNull();
  expect(body.querySelector('.zen-panel-col-main [data-field="words"]')).not.toBeNull();
  expect(body.querySelector('.zen-panel-col-side [data-field="path"]')).not.toBeNull();
  expect(body.querySelector('.zen-panel-col-side [data-field="size"]')).not.toBeNull();
  expect(body.querySelector('.zen-panel-col-main [data-field="size"]')).toBeNull();
});

test("collapse: the chevron folds the panel to its tab bar and back; the state is remembered per machine", () => {
  const i = make(baseHost(["inspector"]));
  i.open();
  const chevron = () => i.el.querySelector<HTMLButtonElement>(".zen-panel-collapse")!;
  expect(i.collapsed).toBe(false);
  chevron().click();
  expect(i.collapsed).toBe(true);
  expect(i.isOpen).toBe(true);
  expect(i.el.hidden).toBe(false);
  expect(i.el.querySelector('[role="tablist"]')).not.toBeNull();
  expect(Array.from(i.el.querySelectorAll<HTMLElement>('[role="tabpanel"]')).every((p) => p.hidden)).toBe(true);
  expect(localStorage.getItem(PANEL_COLLAPSED_KEY)).toBe("1");
  i.destroy();
  const j = make(baseHost(["inspector"]));
  expect(j.collapsed).toBe(true);
  j.el.querySelector<HTMLButtonElement>(".zen-panel-collapse")!.click();
  expect(j.collapsed).toBe(false);
  expect(localStorage.getItem(PANEL_COLLAPSED_KEY)).toBe("0");
});

test("the panel is resizable and the height is remembered", () => {
  const i = make(baseHost(["inspector"]));
  i.open();
  expect(i.el.style.height).toBe(`${BOTTOM_PANEL_HEIGHT.initial}px`);
  const handle = i.el.querySelector<HTMLElement>(".zen-panel-resizer")!;
  handle.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp", bubbles: true, cancelable: true }));
  expect(localStorage.getItem(BOTTOM_PANEL_HEIGHT.key)).toBe(String(BOTTOM_PANEL_HEIGHT.initial + 16));
});

test("the header holds the tabs, then the tab's tools, the chevron and ✕", () => {
  const i = make(baseHost(["inspector", "developer"]), true, true);
  const head = i.el.querySelector(".zen-panel-head")!;
  expect(Array.from(head.children).map((c) => c.className.split(" ")[0])).toEqual(["zen-panel-tabs", "zen-panel-tools", "zen-panel-collapse", "zen-panel-close"]);
});
