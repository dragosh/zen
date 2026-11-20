// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { LogEntry } from "../host";
import { installConsoleCapture, type ConsoleCapture } from "./console";
import { createConsoleTab, formatEntry, MAX_ENTRIES, type ConsoleTab } from "./devpanel";

let n = 0;
const entry = (level: LogEntry["level"], source: LogEntry["source"], message: string, seq = ++n): LogEntry => ({ seq, time: Date.UTC(2026, 10, 18, 10, 30, 5, 7) + seq, level, source, message });

let cap: ConsoleCapture;
let panel: ConsoleTab;
let tools: HTMLElement;
let push: (e: LogEntry[]) => void;
beforeEach(() => {
  n = 0;
  document.body.replaceChildren();
  vi.spyOn(console, "log").mockImplementation(() => {}); // before the capture wraps it
  cap = installConsoleCapture(window, 50);
});
afterEach(() => { panel?.destroy(); cap.uninstall(); vi.restoreAllMocks(); });

async function open(initial: LogEntry[] = [], logs = true): Promise<void> {
  tools = document.createElement("div");
  panel = createConsoleTab({
    doc: document, capture: cap, tools,
    host: logs ? { logs: async () => initial, onLog: (fn) => { push = fn; return () => { push = () => {}; }; } } : {},
  });
  document.body.append(tools, panel.el);
  await panel.start();
}
const rows = () => Array.from(panel.el.querySelectorAll<HTMLElement>(".zen-dev-row")).map((r) => r.textContent);
/** The source dropdown: All / App / Plugins / Page. */
const source = () => tools.querySelector<HTMLSelectElement>("select.zen-console-source")!;
const pick = (id: string) => { source().value = id; source().dispatchEvent(new Event("change")); };
const field = (name: string) => panel.el.querySelector<HTMLElement>(`.zen-console-details [data-field="${name}"]`)?.textContent;

test("shows time, level, source and message for each entry", async () => {
  await open([entry("warn", "plugin", "calc failed")]);
  const row = panel.el.querySelector<HTMLElement>(".zen-dev-row")!;
  expect(row.dataset.level).toBe("warn");
  expect(row.dataset.source).toBe("plugin");
  expect(row.querySelector(".zen-dev-time")!.textContent).toMatch(/^\d\d:\d\d:\d\d\.\d{3}$/);
  expect(row.querySelector(".zen-dev-level")!.textContent).toBe("warn");
  expect(row.querySelector(".zen-dev-source")!.textContent).toBe("plugin");
  expect(row.querySelector(".zen-dev-msg")!.textContent).toBe("calc failed");
});

test("messages are text, never markup", async () => {
  await open([entry("info", "app", "<img src=x onerror=alert(1)>")]);
  expect(panel.el.querySelector(".zen-dev-row img")).toBeNull();
  expect(rows()[0]).toContain("<img");
});

test("empty state", async () => {
  await open();
  expect(panel.el.querySelector(".zen-dev-empty")!.textContent).toBe("No log entries yet.");
  push([entry("info", "app", "x")]);
  expect(panel.el.querySelector(".zen-dev-empty")).toBeNull();
});

test("the source dropdown filters: All, App, Plugins, Page (the page's own console)", async () => {
  await open([entry("info", "app", "a1"), entry("warn", "plugin", "p1")]);
  console.log("c1");
  expect(Array.from(source().options).map((o) => o.textContent)).toEqual(["All", "App", "Plugins", "Page"]);
  expect(Array.from(source().options).map((o) => o.value)).toEqual(["all", "app", "plugin", "console"]);
  expect(rows()).toHaveLength(3);
  pick("app");
  expect(rows().map((r) => r!.includes("a1"))).toEqual([true]);
  pick("plugin");
  expect(rows()).toHaveLength(1);
  expect(rows()[0]).toContain("p1");
  pick("console");
  expect(rows()[0]).toContain("c1");
  expect(panel.el.querySelector("[role=tab]")).toBeNull(); // tabs are the Inspector's, not the log's
});

test("two columns: the log on the left, the selected entry in full on the right", async () => {
  await open([entry("error", "plugin", "line one\nline two <b>x</b>")]);
  expect(panel.el.querySelector(".zen-panel-col-main .zen-dev-list")).not.toBeNull();
  expect(panel.el.querySelector(".zen-panel-col-side .zen-console-details")).not.toBeNull();
  expect(panel.el.querySelector(".zen-console-details")!.textContent).toContain("Select an entry");
  const row = panel.el.querySelector<HTMLElement>(".zen-dev-row")!;
  row.click();
  expect(row.getAttribute("aria-selected")).toBe("true");
  expect(field("time")).toMatch(/^\d\d:\d\d:\d\d\.\d{3}$/);
  expect(field("source")).toBe("plugin");
  expect(field("level")).toBe("error");
  expect(field("message")).toBe("line one\nline two <b>x</b>"); // the whole message, as text
  expect(panel.el.querySelector(".zen-console-details b")).toBeNull();
  // the selection survives new entries, and goes when its entry is cleared
  push([entry("info", "app", "later")]);
  expect(field("message")).toContain("line one");
  tools.querySelector<HTMLButtonElement>("button.zen-dev-clear")!.click();
  expect(panel.el.querySelector(".zen-console-details")!.textContent).toContain("Select an entry");
});

test("level filter is a minimum; text filter is a case-insensitive substring", async () => {
  await open([entry("debug", "app", "d"), entry("info", "app", "Hello World"), entry("warn", "app", "w"), entry("error", "app", "e")]);
  const level = tools.querySelector<HTMLSelectElement>("select.zen-dev-level-filter")!;
  level.value = "warn";
  level.dispatchEvent(new Event("change"));
  expect(rows()).toHaveLength(2);
  level.value = "debug";
  level.dispatchEvent(new Event("change"));
  const text = tools.querySelector<HTMLInputElement>("input.zen-dev-text-filter")!;
  text.value = "hello";
  text.dispatchEvent(new Event("input"));
  expect(rows()).toHaveLength(1);
  expect(rows()[0]).toContain("Hello World");
});

test("new entries arrive through onLog and are de-duplicated by seq", async () => {
  await open([entry("info", "app", "one")]);
  push([entry("info", "app", "one", 1), entry("info", "app", "two", 2)]);
  expect(rows()).toHaveLength(2);
  push([entry("info", "app", "late duplicate", 2)]);
  expect(rows()).toHaveLength(2);
});

test("Clear empties the list; Copy all copies the visible entries as lines", async () => {
  await open([entry("info", "app", "keep"), entry("error", "plugin", "bad")]);
  const writeText = vi.fn(async () => {});
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  pick("plugin");
  tools.querySelector<HTMLButtonElement>("button.zen-dev-copy")!.click();
  await vi.waitFor(() => expect(writeText).toHaveBeenCalled());
  const text = (writeText.mock.calls[0] as unknown as [string])[0];
  expect(text.split("\n")).toHaveLength(1);
  expect(text).toMatch(/ ERROR \[plugin\] bad$/);
  pick("all");
  tools.querySelector<HTMLButtonElement>("button.zen-dev-clear")!.click();
  expect(rows()).toHaveLength(0);
  expect(panel.el.querySelector(".zen-dev-empty")).not.toBeNull();
});

test("formatEntry is one line: time level [source] message", () => {
  expect(formatEntry(entry("warn", "app", "m\nx"))).toMatch(/^\d{4}-\d\d-\d\dT[\d:.]+Z? WARN \[app\] m\nx$/);
});

test("keeps at most MAX_ENTRIES (2000), dropping the oldest", async () => {
  expect(MAX_ENTRIES).toBe(2000);
  await open();
  const batch: LogEntry[] = [];
  for (let i = 0; i < 2100; i++) batch.push(entry("info", "app", `m${i}`));
  push(batch);
  expect(rows()).toHaveLength(2000);
  expect(rows()[0]).toContain("m100");
  expect(panel.count()).toBe(2000);
});

function scrollState(list: HTMLElement, top: number, height: number, client: number): void {
  Object.defineProperty(list, "scrollHeight", { value: height, configurable: true });
  Object.defineProperty(list, "clientHeight", { value: client, configurable: true });
  list.scrollTop = top;
  list.dispatchEvent(new Event("scroll"));
}

test("follows the newest entry until the user scrolls up; Jump to latest resumes", async () => {
  await open([entry("info", "app", "a")]);
  const list = panel.el.querySelector<HTMLElement>(".zen-dev-list")!;
  const jump = panel.el.querySelector<HTMLButtonElement>("button.zen-dev-jump")!;
  expect(jump.hidden).toBe(true);
  scrollState(list, 0, 1000, 200); // scrolled to the top: not at the bottom
  expect(jump.hidden).toBe(false);
  list.scrollTop = 0;
  push([entry("info", "app", "b")]);
  expect(list.scrollTop).toBe(0); // paused
  jump.click();
  expect(jump.hidden).toBe(true);
  expect(list.scrollTop).toBe(1000);
  scrollState(list, 800, 1000, 200); // at the bottom again
  push([entry("info", "app", "c")]);
  expect(list.scrollTop).toBe(1000);
});

test("without host.logs only the page's console is there", async () => {
  await open([], false);
  expect(Array.from(source().options).map((o) => o.value)).toEqual(["console"]);
  console.log("only me");
  expect(rows()).toHaveLength(1);
});

test("the tools are a source dropdown, the level and text filters, then Clear and Copy", async () => {
  await open();
  expect(Array.from(tools.children).map((c) => c.className.split(" ")[0])).toEqual(["zen-console-source", "zen-dev-level-filter", "zen-dev-text-filter", "zen-dev-clear", "zen-dev-copy", "zen-dev-status"]);
});
