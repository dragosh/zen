// Settings › Export › Website (ui/settings-layout): the form that builds the open folder as a static
// website through the build services, in place in the page (no modal), with progress, Cancel and a
// result. File › Export › Website… (⇧⌘B) opens Settings there (registerBuildSite). All text goes in
// through textContent.
import "./buildsite.css";
import type { BuildOptions, BuildProgress, BuildResult } from "../api";
import { button, el, hasFeature, type BuildInfo } from "@zenmd/kit";

export type { BuildOptions, BuildProgress, BuildResult, BuildWarning } from "../api";

/** The services behind the dialog (apps/zenmark/buildui.go); `on` is the event side of the api. */
export interface BuildApi {
  /** The assembly of the build; without the editor feature there is no Save as folder defaults. Absent: everything. */
  build?: BuildInfo;
  buildOptions(): Promise<BuildOptions>;
  buildWebsite(o: BuildOptions): Promise<BuildResult>;
  cancelBuild(): Promise<void>;
  /** Without confirm: the preview text, nothing written. With it: writes .zenmd.yml and returns the same text. */
  saveBuildDefaults(o: BuildOptions, confirm: boolean): Promise<string>;
  /** The native folder panel; "" when cancelled. */
  pickBuildFolder(): Promise<string>;
  openBuiltSite(out: string): Promise<void>;
  revealBuiltSite(out: string): Promise<void>;
  on(event: "build:progress", fn: (p: BuildProgress) => void): () => void;
  on(event: "menu:build-website", fn: () => void): () => void;
}

const WORKSPACE_CONFIG = "zen:config/workspace";
const message = (err: unknown) => (err instanceof Error ? err.message : String(err));
const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/** What the last build of each folder did, for Settings › Export. */
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
  if (t === "") t = "exports/html";
  if (t === ".") return root;
  return t.startsWith("/") || t.startsWith("~") ? t : `${root}/${t}`;
}
function took(ms: number): string {
  return ms < 1000 ? `${Math.max(0, Math.round(ms))} ms` : `${(ms / 1000).toFixed(1)} s`;
}

export type BuildFormOptions = {
  api: BuildApi;
  doc: Document;
  /** The open folder's path, "" for none. */
  root(): string;
  /** Opens a page in the main window. */
  onOpenPage(path: string): void;
  /** Paths with unsaved changes in editor windows ("zen:config/workspace" is .zenmd.yml); the website uses the saved files. */
  unsaved?(): string[];
};
export type BuildForm = {
  el: HTMLFormElement;
  /** Reads the folder's build options into the form. A build that runs, or a result on show, stays unless the folder changed or force is set (and nothing runs). */
  refresh(force?: boolean): Promise<void>;
  focus(): void;
  /** A build is running. */
  busy(): boolean;
  destroy(): void;
};

let seq = 0;

export function createBuildForm(opts: BuildFormOptions): BuildForm {
  const { api, doc } = opts;
  const id = ++seq;
  const form = el("form", "zen-buildform");
  form.noValidate = true;

  // ---- the options
  const options = el("div", "zen-buildsite-options");
  const outLabel = el("label", "zen-buildsite-label", "Output folder");
  const out = el("input", "zen-buildsite-out");
  out.type = "text";
  out.id = `zen-buildsite-out-${id}`;
  out.autocomplete = "off";
  out.spellcheck = false;
  out.value = "exports/html";
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

  const note = el("p", "zen-buildsite-note");
  const none = el("p", "zen-buildsite-note", "Open a folder to export it.");
  none.hidden = true;
  const last = el("p", "zen-buildsite-note zen-settings-build-last");

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
  const cancel = button("Cancel", () => stop());
  progress.append(bar, progressText, cancel);

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
  const build = el("button", "primary", "Export");
  build.type = "submit";
  const spacer = el("span", "zen-buildsite-spacer");
  // A read-only app never writes into the folder (.zenmd.yml): no Save as folder defaults.
  footer.append(...(hasFeature(api, "editor") ? [saveLink] : []), spacer, build);

  form.append(options, note, none, last, progress, preview, result, error, status, footer);

  const inputs = [out, portable.i, hosted.i, drafts.i, clean.i, print.i];
  let state: "form" | "building" | "result" = "form";
  let destroyed = false;
  let cancelling = false;
  let lastTotal = 0;
  let filledFor: string | null = null;
  let ready: Promise<void> = Promise.resolve();
  const rootNow = () => opts.root();

  function lock(on: boolean): void {
    for (const i of inputs) i.disabled = on;
    choose.disabled = saveLink.disabled = on;
  }
  function read(): BuildOptions {
    return { out: absoluteOut(rootNow(), out.value), portable: portable.i.checked, drafts: drafts.i.checked, clean: clean.i.checked, print: print.i.checked };
  }
  function fill_(o: BuildOptions): void {
    out.value = shownOut(rootNow(), o.out);
    portable.i.checked = o.portable;
    hosted.i.checked = !o.portable;
    drafts.i.checked = o.drafts;
    clean.i.checked = o.clean;
    print.i.checked = o.print;
  }
  function showUnsaved(): void {
    const pages = (opts.unsaved?.() ?? []).filter((p) => p !== WORKSPACE_CONFIG && !p.startsWith("zen:"));
    note.textContent = pages.length > 0 ? `${plural(pages.length, "page has", "pages have")} unsaved changes; the website uses the saved files.` : "";
  }

  function reset(): void {
    state = "form";
    result.hidden = true;
    result.replaceChildren();
    progress.hidden = true;
    options.hidden = footer.hidden = false;
    lock(false);
    build.disabled = false;
    error.textContent = status.textContent = "";
    preview.hidden = true;
    preview.replaceChildren();
  }

  async function refresh(force = false): Promise<void> {
    const root = rootNow();
    showUnsaved();
    none.hidden = root !== "";
    options.hidden = footer.hidden = root === "" || state === "result";
    if (root === "") {
      build.disabled = true;
      filledFor = null;
      last.textContent = "";
      return;
    }
    const l = lastBuild(root);
    last.textContent = state === "form" && l ? `Last build: ${plural(l.result.pages, "page", "pages")} into ${shownOut(root, l.result.out)}, ${new Date(l.at).toLocaleTimeString()}.` : "";
    if (state === "building") return;
    if (state === "result" && root === filledFor && !force) return;
    if (state === "result") reset();
    options.hidden = footer.hidden = false;
    build.disabled = false;
    if (root === filledFor && !force) return;
    filledFor = root;
    fill_({ out: `${root}/exports/html`, portable: true, drafts: false, clean: false, print: true });
    ready = api.buildOptions().then(fill_, (err) => { if (!destroyed) error.textContent = message(err); });
    await ready;
  }

  async function pick(): Promise<void> {
    try {
      const dir = await api.pickBuildFolder();
      if (dir && !destroyed) out.value = shownOut(rootNow(), dir);
    } catch (err) {
      error.textContent = message(err);
    }
  }

  // ---- save as folder defaults: preview, then confirm
  async function saveDefaults(): Promise<void> {
    error.textContent = status.textContent = "";
    if ((opts.unsaved?.() ?? []).includes(WORKSPACE_CONFIG)) {
      error.textContent = ".zenmd.yml has unsaved changes in an editor; save or close it first, then try again.";
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
    const write = button("Write .zenmd.yml", async () => {
      write.disabled = decline.disabled = true;
      try {
        await api.saveBuildDefaults(o, true);
        preview.hidden = true;
        preview.replaceChildren();
        status.textContent = "Saved to .zenmd.yml.";
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
    if (state !== "form" || rootNow() === "") return;
    error.textContent = status.textContent = "";
    preview.hidden = true;
    await ready;
    if (destroyed) return;
    const root = rootNow();
    state = "building";
    cancelling = false;
    lastTotal = 0;
    lock(true);
    build.disabled = true;
    last.textContent = "";
    progress.hidden = false;
    progressText.textContent = "Building…";
    bar.setAttribute("aria-valuenow", "0");
    bar.removeAttribute("aria-valuemax");
    fill.style.width = "0%";
    try {
      const r = await api.buildWebsite(read());
      rememberBuild(root, r);
      if (destroyed) return;
      showResult(r, root);
    } catch (err) {
      if (destroyed) return;
      state = "form";
      lock(false);
      build.disabled = false;
      progress.hidden = true;
      error.textContent = message(err);
      build.focus();
    }
  });

  function stop(): void {
    if (state !== "building" || cancelling) return;
    cancelling = true;
    progressText.textContent = "Cancelling…";
    void api.cancelBuild().catch((err) => { error.textContent = message(err); });
  }

  function showResult(r: BuildResult, root: string): void {
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
          b.addEventListener("click", () => opts.onOpenPage(w.path));
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
    const again = button("Build Again", () => { reset(); build.focus(); });
    const actions = el("div", "zen-buildsite-actions");
    actions.append(open, reveal, again);
    parts.push(actions);
    result.replaceChildren(...parts);
    result.hidden = false;
    if (form.contains(doc.activeElement) || doc.activeElement === doc.body) open.focus();
  }

  // Esc cancels a build that runs; otherwise it is left to Settings (which leaves the page).
  form.addEventListener("keydown", (e) => {
    if (e.key !== "Escape" || state !== "building") return;
    e.preventDefault();
    e.stopPropagation();
    stop();
  });

  return {
    el: form,
    refresh,
    focus() {
      if (state === "building") cancel.focus();
      else if (state === "result") result.querySelector<HTMLElement>(".primary")?.focus();
      else build.focus();
    },
    busy: () => state === "building",
    destroy() {
      destroyed = true;
      offProgress();
      form.remove();
    },
  };
}

/**
 * Listens for File › Export › Website… (⇧⌘B) and asks the shell to show Settings › Export › Website
 * (`showExport`): not without a folder.
 */
export function registerBuildSite(opts: {
  api: BuildApi;
  doc?: Document;
  root(): string;
  name?(): string;
  onOpenPage?(path: string): void;
  unsaved?(): string[];
  showExport?(): void;
}): { open(): void; destroy(): void } {
  function open(): void {
    if (!opts.root()) return;
    opts.showExport?.();
  }
  const off = opts.api.on("menu:build-website", open);
  return { open, destroy() { off(); } };
}
