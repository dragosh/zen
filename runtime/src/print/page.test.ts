import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { installPagePrint, PRINT_PAGE, printPage } from "./page";

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

test("the top bar's Print button (the PRINT_PAGE event) prints like ⌘P, and stops with the remover", async () => {
  expect(PRINT_PAGE).toBe("zen:print-page");
  article("<p>x</p>");
  const f = fakeApi();
  const off = installPagePrint(document, f.api, { quiet: 0 });
  document.dispatchEvent(new CustomEvent(PRINT_PAGE));
  await advance(100);
  expect(print).toHaveBeenCalledTimes(1);
  off();
  document.dispatchEvent(new CustomEvent(PRINT_PAGE));
  await advance(100);
  expect(print).toHaveBeenCalledTimes(1);
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

test("in the app, printing goes through the native service with the page title, not window.print()", async () => {
  article('<details><summary>s</summary>x</details>');
  document.title = "Install — Zenmark";
  const printWindow = vi.fn(async () => {});
  let menu: (() => void) | undefined;
  const off = installPagePrint(document, { on: (_e, fn) => { menu = fn; return () => {}; }, printWindow }, { quiet: 0 });
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  menu!();
  await advance(200);
  expect(printWindow).toHaveBeenCalledWith("Install — Zenmark");
  expect(print).not.toHaveBeenCalled();
  // the page stays prepared while the sheet is up, and is put back on the next interaction
  expect(document.querySelector("details")!.open).toBe(true);
  document.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  expect(document.querySelector("details")!.open).toBe(false);
  off();
});

test("a failing native print puts the page back and lets the next request through", async () => {
  article('<details><summary>s</summary>x</details>');
  const printWindow = vi.fn(async () => { throw new Error("no window"); });
  vi.spyOn(console, "error").mockImplementation(() => {});
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  let menu: (() => void) | undefined;
  const off = installPagePrint(document, { on: (_e, fn) => { menu = fn; return () => {}; }, printWindow }, { quiet: 0 });
  menu!();
  await advance(200);
  expect(printWindow).toHaveBeenCalledTimes(1);
  expect(document.querySelector("details")!.open).toBe(false);
  menu!();
  await advance(200);
  expect(printWindow).toHaveBeenCalledTimes(2);
  off();
});
