// The frame: sidebar (nav) | resize edge | header + main. Owns the sidebar
// width and visibility; knows nothing about what is mounted in the slots.
import { button, el } from "./dom";
import type { Slots } from "./types";

export const SIDEBAR_DEFAULT = 260;
export const SIDEBAR_MIN = 200;
export const SIDEBAR_MAX = 400;
export const WIDTH_KEY = "zen.sidebar.width";
const KEY_STEP = 16;

export type LayoutHandlers = { onBack?(): void; onForward?(): void; onOpenFolder?(): void };

export type Layout = {
  el: HTMLElement;
  slots: Slots;
  toggleSidebar(): void;
  sidebarVisible(): boolean;
  sidebarWidth(): number;
  /** Clamps to 200–400, applies and remembers the width. */
  setSidebarWidth(px: number): void;
  setHistoryState(canBack: boolean, canForward: boolean): void;
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
  const open = button("Open a folder", () => handlers.onOpenFolder?.(), { className: "ghost", text: "Open…", title: "Open a folder ⌘O" });
  back.disabled = forward.disabled = true;
  controls.append(toggle, back, forward);

  const breadcrumb = el("div", "zen-breadcrumb");
  const actions = el("div", "zen-actions");
  header.append(controls, breadcrumb, actions, open);
  const main = el("main", "zen-main");
  content.append(header, main);
  frame.append(nav, edge, content);

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
    slots: { nav, breadcrumb, actions, main },
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
    destroy() {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    },
  };
  apply();
  return layout;
}
