// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { confirmDialog, openMenu, type MenuEntry } from "./menu";
import { renderTree } from "./tree";
import { deepTree, item } from "./fixtures";

const run = vi.fn();
const entries = (): MenuEntry[] => [
  { label: "Open", run: () => run("open") },
  "separator",
  { label: "Move Up", run: () => run("up"), disabled: true },
  { label: "Copy Path", run: () => run("copy") },
];
let opener: HTMLButtonElement;
beforeEach(() => {
  document.body.replaceChildren();
  run.mockClear();
  opener = document.createElement("button");
  document.body.append(opener);
  opener.focus();
  Object.defineProperty(window, "innerWidth", { value: 800, configurable: true });
  Object.defineProperty(window, "innerHeight", { value: 600, configurable: true });
});
afterEach(() => { vi.restoreAllMocks(); });

const menu = () => document.querySelector<HTMLElement>("[role=menu]");
const labels = () => Array.from(document.querySelectorAll("[role=menuitem]")).map((i) => i.textContent);
const press = (key: string, init: KeyboardEventInit = {}) =>
  (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init }));
const open = (o: Partial<Parameters<typeof openMenu>[0]> = {}) => openMenu({ doc: document, entries: entries(), x: 10, y: 20, returnFocus: opener, ...o });

test("role=menu with role=menuitem buttons, separators, disabled items and a label", () => {
  open({ label: "Install" });
  expect(menu()!.getAttribute("aria-label")).toBe("Install");
  expect(labels()).toEqual(["Open", "Move Up", "Copy Path"]);
  expect(menu()!.querySelectorAll("[role=separator]").length).toBe(1);
  expect(document.querySelectorAll("[role=menuitem][aria-disabled=true]").length).toBe(1);
});

test("the first enabled item has the focus; arrows wrap and skip disabled items; Home and End jump", () => {
  open();
  expect(document.activeElement!.textContent).toBe("Open");
  press("ArrowDown"); expect(document.activeElement!.textContent).toBe("Copy Path");
  press("ArrowDown"); expect(document.activeElement!.textContent).toBe("Open");
  press("ArrowUp"); expect(document.activeElement!.textContent).toBe("Copy Path");
  press("Home"); expect(document.activeElement!.textContent).toBe("Open");
  press("End"); expect(document.activeElement!.textContent).toBe("Copy Path");
});

test("Enter and Space run the item and close the menu; a disabled item does nothing", () => {
  open();
  press("Enter");
  expect(run).toHaveBeenCalledWith("open");
  expect(menu()).toBeNull();
  open();
  press("ArrowDown"); press(" ");
  expect(run).toHaveBeenLastCalledWith("copy");
  open();
  (document.querySelectorAll<HTMLElement>("[role=menuitem]")[1]).click(); // Move Up, disabled
  expect(run).toHaveBeenCalledTimes(2);
  expect(menu()).not.toBeNull();
});

test("a click on an item runs it", () => {
  open();
  document.querySelectorAll<HTMLElement>("[role=menuitem]")[2].click();
  expect(run).toHaveBeenCalledWith("copy");
  expect(menu()).toBeNull();
});

test("Esc closes and gives the focus back; Tab closes", () => {
  open();
  press("Escape");
  expect(menu()).toBeNull();
  expect(document.activeElement).toBe(opener);
  open();
  press("Tab");
  expect(menu()).toBeNull();
});

test("closes on an outside pointer, blur, scroll and resize; not on a click inside", () => {
  const closed = vi.fn();
  open({ onClose: closed });
  menu()!.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true }));
  expect(menu()).not.toBeNull();
  opener.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true }));
  expect(menu()).toBeNull();
  expect(closed).toHaveBeenCalledTimes(1);
  for (const trigger of [() => window.dispatchEvent(new Event("blur")), () => document.dispatchEvent(new Event("scroll")), () => window.dispatchEvent(new Event("resize"))]) {
    open();
    trigger();
    expect(menu()).toBeNull();
  }
  // A scroll inside the menu is not "the page scrolled".
  open();
  menu()!.dispatchEvent(new Event("scroll"));
  expect(menu()).not.toBeNull();
});

test("only one menu at a time", () => {
  open();
  open({ entries: [{ label: "Other", run: () => {} }] });
  expect(document.querySelectorAll("[role=menu]").length).toBe(1);
  expect(labels()).toEqual(["Other"]);
});

test("stays inside the window: flips left and up near the edges", () => {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    return this.getAttribute("role") === "menu" ? ({ width: 200, height: 150, top: 0, left: 0, right: 200, bottom: 150, x: 0, y: 0 } as DOMRect) : ({ width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0, x: 0, y: 0 } as DOMRect);
  });
  open({ x: 100, y: 100 });
  expect([menu()!.style.left, menu()!.style.top]).toEqual(["100px", "100px"]);
  open({ x: 700, y: 550 });
  expect([menu()!.style.left, menu()!.style.top]).toEqual(["500px", "400px"]);
  open({ x: 790, y: 5 });
  expect(menu()!.style.left).toBe("590px");
  // Larger than the window: pinned to the margin, never negative.
  Object.defineProperty(window, "innerWidth", { value: 150, configurable: true });
  open({ x: 100, y: 100 });
  expect(parseInt(menu()!.style.left)).toBeGreaterThanOrEqual(0);
});

test("item text goes in as text", () => {
  open({ entries: [{ label: "<img src=x onerror=alert(1)>", run: () => {} }] });
  expect(document.querySelector("img")).toBeNull();
  expect(labels()).toEqual(["<img src=x onerror=alert(1)>"]);
});

// The tree raises the menu: pointer, ⌃-click and keyboard.
const tree = () => {
  const host = document.createElement("div");
  document.body.append(host);
  const onContextMenu = vi.fn();
  renderTree(host, deepTree(), { current: "README.md", onOpen: vi.fn(), onToggle: vi.fn(), expanded: new Set(["guides"]), onContextMenu });
  return { host, onContextMenu };
};
const rowOf = (host: HTMLElement, p: string) => item(host, p).querySelector<HTMLElement>(".zen-nav-row")!;

test("tree: contextmenu on a row asks for the menu at the pointer, selects the row, opens nothing", () => {
  const { host, onContextMenu } = tree();
  const onOpen = vi.fn();
  const ev = new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 40, clientY: 60 });
  rowOf(host, "guides/install.md").dispatchEvent(ev);
  expect(ev.defaultPrevented).toBe(true);
  expect(onContextMenu).toHaveBeenCalledWith("guides/install.md", { x: 40, y: 60 }, rowOf(host, "guides/install.md"));
  expect(rowOf(host, "guides/install.md").tabIndex).toBe(0);
  expect(onOpen).not.toHaveBeenCalled();
});

test("tree: a ⌃-click is the context menu, not an open", () => {
  const { host, onContextMenu } = tree();
  const onOpen = vi.fn();
  renderTree(host, deepTree(), { current: "README.md", onOpen, onToggle: vi.fn(), expanded: new Set(["guides"]), onContextMenu });
  rowOf(host, "api.md").dispatchEvent(new MouseEvent("click", { bubbles: true, ctrlKey: true }));
  expect(onOpen).not.toHaveBeenCalled();
});

test("tree: the context-menu key and ⇧F10 ask for the menu under the focused row", () => {
  const { host, onContextMenu } = tree();
  const r = rowOf(host, "guides/install.md");
  vi.spyOn(r, "getBoundingClientRect").mockReturnValue({ left: 10, bottom: 80, top: 54, right: 200, width: 190, height: 26, x: 10, y: 54 } as DOMRect);
  r.focus();
  const k = new KeyboardEvent("keydown", { key: "ContextMenu", bubbles: true, cancelable: true });
  r.dispatchEvent(k);
  expect(k.defaultPrevented).toBe(true);
  expect(onContextMenu).toHaveBeenCalledTimes(1);
  expect(onContextMenu.mock.calls[0][0]).toBe("guides/install.md");
  expect(onContextMenu.mock.calls[0][1].y).toBe(80);
  r.dispatchEvent(new KeyboardEvent("keydown", { key: "F10", shiftKey: true, bubbles: true, cancelable: true }));
  expect(onContextMenu).toHaveBeenCalledTimes(2);
  r.dispatchEvent(new KeyboardEvent("keydown", { key: "F10", bubbles: true, cancelable: true }));
  expect(onContextMenu).toHaveBeenCalledTimes(2);
});

test("tree: no context menu handler, no default prevention", () => {
  const host = document.createElement("div");
  document.body.append(host);
  renderTree(host, deepTree(), { current: "", onOpen: vi.fn(), onToggle: vi.fn(), expanded: new Set() });
  const ev = new MouseEvent("contextmenu", { bubbles: true, cancelable: true });
  rowOf(host, "api.md").dispatchEvent(ev);
  expect(ev.defaultPrevented).toBe(false);
});

// confirmDialog
test("confirmDialog: alertdialog, Cancel has the focus, Esc and Cancel say no, the button says yes", () => {
  const yes = vi.fn(); const closed = vi.fn();
  let d = confirmDialog({ doc: document, title: "Hide Install?", message: "It leaves the sidebar.", confirm: "Hide", onConfirm: yes, onClose: closed });
  const box = document.querySelector("[role=alertdialog]")!;
  expect(box.getAttribute("aria-modal")).toBe("true");
  expect(document.getElementById(box.getAttribute("aria-labelledby")!)!.textContent).toBe("Hide Install?");
  expect(document.getElementById(box.getAttribute("aria-describedby")!)!.textContent).toBe("It leaves the sidebar.");
  expect(document.activeElement!.textContent).toBe("Cancel");
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  expect(document.querySelector("[role=alertdialog]")).toBeNull();
  expect(yes).not.toHaveBeenCalled();
  expect(closed).toHaveBeenCalledTimes(1);
  expect(document.activeElement).toBe(opener);

  d = confirmDialog({ doc: document, title: "t", message: "m", confirm: "Hide", onConfirm: yes });
  Array.from(document.querySelectorAll("button")).find((b) => b.textContent === "Hide")!.click();
  expect(yes).toHaveBeenCalledTimes(1);
  expect(document.querySelector("[role=alertdialog]")).toBeNull();
  d.close(); // closing twice is fine
});

test("confirmDialog: Tab stays inside; a click on the backdrop cancels", () => {
  const yes = vi.fn();
  confirmDialog({ doc: document, title: "t", message: "m", confirm: "Hide", onConfirm: yes });
  const buttons = Array.from(document.querySelectorAll<HTMLButtonElement>("[role=alertdialog] button"));
  buttons[buttons.length - 1].focus();
  const tab = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
  document.dispatchEvent(tab);
  expect(tab.defaultPrevented).toBe(true);
  expect(document.activeElement).toBe(buttons[0]);
  document.querySelector(".zen-confirm-backdrop")!.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  expect(document.querySelector("[role=alertdialog]")).toBeNull();
  expect(yes).not.toHaveBeenCalled();
});
