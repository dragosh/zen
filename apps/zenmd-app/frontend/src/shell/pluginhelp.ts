// Settings › Plugins: the list of plugins on the left (name, kind, status,
// Allow / Block for external ones) and the help of the selected plugin next to
// it: what it adds, and snippets to copy, with a rendered preview for built-in
// plugins. An "All plugins" entry shows every plugin's main snippet in one
// page. All text goes in through textContent; the only HTML mounted is what
// api.preview returned, through the page's own mountPage.
import type { Api, PluginInfo, PluginSnippet } from "../api";
import { mountPage } from "../page";
import { button, el } from "./dom";
import { copyText } from "./clipboard";

export type PluginsSection = {
  el: HTMLElement;
  /** Reads the plugins again and redraws the list and the help. */
  refresh(): Promise<void>;
  destroy(): void;
};

const SOURCE_LABEL: Record<PluginInfo["source"], string> = { builtin: "Built in", folder: "This folder", user: "Your plugins" };
const STATUS_LABEL: Record<PluginInfo["status"], string> = {
  on: "On", off: "Off", blocked: "Blocked", undecided: "Not allowed yet", invalid: "Invalid",
};
const ALL = "*"; // not a valid plugin id, so it never clashes with one
const PREVIEW_PATH = "zen-plugin-help.md";
const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

/** A snippet that starts with frontmatter is a whole page (a layout, options): no preview. */
const isWholePage = (markdown: string) => /^---\s*\n/.test(markdown);

export function createPluginsSection(opts: { api: Api }): PluginsSection {
  const { api } = opts;
  const root = el("div", "zen-plugins");

  const side = el("div", "zen-plugins-side");
  const list = el("div", "zen-settings-plugins");
  list.setAttribute("role", "list");
  list.setAttribute("aria-label", "Plugins");
  const status = el("p", "zen-settings-status zen-settings-plugins-status");
  status.setAttribute("role", "status");
  side.append(list, status);
  const pane = el("div", "zen-plugin-help");
  pane.setAttribute("role", "region");
  pane.setAttribute("aria-label", "Plugin help");
  root.append(side, pane);

  let plugins: PluginInfo[] = [];
  let selected = ALL;
  let picked = false;
  let listSeq = 0;
  let helpSeq = 0; // a preview that arrives for an older pane is dropped
  let shown = "";

  // ---- the list

  const selects = () => Array.from(list.querySelectorAll<HTMLButtonElement>(".zen-settings-plugin-select"));

  function select(id: string): void {
    selected = id;
    for (const b of selects()) {
      const on = b.dataset.entry === id;
      if (on) b.setAttribute("aria-current", "true");
      else b.removeAttribute("aria-current");
      b.tabIndex = on ? 0 : -1;
    }
    renderHelp();
  }

  /** The reader picked an entry: it stays selected when the list is read again. */
  function choose(id: string): void {
    picked = true;
    select(id);
  }

  function entry(id: string, row: HTMLElement): HTMLButtonElement {
    const b = el("button", "zen-settings-plugin-select");
    b.type = "button";
    b.dataset.entry = id;
    b.addEventListener("click", () => choose(id));
    row.append(b);
    return b;
  }

  function pluginRow(p: PluginInfo): HTMLElement {
    const r = el("div", "zen-settings-plugin");
    r.setAttribute("role", "listitem");
    r.dataset.plugin = p.id;
    r.dataset.status = p.status;
    const pick = entry(p.id, r);
    const head = el("span", "zen-settings-plugin-head");
    head.append(el("span", "zen-settings-plugin-name", p.id));
    if (p.version) head.append(el("span", "zen-settings-plugin-version", p.version));
    head.append(el("span", "zen-settings-plugin-kind", p.kind));
    const meta = el("span", "zen-settings-plugin-meta");
    meta.append(el("span", "zen-settings-plugin-hint", SOURCE_LABEL[p.source]), el("span", "zen-settings-plugin-state", STATUS_LABEL[p.status]));
    pick.append(head, meta);
    if (p.runs.length > 0) pick.append(el("span", "zen-settings-plugin-runs", `Runs: ${p.runs.join(", ")}`));
    if (p.problem) pick.append(el("span", "zen-settings-plugin-problem", p.problem));
    if (p.source !== "builtin" && p.status !== "invalid") {
      const allow = p.status === "blocked" || p.status === "undecided";
      const verb = allow ? "Allow" : "Block";
      r.append(button(`${verb} ${p.id}`, () => void trust(p.id, allow), { className: "zen-settings-plugin-trust", text: verb }));
    }
    return r;
  }

  function renderList(): void {
    if (plugins.length === 0) {
      list.replaceChildren(el("p", "zen-settings-plugins-empty", "No plugins in this folder."));
      return;
    }
    const all = el("div", "zen-settings-plugin-all");
    all.setAttribute("role", "listitem");
    const b = entry(ALL, all);
    b.append(el("span", "zen-settings-plugin-name", "All plugins"), el("span", "zen-settings-plugin-hint", "Every plugin's main snippet on one page"));
    list.replaceChildren(all, ...plugins.map(pluginRow));
    select(selected);
  }

  // ↑ ↓ Home End move between the entries; Enter selects the one in focus.
  list.addEventListener("keydown", (e) => {
    const items = selects();
    if (items.length === 0) return;
    const at = items.indexOf(list.ownerDocument.activeElement as HTMLButtonElement);
    if (e.key === "Enter" && at >= 0) {
      e.preventDefault();
      choose(items[at].dataset.entry!);
      return;
    }
    const to = ({ ArrowDown: at + 1, ArrowUp: at - 1, Home: 0, End: items.length - 1 } as Record<string, number>)[e.key];
    if (to === undefined) return;
    e.preventDefault();
    const next = items[Math.min(items.length - 1, Math.max(0, to))];
    // Roving tab stop: the entry in focus is the one Tab comes back to.
    for (const b of items) b.tabIndex = b === next ? 0 : -1;
    next.focus();
  });

  async function refresh(): Promise<void> {
    const mine = ++listSeq;
    try {
      const got = await api.plugins();
      if (mine !== listSeq) return;
      plugins = got;
    } catch (err) {
      if (mine !== listSeq) return;
      plugins = [];
      list.replaceChildren(el("p", "zen-settings-plugins-empty", `Could not list plugins: ${message(err)}`));
      renderHelp();
      return;
    }
    if (selected !== ALL && !plugins.some((p) => p.id === selected)) selected = ALL;
    // The first plugin is selected until the reader picks another entry.
    if (!picked && plugins.length > 0) selected = plugins[0].id;
    renderList();
  }

  async function trust(id: string, allow: boolean): Promise<void> {
    status.textContent = "";
    for (const b of Array.from(list.querySelectorAll("button"))) b.disabled = true;
    try {
      await api.pluginTrust(id, allow);
    } catch (err) {
      status.textContent = `Could not ${allow ? "allow" : "block"} ${id}: ${message(err)}`;
    }
    await refresh();
    list.querySelector<HTMLButtonElement>(`.zen-settings-plugin[data-plugin="${id}"] .zen-settings-plugin-trust`)?.focus();
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

  let copyTimer: ReturnType<typeof setTimeout> | undefined;
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

  function heading(p: PluginInfo, level: "h3" | "h4"): HTMLElement {
    const h = el("div", "zen-plugin-help-head");
    h.append(el(level, "zen-plugin-help-title", p.id));
    if (p.version) h.append(el("span", "zen-settings-plugin-version", p.version));
    h.append(el("span", "zen-settings-plugin-kind", p.kind), el("span", "zen-settings-plugin-state", STATUS_LABEL[p.status]));
    return h;
  }

  function pluginHelp(p: PluginInfo, seq: number): HTMLElement[] {
    const out: HTMLElement[] = [heading(p, "h3")];
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
    const out: HTMLElement[] = [el("h3", "zen-plugin-help-title", "All plugins")];
    out.push(el("p", "zen-plugin-help-summary", "The main snippet of every plugin, to copy into a page."));
    for (const p of plugins.filter((x) => x.status !== "invalid")) {
      const item = el("section", "zen-plugin-all-item");
      item.dataset.plugin = p.id;
      item.append(heading(p, "h4"));
      const text = p.help.summary || p.description;
      if (text) item.append(el("p", "zen-plugin-help-summary", text));
      if (p.help.snippets.length > 0) item.append(snippetBox(p, p.help.snippets[0], seq));
      else item.append(el("p", "zen-plugin-help-none", "No help for this plugin yet."));
      out.push(item);
    }
    return out;
  }

  function renderHelp(): void {
    const target = plugins.find((p) => p.id === selected);
    const key = JSON.stringify([selected, selected === ALL ? plugins : target ?? null]);
    if (key === shown && pane.childElementCount > 0) return;
    shown = key;
    const seq = ++helpSeq;
    delete pane.dataset.view;
    if (plugins.length === 0) {
      pane.replaceChildren(el("p", "zen-plugin-help-none", "No plugins to show help for."));
    } else if (selected === ALL) {
      pane.dataset.view = "all";
      pane.replaceChildren(...allHelp(seq));
    } else if (target) {
      pane.replaceChildren(...pluginHelp(target, seq));
    } else {
      pane.replaceChildren(el("p", "zen-plugin-help-none", "Select a plugin to read its help."));
    }
  }

  renderHelp();
  return {
    el: root,
    refresh,
    destroy() {
      listSeq++;
      helpSeq++;
      clearTimeout(copyTimer);
      root.remove();
    },
  };
}
