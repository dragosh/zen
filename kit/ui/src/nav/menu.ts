// A small in-page menu (not native) and a confirm dialog for the sidebar. Text goes in through
// textContent only. Colours and sizes are tokens (nav.css).
import { el } from "../shell/dom";

export type MenuEntry = { label: string; run(): void; disabled?: boolean } | "separator";
export type MenuHandle = { el: HTMLElement; close(): void };
export type MenuOptions = {
  doc: Document;
  entries: MenuEntry[];
  /** Where the pointer is (or where the focused row ends); the menu flips to stay inside the window. */
  x: number;
  y: number;
  /** Takes the focus back when the menu closes by Esc, Tab or a choice. */
  returnFocus?: HTMLElement | null;
  /** The menu's accessible name (the page it is for). */
  label?: string;
  onClose?(): void;
};

const MARGIN = 4;
let current: MenuHandle | null = null;

export function openMenu(o: MenuOptions): MenuHandle {
  current?.close();
  const { doc } = o;
  const view = doc.defaultView ?? window;
  const menu = el("div", "zen-menu");
  menu.setAttribute("role", "menu");
  if (o.label) menu.setAttribute("aria-label", o.label);
  const items: HTMLElement[] = [];
  for (const e of o.entries) {
    if (e === "separator") {
      const sep = el("div", "zen-menu-sep");
      sep.setAttribute("role", "separator");
      menu.append(sep);
      continue;
    }
    const b = el("button", "zen-menu-item", e.label);
    b.type = "button";
    b.tabIndex = -1;
    b.setAttribute("role", "menuitem");
    if (e.disabled) b.setAttribute("aria-disabled", "true");
    b.addEventListener("click", () => {
      if (e.disabled) return;
      close(true);
      e.run();
    });
    b.addEventListener("mouseenter", () => { if (!e.disabled) b.focus(); });
    items.push(b);
    menu.append(b);
  }
  const enabled = () => items.filter((i) => i.getAttribute("aria-disabled") !== "true");

  menu.addEventListener("keydown", (e) => {
    const list = enabled();
    const i = list.indexOf(doc.activeElement as HTMLElement);
    const go = (to: HTMLElement | undefined) => { e.preventDefault(); to?.focus(); };
    switch (e.key) {
      case "ArrowDown": go(list[(i + 1) % list.length]); break;
      case "ArrowUp": go(list[(i - 1 + list.length) % list.length]); break;
      case "Home": go(list[0]); break;
      case "End": go(list[list.length - 1]); break;
      case "Enter":
      case " ":
        e.preventDefault();
        (doc.activeElement as HTMLElement | null)?.click();
        break;
      case "Escape":
        e.preventDefault();
        e.stopPropagation();
        close(true);
        break;
      case "Tab":
        e.preventDefault();
        close(true);
        break;
    }
  });

  const outside = (e: Event) => { if (!menu.contains(e.target as Node)) close(false); };
  const scrolled = (e: Event) => { if (!menu.contains(e.target as Node)) close(false); };
  const away = () => close(false);
  doc.addEventListener("pointerdown", outside, true);
  doc.addEventListener("scroll", scrolled, true);
  view.addEventListener("blur", away);
  view.addEventListener("resize", away);

  menu.style.left = `${o.x}px`;
  menu.style.top = `${o.y}px`;
  doc.body.append(menu);
  // Inside the window: flip to the left of / above the point when it does not fit, and never past the margin.
  const r = menu.getBoundingClientRect();
  const place = (at: number, size: number, room: number) => Math.max(MARGIN, Math.min(at + size + MARGIN > room ? at - size : at, room - size - MARGIN));
  menu.style.left = `${place(o.x, r.width, view.innerWidth)}px`;
  menu.style.top = `${place(o.y, r.height, view.innerHeight)}px`;
  enabled()[0]?.focus();

  let closed = false;
  function close(restore: boolean): void {
    if (closed) return;
    closed = true;
    doc.removeEventListener("pointerdown", outside, true);
    doc.removeEventListener("scroll", scrolled, true);
    view.removeEventListener("blur", away);
    view.removeEventListener("resize", away);
    menu.remove();
    if (current === handle) current = null;
    if (restore && o.returnFocus?.isConnected) o.returnFocus.focus();
    o.onClose?.();
  }
  const handle: MenuHandle = { el: menu, close: () => close(false) };
  current = handle;
  return handle;
}

export type ConfirmOptions = {
  doc: Document;
  title: string;
  message: string;
  /** The label of the button that goes ahead; Cancel is the other and has the focus. */
  confirm: string;
  onConfirm(): void;
  onClose?(): void;
};

let seq = 0;

/** A modal alert dialog: Cancel (focused), Esc and a click outside say no. */
export function confirmDialog(o: ConfirmOptions): { el: HTMLElement; close(): void } {
  const { doc } = o;
  const id = ++seq;
  const backdrop = el("div", "zen-confirm-backdrop");
  const box = el("div", "zen-confirm");
  box.setAttribute("role", "alertdialog");
  box.setAttribute("aria-modal", "true");
  const title = el("h2", "zen-confirm-title", o.title);
  title.id = `zen-confirm-title-${id}`;
  const message = el("p", "zen-confirm-message", o.message);
  message.id = `zen-confirm-message-${id}`;
  box.setAttribute("aria-labelledby", title.id);
  box.setAttribute("aria-describedby", message.id);
  const cancel = el("button", "", "Cancel");
  cancel.type = "button";
  const ok = el("button", "primary", o.confirm);
  ok.type = "button";
  const actions = el("div", "zen-confirm-actions");
  actions.append(cancel, ok);
  box.append(title, message, actions);
  backdrop.append(box);
  doc.body.append(backdrop);
  const before = doc.activeElement as HTMLElement | null;
  cancel.focus();

  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      close();
    } else if (e.key === "Tab") {
      const last = e.shiftKey ? cancel : ok;
      const first = e.shiftKey ? ok : cancel;
      if (doc.activeElement === last || !box.contains(doc.activeElement)) {
        e.preventDefault();
        first.focus();
      }
    }
  };
  doc.addEventListener("keydown", onKey, true);
  backdrop.addEventListener("mousedown", (e) => { if (e.target === backdrop) close(); });
  cancel.addEventListener("click", () => close());
  ok.addEventListener("click", () => {
    close();
    o.onConfirm();
  });

  let closed = false;
  function close(): void {
    if (closed) return;
    closed = true;
    doc.removeEventListener("keydown", onKey, true);
    backdrop.remove();
    if (before?.isConnected) before.focus();
    o.onClose?.();
  }
  return { el: backdrop, close };
}
