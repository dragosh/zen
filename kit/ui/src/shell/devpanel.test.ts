// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { LogEntry } from "../host";
import { installConsoleCapture, type ConsoleCapture } from "./console";
import { createDevPanel, formatEntry, MAX_ENTRIES, type DevPanel } from "./devpanel";

let n = 0;
const entry = (level: LogEntry["level"], source: LogEntry["source"], message: string, seq = ++n): LogEntry => ({ seq, time: Date.UTC(2026, 10, 18, 10, 30, 5, 7) + seq, level, source, message });

let cap: ConsoleCapture;
let panel: DevPanel;
let push: (e: LogEntry[]) => void;
beforeEach(() => {
  n = 0;
  document.body.replaceChildren();
  vi.spyOn(console, "log").mockImplementation(() => {}); // before the capture wraps it
  cap = installConsoleCapture(window, 50);
});
afterEach(() => { panel?.destroy(); cap.uninstall(); vi.restoreAllMocks(); });

async function open(initial: LogEntry[] = [], logs = true, onClose = () => {}): Promise<void> {
  panel = createDevPanel({
    doc: document, capture: cap, onClose,
    host: logs ? { logs: async () => initial, onLog: (fn) => { push = fn; return () => { push = () => {}; }; } } : {},
  });
  document.body.append(panel.el);
  await panel.start();
}
const rows = () => Array.from(panel.el.querySelectorAll<HTMLElement>(".zen-dev-row")).map((r) => r.textContent);
const tab = (id: string) => panel.el.querySelector<HTMLButtonElement>(`[role=tab][data-tab="${id}"]`)!;

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

test("tabs filter by source: All, App, Plugins, Console", async () => {
  await open([entry("info", "app", "a1"), entry("warn", "plugin", "p1")]);
  console.log("c1");
  expect(Array.from(panel.el.querySelectorAll("[role=tab]")).map((t) => t.textContent)).toEqual(["All", "App", "Plugins", "Console"]);
  expect(rows()).toHaveLength(3);
  tab("app").click();
  expect(rows().map((r) => r!.includes("a1"))).toEqual([true]);
  tab("plugin").click();
  expect(rows()).toHaveLength(1);
  expect(rows()[0]).toContain("p1");
  tab("console").click();
  expect(rows()[0]).toContain("c1");
  expect(tab("console").getAttribute("aria-selected")).toBe("true");
});

test("level filter is a minimum; text filter is a case-insensitive substring", async () => {
  await open([entry("debug", "app", "d"), entry("info", "app", "Hello World"), entry("warn", "app", "w"), entry("error", "app", "e")]);
  const level = panel.el.querySelector<HTMLSelectElement>("select.zen-dev-level-filter")!;
  level.value = "warn";
  level.dispatchEvent(new Event("change"));
  expect(rows()).toHaveLength(2);
  level.value = "debug";
  level.dispatchEvent(new Event("change"));
  const text = panel.el.querySelector<HTMLInputElement>("input.zen-dev-text-filter")!;
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
  tab("plugin").click();
  panel.el.querySelector<HTMLButtonElement>("button.zen-dev-copy")!.click();
  await vi.waitFor(() => expect(writeText).toHaveBeenCalled());
  const text = (writeText.mock.calls[0] as unknown as [string])[0];
  expect(text.split("\n")).toHaveLength(1);
  expect(text).toMatch(/ ERROR \[plugin\] bad$/);
  tab("all").click();
  panel.el.querySelector<HTMLButtonElement>("button.zen-dev-clear")!.click();
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

test("without host.logs only the Console tab exists", async () => {
  await open([], false);
  expect(Array.from(panel.el.querySelectorAll("[role=tab]")).map((t) => t.textContent)).toEqual(["Console"]);
  console.log("only me");
  expect(rows()).toHaveLength(1);
});

test("the close button and Esc call onClose", async () => {
  let closed = 0;
  await open([], true, () => closed++);
  panel.el.querySelector<HTMLButtonElement>("button.zen-dev-close")!.click();
  expect(closed).toBe(1);
  const ev = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
  panel.el.querySelector<HTMLElement>(".zen-dev-list")!.dispatchEvent(ev);
  expect(closed).toBe(2);
  expect(ev.defaultPrevented).toBe(true);
});
