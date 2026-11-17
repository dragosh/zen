// The loader shown while a folder opens (workspace:opening events from Go):
// the folder's name, what is happening (reading the folder, indexing pages),
// a progress bar, and Cancel, which keeps the folder that was open before.
// Fast opens never show it: it appears only after `delay` ms.
import type { OpenProgress } from "../api";
import { button, el } from "./dom";

export type Opening = {
  /** Feed every workspace:opening event here. */
  update(p: OpenProgress): void;
  /** Shows why opening failed, with an OK button. */
  fail(message: string): void;
  hide(): void;
  destroy(): void;
  readonly el: HTMLElement;
};

export type OpeningOptions = {
  onCancel(): void;
  /** ms before the loader appears (default 250). */
  delay?: number;
  /** ms before the "large folders take a while" hint (default 5000). */
  slowAfter?: number;
};

const fmt = (n: number) => n.toLocaleString("en-US");

export function createOpening(doc: Document, opts: OpeningOptions): Opening {
  const delay = opts.delay ?? 250;
  const slowAfter = opts.slowAfter ?? 5000;
  const win = doc.defaultView ?? window;

  const root = el("div", "zen-opening");
  root.hidden = true;
  root.setAttribute("role", "dialog");
  root.setAttribute("aria-modal", "true");
  root.setAttribute("aria-labelledby", "zen-opening-title");
  const box = el("div", "zen-opening-box");
  const title = el("h2", "zen-opening-title");
  title.id = "zen-opening-title";
  const detail = el("p", "zen-opening-detail");
  detail.setAttribute("role", "status");
  detail.setAttribute("aria-live", "polite");
  const bar = el("div", "zen-opening-bar");
  bar.setAttribute("role", "progressbar");
  bar.setAttribute("aria-labelledby", "zen-opening-title");
  const fill = el("div", "zen-opening-fill");
  bar.append(fill);
  const hint = el("p", "zen-opening-hint", "Large folders take a while. Cancel keeps the folder that is open now.");
  hint.hidden = true;
  const cancel = button("Cancel", () => {
    if (state !== "opening") return;
    cancel.disabled = true;
    cancel.textContent = "Cancelling…";
    opts.onCancel();
  });
  const ok = button("OK", () => hide(), { className: "primary" });
  ok.hidden = true;
  const actions = el("div", "zen-opening-actions");
  actions.append(cancel, ok);
  box.append(title, detail, bar, hint, actions);
  root.append(box);
  doc.body.append(root);

  let state: "idle" | "opening" | "failed" = "idle";
  let showTimer: number | undefined;
  let slowTimer: number | undefined;
  let before: Element | null = null;

  function clearTimers(): void {
    if (showTimer !== undefined) win.clearTimeout(showTimer);
    if (slowTimer !== undefined) win.clearTimeout(slowTimer);
    showTimer = slowTimer = undefined;
  }

  function reveal(focus: HTMLElement): void {
    if (root.hidden) {
      before = doc.activeElement;
      root.hidden = false;
    }
    focus.focus();
  }

  function hide(): void {
    clearTimers();
    state = "idle";
    if (!root.hidden) {
      root.hidden = true;
      if (before instanceof HTMLElement && before.isConnected) before.focus();
    }
    before = null;
  }

  function begin(name: string): void {
    state = "opening";
    root.classList.remove("zen-opening-failed");
    title.textContent = name ? `Opening ${name}…` : "Opening the folder…";
    cancel.hidden = false;
    cancel.disabled = false;
    cancel.textContent = "Cancel";
    ok.hidden = true;
    bar.hidden = false;
    hint.hidden = true;
    showTimer = win.setTimeout(() => {
      showTimer = undefined;
      if (state === "opening") reveal(cancel);
    }, delay);
    slowTimer = win.setTimeout(() => {
      slowTimer = undefined;
      if (state === "opening") hint.hidden = false;
    }, slowAfter);
  }

  function update(p: OpenProgress): void {
    if (p.phase === "done" || p.phase === "failed") {
      if (state === "opening") hide();
      return;
    }
    if (state !== "opening") {
      clearTimers();
      begin(p.name);
    }
    if (p.phase === "reading") {
      detail.textContent = p.done > 0 ? `Reading the folder · ${fmt(p.done)} items` : "Reading the folder";
      bar.classList.add("zen-opening-indeterminate");
      bar.removeAttribute("aria-valuenow");
      fill.style.width = "";
    } else {
      const total = Math.max(p.total, p.done, 1);
      detail.textContent = `Indexing pages · ${fmt(p.done)} of ${fmt(total)}`;
      bar.classList.remove("zen-opening-indeterminate");
      bar.setAttribute("aria-valuemin", "0");
      bar.setAttribute("aria-valuemax", String(total));
      bar.setAttribute("aria-valuenow", String(p.done));
      fill.style.width = `${Math.round((p.done / total) * 100)}%`;
    }
  }

  function fail(message: string): void {
    clearTimers();
    state = "failed";
    root.classList.add("zen-opening-failed");
    title.textContent = "Could not open the folder";
    detail.textContent = message;
    bar.hidden = true;
    hint.hidden = true;
    cancel.hidden = true;
    ok.hidden = false;
    reveal(ok);
  }

  const onKey = (e: KeyboardEvent) => {
    if (root.hidden || e.key !== "Escape") return;
    e.preventDefault();
    e.stopPropagation();
    if (state === "opening") cancel.click();
    else hide();
  };
  doc.addEventListener("keydown", onKey, true);

  return {
    el: root,
    update,
    fail,
    hide,
    destroy() {
      clearTimers();
      doc.removeEventListener("keydown", onKey, true);
      root.remove();
    },
  };
}
