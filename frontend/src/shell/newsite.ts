// File › New Site from Template… (⇧⌘N) and the welcome screen's template
// cards (core/init): a modal that picks a site template, asks for its values,
// lets the user choose a folder in the native panel, and creates the site
// through api.createSite. A pattern like newpage.ts.
//
// The api methods are declared here (SiteApi) until api/ lists them; the real
// Api satisfies this type once it has siteTemplates, pickSiteFolder,
// createSite, revealTemplates and the "menu:new-site" event.
import type { Api, SiteTemplate, Startup } from "../api";
import { button, el } from "./dom";

/** A site template as the Go service lists it. An invalid one has `error` set. */
export type { SiteTemplate } from "../api";

export type SiteApi = Pick<Api, "on"> & {
  siteTemplates(): Promise<SiteTemplate[]>;
  /** The templates folder, for the empty state (optional for older apis). */
  templatesDir?(): Promise<string>;
  /** The native folder panel with New Folder enabled; "" when cancelled. */
  pickSiteFolder(): Promise<string>;
  /** Creates the site in dir (nothing is overwritten) and opens it. */
  createSite(id: string, dir: string, values: Record<string, string>): Promise<Startup>;
  /** Shows the templates folder in Finder. */
  revealTemplates(): Promise<void>;
  on(event: "menu:new-site", fn: () => void): () => void;
};

export type NewSiteOptions = {
  api: SiteApi;
  doc: Document;
  /** Called with what to show once the site is created and opened. */
  onCreated(s: Startup): void;
  /** Called once when the dialog goes away, however it ends. */
  onClose?(): void;
  /** The template that starts chosen (when it exists and is valid). */
  preselect?: string;
};

export type NewSiteDialog = { el: HTMLElement; close(): void };

let seq = 0;

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));
const today = () => new Date().toISOString().slice(0, 10);
const baseName = (p: string) => p.replace(/[\\/]+$/, "").split(/[\\/]/).pop() ?? "";

/** An ask default for a folder: {{folder}} and {{date}} are filled in. */
export function resolveDefault(d: string, folder: string): string {
  return d.replaceAll("{{folder}}", folder).replaceAll("{{date}}", today());
}

export function openNewSite(opts: NewSiteOptions): NewSiteDialog {
  const { api, doc } = opts;
  const id = ++seq;
  const backdrop = el("div", "zen-newsite-backdrop");
  const form = el("form", "zen-newsite");
  form.setAttribute("role", "dialog");
  form.setAttribute("aria-modal", "true");
  form.noValidate = true;
  const title = el("h2", "zen-newsite-title", "New site from a template");
  title.id = `zen-newsite-title-${id}`;
  form.setAttribute("aria-labelledby", title.id);

  const listLabel = el("p", "zen-newsite-label", "Template");
  listLabel.id = `zen-newsite-templates-${id}`;
  const status = el("p", "zen-newsite-status", "Loading templates…");
  status.setAttribute("role", "status");
  const group = el("div", "zen-newsite-templates");
  group.setAttribute("role", "radiogroup");
  group.setAttribute("aria-labelledby", listLabel.id);
  group.hidden = true;
  const empty = el("div", "zen-newsite-empty");
  empty.hidden = true;
  const fields = el("div", "zen-newsite-fields");

  const folderLabel = el("p", "zen-newsite-label", "Folder");
  const folderRow = el("div", "zen-newsite-folder");
  const choose = button("Choose folder…", () => void pickFolder());
  const pathText = el("p", "zen-newsite-path", "No folder chosen");
  pathText.setAttribute("aria-live", "polite");
  folderRow.append(choose, pathText);
  const hint = el("p", "zen-newsite-hint", "A new or an existing folder. Nothing in it is overwritten.");
  const error = el("p", "zen-newsite-error");
  error.setAttribute("role", "alert");

  const reveal = button("Open templates folder", () => void revealFolder(), { className: "zen-newsite-link" });
  const actions = el("div", "zen-newsite-actions");
  const cancel = button("Cancel", () => close());
  const create = el("button", "primary", "Create");
  create.type = "submit";
  create.disabled = true;
  actions.append(reveal, cancel, create);

  form.append(title, listLabel, status, group, empty, fields, folderLabel, folderRow, hint, error, actions);
  backdrop.append(form);
  doc.body.append(backdrop);
  const before = doc.activeElement as HTMLElement | null;
  cancel.focus();

  let templates: SiteTemplate[] = [];
  let cards: HTMLButtonElement[] = [];
  let chosen: SiteTemplate | null = null;
  let dir = "";
  let busy = false;
  let closed = false;
  /** Keys of the fields the user typed in: their values are kept. */
  const touched = new Set<string>();
  const inputs = new Map<string, HTMLInputElement>();

  const usable = () => templates.filter((t) => !t.error);
  const folderName = () => (dir ? baseName(dir) : "");
  const refreshCreate = () => { create.disabled = busy || !chosen || dir === ""; };
  const fail = (err: unknown) => { error.textContent = message(err); };

  function drawTemplates(): void {
    cards = templates.map((t) => {
      const b = el("button", "zen-newsite-template");
      b.type = "button";
      b.setAttribute("role", "radio");
      b.dataset.template = t.id;
      b.append(el("span", "zen-newsite-template-name", t.name), el("span", "zen-newsite-template-hint", t.description));
      if (t.error) {
        b.setAttribute("aria-disabled", "true");
        b.append(el("span", "zen-newsite-template-error", t.error));
      }
      b.addEventListener("click", () => { if (!t.error) choose_(t, false); });
      return b;
    });
    group.replaceChildren(...cards);
  }

  function choose_(t: SiteTemplate | null, focus: boolean): void {
    if (chosen !== t) { touched.clear(); error.textContent = ""; }
    chosen = t;
    cards.forEach((c, i) => {
      const on = templates[i] === t;
      c.setAttribute("aria-checked", String(on));
      c.tabIndex = on || (t === null && templates[i] === usable()[0]) ? 0 : -1;
      if (on && focus) c.focus();
    });
    drawFields();
    refreshCreate();
  }

  function drawFields(): void {
    inputs.clear();
    const rows: HTMLElement[] = [];
    for (const a of chosen?.ask ?? []) {
      const row = el("div", "zen-newsite-field");
      const label = el("label", "zen-newsite-label", a.label || a.key);
      const input = el("input", "zen-newsite-input");
      input.type = "text";
      input.name = a.key;
      input.id = `zen-newsite-${id}-${a.key}`;
      input.autocomplete = "off";
      input.spellcheck = false;
      input.value = resolveDefault(a.default, folderName());
      if (a.default.includes("{{folder}}")) input.placeholder = "The folder's name";
      label.htmlFor = input.id;
      input.addEventListener("input", () => touched.add(a.key));
      inputs.set(a.key, input);
      row.append(label, input);
      rows.push(row);
    }
    fields.replaceChildren(...rows);
  }

  /** A folder was chosen: the fields whose default uses the folder's name follow it. */
  function refreshDefaults(): void {
    for (const a of chosen?.ask ?? []) {
      const input = inputs.get(a.key);
      if (input && !touched.has(a.key)) input.value = resolveDefault(a.default, folderName());
    }
  }

  group.addEventListener("keydown", (e) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (step === undefined) return;
    const ok = usable();
    if (ok.length === 0) return;
    e.preventDefault();
    const i = chosen ? ok.indexOf(chosen) : -1;
    choose_(ok[(i + step + ok.length) % ok.length], true);
  });

  async function pickFolder(): Promise<void> {
    if (busy) return;
    error.textContent = "";
    try {
      const picked = await api.pickSiteFolder();
      if (closed || !picked) return;
      dir = picked;
      pathText.textContent = picked;
      refreshDefaults();
      refreshCreate();
    } catch (err) {
      if (!closed) fail(err);
    }
  }

  async function revealFolder(): Promise<void> {
    try {
      await api.revealTemplates();
    } catch (err) {
      if (!closed) fail(err);
    }
  }

  function showList(all: SiteTemplate[]): void {
    templates = all;
    status.remove();
    if (all.length === 0) {
      empty.hidden = false;
      const msg = el("p", "", "No templates in the templates folder.");
      empty.replaceChildren(msg, button("Reveal in Finder", () => void revealFolder()));
      void opts.api.templatesDir?.().then((dir) => { if (dir) msg.textContent = `No templates in ${dir}.`; }, () => {});
      return;
    }
    group.hidden = false;
    drawTemplates();
    const first = usable()[0] ?? null;
    const pre = usable().find((t) => t.id === opts.preselect) ?? first;
    choose_(pre, false);
    if (pre) {
      cards[templates.indexOf(pre)].tabIndex = 0;
      (inputs.values().next().value ?? choose).focus();
    }
  }

  api.siteTemplates().then((all) => {
    if (!closed) showList(all);
  }, (err) => {
    if (closed) return;
    status.remove();
    fail(`Could not list the templates: ${message(err)}`);
  });

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (busy || !chosen || !dir) return;
    busy = true;
    create.disabled = cancel.disabled = choose.disabled = true;
    error.textContent = "";
    const values: Record<string, string> = {};
    inputs.forEach((input, key) => { values[key] = input.value.trim(); });
    try {
      const started = await api.createSite(chosen.id, dir, values);
      close();
      opts.onCreated(started);
    } catch (err) {
      fail(err);
      busy = false;
      cancel.disabled = choose.disabled = false;
      refreshCreate();
    }
  });

  // Esc closes, Tab stays inside, a click outside closes.
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      if (!busy) close();
    } else if (e.key === "Tab") {
      const items = Array.from(form.querySelectorAll<HTMLElement>("input, button:not([disabled])")).filter((x) => x.tabIndex >= 0 && !x.hidden);
      const first = items[0];
      const last = items[items.length - 1];
      if (!first) return;
      if (e.shiftKey && doc.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && doc.activeElement === last) { e.preventDefault(); first.focus(); }
    }
  };
  doc.addEventListener("keydown", onKey, true);
  backdrop.addEventListener("mousedown", (e) => { if (e.target === backdrop && !busy) close(); });

  function close(): void {
    if (closed) return;
    closed = true;
    doc.removeEventListener("keydown", onKey, true);
    backdrop.remove();
    if (before?.isConnected) before.focus();
    opts.onClose?.();
  }

  return { el: backdrop, close };
}
