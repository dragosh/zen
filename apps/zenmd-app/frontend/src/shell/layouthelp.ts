// Settings › Layouts: the list of layouts on the left (name, version, origin,
// status, template/script hints, Allow / Block for external ones) and the help
// of the selected layout next to it: description, the frontmatter fields it
// reads, its options, and snippets to copy, with a static preview for built-in
// layouts. Same pattern and styles as Settings › Plugins (shell/pluginhelp.ts).
// All text goes in through textContent; the only HTML mounted is what
// api.preview returned, through the page's own mountPage.
import type { Api, LayoutInfo, PluginSnippet } from "../api";
import { button, copyText, el, mountPage } from "@zenmd/kit";

export type LayoutsSection = {
  el: HTMLElement;
  /** Reads the layouts again and redraws the list and the help. */
  refresh(): Promise<void>;
  destroy(): void;
};

const SOURCE_LABEL: Record<LayoutInfo["source"], string> = { builtin: "Built in", folder: "This folder", user: "Your layouts" };
const STATUS_LABEL: Record<LayoutInfo["status"], string> = {
  on: "On", off: "Off", blocked: "Blocked", undecided: "Not allowed yet", invalid: "Invalid",
};
const PREVIEW_PATH = "zen-layout-help.md";
const message = (err: unknown) => (err instanceof Error ? err.message : String(err));
const fmt = (v: unknown) => (typeof v === "string" ? (v === "" ? '""' : v) : JSON.stringify(v) ?? "");

export function createLayoutsSection(opts: { api: Api }): LayoutsSection {
  const { api } = opts;
  const root = el("div", "zen-plugins zen-layouts");

  const side = el("div", "zen-plugins-side");
  const list = el("div", "zen-settings-plugins zen-settings-layouts");
  list.setAttribute("role", "list");
  list.setAttribute("aria-label", "Layouts");
  const status = el("p", "zen-settings-status zen-settings-plugins-status");
  status.setAttribute("role", "status");
  side.append(list, status);
  const pane = el("div", "zen-plugin-help zen-layout-help");
  pane.setAttribute("role", "region");
  pane.setAttribute("aria-label", "Layout help");
  root.append(side, pane);

  let layouts: LayoutInfo[] = [];
  let selected = "";
  let picked = false;
  let listSeq = 0;
  let helpSeq = 0; // a preview that arrives for an older pane is dropped
  let shown = "";

  // ---- the list

  const selects = () => Array.from(list.querySelectorAll<HTMLButtonElement>(".zen-settings-layout-select"));

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

  function choose(id: string): void {
    picked = true;
    select(id);
  }

  function layoutRow(l: LayoutInfo): HTMLElement {
    const r = el("div", "zen-settings-plugin zen-settings-layout");
    r.setAttribute("role", "listitem");
    r.dataset.layout = l.id;
    r.dataset.status = l.status;
    const pick = el("button", "zen-settings-plugin-select zen-settings-layout-select");
    pick.type = "button";
    pick.dataset.entry = l.id;
    pick.addEventListener("click", () => choose(l.id));
    const head = el("span", "zen-settings-plugin-head");
    head.append(el("span", "zen-settings-plugin-name", l.id));
    if (l.version) head.append(el("span", "zen-settings-plugin-version", l.version));
    const source = el("span", "zen-settings-plugin-hint zen-settings-layout-source", SOURCE_LABEL[l.source]);
    source.dataset.source = l.source;
    const meta = el("span", "zen-settings-plugin-meta");
    meta.append(source);
    if (l.hasTemplate) meta.append(el("span", "zen-settings-plugin-kind", "template"));
    if (l.hasScript) meta.append(el("span", "zen-settings-plugin-kind", "script"));
    meta.append(el("span", "zen-settings-plugin-state", STATUS_LABEL[l.status]));
    pick.append(head, meta);
    if (l.problem) pick.append(el("span", "zen-settings-plugin-problem", l.problem));
    r.append(pick);
    if (l.source !== "builtin" && l.status !== "invalid") {
      const allow = l.status === "blocked" || l.status === "undecided";
      const verb = allow ? "Allow" : "Block";
      r.append(button(`${verb} ${l.id}`, () => void trust(l.id, allow), { className: "zen-settings-plugin-trust zen-settings-layout-trust", text: verb }));
    }
    return r;
  }

  function renderList(): void {
    if (layouts.length === 0) {
      list.replaceChildren(el("p", "zen-settings-plugins-empty", "No layouts in this folder."));
      return;
    }
    list.replaceChildren(...layouts.map(layoutRow));
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
    for (const b of items) b.tabIndex = b === next ? 0 : -1;
    next.focus();
  });

  async function refresh(): Promise<void> {
    const mine = ++listSeq;
    try {
      const got = await api.layouts();
      if (mine !== listSeq) return;
      layouts = got;
    } catch (err) {
      if (mine !== listSeq) return;
      layouts = [];
      list.replaceChildren(el("p", "zen-settings-plugins-empty", `Could not list layouts: ${message(err)}`));
      renderHelp();
      return;
    }
    if (!layouts.some((l) => l.id === selected)) selected = "";
    // The first layout is selected until the reader picks another entry.
    if ((!picked || selected === "") && layouts.length > 0) selected = layouts[0].id;
    renderList();
  }

  async function trust(id: string, allow: boolean): Promise<void> {
    status.textContent = "";
    for (const b of Array.from(list.querySelectorAll("button"))) b.disabled = true;
    try {
      await api.layoutTrust(id, allow);
    } catch (err) {
      status.textContent = `Could not ${allow ? "allow" : "block"} ${id}: ${message(err)}`;
    }
    await refresh();
    list.querySelector<HTMLButtonElement>(`.zen-settings-layout[data-layout="${id}"] .zen-settings-layout-trust`)?.focus();
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
    head.append(el("h3", "zen-plugin-help-title", l.id));
    if (l.version) head.append(el("span", "zen-settings-plugin-version", l.version));
    head.append(el("span", "zen-settings-plugin-kind", SOURCE_LABEL[l.source]), el("span", "zen-settings-plugin-state", STATUS_LABEL[l.status]));
    const out: HTMLElement[] = [head];
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

  function renderHelp(): void {
    const target = layouts.find((l) => l.id === selected);
    const key = JSON.stringify([selected, target ?? null]);
    if (key === shown && pane.childElementCount > 0) return;
    shown = key;
    const seq = ++helpSeq;
    if (layouts.length === 0) pane.replaceChildren(el("p", "zen-plugin-help-none", "No layouts to show help for."));
    else if (target) pane.replaceChildren(...layoutHelp(target, seq));
    else pane.replaceChildren(el("p", "zen-plugin-help-none", "Select a layout to read its help."));
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
