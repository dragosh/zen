// The parts the two forms of Settings › Export share (the website's, buildsite.ts, and the PDF's, pdfform.ts): the
// field helpers (path with Choose…, check, radio), the frame of the form (options, notes, progress, preview, result,
// the buttons with the primary one first and Save as folder defaults last, then the error and the status line) and the
// Save as folder defaults flow (preview the change, then write). Both forms look and read the same because they are
// built here. All text goes in through textContent.
import "./buildsite.css";
import { button, el } from "@zenmd/kit";

/** The path of the open folder's .zenmd.yml in an editor window, as the shell names it. */
export const WORKSPACE_CONFIG = "zen:config/workspace";

export const message = (err: unknown) => (err instanceof Error ? err.message : String(err));
export const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/** The path as the form shows it: relative to the folder when it is inside. */
export function shownOut(root: string, out: string): string {
  return root && out.startsWith(root + "/") ? out.slice(root.length + 1) : out;
}

let seq = 0;
/** A number that makes the ids of one form unique. */
export const nextId = () => ++seq;

/** A check box with its label. */
export function check(name: string, label: string) {
  const l = el("label", "zen-buildsite-check");
  const i = el("input");
  i.type = "checkbox";
  i.name = name;
  l.append(i, el("span", "", label));
  return { l, i };
}

/** A path field with its label and Choose…. */
export function pathField(id: number, label: string, initial: string, choose: () => void) {
  const l = el("label", "zen-buildsite-label", label);
  const input = el("input", "zen-buildsite-out");
  input.type = "text";
  input.id = `zen-buildsite-out-${id}`;
  input.autocomplete = "off";
  input.spellcheck = false;
  input.value = initial;
  l.htmlFor = input.id;
  const pick = button("Choose…", choose);
  const row = el("div", "zen-buildsite-row");
  row.append(input, pick);
  return { label: l, input, choose: pick, row };
}

/** A group of radio buttons under a label; hint is the small line under a choice. */
export function radioGroup(id: number, name: string, label: string, choices: { value: string; label: string; hint?: string }[]) {
  const l = el("p", "zen-buildsite-label", label);
  const group = el("div", "zen-buildsite-opens");
  group.setAttribute("role", "radiogroup");
  group.setAttribute("aria-labelledby", (l.id = `zen-buildsite-${name}-${id}`));
  const radios = choices.map((c) => {
    const lab = el("label", "zen-buildsite-choice");
    const i = el("input");
    i.type = "radio";
    i.name = `zen-buildsite-${name}-${id}`;
    i.value = c.value;
    lab.append(i, el("span", "zen-buildsite-choice-name", c.label));
    if (c.hint) lab.append(el("span", "zen-buildsite-hint", c.hint));
    group.append(lab);
    return i;
  });
  return { label: l, group, radios };
}

export type ExportShellOptions = {
  /** The form's accessible name for the progress bar, "Building the website". */
  progressLabel: string;
  /** The first words of the progress line. */
  progressText: string;
  /** The primary button's label; it submits the form. */
  primary: string;
  /** Buttons after the primary one (Preview…). */
  secondary?: HTMLButtonElement[];
  /** Save as folder defaults is there (the build has the editor). */
  canSave: boolean;
  onSave(): void;
  /** Cancel in the progress block; without it there is none. */
  onCancel?(): void;
};

export type ExportShell = {
  form: HTMLFormElement;
  options: HTMLElement;
  note: HTMLElement;
  none: HTMLElement;
  last: HTMLElement;
  progress: HTMLElement;
  bar: HTMLElement;
  fill: HTMLElement;
  progressText: HTMLElement;
  cancel: HTMLButtonElement | null;
  preview: HTMLElement;
  result: HTMLElement;
  error: HTMLElement;
  status: HTMLElement;
  footer: HTMLElement;
  primary: HTMLButtonElement;
  saveLink: HTMLButtonElement;
  /** Hides the options and buttons (a result is on show) or shows them. */
  showForm(on: boolean): void;
  /** Clears the preview of the change. */
  clearPreview(): void;
};

/** The frame of an export form: options go in `options` by the caller; everything else is built here, in one order. */
export function createShell(o: ExportShellOptions): ExportShell {
  const form = el("form", "zen-buildform");
  form.noValidate = true;
  const options = el("div", "zen-buildsite-options");
  const note = el("p", "zen-buildsite-note");
  const none = el("p", "zen-buildsite-note", "Open a folder to export it.");
  none.hidden = true;
  const last = el("p", "zen-buildsite-note zen-settings-build-last");

  const progress = el("div", "zen-buildsite-progress");
  progress.hidden = true;
  const bar = el("div", "zen-buildsite-bar");
  bar.setAttribute("role", "progressbar");
  bar.setAttribute("aria-label", o.progressLabel);
  bar.setAttribute("aria-valuemin", "0");
  bar.setAttribute("aria-valuenow", "0");
  const fill = el("div", "zen-buildsite-bar-fill");
  bar.append(fill);
  const progressText = el("p", "zen-buildsite-progress-text", o.progressText);
  progress.append(bar, progressText);
  let cancel: HTMLButtonElement | null = null;
  if (o.onCancel) {
    cancel = button("Cancel", o.onCancel);
    progress.append(cancel);
  }

  const preview = el("div", "zen-buildsite-preview-box");
  preview.hidden = true;
  const result = el("div", "zen-buildsite-result");
  result.hidden = true;

  // The buttons: the primary one first, then the others, and Save as folder defaults last.
  const footer = el("div", "zen-buildsite-actions-form");
  const primary = el("button", "primary", o.primary);
  primary.type = "submit";
  const saveLink = button("Save as folder defaults", o.onSave, { className: "zen-buildsite-link" });
  // A read-only app never writes into the folder (.zenmd.yml): no Save as folder defaults.
  footer.append(primary, ...(o.secondary ?? []), el("span", "zen-buildsite-spacer"), ...(o.canSave ? [saveLink] : []));

  // The error and the status line sit under the buttons.
  const error = el("p", "zen-buildsite-error");
  error.setAttribute("role", "alert");
  const status = el("p", "zen-buildsite-status");
  status.setAttribute("role", "status");

  form.append(options, note, none, last, progress, preview, result, footer, error, status);
  return {
    form, options, note, none, last, progress, bar, fill, progressText, cancel, preview, result, error, status, footer, primary, saveLink,
    showForm(on) { options.hidden = footer.hidden = !on; },
    clearPreview() { preview.hidden = true; preview.replaceChildren(); },
  };
}

/** Sets the progress bar to done of total pages; nothing known leaves it empty. */
export function setBar(s: ExportShell, done: number, total: number): void {
  if (total > 0) s.bar.setAttribute("aria-valuemax", String(total)); else s.bar.removeAttribute("aria-valuemax");
  s.bar.setAttribute("aria-valuenow", String(done));
  s.fill.style.width = total > 0 ? `${Math.min(100, Math.round((done / total) * 100))}%` : "0%";
}

/**
 * Save as folder defaults: asks for the preview of the change (nothing written), shows it with Cancel changes and
 * Write .zenmd.yml, and writes on the second. `save(confirm)` is the api call; `unsaved` says whether an editor
 * holds .zenmd.yml with changes (it would be overwritten).
 */
export async function saveDefaultsFlow(s: ExportShell, save: (confirm: boolean) => Promise<string>, unsaved: string[]): Promise<void> {
  s.error.textContent = s.status.textContent = "";
  if (unsaved.includes(WORKSPACE_CONFIG)) {
    s.error.textContent = ".zenmd.yml has unsaved changes in an editor; save or close it first, then try again.";
    return;
  }
  let text: string;
  try {
    text = await save(false);
  } catch (err) {
    s.error.textContent = message(err);
    return;
  }
  const pre = el("pre", "zen-buildsite-preview", text);
  const row = el("div", "zen-buildsite-preview-actions");
  const decline = button("Cancel changes", () => { s.clearPreview(); s.saveLink.focus(); });
  const write = button("Write .zenmd.yml", async () => {
    write.disabled = decline.disabled = true;
    try {
      await save(true);
      s.clearPreview();
      s.status.textContent = "Saved to .zenmd.yml.";
    } catch (err) {
      s.error.textContent = message(err);
      write.disabled = decline.disabled = false;
    }
  }, { className: "primary" });
  row.append(decline, write);
  s.preview.replaceChildren(pre, row);
  s.preview.hidden = false;
  write.focus();
}

/** A result button: runs fn, and shows its failure in the error line. */
export function actionButton(s: ExportShell, label: string, fn: () => Promise<void>, cls = ""): HTMLButtonElement {
  return button(label, () => { s.error.textContent = ""; fn().catch((err) => { s.error.textContent = message(err); }); }, { className: cls });
}
