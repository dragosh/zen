// The Settings page frame, shown in the main pane (the sidebar stays): a section list on the left and
// the chosen section on the right. Every section is laid out the same way (ui/settings-layout): what you
// pick in a column of about 240 px, and what it does (preview, description, actions, help) in the rest.
// The frame owns the section list, the item lists with their keyboard arrows and selection, and the
// Appearance and Developer sections; the app gives the others (configuration files, plugins, export, …).
// All text through textContent.
import "./settings.css";
import { button, el } from "./dom";
import { developerEnabled, setDeveloperEnabled } from "./developer";
import { setTheme, storedTheme, type Theme } from "./theme";

export type SettingsView = {
  el: HTMLElement;
  /** Called each time the page is shown: every section re-reads what it shows, then the list takes the focus. */
  show(): void;
  /** Chooses a section, and an item of it when given (File › Export › Website… opens Settings › Export › Website). */
  open(sectionId: string, itemId?: string): void;
  destroy(): void;
};

/** One row of a section's list. `note` is a short mark beside the label; a disabled row cannot be selected. */
export type SettingsItem = { id: string; label: string; note?: string; disabled?: boolean };

/**
 * One section of the page. Its heading (the label) is added by the frame. A section gives either `el` (old
 * single-column content, shown in the right column) or `items` and `render` (the frame draws the two columns).
 */
export type SettingsSection = {
  id: string;
  label: string;
  /** Old single-column content, shown in the right column under the section's title. */
  el?: HTMLElement;
  /** The rows of the left column. Read each time the section is shown or chosen. */
  items?: () => SettingsItem[] | Promise<SettingsItem[]>;
  /** Fills the right column (after the item's title as h3) for the selected item; may return a clean-up function. */
  render?: (item: SettingsItem, right: HTMLElement) => void | (() => void);
  /** What the right column says when the list has no rows. */
  empty?: string | (() => string);
  /** The id of the row that is current (Appearance: the theme in use); it wins over the remembered selection. */
  current?: () => string | undefined;
  /** The reader chose this row (by click or key), not the frame (Appearance applies the theme here). */
  onPick?: (item: SettingsItem) => void;
  /** Called each time the page is shown. */
  show?: () => void;
  destroy?: () => void;
};

/**
 * Content in a section's right column asks the frame to read the section's list again and draw it
 * (the selection stays); `then` runs after the redraw. Used after Allow / Block and the like.
 */
export const SETTINGS_RELOAD = "zen-settings-reload";
export function reloadSettings(from: Element, then?: () => void): void {
  from.dispatchEvent(new CustomEvent(SETTINGS_RELOAD, { bubbles: true, detail: { then } }));
}

/** Below this width of the page the right column goes under the left one. */
export const SETTINGS_NARROW = 720;

const THEMES: { id: Theme; label: string; note: string }[] = [
  { id: "system", label: "System", note: "Follows the light or dark setting of this Mac." },
  { id: "light", label: "Light", note: "Black text on white." },
  { id: "dark", label: "Dark", note: "White text on black." },
];

function themePreview(theme: Theme): HTMLElement {
  const box = el("div", "zen-theme-preview");
  box.dataset.theme = theme;
  box.setAttribute("role", "img");
  box.setAttribute("aria-label", `Preview of a page in the ${theme} theme`);
  const link = el("a", "", "a link");
  link.href = "#";
  link.tabIndex = -1;
  link.addEventListener("click", (e) => e.preventDefault());
  const p = el("p", "zen-theme-preview-text");
  p.append("Text in the reading column, ", link, " and a line of ", el("code", "", "code()"), ".");
  box.append(el("h4", "zen-theme-preview-title", "A page title"), p);
  return box;
}

/**
 * Appearance: the theme as a list (System, Light, Dark) and a small preview of a page in the one chosen. It applies to
 * this machine and all windows. `extra` rows (the sidebar side, ui/nav-panel) go under the preview.
 */
export function appearanceSection(doc: () => Document, extra: HTMLElement[] = []): SettingsSection {
  return {
    id: "appearance", label: "Appearance",
    items: () => THEMES.map(({ id, label }) => ({ id, label })),
    current: () => storedTheme(),
    onPick: (item) => setTheme(item.id as Theme, doc()),
    render(item, right) {
      const t = THEMES.find((x) => x.id === item.id) ?? THEMES[0];
      right.append(el("p", "zen-settings-hint", t.note), themePreview(t.id), ...extra);
    },
  };
}

/**
 * Developer: the Console. "Show Console in the Inspector" (the setting ui/developer-tab stored, now the Inspector's
 * tab) and Open Console (⌥⌘J), which the shell wires through `openConsole`. It applies to this machine and all windows.
 */
export function developerSection(o: { openConsole?: () => void } = {}): SettingsSection {
  return {
    id: "developer", label: "Developer",
    items: () => [{ id: "console", label: "Console" }],
    render(_item, right) {
      const row = el("label", "zen-settings-check");
      const box = el("input");
      box.type = "checkbox";
      box.id = "zen-settings-developer";
      box.checked = developerEnabled();
      row.append(box, el("span", "", "Show Console in the Inspector"));
      const hint = el("p", "zen-settings-hint", "The Console shows the app's log, the plugins' errors and what pages print.");
      const open = button("Open Console", () => o.openConsole?.(), { title: "Open Console (⌥⌘J)" });
      open.disabled = !box.checked;
      box.addEventListener("change", () => {
        setDeveloperEnabled(box.checked);
        open.disabled = !box.checked;
      });
      right.append(row, hint, open);
    },
  };
}

type Pane = {
  section: SettingsSection;
  list: HTMLElement;
  detail: HTMLElement;
  items: SettingsItem[];
  selected: string;
  cleanup: (() => void) | null;
  seq: number;
};

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

/** The page: sections in the order given; the first one shows first. */
export function createSettingsFrame(sections: SettingsSection[]): SettingsView {
  const page = el("section", "zen-settings-page");
  page.setAttribute("aria-label", "Settings");

  // ---- the section list

  const nav = el("nav", "zen-settings-sections");
  nav.setAttribute("aria-label", "Settings sections");
  nav.append(el("h1", "zen-settings-title", "Settings"));
  const tabs = sections.map(({ id, label }) => {
    const b = button(label, () => choose(id), { className: "zen-settings-section" });
    b.dataset.section = id;
    return b;
  });
  const sectionList = el("div", "zen-settings-sections-list");
  sectionList.append(...tabs);
  nav.addEventListener("keydown", (e) => {
    const step = ({ ArrowDown: 1, ArrowUp: -1 } as Record<string, number>)[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const i = sections.findIndex((s) => s.id === current);
    const next = (i + step + sections.length) % sections.length;
    choose(sections[next].id);
    tabs[next].focus();
  });
  nav.append(sectionList);

  // ---- the panels: a title, then the two columns (or the old content)

  const body = el("div", "zen-settings-body");
  const panels = new Map<string, HTMLElement>();
  const panes = new Map<string, Pane>();
  for (const section of sections) {
    const p = el("div", "zen-settings-panel");
    p.dataset.section = section.id;
    p.append(el("h2", "zen-settings-heading", section.label));
    if (section.items) {
      const cols = el("div", "zen-settings-cols");
      cols.dataset.section = section.id;
      const list = el("div", "zen-settings-list");
      list.setAttribute("role", "listbox");
      list.setAttribute("aria-label", section.label);
      const detail = el("div", "zen-settings-detail");
      cols.append(list, detail);
      p.append(cols);
      const pane: Pane = { section, list, detail, items: [], selected: "", cleanup: null, seq: 0 };
      panes.set(section.id, pane);
      list.addEventListener("keydown", (e) => keys(pane, e));
      list.addEventListener("click", (e) => {
        const row = (e.target as Element).closest<HTMLElement>("[role=option]");
        if (row?.dataset.item !== undefined) select(pane, row.dataset.item, true);
      });
      p.addEventListener(SETTINGS_RELOAD, (e) => {
        e.stopPropagation();
        const then = (e as CustomEvent<{ then?: () => void } | null>).detail?.then;
        void load(pane).then(() => then?.());
      });
    } else if (section.el) {
      const cols = el("div", "zen-settings-cols zen-settings-single");
      const detail = el("div", "zen-settings-detail");
      detail.append(section.el);
      cols.append(detail);
      p.append(cols);
    }
    p.hidden = true;
    panels.set(section.id, p);
    body.append(p);
  }
  page.append(nav, body);

  // ---- the lists

  const rowOf = (pane: Pane, id: string) => Array.from(pane.list.querySelectorAll<HTMLElement>("[role=option]")).find((r) => r.dataset.item === id);
  const enabled = (pane: Pane) => pane.items.filter((i) => !i.disabled);

  function drawList(pane: Pane): void {
    pane.list.replaceChildren(...pane.items.map((it) => {
      const r = el("div", "zen-settings-item");
      r.setAttribute("role", "option");
      r.dataset.item = it.id;
      r.setAttribute("aria-selected", "false");
      r.tabIndex = -1;
      if (it.disabled) r.setAttribute("aria-disabled", "true");
      r.append(el("span", "zen-settings-item-label", it.label));
      if (it.note) r.append(el("span", "zen-settings-item-note", it.note));
      return r;
    }));
  }

  function mark(pane: Pane): void {
    for (const r of Array.from(pane.list.querySelectorAll<HTMLElement>("[role=option]"))) {
      const on = r.dataset.item === pane.selected;
      r.setAttribute("aria-selected", String(on));
      r.tabIndex = on ? 0 : -1;
    }
  }

  function renderDetail(pane: Pane): void {
    pane.cleanup?.();
    pane.cleanup = null;
    const it = pane.items.find((i) => i.id === pane.selected);
    if (!it) {
      const empty = pane.section.empty;
      pane.detail.replaceChildren(...(empty ? [document.createTextNode(typeof empty === "function" ? empty() : empty)] : []));
      return;
    }
    pane.detail.replaceChildren(el("h3", "zen-settings-detail-title", it.label));
    const done = pane.section.render?.(it, pane.detail);
    if (typeof done === "function") pane.cleanup = done;
  }

  function select(pane: Pane, id: string, byReader: boolean): void {
    const it = pane.items.find((i) => i.id === id);
    if (!it || it.disabled) return;
    pane.selected = id;
    if (byReader) pane.section.onPick?.(it);
    mark(pane);
    renderDetail(pane);
  }

  async function load(pane: Pane): Promise<void> {
    const mine = ++pane.seq;
    let got: SettingsItem[] = [];
    let failed = "";
    try {
      got = await pane.section.items!();
    } catch (err) {
      failed = `Could not read ${pane.section.label}: ${message(err)}`;
    }
    if (mine !== pane.seq) return;
    pane.items = got;
    drawList(pane);
    const want = pane.section.current?.() ?? pane.selected;
    pane.selected = got.some((i) => i.id === want && !i.disabled) ? want : (enabled(pane)[0]?.id ?? "");
    mark(pane);
    renderDetail(pane);
    if (failed) pane.detail.textContent = failed;
  }

  function keys(pane: Pane, e: KeyboardEvent): void {
    const rows = enabled(pane);
    if (rows.length === 0) return;
    const at = rows.findIndex((i) => rowOf(pane, i.id) === pane.list.ownerDocument.activeElement);
    if ((e.key === "Enter" || e.key === " ") && at >= 0) {
      e.preventDefault();
      select(pane, rows[at].id, true);
      return;
    }
    const from = at >= 0 ? at : Math.max(0, rows.findIndex((i) => i.id === pane.selected));
    const to = ({ ArrowDown: from + 1, ArrowUp: from - 1, Home: 0, End: rows.length - 1 } as Record<string, number>)[e.key];
    if (to === undefined) return;
    e.preventDefault();
    e.stopPropagation();
    const next = rows[Math.min(rows.length - 1, Math.max(0, to))];
    select(pane, next.id, true);
    rowOf(pane, next.id)?.focus();
  }

  // ---- choosing a section

  let current = sections[0]?.id ?? "";
  function choose(id: string, reload = true): void {
    current = id;
    for (const t of tabs) {
      const on = t.dataset.section === id;
      if (on) t.setAttribute("aria-current", "true");
      else t.removeAttribute("aria-current");
      t.tabIndex = on ? 0 : -1;
    }
    panels.forEach((p, pid) => { p.hidden = pid !== id; });
    const pane = panes.get(id);
    if (pane && reload) void load(pane);
  }
  choose(current, false);

  // Below 720 px the right column goes under the left one.
  let observer: ResizeObserver | null = null;
  const narrow = (w: number) => page.classList.toggle("zen-settings-narrow", w > 0 && w < SETTINGS_NARROW);
  if (typeof ResizeObserver !== "undefined") {
    observer = new ResizeObserver((entries) => narrow(entries[entries.length - 1]?.contentRect.width ?? 0));
    observer.observe(page);
  }

  return {
    el: page,
    show() {
      for (const s of sections) s.show?.();
      const pane = panes.get(current);
      if (pane) void load(pane);
      narrow(page.clientWidth);
      (tabs.find((t) => t.getAttribute("aria-current") === "true") ?? tabs[0])?.focus();
    },
    open(sectionId, itemId) {
      const pane = panes.get(sectionId);
      if (pane && itemId !== undefined) pane.selected = itemId;
      choose(sectionId);
    },
    destroy() {
      observer?.disconnect();
      panes.forEach((p) => { p.seq++; p.cleanup?.(); });
      for (const s of sections) s.destroy?.();
      page.remove();
    },
  };
}
