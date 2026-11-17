// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import { mountPreview } from "./preview";

let root: HTMLElement;
let api: FakeApi;
let stop: (() => void) | undefined;
let print: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.useFakeTimers();
  document.head.replaceChildren();
  document.body.replaceChildren();
  root = document.body.appendChild(document.createElement("div"));
  api = createFakeApi();
  print = vi.fn();
  (window as unknown as { print: unknown }).print = print;
});
afterEach(() => { stop?.(); stop = undefined; vi.useRealTimers(); vi.restoreAllMocks(); });

const sections = (n: number, extra = "") =>
  Array.from({ length: n }, (_, i) =>
    `<section id="zen-print-${i + 1}" data-path="p${i + 1}.md" class="zen-print-page"><p class="zen-print-crumb">p${i + 1}.md</p><h1>Page ${i + 1}</h1>${i === 0 ? extra : ""}</section>`).join("");
const docHtml = (n: number, extra = "", head = "") => `<!doctype html><html><head><title>x</title>${head}</head><body class="zen-site zen-print">
<div class="zen-page zen-print-doc" role="document"><section class="zen-print-cover"><h1 class="zen-print-title">My Folder</h1><p class="zen-print-meta">${n} pages</p></section>
<nav class="zen-print-toc" aria-label="Contents"><h2>Contents</h2><ul><li><a href="#zen-print-1">Page 1</a></li></ul></nav>${sections(n, extra)}</div></body></html>`;

type Reply = { status?: number; body: string };
type Fetch = (url: string, init?: { headers?: Record<string, string> }) => Promise<{ ok: boolean; status: number; text(): Promise<string> }>;
function fetcher(reply: (url: string) => Reply | Promise<Reply>) {
  const calls: { url: string; headers: Record<string, string> }[] = [];
  const fn: Fetch = async (url, init) => {
    calls.push({ url, headers: init?.headers ?? {} });
    const r = await reply(url);
    const status = r.status ?? 200;
    return { ok: status >= 200 && status < 300, status, text: async () => r.body };
  };
  return { fn, calls };
}

const flush = () => vi.advanceTimersByTimeAsync(500);
const q = <T extends Element = HTMLElement>(sel: string) => root.querySelector(sel) as T;
const btn = (label: string) => Array.from(root.querySelectorAll("button")).find((b) => b.textContent?.includes(label)) as HTMLButtonElement;
const status = () => q(".zen-print-status").textContent;

function open(f: ReturnType<typeof fetcher>, search = "?window=print&page=p2.md", more: Record<string, unknown> = {}) {
  stop = mountPreview(root, api, document, { fetch: f.fn, search, quiet: 0, ...more } as never);
}

test("a toolbar, the document landmark, focus on Print…", async () => {
  const f = fetcher(() => ({ body: docHtml(3) }));
  open(f);
  await flush();
  const bar = q('[role="toolbar"]');
  expect(bar).not.toBeNull();
  expect(q(".zen-print-bar-title").textContent).toBe("My Folder");
  expect(document.title).toBe("My Folder — Print");
  expect((q('input[type="checkbox"]') as HTMLInputElement).checked).toBe(false);
  expect(bar.textContent).toContain("Include drafts");
  const select = q<HTMLSelectElement>("select");
  expect(Array.from(select.options).map((o) => o.textContent)).toEqual(["All", "This section"]);
  expect(btn("Print…")).toBeDefined();
  expect(btn("Close")).toBeDefined();
  expect(btn("Print…").classList.contains("primary")).toBe(true);
  expect(document.activeElement).toBe(btn("Print…"));
  expect(q('[role="document"]')).not.toBeNull();
  expect(root.querySelectorAll("section.zen-print-page").length).toBe(3);
  expect(root.querySelector("nav.zen-print-toc")).not.toBeNull();
});

test("asks the app for the document, as the preview window only", async () => {
  const f = fetcher(() => ({ body: docHtml(2) }));
  open(f);
  await flush();
  expect(f.calls).toHaveLength(1);
  expect(f.calls[0].url).toBe("/_print/site?drafts=0");
  expect(f.calls[0].headers["X-Zen-Print"]).toBe("1");
});

test("options render the document again", async () => {
  let n = 0;
  const f = fetcher((url) => ({ body: docHtml(url.includes("section") ? 1 : url.includes("drafts=1") ? 4 : 3 + 0 * n++) }));
  open(f);
  await flush();
  expect(root.querySelectorAll("section.zen-print-page").length).toBe(3);

  const drafts = q<HTMLInputElement>('input[type="checkbox"]');
  drafts.checked = true;
  drafts.dispatchEvent(new Event("change", { bubbles: true }));
  await flush();
  expect(f.calls.at(-1)!.url).toBe("/_print/site?drafts=1");
  expect(root.querySelectorAll("section.zen-print-page").length).toBe(4);

  const select = q<HTMLSelectElement>("select");
  select.value = "section";
  select.dispatchEvent(new Event("change", { bubbles: true }));
  await flush();
  expect(f.calls.at(-1)!.url).toBe("/_print/site?drafts=1&section=p2.md");
  expect(root.querySelectorAll("section.zen-print-page").length).toBe(1);
  expect(root.querySelectorAll('[role="document"]').length).toBe(1); // replaced, not added
});

test("without a current page there is no section to choose", async () => {
  open(fetcher(() => ({ body: docHtml(2) })), "?window=print");
  await flush();
  const select = q<HTMLSelectElement>("select");
  expect(select.options[1].disabled).toBe(true);
});

test("states: preparing with a count, then ready; Print… only when ready", async () => {
  let release!: (r: Reply) => void;
  const f = fetcher(() => new Promise<Reply>((r) => (release = r)));
  let mounted!: () => void;
  const gate = new Promise<void>((r) => (mounted = r));
  open(f, "?window=print&page=p2.md", {
    importModule: async () => { await gate; return { default: { mount(el: HTMLElement) { el.textContent = "drawn"; } } }; },
  });
  await flush();
  expect(status()).toMatch(/^Preparing/);
  expect(btn("Print…").getAttribute("aria-disabled")).toBe("true");
  expect(q(".zen-print-spinner")).not.toBeNull();
  btn("Print…").click();
  expect(print).not.toHaveBeenCalled();

  release({ body: docHtml(34, '<div data-zen-plugin="demo">src</div>').replace("<section id=\"zen-print-1\"", '<section data-zen-js=\'["/_plugins/demo/demo.js"]\' id="zen-print-1"') });
  await flush();
  expect(status()).toBe("Preparing 34 pages…"); // fetched, the plugins are mounting
  expect(btn("Print…").getAttribute("aria-disabled")).toBe("true");

  mounted();
  await flush();
  expect(q('[data-zen-plugin="demo"]').textContent).toBe("drawn"); // the page's scripts ran, in the page's section
  expect(status()).toBe("34 pages");
  expect(btn("Print…").getAttribute("aria-disabled")).not.toBe("true");

  btn("Print…").click();
  await flush();
  expect(print).toHaveBeenCalledTimes(1);
});

test("too many pages: a message, Print… disabled, the section select still works", async () => {
  const f = fetcher((url) => (url.includes("section") ? { body: docHtml(2) } : { status: 413, body: "too many pages to print at once; print a section" }));
  open(f);
  await flush();
  expect(status()).toMatch(/print a section/);
  expect(btn("Print…").getAttribute("aria-disabled")).toBe("true");
  btn("Print…").click();
  await flush();
  expect(print).not.toHaveBeenCalled();
  const select = q<HTMLSelectElement>("select");
  expect(select.disabled).toBe(false);
  select.value = "section";
  select.dispatchEvent(new Event("change", { bubbles: true }));
  await flush();
  expect(status()).toBe("2 pages");
  expect(btn("Print…").getAttribute("aria-disabled")).not.toBe("true");
});

test("an error: the message and Retry", async () => {
  let fail = true;
  const f = fetcher(() => (fail ? { status: 500, body: "boom" } : { body: docHtml(2) }));
  open(f);
  await flush();
  const alert = q('[role="alert"]');
  expect(alert.textContent).toContain("boom");
  fail = false;
  btn("Retry").click();
  await flush();
  expect(root.querySelector('[role="alert"]')).toBeNull();
  expect(status()).toBe("2 pages");
});

test("a failing request is an error too", async () => {
  stop = mountPreview(root, api, document, { fetch: () => Promise.reject(new Error("offline")), search: "", quiet: 0 } as never);
  await flush();
  expect(q('[role="alert"]').textContent).toContain("offline");
});

test("Close and Esc close this window; ⌘P prints; the arrow keys move along the toolbar", async () => {
  open(fetcher(() => ({ body: docHtml(2) })));
  await flush();
  const closed = vi.fn();
  window.addEventListener("zen:close-window", closed);
  btn("Close").click();
  expect(closed).toHaveBeenCalledTimes(1);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  expect(closed).toHaveBeenCalledTimes(2);
  window.removeEventListener("zen:close-window", closed);

  const e = new KeyboardEvent("keydown", { key: "p", metaKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(true);
  await flush();
  expect(print).toHaveBeenCalledTimes(1);

  btn("Print…").focus();
  q('[role="toolbar"]').dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
  expect(document.activeElement).toBe(btn("Close"));
  q('[role="toolbar"]').dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(document.activeElement).toBe(btn("Print…"));
});

test("styles of the plugins and layouts come along; links open through the app; cleanup", async () => {
  const f = fetcher(() => ({ body: docHtml(1, '<p><a data-zen-link="external" href="https://e.com/x" target="_blank">e</a></p>', "<style>.zl-x{color:red}</style>") }));
  open(f);
  await flush();
  expect(Array.from(document.head.querySelectorAll("style")).some((s) => s.textContent?.includes(".zl-x"))).toBe(true);
  const a = q<HTMLAnchorElement>('a[data-zen-link="external"]');
  const click = new MouseEvent("click", { bubbles: true, cancelable: true });
  a.dispatchEvent(click);
  expect(click.defaultPrevented).toBe(true);
  expect(api.callsTo("openExternal")).toEqual([["https://e.com/x"]]);

  stop!();
  stop = undefined;
  expect(root.childElementCount).toBe(0);
  expect(Array.from(document.head.querySelectorAll("style")).some((s) => s.textContent?.includes(".zl-x"))).toBe(false);
});
