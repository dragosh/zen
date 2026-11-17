// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createOpening, type Opening } from "./opening";

let o: Opening;
let cancels = 0;
beforeEach(() => {
  vi.useFakeTimers();
  cancels = 0;
  o = createOpening(document, { onCancel: () => cancels++ });
});
afterEach(() => {
  o.destroy();
  vi.useRealTimers();
});

const text = (sel: string) => o.el.querySelector(sel)?.textContent ?? "";
const btn = (label: string) => Array.from(o.el.querySelectorAll("button")).find((b) => b.textContent === label)!;

test("a fast open never shows the loader", () => {
  o.update({ name: "notes", phase: "reading", done: 256, total: 0 });
  vi.advanceTimersByTime(100);
  o.update({ name: "notes", phase: "done", done: 0, total: 0 });
  vi.advanceTimersByTime(1000);
  expect(o.el.hidden).toBe(true);
});

test("a slow open shows the folder, the phase and the counts", () => {
  o.update({ name: "Developer", phase: "reading", done: 12400, total: 0 });
  expect(o.el.hidden).toBe(true);
  vi.advanceTimersByTime(300);
  expect(o.el.hidden).toBe(false);
  expect(text(".zen-opening-title")).toBe("Opening Developer…");
  expect(text(".zen-opening-detail")).toBe("Reading the folder · 12,400 items");
  expect(o.el.querySelector(".zen-opening-bar")!.classList.contains("zen-opening-indeterminate")).toBe(true);
  expect(document.activeElement).toBe(btn("Cancel"));

  o.update({ name: "Developer", phase: "indexing", done: 412, total: 945 });
  const bar = o.el.querySelector<HTMLElement>(".zen-opening-bar")!;
  expect(text(".zen-opening-detail")).toBe("Indexing pages · 412 of 945");
  expect(bar.getAttribute("aria-valuenow")).toBe("412");
  expect(bar.getAttribute("aria-valuemax")).toBe("945");
  expect(o.el.querySelector<HTMLElement>(".zen-opening-fill")!.style.width).toBe("44%");

  o.update({ name: "Developer", phase: "done", done: 0, total: 0 });
  expect(o.el.hidden).toBe(true);
});

test("after a while it says large folders take time", () => {
  o.update({ name: "big", phase: "reading", done: 0, total: 0 });
  vi.advanceTimersByTime(1000);
  expect(o.el.querySelector<HTMLElement>(".zen-opening-hint")!.hidden).toBe(true);
  vi.advanceTimersByTime(5000);
  expect(o.el.querySelector<HTMLElement>(".zen-opening-hint")!.hidden).toBe(false);
});

test("Cancel and Esc cancel once; the failed event hides it", () => {
  o.update({ name: "big", phase: "reading", done: 0, total: 0 });
  vi.advanceTimersByTime(300);
  btn("Cancel").click();
  btn("Cancelling…").click();
  expect(cancels).toBe(1);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(cancels).toBe(1);
  o.update({ name: "big", phase: "failed", done: 0, total: 0 });
  expect(o.el.hidden).toBe(true);

  // A new open starts fresh, and Esc cancels it.
  o.update({ name: "next", phase: "reading", done: 0, total: 0 });
  vi.advanceTimersByTime(300);
  expect(btn("Cancel").disabled).toBe(false);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(cancels).toBe(2);
});

test("fail shows why at once, with OK; the text stays text", () => {
  o.fail("opening <b>x</b> took longer than 30s.");
  expect(o.el.hidden).toBe(false);
  expect(text(".zen-opening-title")).toBe("Could not open the folder");
  expect(text(".zen-opening-detail")).toBe("opening <b>x</b> took longer than 30s.");
  expect(o.el.querySelector("b")).toBeNull();
  expect(btn("Cancel").hidden).toBe(true);
  btn("OK").click();
  expect(o.el.hidden).toBe(true);
});
