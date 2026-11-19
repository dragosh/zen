// The Developer tab (ui/developer-tab): the "Show developer tab" setting, the sidebar button, ⌥⌘J and
// the bottom panel below the page with its drag handle. The setting is per machine (localStorage, like
// the theme) and every window follows it.
import { INSPECTOR_CLOSE, INSPECTOR_STATE, type InspectorState } from "../inspector/events";
import type { KitHost } from "../host";
import { installConsoleCapture } from "./console";
import { createDevPanel, type DevPanel } from "./devpanel";
import { el } from "./dom";
import type { Layout } from "./layout";

export const DEV_KEY = "zen.developer";
export const DEV_HEIGHT_KEY = "zen.developer.height";
export const DEV_HEIGHT_DEFAULT = 240;
export const DEV_HEIGHT_MIN = 120;
const DEV_MAX_SHARE = 0.7; // of the content area
const KEY_STEP = 16;
const EVENT = "zen:developer";

export function developerEnabled(): boolean {
  try {
    return localStorage.getItem(DEV_KEY) === "1";
  } catch {
    return false;
  }
}

/** Remembers the setting and tells this window (other windows hear the storage event). */
export function setDeveloperEnabled(on: boolean): void {
  try {
    if (on) localStorage.setItem(DEV_KEY, "1");
    else localStorage.removeItem(DEV_KEY);
  } catch { /* storage unavailable: this window only */ }
  window.dispatchEvent(new CustomEvent(EVENT));
}

/** Calls fn when the setting changes, here or in another window. */
export function followDeveloper(fn: () => void, win: Window = window): () => void {
  const onStorage = (e: StorageEvent) => { if (e.key === null || e.key === DEV_KEY) fn(); };
  win.addEventListener(EVENT, fn);
  win.addEventListener("storage", onStorage);
  return () => {
    win.removeEventListener(EVENT, fn);
    win.removeEventListener("storage", onStorage);
  };
}

function storedHeight(): number {
  try {
    const n = Number.parseFloat(localStorage.getItem(DEV_HEIGHT_KEY) ?? "");
    return Number.isFinite(n) ? Math.max(DEV_HEIGHT_MIN, Math.round(n)) : DEV_HEIGHT_DEFAULT;
  } catch {
    return DEV_HEIGHT_DEFAULT;
  }
}

export type Developer = {
  toggle(): void;
  isOpen(): boolean;
  height(): number;
  /** Clamps to 120 px … 70% of the content, applies and remembers. */
  setHeight(px: number): void;
  destroy(): void;
};

/** ⌥⌘J (or ⌥⌃J): the key is "∆" with Option held, so the physical key is what counts. */
export function developerKey(e: KeyboardEvent): boolean {
  return e.altKey && (e.metaKey || e.ctrlKey) && !e.shiftKey && e.code === "KeyJ";
}

export function createDeveloper(layout: Layout, host: Pick<KitHost, "logs" | "onLog">, doc: Document = document): Developer {
  const capture = installConsoleCapture(doc.defaultView as (Window & typeof globalThis) | undefined);
  let wrap: HTMLElement | null = null;
  let panel: DevPanel | null = null;
  let open = false;
  let height = storedHeight();

  const maxHeight = (): number => {
    const h = layout.el.getBoundingClientRect().height;
    return h > 0 ? Math.max(DEV_HEIGHT_MIN, Math.floor(h * DEV_MAX_SHARE)) : Number.POSITIVE_INFINITY;
  };
  const clamp = (px: number) => Math.round(Math.min(maxHeight(), Math.max(DEV_HEIGHT_MIN, px)));
  const apply = () => {
    if (!wrap) return;
    wrap.style.height = `${height}px`;
    handle.setAttribute("aria-valuenow", String(height));
    // Side panels (the Inspector) end above the open panel.
    layout.el.style.setProperty("--zen-dev-height", `${height}px`);
  };
  const developer: Developer = {
    toggle() {
      if (open) hide();
      else if (developerEnabled()) show();
    },
    isOpen: () => open,
    height: () => height,
    setHeight(px) {
      height = clamp(px);
      apply();
      try { localStorage.setItem(DEV_HEIGHT_KEY, String(height)); } catch { /* not remembered */ }
    },
    destroy() {
      doc.removeEventListener(INSPECTOR_STATE, onInspector);
      stopFollow();
      doc.removeEventListener("keydown", onKey);
      doc.defaultView?.removeEventListener("mousemove", onMove);
      doc.defaultView?.removeEventListener("mouseup", onUp);
      hide();
      panel?.destroy();
      panel = null;
      layout.setDeveloperAvailable(false);
    },
  };

  // The drag handle on the panel's top edge: the panel ends at the content's bottom, so its height
  // is the pointer's distance from there.
  const handle = el("div", "zen-dev-resizer");
  handle.setAttribute("role", "separator");
  handle.setAttribute("aria-orientation", "horizontal");
  handle.setAttribute("aria-label", "Resize developer panel");
  handle.setAttribute("aria-valuemin", String(DEV_HEIGHT_MIN));
  handle.tabIndex = 0;
  let dragging = false;
  const onMove = (e: MouseEvent) => {
    if (!dragging) return;
    height = clamp(layout.el.getBoundingClientRect().bottom - e.clientY);
    apply();
  };
  const onUp = () => {
    if (!dragging) return;
    dragging = false;
    layout.el.classList.remove("is-resizing-dev");
    developer.setHeight(height);
  };
  handle.addEventListener("mousedown", (e) => {
    if (e.button !== 0) return;
    e.preventDefault();
    dragging = true;
    layout.el.classList.add("is-resizing-dev");
  });
  handle.addEventListener("keydown", (e) => {
    const to = { ArrowUp: height + KEY_STEP, ArrowDown: height - KEY_STEP, Home: DEV_HEIGHT_MIN, End: maxHeight() }[e.key];
    if (to === undefined) return;
    e.preventDefault();
    developer.setHeight(Number.isFinite(to) ? to : height);
  });
  doc.defaultView?.addEventListener("mousemove", onMove);
  doc.defaultView?.addEventListener("mouseup", onUp);

  function show(): void {
    if (!wrap) {
      panel = createDevPanel({ doc, host, capture, onClose: hide });
      wrap = el("div", "zen-dev");
      wrap.append(handle, panel.el);
    }
    open = true;
    // One bottom panel at a time: the Inspector closes when this one opens, and the other way round.
    doc.dispatchEvent(new CustomEvent(INSPECTOR_CLOSE));
    layout.setBottomPanel(wrap);
    layout.setDeveloperOpen(true);
    apply();
    void panel!.start();
    panel!.focus();
  }
  function hide(): void {
    if (!open) return;
    open = false;
    const inside = wrap?.contains(doc.activeElement) ?? false;
    layout.setBottomPanel(null);
    layout.setDeveloperOpen(false);
    layout.el.style.removeProperty("--zen-dev-height");
    if (inside) layout.el.querySelector<HTMLElement>(".zen-developer-button")?.focus();
  }

  const onInspector = (e: Event) => { if ((e as CustomEvent<InspectorState>).detail?.open) hide(); };
  doc.addEventListener(INSPECTOR_STATE, onInspector);

  const sync = () => {
    const on = developerEnabled();
    layout.setDeveloperAvailable(on);
    if (!on) hide();
  };
  const stopFollow = followDeveloper(sync, doc.defaultView ?? window);
  const onKey = (e: KeyboardEvent) => {
    if (!developerKey(e) || !developerEnabled()) return;
    e.preventDefault();
    developer.toggle();
  };
  doc.addEventListener("keydown", onKey);
  sync();
  return developer;
}
