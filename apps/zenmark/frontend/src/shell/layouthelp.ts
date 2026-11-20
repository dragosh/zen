// Settings › Layouts (ui/settings-layout): the list of layouts in the left column (name, with a mark only for
// layouts that are off, blocked, invalid or not built in) and the help of the selected layout in the right one:
// state, Allow / Block for external ones, description, the frontmatter fields it reads, its options, and
// snippets to copy, with a static preview for built-in layouts. Same pattern as Settings › Plugins
// (shell/pluginhelp.ts). All text goes in through textContent; the only HTML mounted is what api.preview
// returned, through the page's own mountPage.
import type { Api, LayoutInfo, PluginSnippet } from "../api";
import { button, copyText, el, mountPage } from "@zenmd/kit";
import { reloadSettings, type SettingsItem } from "@zenmd/kit/shell/settings";
import { exceptionNote, STATUS_LABEL } from "./pluginhelp";

export type LayoutsSection = {
  /** The rows of the list; reads the layouts again. */
  items(): Promise<SettingsItem[]>;
  /** The right column for a row. */
  render(item: SettingsItem, right: HTMLElement): () => void;
  /** What the right column says without rows (no layouts, or the list failed). */
  empty(): string;
  destroy(): void;
};

const SOURCE_LABEL: Record<LayoutInfo["source"], string> = { builtin: "Built in", folder: "This folder", user: "Your layouts" };
const PREVIEW_PATH = "zen-layout-help.md";
const message = (err: unknown) => (err instanceof Error ? err.message : String(err));
const fmt = (v: unknown) => (typeof v === "string" ? (v === "" ? '""' : v) : JSON.stringify(v) ?? "");

export function createLayoutsSection(opts: { api: Api }): LayoutsSection {
  const { api } = opts;
  let layouts: LayoutInfo[] = [];
  let failure = "";
  let helpSeq = 0; // a preview that arrives for an older pane is dropped
  let trustError = "";

  async function items(): Promise<SettingsItem[]> {
    try {
      layouts = await api.layouts();
      failure = "";
    } catch (err) {
      layouts = [];
      failure = `Could not list layouts: ${message(err)}`;
      return [];
    }
    return layouts.map((l): SettingsItem => ({ id: l.id, label: l.id, note: exceptionNote(l.status, l.source === "builtin", SOURCE_LABEL[l.source]) || undefined }));
  }

  async function trust(btn: HTMLElement, id: string, allow: boolean): Promise<void> {
    trustError = "";
    try {
      await api.layoutTrust(id, allow);
    } catch (err) {
      trustError = `Could not ${allow ? "allow" : "block"} ${id}: ${message(err)}`;
    }
    reloadSettings(btn, () => {
      document.querySelector<HTMLButtonElement>(`.zen-settings-layout-trust[data-layout="${id}"]`)?.focus();
    });
  }

  // ---- the help pane

  function snippetBox(l: LayoutInfo, s: PluginSnippet, seq: number): HTMLElement {
    const box = el("div", "zen-plugin-snippet");
    const head = el("div", "zen-plugin-snippet-head");
    head.append(el("h4", "zen-plugin-snippet-title", s.title));
    const copy = button(`Copy ${s.title}`, () => void doCopy(copy, s.markdown), { className: "zen-plugin-copy", text: "Copy" });
    head.append(copy);
    const pre = el("pre", "zen-plugin-snippet-code");
    pre.append(el("code", "", s.markdown));
    box.append(head, pre);
    // A snippet is a whole page with the layout in its frontmatter: its static preview shows the layout's HTML and styles.
    if (l.source === "builtin") {
      api.preview(PREVIEW_PATH, s.markdown).then((page) => {
        if (seq !== helpSeq || !box.isConnected) return;
        const preview = el("div", "zen-page zen-plugin-snippet-preview");
        preview.setAttribute("aria-label", `Preview of ${s.title}`);
        mountPage(preview, page);
        box.append(preview);
      }).catch(() => { /* the snippet stays without a preview */ });
    }
    return box;
  }

  let copyTimer: ReturnType<typeof setTimeout> | undefined;
  async function doCopy(btn: HTMLButtonElement, text: string): Promise<void> {
    const ok = await copyText(btn.ownerDocument, text);
    btn.textContent = ok ? "Copied" : "Copy failed";
    clearTimeout(copyTimer);
    copyTimer = setTimeout(() => { btn.textContent = "Copy"; }, 1600);
  }

  function made(l: LayoutInfo): string {
    if (l.hasTemplate && l.hasScript) return "This layout has a page template and runs a page script.";
    if (l.hasTemplate) return "This layout has a page template; it runs nothing.";
    if (l.hasScript) return "This layout runs a page script.";
    return "This layout only changes styles; it runs nothing.";
  }

  function fieldsTable(l: LayoutInfo): HTMLElement[] {
    if (l.fields.length === 0) return [];
    const table = el("div", "zen-plugin-options zen-layout-fields");
    for (const f of l.fields) {
      const row = el("div", "zen-plugin-option zen-layout-field");
      const line = el("div", "zen-plugin-option-line");
      line.append(el("code", "zen-plugin-option-name zen-layout-field-name", f.name), el("span", "zen-plugin-option-type", f.type));
      row.append(line);
      if (f.description) row.append(el("p", "zen-plugin-option-text", f.description));
      table.append(row);
    }
    return [el("h4", "zen-plugin-section", "Frontmatter fields"), table];
  }

  function optionsTable(l: LayoutInfo): HTMLElement[] {
    if (l.options.length === 0) return [];
    const table = el("div", "zen-plugin-options zen-layout-options");
    for (const o of l.options) {
      const row = el("div", "zen-plugin-option");
      const line = el("div", "zen-plugin-option-line");
      line.append(el("code", "zen-plugin-option-name", o.name), el("span", "zen-plugin-option-type", o.type),
        el("span", "zen-plugin-option-default", `default ${fmt(o.default)}`));
      row.append(line);
      if (o.description) row.append(el("p", "zen-plugin-option-text", o.description));
      table.append(row);
    }
    return [el("h4", "zen-plugin-section", "Options (layoutOptions)"), table];
  }

  function layoutHelp(l: LayoutInfo, seq: number): HTMLElement[] {
    const head = el("div", "zen-plugin-help-head");
    if (l.version) head.append(el("span", "zen-settings-plugin-version", l.version));
    head.append(el("span", "zen-settings-plugin-kind", SOURCE_LABEL[l.source]), el("span", "zen-settings-plugin-state", STATUS_LABEL[l.status]));
    const status = el("p", "zen-settings-status zen-settings-plugins-status", trustError);
    status.setAttribute("role", "status");
    trustError = "";
    const out: HTMLElement[] = [head];
    if (l.source !== "builtin" && l.status !== "invalid") {
      const allow = l.status === "blocked" || l.status === "undecided";
      const verb = allow ? "Allow" : "Block";
      const b = button(`${verb} ${l.id}`, () => { status.textContent = ""; b.disabled = true; void trust(b, l.id, allow); }, { className: "zen-settings-plugin-trust zen-settings-layout-trust", text: verb });
      b.dataset.layout = l.id;
      out.push(b);
    }
    out.push(status);
    if (l.description) out.push(el("p", "zen-plugin-help-description", l.description));
    if (l.help.summary) out.push(el("p", "zen-plugin-help-summary", l.help.summary));
    if (l.problem) out.push(el("p", "zen-settings-plugin-problem", l.problem));
    if (l.status !== "invalid") out.push(el("p", "zen-plugin-help-runs", made(l)));
    out.push(...fieldsTable(l), ...optionsTable(l));
    if (l.help.snippets.length > 0) {
      out.push(el("h4", "zen-plugin-section", l.help.snippets.length === 1 ? "In a page" : "In a page: examples"));
      out.push(...l.help.snippets.map((s) => snippetBox(l, s, seq)));
    } else if (!l.help.summary && l.status !== "invalid") {
      out.push(el("p", "zen-plugin-help-none", "No help for this layout yet. Its author can add a help: section to layout.yml."));
    }
    return out;
  }

  function render(item: SettingsItem, right: HTMLElement): () => void {
    const seq = ++helpSeq;
    const pane = el("div", "zen-plugin-help zen-layout-help");
    pane.setAttribute("role", "region");
    pane.setAttribute("aria-label", "Layout help");
    const l = layouts.find((x) => x.id === item.id);
    if (l) pane.append(...layoutHelp(l, seq));
    else pane.append(el("p", "zen-plugin-help-none", "Select a layout to read its help."));
    right.append(pane);
    return () => { helpSeq++; };
  }

  return {
    items, render,
    empty: () => failure || "No layouts in this folder.",
    destroy() { helpSeq++; clearTimeout(copyTimer); },
  };
}
