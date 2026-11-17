// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { whenReady } from "./ready";

let root: HTMLElement;
beforeEach(() => {
  vi.useFakeTimers();
  document.body.replaceChildren();
  root = document.body.appendChild(document.createElement("article"));
});
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

/** Runs whenReady and reports whether it has settled after each step. */
function watch(p: Promise<{ late: string[] }>) {
  const s = { done: false, value: undefined as { late: string[] } | undefined };
  void p.then((v) => { s.done = true; s.value = v; });
  return s;
}
const advance = (ms: number) => vi.advanceTimersByTimeAsync(ms);

test("nothing to wait for: ready after the page has been quiet", async () => {
  root.innerHTML = "<h1>Hi</h1><p>text</p>";
  const s = watch(whenReady(root));
  await advance(10);
  expect(s.done).toBe(false); // the page must be quiet first
  await advance(300);
  expect(s.value).toEqual({ late: [] });
});

test("waits for the plugins to be mounted, then for the page to settle", async () => {
  root.innerHTML = '<div data-zen-plugin="diagram"></div>';
  let mounted!: () => void;
  const s = watch(whenReady(root, { mounted: new Promise<void>((r) => (mounted = r)), quiet: 50 }));
  await advance(2000);
  expect(s.done).toBe(false);
  mounted();
  await advance(10);
  // the diagram draws a little later: that restarts the quiet period
  setTimeout(() => root.firstElementChild!.append(document.createElement("svg")), 30);
  await advance(60);
  expect(s.done).toBe(false);
  await advance(100);
  expect(s.value).toEqual({ late: [] });
});

test("a mount that fails does not hold the page back", async () => {
  root.innerHTML = '<div data-zen-plugin="x"></div>';
  const s = watch(whenReady(root, { mounted: Promise.reject(new Error("no")), quiet: 0 }));
  await advance(100);
  expect(s.value).toEqual({ late: [] });
});

test("blocks that say they are busy are waited for", async () => {
  root.innerHTML = '<div data-zen-plugin="quiz" aria-busy="true"></div><div data-zen-layout="course" data-zen-pending></div>';
  const s = watch(whenReady(root, { quiet: 0 }));
  await advance(1000);
  expect(s.done).toBe(false);
  root.querySelector("[aria-busy]")!.removeAttribute("aria-busy");
  await advance(1000);
  expect(s.done).toBe(false);
  root.querySelector("[data-zen-pending]")!.removeAttribute("data-zen-pending");
  await advance(100);
  expect(s.done).toBe(true);
});

test("images and fonts are waited for", async () => {
  root.innerHTML = '<img src="/_ws/a.png"><img src="/_ws/b.png">';
  const [a, b] = Array.from(root.querySelectorAll("img"));
  for (const i of [a, b]) Object.defineProperty(i, "complete", { value: false, configurable: true, writable: true });
  const fonts = { status: "loading" };
  Object.defineProperty(document, "fonts", { value: fonts, configurable: true });
  const s = watch(whenReady(root, { quiet: 0 }));
  await advance(500);
  expect(s.done).toBe(false);
  (a as { complete: boolean }).complete = true;
  fonts.status = "loaded";
  await advance(500);
  expect(s.done).toBe(false); // b is still loading
  (b as { complete: boolean }).complete = true;
  await advance(100);
  expect(s.done).toBe(true);
  Reflect.deleteProperty(document, "fonts");
});

test("a broken image does not hold the page back for ever", async () => {
  root.innerHTML = '<img src="/_ws/gone.png">';
  const img = root.querySelector("img")!;
  Object.defineProperty(img, "complete", { value: false, configurable: true, writable: true });
  const s = watch(whenReady(root, { quiet: 0 }));
  await advance(100);
  img.dispatchEvent(new Event("error")); // the browser reports it; complete may stay false
  await advance(100);
  expect(s.done).toBe(true);
});

test("after the timeout (5 s by default) it goes on and names what was not ready", async () => {
  root.innerHTML = '<div data-zen-plugin="diagram"><pre>graph TD</pre></div><div data-zen-layout="slides"></div>';
  const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
  const s = watch(whenReady(root, { mounted: new Promise(() => {}) }));
  await advance(4900);
  expect(s.done).toBe(false);
  await advance(200);
  expect(s.value?.late).toEqual(["diagram", "slides"]);
  expect(warn).toHaveBeenCalledTimes(1);
  expect(String(warn.mock.calls[0].join(" "))).toMatch(/diagram.*slides/);
});

test("the timeout can be shorter", async () => {
  root.innerHTML = '<div data-zen-plugin="x" aria-busy="true"></div>';
  vi.spyOn(console, "warn").mockImplementation(() => {});
  const s = watch(whenReady(root, { timeout: 200 }));
  await advance(250);
  expect(s.value).toEqual({ late: ["x"] });
});
