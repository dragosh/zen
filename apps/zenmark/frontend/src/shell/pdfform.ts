// Settings › Export › PDF (ui/export-pdf-settings): the form that writes the open folder as one PDF file, in place in
// the page (no modal), built from the same parts as the website's form (exportform.ts, the same fields, buttons,
// status line and Save as folder defaults). Export PDF writes the file straight away, with no print panel (the app lays
// the document out in a preview window and writes it; pdf:export says how it went); Preview… opens the print preview
// with these options. The folder's defaults are export.pdf in .zenmd.yml. All text goes in through textContent.
import type { PdfExported, PdfOptions, PdfRun } from "../api";
import { button, el, hasFeature, type BuildInfo } from "@zenmd/kit";
import { actionButton, check, createShell, message, nextId, pathField, radioGroup, saveDefaultsFlow, setBar, shownOut } from "./exportform";

export type { PdfExported, PdfOptions, PdfRun } from "../api";

/** The services behind the form (apps/zenmark exportpdf.go); `on` is the event side of the api. */
export interface PdfApi {
  /** The assembly of the build; without the editor feature there is no Save as folder defaults. Absent: everything. */
  build?: BuildInfo;
  pdfDefaults(): Promise<PdfOptions>;
  /** Without confirm: the preview text, nothing written. With it: writes export.pdf in .zenmd.yml and returns the same text. */
  savePdfDefaults(o: PdfOptions, confirm: boolean): Promise<string>;
  /** The native save panel; "" when cancelled. */
  pickPdfFile(): Promise<string>;
  /** Returns once the export has begun; pdf:export says how it ended. */
  runPdfExport(o: PdfRun): Promise<void>;
  previewPdf(o: PdfRun): Promise<void>;
  revealExport(): Promise<void>;
  openExport(): Promise<void>;
  on(event: "pdf:export", fn: (r: PdfExported) => void): () => void;
}

/** The PDF written last is recorded here (the windows share one origin); the form shows it. */
export const LAST_PDF_KEY = "zen.lastPdfExport";
export function lastPdfExport(): { path: string; at: number } | undefined {
  try {
    const v = JSON.parse(localStorage.getItem(LAST_PDF_KEY) ?? "null");
    if (v && typeof v.path === "string" && typeof v.at === "number") return v;
  } catch { /* no storage */ }
  return undefined;
}
function rememberPdf(path: string): void {
  try { localStorage.setItem(LAST_PDF_KEY, JSON.stringify({ path, at: Date.now() })); } catch { /* no storage: the form just shows no last file */ }
}

export type PdfFormOptions = {
  api: PdfApi;
  doc: Document;
  /** The open folder's path, "" for none. */
  root(): string;
  /** The page the main window shows, "" for none: This section needs one. */
  currentPage(): string;
  /** Paths with unsaved changes in editor windows ("zen:config/workspace" is .zenmd.yml). */
  unsaved?(): string[];
};
export type PdfForm = {
  el: HTMLFormElement;
  /** Reads the folder's PDF defaults into the form; an export that runs, or a result on show, stays unless force is set. */
  refresh(force?: boolean): Promise<void>;
  focus(): void;
  /** An export is running. */
  busy(): boolean;
  destroy(): void;
};

export function createPdfForm(opts: PdfFormOptions): PdfForm {
  const { api, doc } = opts;
  const id = nextId();
  const preview = button("Preview…", () => void showPreview());
  const shell = createShell({
    progressLabel: "Exporting the PDF", progressText: "Exporting…", primary: "Export PDF", secondary: [preview],
    canSave: hasFeature(api, "editor"), onSave: () => void saveDefaults(), onCancel: () => stop(),
  });
  const { form, options, last, progress, progressText, cancel, result, error, status, primary: exportBtn, saveLink } = shell;

  // ---- the options: where, then the choices
  const field = pathField(id, "Output file", "exports/pdf", () => void pick());
  const { input: out, choose } = field;
  const pages = radioGroup(id, "pages", "Pages", [{ value: "all", label: "All" }, { value: "section", label: "This section" }]);
  const [allPages, section] = pages.radios;
  const drafts = check("drafts", "Include drafts");
  const cover = check("cover", "Cover and contents");
  const open = check("open", "Open when done");
  options.append(field.label, field.row, pages.label, pages.group, drafts.l, cover.l, open.l);

  const inputs = [out, allPages, section, drafts.i, cover.i, open.i];
  let state: "form" | "exporting" | "result" = "form";
  let destroyed = false;
  let filledFor: string | null = null;
  let ready: Promise<void> = Promise.resolve();
  const rootNow = () => opts.root();

  function lock(on: boolean): void {
    for (const i of inputs) i.disabled = on;
    choose.disabled = saveLink.disabled = preview.disabled = on;
    if (!on) updateSection();
  }
  /** This section is the page that is open and the pages under it: without a page there is none. */
  function updateSection(): void {
    const has = opts.currentPage() !== "";
    section.disabled = !has;
    if (!has && section.checked) allPages.checked = true;
  }
  function read(): PdfRun {
    return { out: out.value.trim().replace(/^\.\//, ""), drafts: drafts.i.checked, cover: cover.i.checked, open: open.i.checked, section: section.checked };
  }
  function fill(o: PdfOptions): void {
    out.value = shownOut(rootNow(), o.out);
    drafts.i.checked = o.drafts;
    cover.i.checked = o.cover;
    open.i.checked = o.open;
    allPages.checked = true;
    section.checked = false;
  }

  function reset(): void {
    state = "form";
    result.hidden = true;
    result.replaceChildren();
    progress.hidden = true;
    shell.showForm(true);
    lock(false);
    exportBtn.disabled = false;
    error.textContent = status.textContent = "";
    shell.clearPreview();
  }

  function showLast(root: string): void {
    last.replaceChildren();
    const l = lastPdfExport();
    if (state !== "form" || root === "" || !l) return;
    last.append(`Last PDF: ${l.path.split(/[\\/]/).pop()}, ${new Date(l.at).toLocaleTimeString()}. `);
    last.append(actionButton(shell, "Show in Finder", () => api.revealExport(), "zen-buildsite-link"));
  }

  async function refresh(force = false): Promise<void> {
    const root = rootNow();
    shell.none.hidden = root !== "";
    shell.showForm(!(root === "" || state === "result"));
    updateSection();
    if (root === "") {
      exportBtn.disabled = true;
      preview.disabled = true;
      filledFor = null;
      last.replaceChildren();
      return;
    }
    showLast(root);
    if (state === "exporting") return;
    if (state === "result" && root === filledFor && !force) return;
    if (state === "result") reset();
    shell.showForm(true);
    exportBtn.disabled = preview.disabled = false;
    if (root === filledFor && !force) return;
    filledFor = root;
    fill({ out: `${root}/exports/pdf`, drafts: false, cover: true, open: true });
    ready = api.pdfDefaults().then(fill, (err) => { if (!destroyed) error.textContent = message(err); });
    await ready;
  }

  async function pick(): Promise<void> {
    try {
      const file = await api.pickPdfFile();
      if (file && !destroyed) out.value = shownOut(rootNow(), file);
    } catch (err) {
      error.textContent = message(err);
    }
  }

  async function showPreview(): Promise<void> {
    error.textContent = status.textContent = "";
    try {
      await api.previewPdf(read());
    } catch (err) {
      error.textContent = message(err);
    }
  }

  async function saveDefaults(): Promise<void> {
    const { section: _section, ...o } = read();
    void _section;
    await saveDefaultsFlow(shell, (confirm) => api.savePdfDefaults(o, confirm), opts.unsaved?.() ?? []);
  }

  // ---- exporting: the app does the work in a window of its own and says how it went
  const offExport = api.on("pdf:export", (r) => {
    if (state !== "exporting") return;
    if (r.error || !r.path) {
      back(r.error || "The PDF was not written.");
      return;
    }
    rememberPdf(r.path);
    showResult(r.path);
  });

  function back(text: string): void {
    state = "form";
    lock(false);
    exportBtn.disabled = false;
    progress.hidden = true;
    error.textContent = text;
    exportBtn.focus();
  }

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (state !== "form" || rootNow() === "") return;
    error.textContent = status.textContent = "";
    shell.clearPreview();
    await ready;
    if (destroyed) return;
    state = "exporting";
    lock(true);
    exportBtn.disabled = true;
    last.replaceChildren();
    progress.hidden = false;
    progressText.textContent = "Exporting…";
    setBar(shell, 0, 0);
    try {
      await api.runPdfExport(read());
    } catch (err) {
      if (!destroyed && state === "exporting") back(message(err));
    }
  });

  /** Cancel stops waiting; a file that is written later is not shown. */
  function stop(): void {
    if (state !== "exporting") return;
    back("");
  }

  function showResult(path: string): void {
    state = "result";
    progress.hidden = true;
    shell.showForm(false);
    const summary = el("p", "zen-buildsite-summary");
    summary.setAttribute("role", "status");
    summary.textContent = `Saved ${path.split(/[\\/]/).pop()}.`;
    const opener = actionButton(shell, "Open", () => api.openExport(), "primary");
    const reveal = actionButton(shell, "Show in Finder", () => api.revealExport());
    const again = button("Export Again", () => { reset(); exportBtn.focus(); });
    const actions = el("div", "zen-buildsite-actions");
    actions.append(opener, reveal, again);
    result.replaceChildren(summary, actions);
    result.hidden = false;
    if (form.contains(doc.activeElement) || doc.activeElement === doc.body) opener.focus();
  }

  // Esc cancels the wait; otherwise it is left to Settings (which leaves the page).
  form.addEventListener("keydown", (e) => {
    if (e.key !== "Escape" || state !== "exporting") return;
    e.preventDefault();
    e.stopPropagation();
    stop();
  });

  return {
    el: form,
    refresh,
    focus() {
      if (state === "exporting") cancel?.focus();
      else if (state === "result") result.querySelector<HTMLElement>(".primary")?.focus();
      else exportBtn.focus();
    },
    busy: () => state === "exporting",
    destroy() {
      destroyed = true;
      offExport();
      form.remove();
    },
  };
}
