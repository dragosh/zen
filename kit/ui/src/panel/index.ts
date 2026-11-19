// A tabbed bottom panel (ui/panel): a header with tabs (each with an optional count chip), a slot
// for the panel's own tools and a close button, one scrolling body per tab, and a drag handle on
// the top edge that resizes it. The Inspector is built on it; a new tab is one more entry in
// `tabs` and content in its body. All text goes in through textContent.
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
};

export type TabbedPanel = {
  el: HTMLElement;
  /** The right-hand side of the header, for the panel's own controls. */
  tools: HTMLElement;
  readonly selected: string;
  body(id: string): HTMLElement;
  select(id: string, remember?: boolean): void;
  /** A count chip on the tab: hidden for null or 0. */
  setCount(id: string, n: number | null, tone?: CountTone): void;
  readonly height: number;
  setHeight(px: number): void;
  destroy(): void;
};

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

  const tabs = new Map<string, { tab: HTMLButtonElement; count: HTMLElement; body: HTMLElement }>();
  for (const { id, label } of o.tabs) {
    const tab = button("", () => select(id), { className: "zen-panel-tab" });
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
    tablist.append(tab);
    tabs.set(id, { tab, count, body });
  }
  const ids = o.tabs.map((t) => t.id);
  let selected = ids.includes(o.tabKey ? read(o.tabKey) ?? "" : "") ? read(o.tabKey!)! : ids[0];

  function select(id: string, remember = true): void {
    if (!tabs.has(id)) return;
    selected = id;
    if (remember && o.tabKey) write(o.tabKey, id);
    for (const [k, t] of Array.from(tabs)) {
      t.tab.setAttribute("aria-selected", String(k === id));
      t.tab.tabIndex = k === id ? 0 : -1;
      t.body.hidden = k !== id;
    }
  }
  const onTabKey = (e: KeyboardEvent) => {
    const i = ids.indexOf(selected);
    const to = ({ ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: ids.length - 1 } as Record<string, number | undefined>)[e.key];
    if (to === undefined) return;
    e.preventDefault();
    const next = ids[(to + ids.length) % ids.length];
    select(next);
    tabs.get(next)!.tab.focus();
  };
  tablist.addEventListener("keydown", onTabKey);

  if (o.onClose) {
    const close = button("", () => o.onClose!(), { className: "zen-panel-close ghost", title: `Close ${o.label} (Esc)` });
    close.setAttribute("aria-label", `Close ${o.label}`);
    close.textContent = "✕";
    tools.after(close);
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
    root.style.height = `${height}px`;
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
  if (o.resize) {
    const n = Number.parseFloat(read(o.resize.key) ?? "");
    height = Math.max(o.resize.min, Math.round(Number.isFinite(n) ? n : o.resize.initial));
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
  select(selected, false);

  return {
    el: root,
    tools,
    get selected() { return selected; },
    body: (id) => tabs.get(id)!.body,
    select,
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
    destroy() {
      tablist.removeEventListener("keydown", onTabKey);
      view?.removeEventListener("mousemove", onMove);
      view?.removeEventListener("mouseup", onUp);
      o.doc.body.classList.remove("zen-panel-resizing");
      root.remove();
    },
  };
}
