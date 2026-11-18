// zenmark-org/zenmd#1: the release app showed the fake "Sample docs" because getApi
// looked for window._wails, which Wails on macOS only adds once the page has loaded.
import { afterEach, expect, test, vi } from "vitest";
import { inApp, wailsReady } from "./detect";

afterEach(() => { vi.useRealTimers(); });

type W = Record<string, any> & EventTarget;
const win = (extra: Record<string, unknown> = {}): W => Object.assign(new EventTarget(), extra) as W;

test("the app is recognised before window._wails exists: Wails registers the external message handler first", () => {
  expect(inApp(win({ webkit: { messageHandlers: { external: {} } } }))).toBe(true);
  expect(inApp(win({ _wails: {} }))).toBe(true);
});

test("a browser (vite, the preview) is not the app", () => {
  expect(inApp(win())).toBe(false);
  expect(inApp(win({ webkit: { messageHandlers: {} } }))).toBe(false); // Safari has webkit, not our handler
  expect(inApp(undefined)).toBe(false);
});

test("wailsReady resolves at once when Wails is already there", async () => {
  await expect(wailsReady(win({ _wails: { invoke() {} } }))).resolves.toBe(true);
});

test("wailsReady waits until Wails has added window._wails.invoke (page loaded)", async () => {
  vi.useFakeTimers();
  const w = win();
  let done: boolean | undefined;
  void wailsReady(w, 5000).then((v) => { done = v; });
  await vi.advanceTimersByTimeAsync(200);
  expect(done).toBeUndefined();
  w._wails = { invoke() {} }; // what WebViewDidFinishNavigation injects
  w.dispatchEvent(new Event("wails:runtime-config-ready"));
  await vi.advanceTimersByTimeAsync(0);
  expect(done).toBe(true);
});

test("wailsReady also notices window._wails.invoke without the event, and gives up after the timeout", async () => {
  vi.useFakeTimers();
  const late = win();
  let a: boolean | undefined;
  void wailsReady(late, 5000).then((v) => { a = v; });
  late._wails = { invoke() {} };
  await vi.advanceTimersByTimeAsync(50);
  expect(a).toBe(true);

  const never = win();
  let b: boolean | undefined;
  void wailsReady(never, 1000).then((v) => { b = v; });
  await vi.advanceTimersByTimeAsync(999);
  expect(b).toBeUndefined();
  await vi.advanceTimersByTimeAsync(50);
  expect(b).toBe(false);
});
