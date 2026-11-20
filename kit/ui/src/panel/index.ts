// A tabbed bottom panel (ui/panel): a header with tabs (each with an optional count chip), the
// current tab's own tools (a filter, actions), a collapse chevron and a close button, one scrolling
// body per tab, and a drag handle on the top edge that resizes it. The Inspector is built on it; a
// new tab is one more entry in `tabs` and content in its body. A body can have two columns
// (panelColumns). All text goes in through textContent.
import "./panel.css";
import { button, el } from "../shell/dom";

export type PanelTab = { id: string; label: string };
export type CountTone = "neutral" | "error" | "warning";

export type TabbedPanelOptions = {
  doc: Document;
  /** Accessible name of the panel and its tab list ("Inspector"). */
  label: string;
  /** Prefix for the element ids (unique per panel). */
  idPrefix: string;
  tabs: readonly PanelTab[];
  /** localStorage key that remembers the chosen tab; none: always the first. */
  tabKey?: string;
  /** Shows a close button (✕) that calls it. */
  onClose?: () => void;
  /** Resizable from its top edge: the height is remembered under `key`. */
  resize?: { key: string; initial: number; min: number };
  /** localStorage key that remembers whether the panel is collapsed to its header; shows the chevron whatever the key. */
  collapseKey?: string;
};

export type TabbedPanel = {
  el: HTMLElement;
  /** The right-hand side of the header, for the panel's own controls. */
  tools: HTMLElement;
  /** The controls of one tab (a filter, actions): shown in the header only while that tab is selected. */
  toolsFor(id: string): HTMLElement;
  readonly selected: string;
  /** A tab that is not visible is out of the tab list, the tab order and the header; the selection moves to the first visible one. */
  setTabVisible(id: string, visible: boolean): void;
  isTabVisible(id: string): boolean;
  /** Visible tab ids in order. */
  readonly visibleTabs: string[];
  /** Only the header shows. Remembered under `collapseKey`. */
  readonly collapsed: boolean;
  setCollapsed(collapsed: boolean): void;
  body(id: string): HTMLElement;
  select(id: string, remember?: boolean): void;
  /** A count chip on the tab: hidden for null or 0. */
  setCount(id: string, n: number | null, tone?: CountTone): void;
  readonly height: number;
  setHeight(px: number): void;
  /** Reads the remembered height again (another panel may have changed it). */
  reload(): void;
  destroy(): void;
};

/** The height of the bottom panel (the Inspector). */
export const BOTTOM_PANEL_HEIGHT = { key: "zen.panel.height", initial: 240, min: 120 } as const;

/** localStorage key of the collapsed state. */
export const PANEL_COLLAPSED_KEY = "zen.panel.collapsed";

/** Share of the parent the panel may take when it is resized. */
export const PANEL_MAX_SHARE = 0.7;
const KEY_STEP = 16;

function read(key: string): string | null {
  try { return localStorage.getItem(key); } catch { return null; }
}
function write(key: string, value: string): void {
  try { localStorage.setItem(key, value); } catch { /* not remembered */ }
}

/** An empty state: a short title and an optional hint underneath, centred in the body. */
export function panelEmpty(title: string, hint = ""): HTMLElement {
  const box = el("div", "zen-panel-empty");
  box.append(el("p", "zen-panel-empty-title", title));
  if (hint) box.append(el("p", "zen-panel-empty-hint", hint));
  return box;
}

/**
 * The two columns of a tab: the list or main info on the left (`.zen-panel-col-main`), the details on the
 * right (`.zen-panel-col-side`, narrower). Each scrolls on its own; under about 700 px wide they stack and the tab scrolls.
 */
export function panelColumns(): { el: HTMLElement; left: HTMLElement; right: HTMLElement } {
  const root = el("div", "zen-panel-cols");
  const left = el("div", "zen-panel-col zen-panel-col-main");
  const right = el("div", "zen-panel-col zen-panel-col-side");
  root.append(left, right);
  return { el: root, left, right };
}

/** A grid of label/value pairs that flows into as many columns as the width allows. */
export function panelProps(items: readonly [label: string, field: string, value: string, mono?: boolean][]): HTMLElement {
  const dl = el("dl", "zen-panel-props");
  for (const [label, field, value, mono] of items) {
    const pair = el("div", "zen-panel-prop");
    const dd = el("dd", mono ? "zen-panel-mono" : "", value);
    dd.dataset.field = field;
    dd.title = value;
    pair.append(el("dt", "", label), dd);
    dl.append(pair);
  }
  return dl;
}

export function createTabbedPanel(o: TabbedPanelOptions): TabbedPanel {
  if (o.tabs.length === 0) throw new Error("panel: at least one tab");
  const root = el("section", "zen-panel");
  root.setAttribute("aria-label", o.label);

  const head = el("div", "zen-panel-head");
  const tablist = el("div", "zen-panel-tabs");
  tablist.setAttribute("role", "tablist");
  tablist.setAttribute("aria-label", o.label);
  const tools = el("div", "zen-panel-tools");
  head.append(tablist, tools);

  const tabs = new Map<string, { tab: HTMLButtonElement; count: HTMLElement; body: HTMLElement; tools: HTMLElement; visible: boolean }>();
  for (const { id, label } of o.tabs) {
    const tab = button("", () => { select(id); setCollapsed(false); }, { className: "zen-panel-tab" });
    tab.setAttribute("role", "tab");
    tab.id = `${o.idPrefix}-tab-${id}`;
    tab.dataset.tab = id;
    const name = el("span", "zen-panel-tab-label", label);
    const count = el("span", "zen-panel-count");
    count.hidden = true;
    count.setAttribute("aria-hidden", "true");
    tab.append(name, count);
    const body = el("div", "zen-panel-body");
    body.id = `${o.idPrefix}-panel-${id}`;
    body.dataset.section = id;
    body.setAttribute("role", "tabpanel");
    body.setAttribute("aria-labelledby", tab.id);
    body.tabIndex = 0;
    tab.setAttribute("aria-controls", body.id);
    const group = el("div", "zen-panel-tools-group");
    group.dataset.tools = id;
    group.hidden = true;
    tools.append(group);
    tablist.append(tab);
    tabs.set(id, { tab, count, body, tools: group, visible: true });
  }
  const ids = o.tabs.map((t) => t.id);
  const visibleIds = () => ids.filter((i) => tabs.get(i)!.visible);
  let selected = ids.includes(o.tabKey ? read(o.tabKey) ?? "" : "") ? read(o.tabKey!)! : ids[0];
  // The tab the user wants: it comes back when it is visible again, whatever tab stood in for it meanwhile.
  let wanted = selected;
  let collapsed = false;
  const resolve = (): void => {
    selected = tabs.get(wanted)!.visible ? wanted : visibleIds()[0] ?? selected;
  };

  function paint(): void {
    for (const [k, t] of Array.from(tabs)) {
      const on = k === selected && t.visible;
      t.tab.hidden = !t.visible;
      t.tab.setAttribute("aria-selected", String(on));
      t.tab.tabIndex = on ? 0 : -1;
      t.body.hidden = !on || collapsed;
      t.tools.hidden = !on;
    }
  }
  function select(id: string, remember = true): void {
    const t = tabs.get(id);
    if (!t || !t.visible) return;
    selected = wanted = id;
    if (remember && o.tabKey) write(o.tabKey, id);
    paint();
  }
  function setTabVisible(id: string, visible: boolean): void {
    const t = tabs.get(id);
    if (!t || t.visible === visible) return;
    t.visible = visible;
    resolve();
    paint();
  }
  const onTabKey = (e: KeyboardEvent) => {
    const vis = visibleIds();
    const i = vis.indexOf(selected);
    const to = ({ ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: vis.length - 1 } as Record<string, number | undefined>)[e.key];
    if (to === undefined || vis.length === 0) return;
    e.preventDefault();
    const next = vis[(to + vis.length) % vis.length];
    select(next);
    tabs.get(next)!.tab.focus();
  };
  tablist.addEventListener("keydown", onTabKey);

  // The chevron collapses the panel to its header and expands it again.
  const chevron = button("", () => setCollapsed(!collapsed), { className: "zen-panel-collapse ghost" });
  function setCollapsed(next: boolean, remember = true): void {
    if (next === collapsed) return;
    collapsed = next;
    root.classList.toggle("is-collapsed", collapsed);
    chevron.textContent = collapsed ? "▴" : "▾";
    chevron.title = `${collapsed ? "Expand" : "Collapse"} ${o.label}`;
    chevron.setAttribute("aria-label", chevron.title);
    chevron.setAttribute("aria-expanded", String(!collapsed));
    if (resizer) resizer.hidden = collapsed;
    if (remember && o.collapseKey) write(o.collapseKey, collapsed ? "1" : "0");
    paint();
    apply();
  }
  tools.after(chevron);

  if (o.onClose) {
    const close = button("", () => o.onClose!(), { className: "zen-panel-close ghost", title: `Close ${o.label} (Esc)` });
    close.setAttribute("aria-label", `Close ${o.label}`);
    close.textContent = "✕";
    chevron.after(close);
  }

  // ---- resizing from the top edge (like the Developer panel)
  let height = 0;
  let resizer: HTMLElement | null = null;
  const maxHeight = () => {
    const h = root.parentElement?.getBoundingClientRect().height ?? 0;
    return h > 0 ? Math.max(o.resize!.min, Math.floor(h * PANEL_MAX_SHARE)) : Number.POSITIVE_INFINITY;
  };
  const clamp = (px: number) => Math.round(Math.min(maxHeight(), Math.max(o.resize!.min, px)));
  function apply(): void {
    if (!o.resize) return;
    root.style.height = collapsed ? "" : `${height}px`;
    resizer?.setAttribute("aria-valuenow", String(height));
  }
  function setHeight(px: number): void {
    if (!o.resize) return;
    height = clamp(px);
    apply();
    write(o.resize.key, String(height));
  }
  let dragging = false;
  const view = o.doc.defaultView;
  const onMove = (e: MouseEvent) => {
    if (!dragging) return;
    height = clamp(root.getBoundingClientRect().bottom - e.clientY);
    apply();
  };
  const onUp = () => {
    if (!dragging) return;
    dragging = false;
    root.classList.remove("is-resizing");
    o.doc.body.classList.remove("zen-panel-resizing");
    if (o.resize) write(o.resize.key, String(height));
  };
  const stored = () => {
    const n = Number.parseFloat(read(o.resize!.key) ?? "");
    return Math.max(o.resize!.min, Math.round(Number.isFinite(n) ? n : o.resize!.initial));
  };
  if (o.resize) {
    height = stored();
    resizer = el("div", "zen-panel-resizer");
    resizer.setAttribute("role", "separator");
    resizer.setAttribute("aria-orientation", "horizontal");
    resizer.setAttribute("aria-label", `Resize ${o.label}`);
    resizer.setAttribute("aria-valuemin", String(o.resize.min));
    resizer.tabIndex = 0;
    resizer.addEventListener("mousedown", (e) => {
      e.preventDefault();
      dragging = true;
      root.classList.add("is-resizing");
      o.doc.body.classList.add("zen-panel-resizing");
    });
    resizer.addEventListener("keydown", (e) => {
      const to = ({ ArrowUp: height + KEY_STEP, ArrowDown: height - KEY_STEP, Home: o.resize!.min, End: maxHeight() } as Record<string, number | undefined>)[e.key];
      if (to === undefined) return;
      e.preventDefault();
      setHeight(Number.isFinite(to) ? to : height);
    });
    view?.addEventListener("mousemove", onMove);
    view?.addEventListener("mouseup", onUp);
    root.append(resizer);
    apply();
  }

  root.append(head, ...Array.from(tabs.values(), (t) => t.body));
  resolve();
  paint();
  setCollapsed(false, false);
  chevron.textContent = "▾";
  chevron.title = `Collapse ${o.label}`;
  chevron.setAttribute("aria-label", chevron.title);
  chevron.setAttribute("aria-expanded", "true");
  if (o.collapseKey && read(o.collapseKey) === "1") setCollapsed(true, false);

  return {
    el: root,
    tools,
    toolsFor: (id) => tabs.get(id)!.tools,
    get selected() { return selected; },
    body: (id) => tabs.get(id)!.body,
    select,
    setTabVisible,
    isTabVisible: (id) => tabs.get(id)?.visible ?? false,
    get visibleTabs() { return visibleIds(); },
    get collapsed() { return collapsed; },
    setCollapsed,
    setCount(id, n, tone = "neutral") {
      const t = tabs.get(id);
      if (!t) return;
      const show = typeof n === "number" && n > 0;
      t.count.hidden = !show;
      t.count.textContent = show ? String(n) : "";
      t.count.dataset.tone = tone;
      const label = t.tab.querySelector(".zen-panel-tab-label")!.textContent ?? "";
      t.tab.setAttribute("aria-label", show ? `${label}, ${n}` : label);
    },
    get height() { return height; },
    setHeight,
    reload() {
      if (!o.resize) return;
      height = stored();
      apply();
    },
    destroy() {
      tablist.removeEventListener("keydown", onTabKey);
      view?.removeEventListener("mousemove", onMove);
      view?.removeEventListener("mouseup", onUp);
      o.doc.body.classList.remove("zen-panel-resizing");
      root.remove();
    },
  };
}
