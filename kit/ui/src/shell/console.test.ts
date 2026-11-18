// @vitest-environment happy-dom
import { afterEach, expect, test, vi } from "vitest";
import { installConsoleCapture, type ConsoleCapture } from "./console";

let cap: ConsoleCapture | null = null;
afterEach(() => { cap?.uninstall(); cap = null; vi.restoreAllMocks(); });

test("records console calls and still calls the real console", () => {
  const real = vi.spyOn(console, "warn").mockImplementation(() => {});
  cap = installConsoleCapture(window);
  console.warn("careful", 42, { a: 1 });
  expect(real).toHaveBeenCalledWith("careful", 42, { a: 1 });
  const [e] = cap.entries();
  expect(e).toMatchObject({ level: "warn", source: "console", message: 'careful 42 {"a":1}' });
  expect(e.seq).toBe(1);
  expect(e.time).toBeGreaterThan(0);
});

test("log and info are info, error is error, debug is debug", () => {
  for (const m of ["log", "info", "error", "debug"] as const) vi.spyOn(console, m).mockImplementation(() => {});
  cap = installConsoleCapture(window);
  console.log("a"); console.info("b"); console.error("c"); console.debug("d");
  expect(cap.entries().map((e) => e.level)).toEqual(["info", "info", "error", "debug"]);
});

test("uncaught errors and unhandled rejections are recorded", () => {
  cap = installConsoleCapture(window);
  window.dispatchEvent(new ErrorEvent("error", { message: "boom", filename: "app.js", lineno: 3, error: new Error("boom") }));
  const rej = new Event("unhandledrejection") as Event & { reason?: unknown };
  rej.reason = new Error("nope");
  window.dispatchEvent(rej);
  const [a, b] = cap.entries();
  expect(a.level).toBe("error");
  expect(a.message).toContain("boom");
  expect(b.level).toBe("error");
  expect(b.message).toContain("Unhandled rejection");
  expect(b.message).toContain("nope");
});

test("keeps at most the cap, drops the oldest, notifies subscribers, and uninstall restores the console", () => {
  const before = console.log;
  vi.spyOn(console, "log").mockImplementation(() => {});
  const wrapped = console.log;
  cap = installConsoleCapture(window, 3);
  const seen: string[] = [];
  const off = cap.subscribe((e) => seen.push(e.message));
  for (let i = 1; i <= 5; i++) console.log(`m${i}`);
  expect(cap.entries().map((e) => e.message)).toEqual(["m3", "m4", "m5"]);
  expect(seen).toEqual(["m1", "m2", "m3", "m4", "m5"]);
  off();
  console.log("m6");
  expect(seen).toHaveLength(5);
  cap.uninstall();
  cap = null;
  expect(console.log).toBe(wrapped);
  expect(before).toBeTypeOf("function");
});

test("a circular object, an Error and a huge string do not break it", () => {
  vi.spyOn(console, "log").mockImplementation(() => {});
  cap = installConsoleCapture(window);
  const o: Record<string, unknown> = {};
  o.self = o;
  console.log(o, new Error("bad"), "x".repeat(10000));
  const [e] = cap.entries();
  expect(e.message).toContain("bad");
  expect(e.message.length).toBeLessThanOrEqual(4100);
});

test("installing twice on the same window shares one capture", () => {
  cap = installConsoleCapture(window);
  expect(installConsoleCapture(window)).toBe(cap);
});
