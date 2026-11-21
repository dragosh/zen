// The Settings page frame, shown in the main pane (the sidebar stays): a section list on the left and
// the chosen section on the right. Every section is laid out the same way (ui/settings-layout): what you
// pick in a column of about 240 px, and what it does (preview, description, actions, help) in the rest.
// The frame owns the section list, the item lists with their keyboard arrows and selection, and the
// Appearance and Developer sections; the app gives the others (configuration files, plugins, export, …).
// All text through textContent.
import "./settings.css";
import { button, el } from "./dom";
import { A11Y_DEFAULTS, a11yPersists, currentZoomSink, resetAccessibility, setAccessibility, storedAccessibility, TEXT_SIZE_MAX, TEXT_SIZE_MIN, ZOOM_STEPS, zoomStep, type Accessibility } from "./a11y";
import { developerEnabled, setDeveloperEnabled } from "./developer";
import { setTheme, storedTheme, type Theme } from "./theme";

export type SettingsView = {
  el: HTMLElement;
  /** Called each time the page is shown: every section re-reads what it shows, then the list takes the focus. */
  show(): void;
  /** Chooses a section, and an item of it when given (File › Export › Website… opens Settings › Export › Website). */
  open(sectionId: string, itemId?: string): void;
  /** The id of the section that shows. The page also fires a `zen-settings-section` event ({ detail: { section } }) when it changes. */
  current(): string;
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
  /** A small heading over the rows of the left column (a group name); not shown when the list is empty. */
  heading?: string;
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

/** The name of an extra Appearance row: its aria-label, else its title text, else a number. */
function rowLabel(row: HTMLElement, i: number): string {
  return row.getAttribute("aria-label") || row.querySelector(".zen-navside-title")?.textContent || `Option ${i + 1}`;
}

/**
 * Appearance: a list like Accessibility's. Theme (System, Light or Dark as radios, and a small preview of a page in
 * the one chosen) applies to this machine and all windows. Each `extra` row (the sidebar side, ui/nav-panel) is an
 * item after Theme, named by its label, and shows in the right column when chosen.
 */
export function appearanceSection(doc: () => Document, extra: HTMLElement[] = []): SettingsSection {
  return {
    id: "appearance", label: "Appearance",
    items: () => [{ id: "theme", label: "Theme" }, ...extra.map((row, i): SettingsItem => ({ id: `extra-${i}`, label: rowLabel(row, i) }))],
    render(item, right) {
      if (item.id !== "theme") {
        const row = extra[Number(item.id.slice("extra-".length))];
        if (row) right.append(row);
        return;
      }
      const hint = el("p", "zen-settings-hint");
      const shown = el("div", "zen-theme-preview-slot");
      const sync = () => {
        const t = THEMES.find((x) => x.id === storedTheme()) ?? THEMES[0];
        hint.textContent = t.note;
        shown.replaceChildren(themePreview(t.id));
      };
      const radios = segment<Theme>("Theme", THEMES.map((t) => ({ value: t.id, label: t.label })), () => storedTheme(), (v) => { setTheme(v, doc()); sync(); });
      sync();
      right.append(radios.el, hint, shown);
    },
  };
}

// ---- Accessibility (ui/accessibility)

const percent = (z: number) => `${Math.round(z * 100)}%`;

/** A labelled row of the right column: the label, the control and what the default is. */
function a11yRow(label: string, control: HTMLElement, def: string): HTMLElement {
  const row = el("div", "zen-a11y-row");
  const name = el("span", "zen-a11y-label", label);
  const note = el("span", "zen-a11y-default", `Default: ${def}`);
  row.append(name, control, note);
  return row;
}

/** A group of radio buttons (role radiogroup) with the arrow keys: the zen-settings-segment of the page. */
function segment<T extends string>(label: string, options: { value: T; label: string }[], get: () => T, set: (v: T) => void): { el: HTMLElement; sync(): void } {
  const group = el("div", "zen-settings-segment");
  group.setAttribute("role", "radiogroup");
  group.setAttribute("aria-label", label);
  const radios = options.map((o) => {
    const b = el("button", "zen-settings-segment-option", o.label);
    b.type = "button";
    b.setAttribute("role", "radio");
    b.addEventListener("click", () => { set(o.value); sync(); });
    return b;
  });
  function sync(): void {
    options.forEach((o, i) => {
      const on = o.value === get();
      radios[i].setAttribute("aria-checked", String(on));
      radios[i].tabIndex = on ? 0 : -1;
    });
  }
  group.addEventListener("keydown", (e) => {
    const step = ({ ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 } as Record<string, number>)[e.key];
    if (step === undefined) return;
    e.preventDefault();
    e.stopPropagation();
    const at = Math.max(0, options.findIndex((o) => o.value === get()));
    const next = Math.min(options.length - 1, Math.max(0, at + step));
    set(options[next].value);
    sync();
    radios[next].focus();
  });
  group.append(...radios);
  sync();
  return { el: group, sync };
}

/** A stepper: − and + beside the value (role spinbutton, with ↑ ↓ Home and End). `move` is -1, 1, or "min" / "max". */
function stepper(label: string, o: { text(): string; spoken(): string; now(): number; min: number; max: number; move(to: -1 | 1 | "min" | "max"): void }): { el: HTMLElement; sync(): void } {
  const box = el("div", "zen-stepper");
  const value = el("div", "zen-stepper-value");
  value.setAttribute("role", "spinbutton");
  value.setAttribute("aria-label", label);
  value.tabIndex = 0;
  value.setAttribute("aria-valuemin", String(o.min));
  value.setAttribute("aria-valuemax", String(o.max));
  const less = button(`Decrease ${label.toLowerCase()}`, () => go(-1), { text: "\u2212" });
  const more = button(`Increase ${label.toLowerCase()}`, () => go(1), { text: "+" });
  function sync(): void {
    value.textContent = o.text();
    value.setAttribute("aria-valuenow", String(o.now()));
    value.setAttribute("aria-valuetext", o.spoken());
    less.disabled = o.now() <= o.min;
    more.disabled = o.now() >= o.max;
  }
  function go(to: -1 | 1 | "min" | "max"): void { o.move(to); sync(); }
  value.addEventListener("keydown", (e) => {
    const to = ({ ArrowUp: 1, ArrowRight: 1, ArrowDown: -1, ArrowLeft: -1, Home: "min", End: "max" } as Record<string, -1 | 1 | "min" | "max">)[e.key];
    if (to === undefined) return;
    e.preventDefault();
    e.stopPropagation();
    go(to);
  });
  box.append(less, value, more);
  sync();
  return { el: box, sync };
}

const changed = () => {
  const now = storedAccessibility();
  return (Object.keys(A11Y_DEFAULTS) as (keyof Accessibility)[]).some((k) => now[k] !== A11Y_DEFAULTS[k]);
};

/**
 * Accessibility: Zoom (the whole window), Motion (Reduce motion), Contrast (Increase contrast) and, with `reader`,
 * Reading (text size, font, line spacing, line width, link underlines, with a live sample). Every option takes effect
 * at once and is kept per machine (shell/a11y.ts). `setZoom` scales the web view (the app gives Window.SetZoom);
 * without it the one the app gave `followAccessibility` is used.
 */
export function accessibilitySection(doc: () => Document, o: { reader: boolean; setZoom?: (z: number) => void }): SettingsSection {
  const zoomTo = (z: number) => (o.setZoom ?? currentZoomSink())?.(z);
  return {
    id: "accessibility", label: "Accessibility",
    items: () => [
      { id: "zoom", label: "Zoom" },
      { id: "motion", label: "Motion" },
      { id: "contrast", label: "Contrast" },
      ...(o.reader ? [{ id: "reading", label: "Reading" }] : []),
    ],
    render(item, right) {
      const syncs: (() => void)[] = [];
      const add = <T extends { el: HTMLElement; sync(): void }>(x: T): HTMLElement => { syncs.push(x.sync); return x.el; };
      const set = (patch: Partial<Accessibility>) => { setAccessibility(patch, doc()); refresh(); };
      const reset = button("Reset to defaults", () => {
        const z = storedAccessibility().zoom;
        resetAccessibility(doc());
        if (z !== A11Y_DEFAULTS.zoom) zoomTo(A11Y_DEFAULTS.zoom);
        syncs.forEach((f) => f());
        refresh();
      });
      function refresh(): void { reset.disabled = !changed(); }

      if (item.id === "zoom") {
        right.append(el("p", "zen-settings-hint", "Zooms the whole window: the sidebar, the page, the panels and the dialogs. It is the same in every window. Keys: \u2318\u2212, \u23180 and \u2318+ (View menu)."));
        const z = stepper("Zoom", {
          text: () => percent(storedAccessibility().zoom),
          spoken: () => `${Math.round(storedAccessibility().zoom * 100)} percent`,
          now: () => storedAccessibility().zoom,
          min: ZOOM_STEPS[0], max: ZOOM_STEPS[ZOOM_STEPS.length - 1],
          move(to) {
            const cur = storedAccessibility().zoom;
            const next = to === "min" ? ZOOM_STEPS[0] : to === "max" ? ZOOM_STEPS[ZOOM_STEPS.length - 1] : zoomStep(cur, to);
            if (next === cur) return;
            set({ zoom: next });
            zoomTo(next);
          },
        });
        right.append(a11yRow("Zoom", add(z), "100%"));
      } else if (item.id === "motion") {
        right.append(el("p", "zen-settings-hint", "On does what the macOS setting Reduce motion does, in every window: no fades, slides or moving bars. System follows the Mac."));
        right.append(a11yRow("Reduce motion", add(segment("Reduce motion", [{ value: "system", label: "System" }, { value: "on", label: "On" }], () => storedAccessibility().reduceMotion, (v) => set({ reduceMotion: v }))), "System"));
      } else if (item.id === "contrast") {
        right.append(el("p", "zen-settings-hint", "Muted text becomes full strength, lines are easier to see and the focus ring is thicker, in both themes. System follows the Mac (Increase contrast)."));
        right.append(a11yRow("Increase contrast", add(segment("Increase contrast", [{ value: "system", label: "System" }, { value: "more", label: "On" }], () => storedAccessibility().contrast, (v) => set({ contrast: v }))), "System"));
        const sample = el("div", "zen-a11y-sample zen-a11y-contrast-sample");
        sample.setAttribute("aria-hidden", "true");
        sample.append(el("p", "", "Text in the page"), el("p", "zen-a11y-muted", "Muted text, such as a date or a hint"));
        right.append(sample);
      } else if (item.id === "reading") {
        right.append(el("p", "zen-settings-hint", "How the reader shows a page, and the preview of a page in the editor. The sidebar and the controls keep their size (use Zoom for those). Print, PDF and built sites keep the document's own look."));
        const size = stepper("Text size", {
          text: () => `${storedAccessibility().textSize} px`,
          spoken: () => `${storedAccessibility().textSize} pixels`,
          now: () => storedAccessibility().textSize,
          min: TEXT_SIZE_MIN, max: TEXT_SIZE_MAX,
          move(to) {
            const cur = storedAccessibility().textSize;
            set({ textSize: to === "min" ? TEXT_SIZE_MIN : to === "max" ? TEXT_SIZE_MAX : cur + to });
          },
        });
        right.append(a11yRow("Text size", add(size), "17 px"));
        right.append(a11yRow("Font", add(segment("Font", [{ value: "serif", label: "Serif" }, { value: "sans", label: "Sans" }], () => storedAccessibility().font, (v) => set({ font: v }))), "Serif"));
        right.append(a11yRow("Line spacing", add(segment("Line spacing", [{ value: "normal", label: "Normal" }, { value: "relaxed", label: "Relaxed" }, { value: "loose", label: "Loose" }], () => storedAccessibility().lineSpacing, (v) => set({ lineSpacing: v }))), "Normal"));
        right.append(a11yRow("Line width", add(segment("Line width", [{ value: "narrow", label: "Narrow" }, { value: "normal", label: "Normal" }, { value: "wide", label: "Wide" }], () => storedAccessibility().lineWidth, (v) => set({ lineWidth: v }))), "Normal"));
        const under = el("label", "zen-settings-check");
        const box = el("input");
        box.type = "checkbox";
        box.checked = storedAccessibility().underlineLinks;
        box.addEventListener("change", () => set({ underlineLinks: box.checked }));
        under.append(box, el("span", "", "Underline links"));
        syncs.push(() => { box.checked = storedAccessibility().underlineLinks; });
        const underRow = el("div", "zen-a11y-row");
        underRow.append(el("span", "zen-a11y-label", ""), under, el("span", "zen-a11y-default", "Default: off"));
        right.append(underRow);
        const link = el("a", "", "a link");
        link.href = "#";
        link.tabIndex = -1;
        link.addEventListener("click", (e) => e.preventDefault());
        const sample = el("p", "zen-a11y-sample zen-a11y-reading-sample");
        sample.setAttribute("aria-hidden", "true");
        sample.append("Sample paragraph in the reading style: the quick brown fox jumps over the lazy dog, then reads ", link, " and a line of ", el("code", "", "code()"), " at the size, spacing and width chosen above.");
        right.append(sample);
      }
      const foot = el("div", "zen-settings-actions");
      foot.append(reset);
      right.append(foot);
      if (!a11yPersists()) right.append(el("p", "zen-settings-hint zen-a11y-unsaved", "Not saved: applies to this window only."));
      refresh();
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
    // Nothing to list: the empty text stands where the items would be, in every section alike.
    if (!pane.items.length) {
      const empty = pane.section.empty;
      const text = empty ? (typeof empty === "function" ? empty() : empty) : "";
      pane.list.replaceChildren(...(text ? [el("p", "zen-settings-empty", text)] : []));
      return;
    }
    const head = pane.section.heading ? [Object.assign(el("div", "zen-settings-list-heading", pane.section.heading), { role: "presentation" })] : [];
    pane.list.replaceChildren(...head, ...pane.items.map((it) => {
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
      pane.detail.replaceChildren(); // the empty text is in the left column (drawList)
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
    page.dispatchEvent(new CustomEvent("zen-settings-section", { detail: { section: id } }));
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
    current: () => current,
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
