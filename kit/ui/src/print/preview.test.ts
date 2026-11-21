// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
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

test("a toolbar, the document landmark, focus on Export PDF…", async () => {
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
  expect(btn("Export PDF…").classList.contains("primary")).toBe(true);
  expect(btn("Print…").classList.contains("primary")).toBe(false);
  expect(document.activeElement).toBe(btn("Export PDF…"));
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

// ---- Export PDF (app/export behaviours 4-7)

test("Export PDF is the primary action", async () => {
  open(fetcher(() => ({ body: docHtml(3) })));
  await flush();
  const bar = q('[role="toolbar"]');
  const labels = Array.from(bar.querySelectorAll("button")).map((b) => b.textContent);
  expect(labels).toEqual(["Close", "Print…", "Export PDF…"]);
  expect(btn("Export PDF…").classList.contains("primary")).toBe(true);
  expect(btn("Print…").classList.contains("primary")).toBe(false);
  expect(document.activeElement).toBe(btn("Export PDF…"));
  btn("Print…").click();
  await flush();
  expect(print).toHaveBeenCalledTimes(1); // Print… still prints
  expect(api.callsTo("exportPdf")).toHaveLength(0);
});

test("cancelled export shows nothing", async () => {
  api.exportPath = "";
  open(fetcher(() => ({ body: docHtml(3) })));
  await flush();
  expect(status()).toBe("3 pages");
  btn("Export PDF…").click();
  await flush();
  expect(api.callsTo("exportPdf")).toEqual([["My Folder — Print"]]);
  expect(status()).toBe("3 pages"); // as before: no message, no link
  expect(root.querySelector('[role="alert"]')).toBeNull();
  expect(root.textContent).not.toContain("Saved");
  expect(btn("Show in Finder")).toBeUndefined();
  expect(btn("Export PDF…").hasAttribute("disabled")).toBe(false);
  expect(btn("Export PDF…").getAttribute("aria-disabled")).not.toBe("true");
});

test("export reports the file or the error", async () => {
  open(fetcher(() => ({ body: docHtml(3) })));
  await flush();
  // While it writes: "Exporting…", buttons disabled.
  let finish!: (path: string) => void;
  api.exportPath = () => new Promise<string>((r) => (finish = r));
  btn("Export PDF…").click();
  await flush();
  expect(q(".zen-print-status").textContent).toContain("Exporting…");
  expect(btn("Export PDF…").getAttribute("aria-disabled")).toBe("true");
  expect(btn("Print…").getAttribute("aria-disabled")).toBe("true");
  btn("Export PDF…").click(); // a second click does nothing
  await flush();
  expect(api.callsTo("exportPdf")).toHaveLength(1);

  finish("/Users/me/Documents/Guide.pdf");
  await flush();
  const line = q(".zen-print-status");
  expect(line.textContent).toContain("Saved Guide.pdf");
  expect(line.getAttribute("aria-live")).toBe("polite");
  expect(root.querySelector('[role="alert"]')).toBeNull();
  expect(btn("Export PDF…").getAttribute("aria-disabled")).not.toBe("true");
  btn("Show in Finder").click();
  await flush();
  expect(api.callsTo("revealExport")).toHaveLength(1);

  // An error: shown in the preview, as an alert; the button works again.
  api.exportPath = "/x.pdf";
  api.failNext("exportPdf", new Error("the disk is full"));
  btn("Export PDF…").click();
  await flush();
  const alert = q('[role="alert"]');
  expect(alert.textContent).toContain("the disk is full");
  expect(btn("Show in Finder")).toBeUndefined();
  expect(btn("Export PDF…").getAttribute("aria-disabled")).not.toBe("true");
});

test("export waits until the paper is ready", async () => {
  let release!: (r: Reply) => void;
  const f = fetcher(() => new Promise<Reply>((r) => (release = r)));
  open(f);
  await flush();
  expect(btn("Export PDF…").getAttribute("aria-disabled")).toBe("true");
  btn("Export PDF…").click();
  await flush();
  expect(api.callsTo("exportPdf")).toHaveLength(0);
  release({ body: docHtml(2) });
  await flush();
  expect(btn("Export PDF…").getAttribute("aria-disabled")).not.toBe("true");
  btn("Export PDF…").click();
  await flush();
  expect(api.callsTo("exportPdf")).toHaveLength(1);
});

test("export mode calls printReady", async () => {
  open(fetcher(() => ({ body: docHtml(3) })), "?window=print&export=1&page=");
  await flush();
  expect(root.querySelector('[role="toolbar"]')).toBeNull();
  expect(root.querySelectorAll("button")).toHaveLength(0);
  expect(root.querySelectorAll("section.zen-print-page").length).toBe(3);
  expect(api.callsTo("printReady")).toHaveLength(1);
  expect(api.callsTo("printReady")[0]).toEqual([[]]);
  await flush();
  expect(api.callsTo("printReady")).toHaveLength(1); // once
  expect(api.callsTo("exportPdf")).toHaveLength(0);
});

test("export mode tells the app which blocks were late", async () => {
  const f = fetcher(() => ({ body: docHtml(1, '<div data-zen-plugin="slow">src</div>').replace('<section id="zen-print-1"', "<section data-zen-js='[\"/_plugins/slow/slow.js\"]' id=\"zen-print-1\"") }));
  open(f, "?window=print&export=1&page=", { importModule: () => new Promise(() => {}), timeout: 50 });
  await vi.advanceTimersByTimeAsync(2000);
  const calls = api.callsTo("printReady");
  expect(calls).toHaveLength(1);
  expect(Array.isArray(calls[0][0])).toBe(true);
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

test("export mode: a failed load calls printFailed and not printReady", async () => {
  open(fetcher(() => ({ status: 500, body: "render failed: bad block\nmore detail" })), "?window=print&export=1&page=");
  await flush();
  expect(api.callsTo("printFailed")).toEqual([["render failed: bad block"]]);
  expect(api.callsTo("printReady")).toHaveLength(0);
  expect(root.querySelectorAll("button")).toHaveLength(0);
});

// ---- the options the Settings form and `zenmd export pdf` hand over (ui/export-pdf-settings)

test("the window's query sets the options: drafts, pages and cover", async () => {
  const f = fetcher(() => ({ body: docHtml(3) }));
  open(f, "?window=print&page=p2.md&drafts=1&pages=section&cover=0");
  await flush();
  expect(f.calls[0].url).toBe("/_print/site?drafts=1&section=p2.md");
  expect(q<HTMLInputElement>('input[type="checkbox"]').checked).toBe(true);
  expect(q<HTMLSelectElement>("select").value).toBe("section");
  expect(root.querySelector(".zen-print-cover")).toBeNull();
  expect(root.querySelector("nav.zen-print-toc")).toBeNull();
  expect(root.querySelectorAll("section.zen-print-page").length).toBe(3);
});

test("pages=section without a page falls back to all", async () => {
  const f = fetcher(() => ({ body: docHtml(2) }));
  open(f, "?window=print&pages=section");
  await flush();
  expect(f.calls[0].url).toBe("/_print/site?drafts=0");
  expect(q<HTMLSelectElement>("select").value).toBe("all");
});

test("without a query the folder's defaults start it; the query wins", async () => {
  api.pdfDefaultsValue = { out: "/x/a.pdf", drafts: true, cover: false, open: true };
  const f = fetcher(() => ({ body: docHtml(2) }));
  open(f, "?window=print&page=p2.md");
  await flush();
  expect(f.calls[0].url).toBe("/_print/site?drafts=1");
  expect(root.querySelector(".zen-print-cover")).toBeNull();
  stop?.();
  const g = fetcher(() => ({ body: docHtml(2) }));
  open(g, "?window=print&page=p2.md&drafts=0&cover=1");
  await flush();
  expect(g.calls[0].url).toBe("/_print/site?drafts=0");
  expect(root.querySelector(".zen-print-cover")).not.toBeNull();
});

test("auto: writes the form's file when laid out, with no panel, then closes", async () => {
  const closed = vi.fn();
  window.addEventListener("zen:close-window", closed);
  api.exportPath = "/x/out/Guide.pdf";
  const f = fetcher(() => ({ body: docHtml(2) }));
  open(f, "?window=print&auto=1&page=p1.md");
  await flush();
  expect(api.callsTo("exportPdf")).toEqual([[expect.any(String), true]]);
  expect(closed).toHaveBeenCalledTimes(1);
  window.removeEventListener("zen:close-window", closed);
});

test("auto: a document that cannot be prepared tells the app; a failed save stays open", async () => {
  const closed = vi.fn();
  window.addEventListener("zen:close-window", closed);
  open(fetcher(() => ({ status: 500, body: "boom\nmore" })), "?window=print&auto=1");
  await flush();
  expect(api.callsTo("pdfExportFailed")).toEqual([["boom"]]);
  expect(api.callsTo("exportPdf")).toHaveLength(0);
  stop?.();
  api.failNext("exportPdf", new Error("disk full"));
  open(fetcher(() => ({ body: docHtml(1) })), "?window=print&auto=1");
  await flush();
  expect(closed).not.toHaveBeenCalled();
  expect(root.textContent).toContain("disk full");
  window.removeEventListener("zen:close-window", closed);
});
