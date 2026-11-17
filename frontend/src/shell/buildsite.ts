// File › Build Website… (⇧⌘B): a modal sheet that builds the open folder as a
// static website through the build services, with progress and a result. The
// pattern of newpage.ts. Also the Website section of Settings
// (createWebsiteSection). All text through textContent.
import "./buildsite.css";
import type { BuildOptions, BuildProgress, BuildResult } from "../api";
import { button, el } from "./dom";

export type { BuildOptions, BuildProgress, BuildResult, BuildWarning } from "../api";

/** The services behind the dialog (internal/app/buildui.go); `on` is the event side of the api. */
export interface BuildApi {
  buildOptions(): Promise<BuildOptions>;
  buildWebsite(o: BuildOptions): Promise<BuildResult>;
  cancelBuild(): Promise<void>;
  /** Without confirm: the preview text, nothing written. With it: writes .zen.yml and returns the same text. */
  saveBuildDefaults(o: BuildOptions, confirm: boolean): Promise<string>;
  /** The native folder panel; "" when cancelled. */
  pickBuildFolder(): Promise<string>;
  openBuiltSite(out: string): Promise<void>;
  revealBuiltSite(out: string): Promise<void>;
  on(event: "build:progress", fn: (p: BuildProgress) => void): () => void;
  on(event: "menu:build-website", fn: () => void): () => void;
}

export type BuildSiteOptions = {
  api: BuildApi;
  doc: Document;
  /** The open folder's path and name (the name is shown muted in the title). */
  root: string;
  name?: string;
  /** Opens a page in the main window; the dialog closes after it. */
  onOpenPage(path: string): void;
  /** Paths with unsaved changes in editor windows ("zen:config/workspace" is .zen.yml); the website uses the saved files. */
  unsaved?(): string[];
  onClose?(): void;
};
export type BuildSiteDialog = { el: HTMLElement; close(): void; focus(): void };

const WORKSPACE_CONFIG = "zen:config/workspace";
const message = (err: unknown) => (err instanceof Error ? err.message : String(err));
const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/** What the last build of each folder did, for Settings › Website. */
const lastBuilds = new Map<string, { result: BuildResult; at: number }>();
export function rememberBuild(root: string, result: BuildResult): void {
  lastBuilds.set(root, { result, at: Date.now() });
}
export function lastBuild(root: string): { result: BuildResult; at: number } | undefined {
  return lastBuilds.get(root);
}

/** The output as the dialog shows it: relative to the folder when it is inside. */
export function shownOut(root: string, out: string): string {
  return root && out.startsWith(root + "/") ? out.slice(root.length + 1) : out;
}
function absoluteOut(root: string, text: string): string {
  let t = text.trim().replace(/^\.\//, "");
  if (t === "") t = "_site";
  if (t === ".") return root;
  return t.startsWith("/") || t.startsWith("~") ? t : `${root}/${t}`;
}
function took(ms: number): string {
  return ms < 1000 ? `${Math.max(0, Math.round(ms))} ms` : `${(ms / 1000).toFixed(1)} s`;
}

let seq = 0;

export function openBuildSite(opts: BuildSiteOptions): BuildSiteDialog {
  const { api, doc, root } = opts;
  const id = ++seq;
  const backdrop = el("div", "zen-buildsite-backdrop");
  const form = el("form", "zen-buildsite");
  form.setAttribute("role", "dialog");
  form.setAttribute("aria-modal", "true");
  form.noValidate = true;
  const title = el("h2", "zen-buildsite-title", "Build Website");
  title.id = `zen-buildsite-title-${id}`;
  if (opts.name) title.append(" ", el("span", "zen-buildsite-folder", opts.name));
  form.setAttribute("aria-labelledby", title.id);

  // ---- the options
  const options = el("div", "zen-buildsite-options");
  const outLabel = el("label", "zen-buildsite-label", "Output folder");
  const out = el("input", "zen-buildsite-out");
  out.type = "text";
  out.id = `zen-buildsite-out-${id}`;
  out.autocomplete = "off";
  out.spellcheck = false;
  out.value = "_site";
  outLabel.htmlFor = out.id;
  const choose = button("Choose…", () => void pick());
  const outRow = el("div", "zen-buildsite-row");
  outRow.append(out, choose);

  const opensLabel = el("p", "zen-buildsite-label", "Opens from");
  const opens = el("div", "zen-buildsite-opens");
  opens.setAttribute("role", "radiogroup");
  opens.setAttribute("aria-labelledby", (opensLabel.id = `zen-buildsite-opens-${id}`));
  const radio = (value: string, label: string, hint: string) => {
    const l = el("label", "zen-buildsite-choice");
    const i = el("input");
    i.type = "radio";
    i.name = `zen-buildsite-opens-${id}`;
    i.value = value;
    l.append(i, el("span", "zen-buildsite-choice-name", label), el("span", "zen-buildsite-hint", hint));
    return { l, i };
  };
  const portable = radio("portable", "Disk and web servers", "Opens by double-clicking index.html, and from any web server.");
  const hosted = radio("hosted", "A web server only", "Scripts load as files, which browsers block from disk.");
  opens.append(portable.l, hosted.l);

  const check = (name: string, label: string) => {
    const l = el("label", "zen-buildsite-check");
    const i = el("input");
    i.type = "checkbox";
    i.name = name;
    l.append(i, el("span", "", label));
    return { l, i };
  };
  const drafts = check("drafts", "Include drafts");
  const clean = check("clean", "Start clean (empties the output folder first)");
  const print = check("print", "Also write print.html");
  options.append(outLabel, outRow, opensLabel, opens, drafts.l, clean.l, print.l);

  const pages = (opts.unsaved?.() ?? []).filter((p) => p !== WORKSPACE_CONFIG && !p.startsWith("zen:"));
  const note = el("p", "zen-buildsite-note");
  if (pages.length > 0) note.textContent = `${plural(pages.length, "page has", "pages have")} unsaved changes; the website uses the saved files.`;

  // ---- progress, preview, result
  const progress = el("div", "zen-buildsite-progress");
  progress.hidden = true;
  const bar = el("div", "zen-buildsite-bar");
  bar.setAttribute("role", "progressbar");
  bar.setAttribute("aria-label", "Building the website");
  bar.setAttribute("aria-valuemin", "0");
  bar.setAttribute("aria-valuenow", "0");
  const fill = el("div", "zen-buildsite-bar-fill");
  bar.append(fill);
  const progressText = el("p", "zen-buildsite-progress-text", "Building…");
  progress.append(bar, progressText);

  const preview = el("div", "zen-buildsite-preview-box");
  preview.hidden = true;
  const result = el("div", "zen-buildsite-result");
  result.hidden = true;
  const error = el("p", "zen-buildsite-error");
  error.setAttribute("role", "alert");
  const status = el("p", "zen-buildsite-status");
  status.setAttribute("role", "status");

  // ---- the footer
  const footer = el("div", "zen-buildsite-actions-form");
  const saveLink = button("Save as folder defaults", () => void saveDefaults(), { className: "zen-buildsite-link" });
  const cancel = button("Cancel", () => stop());
  const build = el("button", "primary", "Build");
  build.type = "submit";
  const spacer = el("span", "zen-buildsite-spacer");
  footer.append(saveLink, spacer, cancel, build);

  form.append(title, options, ...(pages.length > 0 ? [note] : []), progress, preview, result, error, status, footer);
  backdrop.append(form);
  doc.body.append(backdrop);
  const before = doc.activeElement as HTMLElement | null;
  build.focus();

  const inputs = [out, portable.i, hosted.i, drafts.i, clean.i, print.i];
  let state: "form" | "building" | "result" = "form";
  let closed = false;
  let cancelling = false;
  let lastTotal = 0;

  function lock(on: boolean): void {
    for (const i of inputs) i.disabled = on;
    choose.disabled = saveLink.disabled = on;
  }
  function read(): BuildOptions {
    return { out: absoluteOut(root, out.value), portable: portable.i.checked, drafts: drafts.i.checked, clean: clean.i.checked, print: print.i.checked };
  }
  function fill_(o: BuildOptions): void {
    out.value = shownOut(root, o.out);
    portable.i.checked = o.portable;
    hosted.i.checked = !o.portable;
    drafts.i.checked = o.drafts;
    clean.i.checked = o.clean;
    print.i.checked = o.print;
  }
  fill_({ out: `${root}/_site`, portable: true, drafts: false, clean: false, print: true });
  const ready = api.buildOptions().then(fill_, (err) => { if (!closed) error.textContent = message(err); });

  async function pick(): Promise<void> {
    try {
      const dir = await api.pickBuildFolder();
      if (dir && !closed) out.value = shownOut(root, dir);
    } catch (err) {
      error.textContent = message(err);
    }
  }

  // ---- save as folder defaults: preview, then confirm
  async function saveDefaults(): Promise<void> {
    error.textContent = status.textContent = "";
    if ((opts.unsaved?.() ?? []).includes(WORKSPACE_CONFIG)) {
      error.textContent = ".zen.yml has unsaved changes in an editor; save or close it first, then try again.";
      return;
    }
    const o = read();
    let text: string;
    try {
      text = await api.saveBuildDefaults(o, false);
    } catch (err) {
      error.textContent = message(err);
      return;
    }
    const pre = el("pre", "zen-buildsite-preview", text);
    const row = el("div", "zen-buildsite-preview-actions");
    const decline = button("Cancel changes", () => { preview.hidden = true; preview.replaceChildren(); saveLink.focus(); });
    const write = button("Write .zen.yml", async () => {
      write.disabled = decline.disabled = true;
      try {
        await api.saveBuildDefaults(o, true);
        preview.hidden = true;
        preview.replaceChildren();
        status.textContent = "Saved to .zen.yml.";
      } catch (err) {
        error.textContent = message(err);
        write.disabled = decline.disabled = false;
      }
    }, { className: "primary" });
    row.append(decline, write);
    preview.replaceChildren(pre, row);
    preview.hidden = false;
    write.focus();
  }

  // ---- building
  const offProgress = api.on("build:progress", (p) => {
    if (state !== "building") return;
    lastTotal = p.total;
    progressText.textContent = `Building… ${p.done} / ${p.total} pages`;
    bar.setAttribute("aria-valuemax", String(p.total));
    bar.setAttribute("aria-valuenow", String(p.done));
    fill.style.width = p.total > 0 ? `${Math.min(100, Math.round((p.done / p.total) * 100))}%` : "0%";
  });

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (state !== "form") return;
    error.textContent = status.textContent = "";
    preview.hidden = true;
    await ready;
    if (closed) return;
    state = "building";
    cancelling = false;
    lastTotal = 0;
    lock(true);
    build.disabled = true;
    progress.hidden = false;
    progressText.textContent = "Building…";
    bar.setAttribute("aria-valuenow", "0");
    bar.removeAttribute("aria-valuemax");
    fill.style.width = "0%";
    try {
      const r = await api.buildWebsite(read());
      if (closed) return;
      rememberBuild(root, r);
      showResult(r);
    } catch (err) {
      if (closed) return;
      state = "form";
      lock(false);
      build.disabled = false;
      progress.hidden = true;
      error.textContent = message(err);
      build.focus();
    }
  });

  function stop(): void {
    if (state === "building") {
      if (cancelling) return;
      cancelling = true;
      progressText.textContent = "Cancelling…";
      void api.cancelBuild().catch((err) => { error.textContent = message(err); });
    } else close();
  }

  function showResult(r: BuildResult): void {
    state = "result";
    progress.hidden = true;
    options.hidden = footer.hidden = true;
    const summary = el("p", "zen-buildsite-summary");
    summary.setAttribute("role", "status");
    if (r.cancelled) {
      const of = lastTotal > 0 ? ` of ${lastTotal}` : "";
      summary.textContent = `Cancelled after ${r.pages}${of} pages; what was written is kept, and the folder can be built again.`;
    } else {
      const n = r.warnings.length;
      summary.textContent = `Built ${plural(r.pages, "page", "pages")} and ${plural(r.files, "file", "files")} into ${shownOut(root, r.out)}`
        + (n > 0 ? ` (${plural(n, "warning", "warnings")})` : "") + `, in ${took(r.millis)}.`;
    }
    const parts: HTMLElement[] = [summary];
    if (r.warnings.length > 0) {
      const list = el("ul", "zen-buildsite-warnings");
      for (const w of r.warnings) {
        const li = el("li", "zen-buildsite-warning");
        if (w.path) {
          const b = el("button", "zen-buildsite-warning-open");
          b.type = "button";
          b.append(el("span", "zen-buildsite-warning-page", w.path), el("span", "zen-buildsite-warning-text", w.message));
          b.addEventListener("click", () => { close(); opts.onOpenPage(w.path); });
          li.append(b);
        } else {
          li.append(el("span", "zen-buildsite-warning-text", w.message));
        }
        list.append(li);
      }
      parts.push(list);
    }
    const act = (label: string, fn: () => Promise<void>, cls = "") => button(label, () => { error.textContent = ""; fn().catch((err) => { error.textContent = message(err); }); }, { className: cls });
    const open = act("Open in Browser", () => api.openBuiltSite(r.out), "primary");
    const reveal = act("Show in Finder", () => api.revealBuiltSite(r.out));
    const again = button("Build Again", () => {
      state = "form";
      result.hidden = true;
      result.replaceChildren();
      options.hidden = footer.hidden = false;
      lock(false);
      build.disabled = false;
      error.textContent = "";
      build.focus();
    });
    const done = button("Done", () => close());
    const actions = el("div", "zen-buildsite-actions");
    actions.append(open, reveal, again, done);
    parts.push(actions);
    result.replaceChildren(...parts);
    result.hidden = false;
    open.focus();
  }

  // Esc cancels a build (or closes), Tab stays inside, a click outside closes when idle.
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      stop();
    } else if (e.key === "Tab") {
      const items = Array.from(form.querySelectorAll<HTMLElement>("input, button")).filter((x) => !(x as HTMLButtonElement).disabled && x.tabIndex >= 0 && !x.closest("[hidden]"));
      if (items.length === 0) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (e.shiftKey && doc.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && doc.activeElement === last) { e.preventDefault(); first.focus(); }
    }
  };
  doc.addEventListener("keydown", onKey, true);
  backdrop.addEventListener("mousedown", (e) => { if (e.target === backdrop && state !== "building") close(); });

  function close(): void {
    if (closed) return;
    closed = true;
    offProgress();
    doc.removeEventListener("keydown", onKey, true);
    backdrop.remove();
    if (before?.isConnected) before.focus();
    opts.onClose?.();
  }

  function focus(): void {
    if (state === "building") cancel.focus();
    else if (state === "result") result.querySelector<HTMLElement>(".primary")?.focus();
    else build.focus();
  }

  return { el: backdrop, close, focus };
}

/**
 * Listens for File › Build Website… and shows the dialog: once at a time (a
 * second event brings it to the front), and not without a folder.
 */
export function registerBuildSite(opts: {
  api: BuildApi;
  doc: Document;
  root(): string;
  name(): string;
  onOpenPage(path: string): void;
  unsaved?(): string[];
}): { open(): void; destroy(): void } {
  let dialog: BuildSiteDialog | null = null;
  function open(): void {
    if (dialog) { dialog.focus(); return; }
    const root = opts.root();
    if (!root) return;
    dialog = openBuildSite({ api: opts.api, doc: opts.doc, root, name: opts.name(), onOpenPage: opts.onOpenPage, unsaved: opts.unsaved, onClose: () => { dialog = null; } });
  }
  const off = opts.api.on("menu:build-website", open);
  return { open, destroy() { off(); dialog?.close(); } };
}

// ---- Settings › Website

const OPENS: Record<string, string> = { true: "Disk and web servers", false: "A web server only" };

/** The Website section: the folder's build options, a Build… button and the last build's result. */
export function createWebsiteSection(opts: {
  api: Partial<BuildApi>;
  root(): string;
  openBuild?(): void;
}): { el: HTMLElement; refresh(): Promise<void> } {
  const box = el("div", "zen-settings-website");
  const summary = el("p", "zen-settings-website-options");
  const lastLine = el("p", "zen-settings-build-last");
  const b = el("button", "zen-settings-build primary", "Build…");
  b.type = "button";
  b.addEventListener("click", () => opts.openBuild?.());
  box.append(summary, lastLine, b);
  async function refresh(): Promise<void> {
    const root = opts.root();
    b.disabled = !root || !opts.openBuild;
    if (!root) {
      summary.textContent = "Open a folder to build it as a website.";
      lastLine.textContent = "";
      return;
    }
    const l = lastBuild(root);
    lastLine.textContent = l
      ? `Last build: ${plural(l.result.pages, "page", "pages")} into ${shownOut(root, l.result.out)}, ${new Date(l.at).toLocaleTimeString()}.`
      : "No build yet.";
    if (!opts.api.buildOptions) return;
    try {
      const o = await opts.api.buildOptions();
      summary.textContent = `Output ${shownOut(root, o.out)}. ${OPENS[String(o.portable)]}. ${o.drafts ? "Drafts included" : "Drafts left out"}.`;
    } catch (err) {
      summary.textContent = message(err);
    }
  }
  return { el: box, refresh };
}
