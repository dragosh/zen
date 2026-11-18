// Pointer drag to reorder the pages of one sibling group in the sidebar. A line shows where the page
// will land; Esc, or letting go outside the group, cancels. What changes is up to the caller (drop).
import type { NavNode } from "../host";
import { el } from "../shell/dom";
import { dropOrder, siblingGroup, type SiblingGroup } from "./siblings";

export type ReorderControl = {
  tree(): NavNode[];
  /** False when the host cannot reorder: a press then starts nothing. */
  enabled(): boolean;
  /** The group's new order: every page sibling, `moved` in its new place. */
  drop(parent: string, order: string[], moved: string): void;
};

const THRESHOLD = 4; // px of movement before a press is a drag
const SLACK = 12; // px above and below the group that still count as inside

type Drag = {
  li: HTMLElement;
  path: string;
  group: SiblingGroup;
  x: number;
  y: number;
  started: boolean;
  cancelled: boolean;
  /** Where the line is (index in the list as it is); -1 outside the group. */
  index: number;
};

/** Listens on `host` (which the tree re-renders into); returns a function that stops listening. */
export function attachReorder(host: HTMLElement, c: ReorderControl): () => void {
  const doc = host.ownerDocument;
  const view = doc.defaultView ?? window;
  let drag: Drag | null = null;
  let line: HTMLElement | null = null;
  let swallowClick = false;

  const rowOf = (li: HTMLElement) => li.querySelector<HTMLElement>(":scope > .zen-nav-row")!;
  const siblingItems = (d: Drag) =>
    Array.from(d.li.parentElement?.children ?? []).filter(
      (n): n is HTMLElement => n instanceof HTMLElement && n.dataset.kind === "page" && d.group.pages.includes(n.dataset.path ?? ""),
    );

  function hideLine(): void {
    line?.remove();
    line = null;
  }

  /** Works out where the pointer is in the group and moves the line there. */
  function track(d: Drag, x: number, y: number): void {
    const items = siblingItems(d);
    const h = host.getBoundingClientRect();
    const first = items[0].getBoundingClientRect();
    const last = items[items.length - 1].getBoundingClientRect();
    if (x < h.left || x > h.right || y < first.top - SLACK || y > last.bottom + SLACK) {
      d.index = -1;
      hideLine();
      return;
    }
    let index = items.length;
    for (let i = 0; i < items.length; i++) {
      const r = rowOf(items[i]).getBoundingClientRect();
      if (y < (r.top + r.bottom) / 2) { index = i; break; }
    }
    d.index = index;
    const edge = index < items.length ? rowOf(items[index]).getBoundingClientRect().top : last.bottom;
    const row = rowOf(items[0]).getBoundingClientRect();
    if (!line) {
      line = el("div", "zen-nav-drop");
      line.setAttribute("aria-hidden", "true");
      host.append(line);
    }
    line.style.top = `${edge - h.top + host.scrollTop}px`;
    line.style.left = `${row.left - h.left + host.scrollLeft}px`;
    line.style.width = `${row.width}px`;
  }

  function finish(): void {
    drag?.li.classList.remove("zen-nav-drag-source");
    host.classList.remove("zen-nav-dragging");
    hideLine();
    doc.removeEventListener("pointermove", onMove);
    doc.removeEventListener("pointerup", onUp);
    doc.removeEventListener("pointercancel", onCancel);
    doc.removeEventListener("keydown", onKey, true);
    view.removeEventListener("blur", onCancel);
  }

  /** Ends the drag without a change; the pointer is still down, and its release must not click. */
  function cancel(): void {
    if (!drag) return;
    drag.cancelled = true;
    drag.li.classList.remove("zen-nav-drag-source");
    host.classList.remove("zen-nav-dragging");
    hideLine();
  }

  function onMove(e: PointerEvent): void {
    const d = drag;
    if (!d || d.cancelled) return;
    if (!d.li.isConnected) return cancel(); // the tree was drawn again under the pointer
    if (!d.started) {
      if (Math.hypot(e.clientX - d.x, e.clientY - d.y) < THRESHOLD) return;
      d.started = true;
      host.classList.add("zen-nav-dragging");
      d.li.classList.add("zen-nav-drag-source");
    }
    track(d, e.clientX, e.clientY);
  }

  function onUp(): void {
    const d = drag;
    drag = null;
    finish();
    if (!d || !d.started) return;
    swallowClick = true; // the click that follows a drag is not an open
    setTimeout(() => { swallowClick = false; }, 0);
    if (d.cancelled || d.index < 0 || !d.li.isConnected) return;
    const order = dropOrder(d.group.pages, d.path, d.index);
    if (order) c.drop(d.group.parent, order, d.path);
  }

  function onCancel(): void {
    cancel();
    finish();
    drag = null;
  }

  function onKey(e: KeyboardEvent): void {
    if (e.key !== "Escape" || !drag?.started || drag.cancelled) return;
    e.preventDefault();
    e.stopPropagation();
    cancel();
  }

  const onDown = (e: PointerEvent) => {
    if (e.button !== 0 || e.ctrlKey || e.metaKey || e.altKey || !c.enabled() || drag) return;
    const target = e.target as Element;
    if (target.closest(".zen-nav-twisty")) return;
    const row = target.closest<HTMLElement>(".zen-nav-row");
    const li = row?.closest<HTMLElement>("[role=treeitem]");
    if (!row || !li || li.dataset.kind !== "page") return;
    const path = li.dataset.path ?? "";
    const group = siblingGroup(c.tree(), path);
    if (!group || group.pages.length < 2) return;
    drag = { li, path, group, x: e.clientX, y: e.clientY, started: false, cancelled: false, index: -1 };
    doc.addEventListener("pointermove", onMove);
    doc.addEventListener("pointerup", onUp);
    doc.addEventListener("pointercancel", onCancel);
    doc.addEventListener("keydown", onKey, true);
    view.addEventListener("blur", onCancel);
  };
  const onClick = (e: Event) => {
    if (!swallowClick) return;
    swallowClick = false;
    e.stopPropagation();
    e.preventDefault();
  };
  host.addEventListener("pointerdown", onDown);
  host.addEventListener("click", onClick, true);
  return () => {
    drag = null;
    finish();
    host.removeEventListener("pointerdown", onDown);
    host.removeEventListener("click", onClick, true);
  };
}
