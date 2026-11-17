// The frame: sidebar (nav) | resize edge | header + main. Owns the sidebar
// width and visibility; knows nothing about what is mounted in the slots.
import { button, el } from "./dom";
import type { Slots } from "./types";

export const SIDEBAR_DEFAULT = 260;
export const SIDEBAR_MIN = 200;
export const SIDEBAR_MAX = 400;
export const WIDTH_KEY = "zen.sidebar.width";
const KEY_STEP = 16;

export type LayoutHandlers = {
  onBack?(): void; onForward?(): void; onOpenFolder?(): void;
  /** The settings cog at the bottom of the sidebar; `anchor` is the cog button. */
  onSettings?(anchor: HTMLButtonElement): void;
};

const SVG = "http://www.w3.org/2000/svg";
/** A gear outline, built with DOM APIs. */
function cogIcon(doc: Document): SVGSVGElement {
  const svg = doc.createElementNS(SVG, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("width", "16");
  svg.setAttribute("height", "16");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("class", "zen-icon");
  const gear = doc.createElementNS(SVG, "path");
  gear.setAttribute("d", "M10.3 2.6h3.4l.5 2.6 1.6.9 2.5-.9 1.7 2.9-2 1.7v1.9l2 1.7-1.7 2.9-2.5-.9-1.6.9-.5 2.6h-3.4l-.5-2.6-1.6-.9-2.5.9-1.7-2.9 2-1.7v-1.9l-2-1.7 1.7-2.9 2.5.9 1.6-.9z");
  const hub = doc.createElementNS(SVG, "circle");
  hub.setAttribute("cx", "12");
  hub.setAttribute("cy", "12");
  hub.setAttribute("r", "3");
  for (const n of [gear, hub]) {
    n.setAttribute("fill", "none");
    n.setAttribute("stroke", "currentColor");
    n.setAttribute("stroke-width", "1.6");
    n.setAttribute("stroke-linejoin", "round");
  }
  svg.append(gear, hub);
  return svg;
}

export type Layout = {
  el: HTMLElement;
  slots: Slots;
  toggleSidebar(): void;
  sidebarVisible(): boolean;
  sidebarWidth(): number;
  /** Clamps to 200–400, applies and remembers the width. */
  setSidebarWidth(px: number): void;
  setHistoryState(canBack: boolean, canForward: boolean): void;
  /**
   * Puts a view in the place of the main pane (Settings): the pane and the header's
   * breadcrumb and actions are hidden, not unmounted. null brings the pane back.
   */
  showView(view: HTMLElement | null): void;
  /** Reflects on the cog whether the Settings view is showing. */
  setSettingsOpen(open: boolean): void;
  destroy(): void;
};

const clamp = (px: number) => Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, Math.round(px)));

function storedWidth(): number {
  try {
    const n = Number.parseFloat(localStorage.getItem(WIDTH_KEY) ?? "");
    return Number.isFinite(n) ? clamp(n) : SIDEBAR_DEFAULT;
  } catch {
    return SIDEBAR_DEFAULT; // storage unavailable
  }
}

export function createLayout(handlers: LayoutHandlers = {}): Layout {
  const frame = el("div", "zen-frame");
  const nav = el("nav", "zen-sidebar");
  nav.id = "zen-sidebar";
  nav.setAttribute("aria-label", "Pages");
  // The nav slot is the body; the sidebar adds Open (top right) and the
  // settings cog (bottom) around it.
  const navBody = el("div", "zen-sidebar-body");
  const foot = el("div", "zen-sidebar-foot");
  const settings = button("Settings", () => handlers.onSettings?.(settings), { className: "ghost zen-settings-button", text: "", title: "Settings ⌘," });
  settings.append(cogIcon(nav.ownerDocument));
  settings.setAttribute("aria-expanded", "false");
  foot.append(settings);

  const edge = el("div", "zen-resizer");
  edge.setAttribute("role", "separator");
  edge.setAttribute("aria-orientation", "vertical");
  edge.setAttribute("aria-label", "Resize sidebar");
  edge.setAttribute("aria-controls", nav.id);
  edge.setAttribute("aria-valuemin", String(SIDEBAR_MIN));
  edge.setAttribute("aria-valuemax", String(SIDEBAR_MAX));
  edge.tabIndex = 0;

  const content = el("div", "zen-content");
  const header = el("header", "zen-top");
  const controls = el("div", "zen-top-controls");
  const toggle = button("Toggle sidebar", () => layout.toggleSidebar(), { className: "ghost", text: "◧", title: "Toggle sidebar ⌘\\" });
  toggle.dataset.action = "toggle-sidebar";
  toggle.setAttribute("aria-controls", nav.id);
  const back = button("Back", () => handlers.onBack?.(), { className: "ghost", text: "‹", title: "Back ⌘[" });
  const forward = button("Forward", () => handlers.onForward?.(), { className: "ghost", text: "›", title: "Forward ⌘]" });
  const open = button("Open a folder", () => handlers.onOpenFolder?.(), { className: "ghost zen-sidebar-open", text: "Open…", title: "Open a folder ⌘O" });
  nav.append(navBody, open, foot);
  back.disabled = forward.disabled = true;
  controls.append(toggle, back, forward);

  const breadcrumb = el("div", "zen-breadcrumb");
  const actions = el("div", "zen-actions");
  header.append(controls, breadcrumb, actions);
  const main = el("main", "zen-main");
  content.append(header, main);
  frame.append(nav, edge, content);

  let view: HTMLElement | null = null;
  let width = storedWidth();
  let visible = true;

  const apply = () => {
    frame.style.setProperty("--sidebar-width", `${width}px`);
    edge.setAttribute("aria-valuenow", String(width));
    nav.hidden = edge.hidden = !visible;
    toggle.setAttribute("aria-expanded", String(visible));
  };

  // Drag: the sidebar starts at the frame's left edge, so its width is the
  // pointer's distance from that edge.
  let dragging = false;
  const onMove = (e: MouseEvent) => {
    if (!dragging) return;
    width = clamp(e.clientX - frame.getBoundingClientRect().left);
    apply();
  };
  const onUp = () => {
    if (!dragging) return;
    dragging = false;
    frame.classList.remove("is-resizing");
    layout.setSidebarWidth(width);
  };
  edge.addEventListener("mousedown", (e) => {
    if (e.button !== 0) return;
    e.preventDefault(); // no text selection while dragging
    dragging = true;
    frame.classList.add("is-resizing");
  });
  edge.addEventListener("keydown", (e) => {
    const to = { ArrowLeft: width - KEY_STEP, ArrowRight: width + KEY_STEP, Home: SIDEBAR_MIN, End: SIDEBAR_MAX }[e.key];
    if (to === undefined) return;
    e.preventDefault();
    layout.setSidebarWidth(to);
  });
  window.addEventListener("mousemove", onMove);
  window.addEventListener("mouseup", onUp);

  const layout: Layout = {
    el: frame,
    slots: { nav: navBody, breadcrumb, actions, main },
    toggleSidebar() {
      visible = !visible;
      apply();
    },
    sidebarVisible: () => visible,
    sidebarWidth: () => width,
    setSidebarWidth(px) {
      width = clamp(px);
      apply();
      try {
        localStorage.setItem(WIDTH_KEY, String(width));
      } catch {
        // not remembered; the width still applies for this session
      }
    },
    setHistoryState(canBack, canForward) {
      back.disabled = !canBack;
      forward.disabled = !canForward;
    },
    showView(next) {
      view?.remove();
      view = next;
      if (next) main.after(next);
      main.hidden = actions.hidden = breadcrumb.hidden = next !== null;
    },
    setSettingsOpen(open) {
      settings.setAttribute("aria-expanded", String(open));
    },
    destroy() {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    },
  };
  apply();
  return layout;
}
