// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { installPagePrint, printPage } from "./page";

let print: ReturnType<typeof vi.fn>;
beforeEach(() => {
  vi.useFakeTimers();
  document.head.replaceChildren();
  document.body.replaceChildren();
  print = vi.fn();
  (window as unknown as { print: unknown }).print = print;
});
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });
const advance = (ms: number) => vi.advanceTimersByTimeAsync(ms);

function article(html: string): HTMLElement {
  const a = document.body.appendChild(document.createElement("article"));
  a.className = "zen-page";
  a.innerHTML = html;
  return a;
}

test("print waits for plugins", async () => {
  const a = article('<div data-zen-plugin="diagram" aria-busy="true"></div>');
  const done = printPage(document, { quiet: 0 });
  await advance(1000);
  expect(print).not.toHaveBeenCalled();
  a.firstElementChild!.removeAttribute("aria-busy");
  await advance(100);
  expect(print).toHaveBeenCalledTimes(1);
  expect(await done).toEqual({ late: [] });
});

test("print goes ahead after the timeout and says what was late", async () => {
  article('<div data-zen-plugin="diagram" aria-busy="true"></div>');
  vi.spyOn(console, "warn").mockImplementation(() => {});
  const done = printPage(document, { timeout: 300, quiet: 0 });
  await advance(400);
  expect(print).toHaveBeenCalledTimes(1);
  expect((await done).late).toEqual(["diagram"]);
});

test("details print open and are put back afterwards; a link whose text is its URL is marked", async () => {
  const a = article(`<details><summary>s</summary>hidden</details><details open><summary>o</summary>x</details>
    <a data-zen-link="external" href="https://e.com/x">https://e.com/x</a><a data-zen-link="external" href="https://e.com/y">text</a>`);
  const [d1, d2] = Array.from(a.querySelectorAll("details"));
  let during: boolean[] = [];
  print.mockImplementation(() => { during = [d1.open, d2.open]; });
  const done = printPage(document, { quiet: 0 });
  await advance(100);
  await done;
  expect(during).toEqual([true, true]);
  expect(a.querySelectorAll("a[data-zen-print=url]").length).toBe(1);
  expect(a.querySelector("a[data-zen-print=url]")!.getAttribute("href")).toBe("https://e.com/x");
  window.dispatchEvent(new Event("afterprint"));
  expect([d1.open, d2.open]).toEqual([false, true]);
});

test("the editor window prints its preview, not the editor", async () => {
  document.body.innerHTML = '<div class="zen-editor"><textarea class="zen-editor-source"></textarea><div class="zen-editor-preview"><article class="zen-page"><div data-zen-plugin="p" aria-busy="true"></div></article></div></div>';
  const done = printPage(document, { quiet: 0 });
  await advance(500);
  expect(print).not.toHaveBeenCalled(); // waits on the preview's block
  document.querySelector("[aria-busy]")!.removeAttribute("aria-busy");
  await advance(100);
  await done;
  expect(print).toHaveBeenCalledTimes(1);
});

function fakeApi() {
  const listeners = new Set<() => void>();
  return {
    api: { on: (_e: "menu:print-page", fn: () => void) => { listeners.add(fn); return () => listeners.delete(fn); } },
    emit: () => listeners.forEach((f) => f()),
    count: () => listeners.size,
  };
}
const key = (init: KeyboardEventInit) => {
  const e = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
  document.dispatchEvent(e);
  return e;
};

test("⌘P and the menu event print; ⇧⌘P and other keys do not", async () => {
  article("<p>x</p>");
  const f = fakeApi();
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  const off = installPagePrint(document, f.api, { quiet: 0 });
  const e = key({ key: "p", metaKey: true });
  expect(e.defaultPrevented).toBe(true);
  await advance(100);
  expect(print).toHaveBeenCalledTimes(1);

  expect(key({ key: "p", metaKey: true, shiftKey: true }).defaultPrevented).toBe(false);
  key({ key: "p" });
  key({ key: "o", metaKey: true });
  await advance(100);
  expect(print).toHaveBeenCalledTimes(1);

  f.emit();
  await advance(100);
  expect(print).toHaveBeenCalledTimes(2);

  // Another window's menu event does not open a panel here.
  vi.spyOn(document, "hasFocus").mockReturnValue(false);
  f.emit();
  await advance(100);
  expect(print).toHaveBeenCalledTimes(2);

  off();
  expect(f.count()).toBe(0);
  key({ key: "p", metaKey: true });
  await advance(100);
  expect(print).toHaveBeenCalledTimes(2);
});

test("a second request while one is waiting is ignored", async () => {
  article('<div data-zen-plugin="d" aria-busy="true"></div>');
  const f = fakeApi();
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  installPagePrint(document, f.api, { quiet: 0 });
  key({ key: "p", metaKey: true });
  key({ key: "p", metaKey: true });
  f.emit();
  document.querySelector("[aria-busy]")!.removeAttribute("aria-busy");
  await advance(200);
  expect(print).toHaveBeenCalledTimes(1);
});
