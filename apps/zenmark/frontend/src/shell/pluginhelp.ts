// Settings › Plugins (ui/settings-layout): the list of plugins in the left column (name, with a mark only for
// plugins that are off, blocked, invalid or not built in) and the help of the selected plugin in the right one:
// its state, Allow / Block for external ones, what it adds, and snippets to copy, with a rendered preview for
// built-in plugins. An "All plugins" entry shows every plugin's main snippet in one page. All text goes in
// through textContent; the only HTML mounted is what api.preview returned, through the page's own mountPage.
import type { Api, PluginInfo, PluginSnippet } from "../api";
import { button, copyText, el, mountPage, reloadSettings, type SettingsItem } from "@zenmd/kit";

export type PluginsSection = {
  /** The rows of the list; reads the plugins again. */
  items(): Promise<SettingsItem[]>;
  /** The right column for a row. */
  render(item: SettingsItem, right: HTMLElement): () => void;
  /** The first plugin is selected until the reader picks another entry. */
  current(): string | undefined;
  onPick(): void;
  /** What the right column says without rows (no plugins, or the list failed). */
  empty(): string;
  destroy(): void;
};

export const STATUS_LABEL: Record<PluginInfo["status"], string> = {
  on: "On", off: "Off", blocked: "Blocked", undecided: "Not allowed yet", invalid: "Invalid",
};
const SOURCE_LABEL: Record<PluginInfo["source"], string> = { builtin: "Built in", folder: "This folder", user: "Your plugins" };
const ALL = "*"; // not a valid plugin id, so it never clashes with one
const PREVIEW_PATH = "zen-plugin-help.md";
const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

/** The marks of a row: only the exceptions. "On" and "Built in" are the normal case and are not shown. */
export function exceptionNote(status: PluginInfo["status"], builtin: boolean, origin: string): string {
  return [status === "on" ? "" : STATUS_LABEL[status], builtin ? "" : origin].filter(Boolean).join(" · ");
}

/** A snippet that starts with frontmatter is a whole page (a layout, options): no preview. */
const isWholePage = (markdown: string) => /^---\s*\n/.test(markdown);

export function createPluginsSection(opts: { api: Api }): PluginsSection {
  const { api } = opts;
  let plugins: PluginInfo[] = [];
  let failure = "";
  let helpSeq = 0; // a preview that arrives for an older pane is dropped
  let trustError = "";
  let picked = false;
  let copyTimer: ReturnType<typeof setTimeout> | undefined;

  async function items(): Promise<SettingsItem[]> {
    try {
      plugins = await api.plugins();
      failure = "";
    } catch (err) {
      plugins = [];
      failure = `Could not list plugins: ${message(err)}`;
      return [];
    }
    if (plugins.length === 0) return [];
    return [
      { id: ALL, label: "All plugins" },
      ...plugins.map((p): SettingsItem => ({ id: p.id, label: p.id, note: exceptionNote(p.status, p.source === "builtin", SOURCE_LABEL[p.source]) || undefined })),
    ];
  }

  async function trust(btn: HTMLElement, id: string, allow: boolean): Promise<void> {
    trustError = "";
    try {
      await api.pluginTrust(id, allow);
    } catch (err) {
      trustError = `Could not ${allow ? "allow" : "block"} ${id}: ${message(err)}`;
    }
    reloadSettings(btn, () => {
      document.querySelector<HTMLButtonElement>(`.zen-settings-plugin-trust[data-plugin="${id}"]`)?.focus();
    });
  }

  // ---- the help pane

  function snippetBox(p: PluginInfo, s: PluginSnippet, seq: number): HTMLElement {
    const box = el("div", "zen-plugin-snippet");
    const head = el("div", "zen-plugin-snippet-head");
    head.append(el("h4", "zen-plugin-snippet-title", s.title));
    const copy = button(`Copy ${s.title}`, () => void doCopy(copy, s.markdown), { className: "zen-plugin-copy", text: "Copy" });
    head.append(copy);
    const pre = el("pre", "zen-plugin-snippet-code");
    pre.append(el("code", "", s.markdown));
    box.append(head, pre);
    if (p.source === "builtin" && !isWholePage(s.markdown)) {
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

  async function doCopy(btn: HTMLButtonElement, text: string): Promise<void> {
    const ok = await copyText(btn.ownerDocument, text);
    btn.textContent = ok ? "Copied" : "Copy failed";
    clearTimeout(copyTimer);
    copyTimer = setTimeout(() => { btn.textContent = "Copy"; }, 1600);
  }

  const fmt = (v: unknown) => (typeof v === "string" ? (v === "" ? '""' : v) : JSON.stringify(v) ?? "");

  function adds(p: PluginInfo): HTMLElement | null {
    const rows: [string, string[]][] = [["Blocks", p.blocks], ["Inlines", p.inlines]]; // layouts are their own section (Settings › Layouts)
    const shownRows = rows.filter(([, names]) => names.length > 0);
    if (shownRows.length === 0 && p.options.length === 0) return null;
    const box = el("div", "zen-plugin-adds");
    if (shownRows.length > 0) {
      box.append(el("h4", "zen-plugin-section", "Adds"));
      const dl = el("dl", "zen-plugin-names");
      for (const [label, names] of shownRows) {
        dl.append(el("dt", "", label));
        const dd = el("dd");
        dd.append(...names.map((n) => el("code", "zen-plugin-name", n)));
        dl.append(dd);
      }
      box.append(dl);
    }
    if (p.options.length > 0) {
      box.append(el("h4", "zen-plugin-section", "Options"));
      const table = el("div", "zen-plugin-options");
      for (const o of p.options) {
        const row = el("div", "zen-plugin-option");
        const line = el("div", "zen-plugin-option-line");
        line.append(el("code", "zen-plugin-option-name", o.name), el("span", "zen-plugin-option-type", o.type),
          el("span", "zen-plugin-option-default", `default ${fmt(o.default)}`));
        row.append(line);
        if (o.description) row.append(el("p", "zen-plugin-option-text", o.description));
        table.append(row);
      }
      box.append(table);
    }
    return box;
  }

  /** Version, kind, state and origin under the title the frame draws (a title of its own in the all-plugins page). */
  function heading(p: PluginInfo, title: boolean): HTMLElement {
    const h = el("div", "zen-plugin-help-head");
    if (title) h.append(el("h4", "zen-plugin-help-title", p.id));
    if (p.version) h.append(el("span", "zen-settings-plugin-version", p.version));
    h.append(el("span", "zen-settings-plugin-kind", p.kind), el("span", "zen-settings-plugin-state", STATUS_LABEL[p.status]), el("span", "zen-settings-plugin-hint", SOURCE_LABEL[p.source]));
    return h;
  }

  function trustButton(p: PluginInfo, status: HTMLElement): HTMLElement | null {
    if (p.source === "builtin" || p.status === "invalid") return null;
    const allow = p.status === "blocked" || p.status === "undecided";
    const verb = allow ? "Allow" : "Block";
    const b = button(`${verb} ${p.id}`, () => {
      status.textContent = "";
      b.disabled = true;
      void trust(b, p.id, allow);
    }, { className: "zen-settings-plugin-trust", text: verb });
    b.dataset.plugin = p.id;
    return b;
  }

  function pluginHelp(p: PluginInfo, seq: number): HTMLElement[] {
    const status = el("p", "zen-settings-status zen-settings-plugins-status", trustError);
    status.setAttribute("role", "status");
    trustError = "";
    const out: HTMLElement[] = [heading(p, false)];
    const tb = trustButton(p, status);
    if (tb) out.push(tb);
    out.push(status);
    if (p.description) out.push(el("p", "zen-plugin-help-description", p.description));
    if (p.help.summary) out.push(el("p", "zen-plugin-help-summary", p.help.summary));
    if (p.problem) out.push(el("p", "zen-settings-plugin-problem", p.problem));
    const a = adds(p);
    if (a) out.push(a);
    if (p.runs.length > 0) out.push(el("p", "zen-plugin-help-runs", `Runs: ${p.runs.join(", ")}`));
    if (p.help.snippets.length > 0) {
      out.push(el("h4", "zen-plugin-section", p.help.snippets.length === 1 ? "In a page" : "In a page: examples"));
      out.push(...p.help.snippets.map((s) => snippetBox(p, s, seq)));
    } else if (!p.help.summary && p.status !== "invalid") {
      out.push(el("p", "zen-plugin-help-none", "No help for this plugin yet. Its author can add a help: section to plugin.yml."));
    }
    return out;
  }

  function allHelp(seq: number): HTMLElement[] {
    const out: HTMLElement[] = [el("p", "zen-plugin-help-summary", "The main snippet of every plugin, to copy into a page.")];
    for (const p of plugins.filter((x) => x.status !== "invalid")) {
      const item = el("section", "zen-plugin-all-item");
      item.dataset.plugin = p.id;
      item.append(heading(p, true));
      const text = p.help.summary || p.description;
      if (text) item.append(el("p", "zen-plugin-help-summary", text));
      if (p.help.snippets.length > 0) item.append(snippetBox(p, p.help.snippets[0], seq));
      else item.append(el("p", "zen-plugin-help-none", "No help for this plugin yet."));
      out.push(item);
    }
    return out;
  }

  function render(item: SettingsItem, right: HTMLElement): () => void {
    const seq = ++helpSeq;
    const pane = el("div", "zen-plugin-help");
    pane.setAttribute("role", "region");
    pane.setAttribute("aria-label", "Plugin help");
    if (item.id === ALL) {
      pane.dataset.view = "all";
      pane.append(...allHelp(seq));
    } else {
      const p = plugins.find((x) => x.id === item.id);
      if (p) pane.append(...pluginHelp(p, seq));
      else pane.append(el("p", "zen-plugin-help-none", "Select a plugin to read its help."));
    }
    right.append(pane);
    return () => { helpSeq++; };
  }

  return {
    items, render,
    current: () => (picked ? undefined : plugins[0]?.id),
    onPick: () => { picked = true; },
    empty: () => failure || "No plugins in this folder.",
    destroy() { helpSeq++; clearTimeout(copyTimer); },
  };
}
