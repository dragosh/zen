// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi, type Mock } from "vitest";
import type { NavNode } from "../host";
import { renderTree } from "./tree";
import { item } from "./fixtures";

const p = (title: string, path: string, children: NavNode[] = []): NavNode => ({ title, path, kind: "page", children });
const g = (title: string, path: string, children: NavNode[]): NavNode => ({ title, path, kind: "group", children });
const tree = (): NavNode[] => [
  p("Home", "README.md", [p("A", "a.md"), p("B", "b.md", [p("B1", "b/1.md")]), g("Folder", "f", [p("F1", "f/1.md")]), p("C", "c.md")]),
];

const ROW = 26;
let host: HTMLElement;
let onReorder: Mock<(parent: string, order: string[], moved: string) => void>;
let onMove: Mock<(path: string, delta: -1 | 1) => void>;

/** Rows are 26px tall, one under the other in document order (hidden ones skipped); a list item spans its row and its children. */
function layout() {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    const rows = Array.from(host.querySelectorAll<HTMLElement>(".zen-nav-row")).filter((r) => !r.closest("[aria-expanded=false] > ul"));
    const box = (top: number, bottom: number) => ({ top, bottom, left: 0, right: 200, width: 200, height: bottom - top, x: 0, y: top }) as DOMRect;
    if (this === host) return box(0, 400);
    if (this.classList.contains("zen-nav-row")) { const i = rows.indexOf(this); return box(i * ROW, (i + 1) * ROW); }
    if (this.getAttribute("role") === "treeitem") {
      const mine = rows.filter((r) => this.contains(r));
      return box(rows.indexOf(mine[0]) * ROW, (rows.indexOf(mine[mine.length - 1]) + 1) * ROW);
    }
    return box(0, 0);
  });
}
const expanded = () => new Set(["README.md", "b.md", "f"]);
const draw = (extra: Record<string, unknown> = {}) =>
  renderTree(host, tree(), { current: "", onOpen: vi.fn(), onToggle: vi.fn(), expanded: expanded(), onReorder, onMove, ...extra });
const row = (path: string) => item(host, path).querySelector<HTMLElement>(".zen-nav-row")!;
const ptr = (type: string, y: number, target: Element = document.body, init: MouseEventInit = {}) =>
  target.dispatchEvent(new MouseEvent(type, { bubbles: true, cancelable: true, clientX: 50, clientY: y, button: 0, ...init }));
// rows: 0 Home, 1 A, 2 B, 3 B1, 4 Folder, 5 F1, 6 C
const y = (i: number, frac = 0.5) => (i + frac) * ROW;
const line = () => host.querySelector<HTMLElement>(".zen-nav-drop");

beforeEach(() => {
  document.body.replaceChildren();
  host = document.createElement("div");
  document.body.append(host);
  onReorder = vi.fn();
  onMove = vi.fn();
  draw();
  layout();
});
afterEach(() => vi.restoreAllMocks());

test("a drag below 4px is a click, not a drag", () => {
  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(1) + 3);
  expect(line()).toBeNull();
  ptr("pointerup", y(1) + 3);
  expect(onReorder).not.toHaveBeenCalled();
});

test("dragging a page below a sibling shows a line and drops with the whole new order", () => {
  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(2, 0.2)); // above the middle of B: still before B: no change
  ptr("pointermove", y(6, 0.2)); // above the middle of C: before C
  expect(line()).not.toBeNull();
  expect(host.classList.contains("zen-nav-dragging")).toBe(true);
  expect(item(host, "a.md").classList.contains("zen-nav-drag-source")).toBe(true);
  ptr("pointerup", y(6, 0.2));
  expect(onReorder).toHaveBeenCalledWith("README.md", ["b.md", "a.md", "c.md"], "a.md");
  expect(line()).toBeNull();
  expect(host.classList.contains("zen-nav-dragging")).toBe(false);
});

test("the line sits between rows; at the end it goes below the last sibling", () => {
  ptr("pointerdown", y(6), row("c.md"));
  ptr("pointermove", y(1, 0.1));
  expect(line()!.style.top).toBe(`${ROW}px`); // above A
  ptr("pointermove", y(2, 0.4)); // before B, between A and B
  expect(line()!.style.top).toBe(`${ROW * 2}px`);
  ptr("pointermove", y(6, 0.9) - 1); // below C's middle: the end
  expect(line()!.style.top).toBe(`${ROW * 7}px`);
  ptr("pointerup", y(6, 0.9) - 1);
  expect(onReorder).not.toHaveBeenCalled(); // C stays last
});

test("a drag that ends with Esc still swallows the click that follows", () => {
  const onOpen = vi.fn();
  draw({ onOpen });
  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(6, 0.2));
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  ptr("pointerup", y(6, 0.2));
  row("a.md").dispatchEvent(new MouseEvent("click", { bubbles: true }));
  expect(onOpen).not.toHaveBeenCalled();
});

test("a click that ends a drag does not open the page", () => {
  const onOpen = vi.fn();
  draw({ onOpen });
  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(6, 0.2));
  ptr("pointerup", y(6, 0.2));
  row("a.md").dispatchEvent(new MouseEvent("click", { bubbles: true }));
  expect(onOpen).not.toHaveBeenCalled();
  row("a.md").dispatchEvent(new MouseEvent("click", { bubbles: true })); // the next click is a real one
  expect(onOpen).toHaveBeenCalledTimes(1);
});

test("dropping where it was, or Esc, or releasing outside the group cancels", () => {
  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(1, 0.2));
  ptr("pointermove", y(2, 0.2));
  ptr("pointerup", y(2, 0.2));
  expect(onReorder).not.toHaveBeenCalled(); // A before B is where A is

  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(6, 0.2));
  expect(line()).not.toBeNull();
  const esc = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
  document.dispatchEvent(esc);
  expect(esc.defaultPrevented).toBe(true);
  expect(line()).toBeNull();
  ptr("pointerup", y(6, 0.2));
  expect(onReorder).not.toHaveBeenCalled();

  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(6, 0.2));
  ptr("pointermove", 380); // far below the group
  expect(line()).toBeNull();
  ptr("pointerup", 380);
  expect(onReorder).not.toHaveBeenCalled();
});

test("only the page siblings of the group are targets: a pointer over a folder row maps to the nearest boundary", () => {
  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(5)); // over F1, inside the Folder group: before C
  ptr("pointerup", y(5));
  expect(onReorder).toHaveBeenCalledWith("README.md", ["b.md", "a.md", "c.md"], "a.md");
  // A page with no siblings cannot be dragged anywhere.
  onReorder.mockClear();
  ptr("pointerdown", y(0), row("README.md"));
  ptr("pointermove", y(6));
  expect(line()).toBeNull();
  ptr("pointerup", y(6));
  expect(onReorder).not.toHaveBeenCalled();
});

test("only a primary press on a page row starts it; groups, the twisty, ⌃ and other buttons do not; no handler, no drag", () => {
  for (const [target, init] of [[row("f"), {}], [row("a.md"), { button: 2 }], [row("a.md"), { ctrlKey: true }], [row("b.md").querySelector<HTMLElement>(".zen-nav-twisty")!, {}]] as const) {
    ptr("pointerdown", y(1), target, init);
    ptr("pointermove", y(6, 0.2));
    expect(line()).toBeNull();
    ptr("pointerup", y(6, 0.2));
  }
  expect(onReorder).not.toHaveBeenCalled();
  host.replaceChildren();
  renderTree(host, tree(), { current: "", onOpen: vi.fn(), onToggle: vi.fn(), expanded: expanded() }); // no onReorder
  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(6, 0.2));
  expect(line()).toBeNull();
});

test("a re-render in the middle of a drag (the tree changed) cancels it", () => {
  ptr("pointerdown", y(1), row("a.md"));
  ptr("pointermove", y(6, 0.2));
  expect(line()).not.toBeNull();
  draw();
  ptr("pointermove", y(6, 0.3));
  expect(line()).toBeNull();
  ptr("pointerup", y(6, 0.3));
  expect(onReorder).not.toHaveBeenCalled();
});

test("Alt+Up and Alt+Down ask to move the focused page", () => {
  row("b.md").focus();
  row("b.md").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp", altKey: true, bubbles: true, cancelable: true }));
  expect(onMove).toHaveBeenCalledWith("b.md", -1);
  row("b.md").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", altKey: true, bubbles: true, cancelable: true }));
  expect(onMove).toHaveBeenLastCalledWith("b.md", 1);
  // Plain arrows still move the focus.
  row("b.md").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, cancelable: true }));
  expect(onMove).toHaveBeenCalledTimes(2);
  // A folder is not moved.
  row("f").focus();
  row("f").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", altKey: true, bubbles: true, cancelable: true }));
  expect(onMove).toHaveBeenCalledTimes(2);
});
