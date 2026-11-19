// The Print Preview window (?window=print): the whole folder as one document under a small toolbar.
// It fetches the document from the app (/_print/site, made by site.PrintDocument), puts it on a sheet,
// mounts the plugins and layouts of each page the way the page view does, and prints through the
// system print panel (Save as PDF is in it), or saves it as a PDF file straight away (Export PDF…,
// app/export). In export mode (?export=1, the headless `zenmd export pdf`) there is no toolbar: the
// document is laid out, prepared for paper and the app is told (printReady) that it can be written. The toolbar is not printed (runtime/src/css/print.css).
import type { KitHost } from "../host";
import { installRuntime, type Runtime } from "@zenmd/runtime/plugins";
import { button, el } from "../shell/dom";
import { prepareForPrint, printPage } from "@zenmd/runtime/print/page";
import { whenReady, type ReadyOptions } from "@zenmd/runtime/print/ready";
import "./preview.css";

/** What the preview needs of the service API: the plugin runtime's part, openExternal and the menu event. */
export type PreviewApi = Pick<
  KitHost,
  | "pluginStorageGet" | "pluginStorageSet" | "pluginStorageKeys"
  | "layoutStorageGet" | "layoutStorageSet" | "layoutStorageKeys"
  | "pluginInvoke" | "pluginStream" | "pluginCancel" | "openExternal" | "on"
> & Partial<Pick<KitHost, "printWindow" | "exportPdf" | "revealExport" | "printReady">>;

type FetchLike = (url: string, init?: { headers?: Record<string, string> }) => Promise<{ ok: boolean; status: number; text(): Promise<string> }>;

export type PreviewOptions = ReadyOptions & {
  /** Default: fetch. */
  fetch?: FetchLike;
  /** The window URL's query string (page=<current page>); default: location.search. */
  search?: string;
  /** Loads a plugin script (default: import()); tests inject it. */
  importModule?: (url: string) => Promise<unknown>;
};

const ROUTE = "/_print/site";
const LETTER = /-(US|CA|MX|PH|CL|CO|CR|GT|PA|PR|VE)$/i;

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));
const pagesText = (n: number) => (n === 1 ? "1 page" : `${n} pages`);

/** Mounts the preview into root; returns the function that takes it away again. */
export function mountPreview(root: HTMLElement, api: PreviewApi, doc: Document = document, opts: PreviewOptions = {}): () => void {
  const win = doc.defaultView ?? window;
  const query = new URLSearchParams(opts.search ?? win.location?.search ?? "");
  const currentPage = query.get("page") ?? "";
  const exportMode = query.get("export") === "1";
  const doFetch: FetchLike = opts.fetch ?? ((url, init) => win.fetch(url, init));

  // ---- the window ----
  const frame = el("div", "zen-print-preview");
  const bar = el("div", "zen-print-bar");
  bar.setAttribute("role", "toolbar");
  bar.setAttribute("aria-label", "Print options");
  const title = el("strong", "zen-print-bar-title", "Print");
  const status = el("span", "zen-print-status");
  status.setAttribute("role", "status");
  status.setAttribute("aria-live", "polite");
  const spinner = el("span", "zen-print-spinner");
  spinner.setAttribute("aria-hidden", "true");
  const statusText = el("span", "zen-print-status-text");
  status.append(spinner, statusText);

  const draftsBox = el("input");
  draftsBox.type = "checkbox";
  const draftsLabel = el("label", "zen-print-option");
  draftsLabel.append(draftsBox, document.createTextNode(" Include drafts"));

  const pagesSelect = el("select");
  pagesSelect.id = "zen-print-pages";
  const all = el("option", "", "All");
  all.value = "all";
  const part = el("option", "", "This section");
  part.value = "section";
  part.disabled = currentPage === "";
  pagesSelect.append(all, part);
  const pagesLabel = el("label", "zen-print-option", "Pages: ");
  pagesLabel.htmlFor = pagesSelect.id;
  pagesLabel.append(pagesSelect);

  const closeBtn = button("Close", () => close());
  // Print… stays focusable while the document is being prepared; it just does nothing then.
  const printBtn = button("Print…", () => void print());
  // Export PDF… is the primary action: it saves the file without the print panel.
  const exportBtn = button("Export PDF…", () => void exportPdf(), { className: "primary" });
  const spacer = el("span", "zen-print-spacer");
  // What an export said: "Saved <file>" with Show in Finder, or the error.
  const note = el("span", "zen-print-note");
  status.append(note);
  bar.append(title, status, spacer, draftsLabel, pagesLabel, closeBtn, printBtn, exportBtn);

  const scroll = el("div", "zen-print-scroll");
  const paper = el("div", "zen-print-paper");
  paper.style.setProperty("--zen-paper-width", LETTER.test(win.navigator?.language ?? "") ? "8.5in" : "210mm");
  scroll.append(paper);
  if (exportMode) frame.append(scroll);
  else frame.append(bar, scroll);
  root.replaceChildren(frame);

  // ---- state ----
  let seq = 0;
  let ready = false;
  let stopped = false;
  let told = false;
  let exporting = false;
  let shown = ""; // the status line without an export's message
  let runtimes: Runtime[] = [];
  let styles: HTMLElement[] = [];

  const setReady = (v: boolean) => {
    ready = v;
    syncButtons();
  };
  const syncButtons = () => {
    const off = !ready || exporting ? "true" : "false";
    printBtn.setAttribute("aria-disabled", off);
    exportBtn.setAttribute("aria-disabled", off);
  };
  const say = (text: string, busy = false, transient = false) => {
    if (!transient) shown = text;
    statusText.textContent = text;
    spinner.hidden = !busy;
    note.replaceChildren();
  };

  function teardown(): void {
    for (const r of runtimes) r.destroy();
    runtimes = [];
    for (const s of styles) s.remove();
    styles = [];
    paper.replaceChildren();
  }

  function showError(text: string): void {
    setReady(false);
    say("Could not prepare the document");
    const box = el("div", "zen-print-error-box");
    box.setAttribute("role", "alert");
    box.append(el("p", "", text), button("Retry", () => void load(), { className: "primary" }));
    paper.replaceChildren(box);
  }

  async function load(): Promise<void> {
    const mine = ++seq;
    teardown();
    setReady(false);
    say("Preparing…", true);
    let url = `${ROUTE}?drafts=${draftsBox.checked ? 1 : 0}`;
    if (pagesSelect.value === "section" && currentPage) url += `&section=${encodeURIComponent(currentPage)}`;
    let res;
    let text: string;
    try {
      res = await doFetch(url, { headers: { "X-Zen-Print": "1" } });
      text = await res.text();
    } catch (err) {
      if (mine === seq && !stopped) showError(message(err));
      return;
    }
    if (mine !== seq || stopped) return;
    if (res.status === 413) {
      say(text.trim() || "Too many pages to print at once; print a section");
      return;
    }
    if (!res.ok) {
      showError(text.trim() || `The app answered ${res.status}`);
      return;
    }

    const parsed = new win.DOMParser().parseFromString(text, "text/html");
    const content = parsed.body.querySelector(".zen-print-doc");
    if (!content) {
      showError("The document is empty");
      return;
    }
    // The layouts' and plugins' styles; the page styles are the app's own.
    for (const s of Array.from(parsed.head.querySelectorAll("style"))) {
      const copy = doc.createElement("style");
      copy.dataset.zenPrint = "";
      copy.textContent = s.textContent;
      doc.head.append(copy);
      styles.push(copy);
    }
    paper.append(doc.importNode(content, true));
    const heading = paper.querySelector(".zen-print-title")?.textContent?.trim() ?? "";
    if (heading) {
      title.textContent = heading;
      doc.title = `${heading} — Print`;
    }
    const sections = Array.from(paper.querySelectorAll<HTMLElement>("section.zen-print-page"));
    say(`Preparing ${pagesText(sections.length)}…`, true);

    // Each page mounts its own plugins and layouts, so a block knows which page it is on.
    const mounts: Promise<void>[] = [];
    for (const sec of sections) {
      let js: string[] = [];
      try {
        const parsedJS = JSON.parse(sec.dataset.zenJs ?? "[]");
        if (Array.isArray(parsedJS)) js = parsedJS.filter((u): u is string => typeof u === "string");
      } catch { /* no scripts */ }
      if (js.length === 0) continue;
      const path = sec.dataset.path ?? "";
      const rt = installRuntime({
        api: api as unknown as KitHost,
        navigate: () => {},
        page: () => ({ path, title: sec.querySelector("h1")?.textContent ?? path }),
        anchor: () => "",
        onAnchor: () => () => {},
        badge: () => {},
        importModule: opts.importModule,
        refresh: () => { if (!stopped) void load(); },
      });
      runtimes.push(rt);
      mounts.push(rt.mountAll(sec, { js, css: [] }).catch(() => {}));
    }
    const { late } = await whenReady(paper, { timeout: opts.timeout, quiet: opts.quiet, mounted: Promise.all(mounts) });
    if (mine !== seq || stopped) return;
    setReady(true);
    if (exportMode) {
      // Laid out for paper; the app writes the file. The call is made once.
      if (!stopped) {
        prepareForPrint(paper);
        say(pagesText(sections.length));
        if (!told) {
          told = true;
          await api.printReady?.(late).catch((err) => console.error("zen: export failed", err));
        }
      }
      return;
    }
    setReady(true);
    say(late.length > 0 ? `${pagesText(sections.length)} (${late.join(", ")} may not be finished)` : pagesText(sections.length));
  }

  async function exportPdf(): Promise<void> {
    if (!ready || stopped || exporting || !api.exportPdf) return;
    exporting = true;
    syncButtons();
    const text = shown;
    say("Exporting…", true, true);
    const restore = prepareForPrint(paper);
    try {
      const path = await api.exportPdf(doc.title);
      if (stopped) return;
      if (!path) {
        say(text); // cancelled: as before
        return;
      }
      say(`Saved ${path.split(/[\\/]/).pop()}`, false, true);
      if (api.revealExport) note.append(" ", button("Show in Finder", () => void api.revealExport!().catch((e) => showNote(message(e))), { className: "link" }));
    } catch (err) {
      if (!stopped) {
        say(text);
        showNote(message(err));
      }
    } finally {
      restore();
      exporting = false;
      syncButtons();
    }
  }

  function showNote(text: string): void {
    const alert = el("span", "zen-print-alert", text);
    alert.setAttribute("role", "alert");
    note.replaceChildren(alert);
  }

  async function print(): Promise<void> {
    if (!ready || stopped) return;
    const printer = api.printWindow ? () => api.printWindow!(doc.title) : undefined;
    await printPage(doc, { root: paper, timeout: opts.timeout, quiet: opts.quiet, printer })
      .catch((err) => console.error("zen: printing failed", err));
  }

  function close(): void {
    win.dispatchEvent(new CustomEvent("zen:close-window"));
  }

  // ---- events ----
  const onOption = () => void load();
  draftsBox.addEventListener("change", onOption);
  pagesSelect.addEventListener("change", onOption);

  const onKey = (e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "p") {
      e.preventDefault();
      void print();
    } else if (e.key === "Escape" && !e.defaultPrevented) {
      close();
    }
  };
  doc.addEventListener("keydown", onKey);

  const onBarKey = (e: KeyboardEvent) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    const items = Array.from(bar.querySelectorAll<HTMLElement>("input, select, button")).filter((x) => !(x as HTMLInputElement).disabled);
    const at = items.indexOf(doc.activeElement as HTMLElement);
    if (at < 0) return;
    const next = items[Math.min(items.length - 1, Math.max(0, at + (e.key === "ArrowRight" ? 1 : -1)))];
    e.preventDefault();
    next.focus();
  };
  bar.addEventListener("keydown", onBarKey);

  const onClick = (e: MouseEvent) => {
    const a = (e.target as Element | null)?.closest?.('a[data-zen-link="external"]');
    const href = a?.getAttribute("href");
    if (a && href) {
      e.preventDefault();
      void api.openExternal(href);
    }
  };
  paper.addEventListener("click", onClick);

  // The menu event reaches every window: only the one with the focus prints.
  let onMenu = () => {};
  try {
    onMenu = (api.on as unknown as (event: string, fn: () => void) => () => void)("menu:print-page", () => {
      if (doc.hasFocus()) void print();
    });
  } catch { /* an api without the event: ⌘P still works through the key */ }

  setReady(false);
  void load();
  if (!exportMode) exportBtn.focus();

  return () => {
    stopped = true;
    seq++;
    draftsBox.removeEventListener("change", onOption);
    pagesSelect.removeEventListener("change", onOption);
    doc.removeEventListener("keydown", onKey);
    onMenu();
    teardown();
    root.replaceChildren();
  };
}
