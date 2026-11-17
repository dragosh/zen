// @vitest-environment happy-dom
import { beforeEach, expect, test } from "vitest";
import { createLayout, SIDEBAR_DEFAULT, SIDEBAR_MAX, SIDEBAR_MIN, WIDTH_KEY } from "./layout";

beforeEach(() => localStorage.clear());

test("has nav, header and main landmarks and the four slots", () => {
  const l = createLayout();
  // The nav slot is the sidebar's body; the sidebar adds Open… and the cog around it.
  expect(l.el.querySelector("nav")!.contains(l.slots.nav)).toBe(true);
  expect(l.el.querySelector("main")).toBe(l.slots.main);
  const header = l.el.querySelector("header")!;
  expect(header.contains(l.slots.breadcrumb)).toBe(true);
  expect(header.contains(l.slots.actions)).toBe(true);
});

test("sidebar starts at 260px and toggles", () => {
  const l = createLayout();
  expect(SIDEBAR_DEFAULT).toBe(260);
  expect(l.sidebarWidth()).toBe(260);
  expect(l.sidebarVisible()).toBe(true);
  l.toggleSidebar();
  expect(l.sidebarVisible()).toBe(false);
  expect(l.el.querySelector("nav")!.hidden).toBe(true);
  l.toggleSidebar();
  expect(l.el.querySelector("nav")!.hidden).toBe(false);
});

test("Open… sits in the sidebar, not the header; the cog at the bottom reports itself", () => {
  let opened = 0;
  let anchor: HTMLButtonElement | null = null;
  const l = createLayout({ onOpenFolder: () => opened++, onSettings: (a) => { anchor = a; } });
  const nav = l.el.querySelector("nav")!;
  const open = nav.querySelector<HTMLButtonElement>(".zen-sidebar-open")!;
  expect(open.getAttribute("aria-label")).toBe("Open a folder");
  expect(l.el.querySelector("header")!.querySelector(".zen-sidebar-open")).toBeNull();
  open.click();
  expect(opened).toBe(1);
  const cog = nav.querySelector<HTMLButtonElement>(".zen-sidebar-foot .zen-settings-button")!;
  expect(cog.getAttribute("aria-label")).toBe("Settings");
  expect(cog.querySelector("svg")).not.toBeNull();
  // Order: body (slot), then the footer last.
  expect(nav.lastElementChild!.classList.contains("zen-sidebar-foot")).toBe(true);
  cog.click();
  expect(anchor).toBe(cog);
});

test("the toggle button toggles the sidebar and reports its state", () => {
  const l = createLayout();
  const btn = l.el.querySelector<HTMLButtonElement>('button[data-action="toggle-sidebar"]')!;
  expect(btn.getAttribute("aria-expanded")).toBe("true");
  btn.click();
  expect(l.sidebarVisible()).toBe(false);
  expect(btn.getAttribute("aria-expanded")).toBe("false");
});

test("width is clamped to 200–400", () => {
  const l = createLayout();
  l.setSidebarWidth(50);
  expect(l.sidebarWidth()).toBe(SIDEBAR_MIN);
  l.setSidebarWidth(1000);
  expect(l.sidebarWidth()).toBe(SIDEBAR_MAX);
  l.setSidebarWidth(321.4);
  expect(l.sidebarWidth()).toBe(321);
  expect(l.el.style.getPropertyValue("--sidebar-width")).toBe("321px");
});

test("width is remembered in localStorage", () => {
  createLayout().setSidebarWidth(300);
  expect(localStorage.getItem(WIDTH_KEY)).toBe("300");
  expect(createLayout().sidebarWidth()).toBe(300);
});

test("a bad stored width falls back to a valid one", () => {
  localStorage.setItem(WIDTH_KEY, "banana");
  expect(createLayout().sidebarWidth()).toBe(260);
  localStorage.setItem(WIDTH_KEY, "9999");
  expect(createLayout().sidebarWidth()).toBe(400);
});

test("dragging the edge resizes within the limits", () => {
  const l = createLayout();
  document.body.append(l.el);
  const edge = l.el.querySelector<HTMLElement>('[role="separator"]')!;
  edge.dispatchEvent(new MouseEvent("mousedown", { clientX: 260, button: 0, bubbles: true }));
  window.dispatchEvent(new MouseEvent("mousemove", { clientX: 310 }));
  expect(l.sidebarWidth()).toBe(310);
  window.dispatchEvent(new MouseEvent("mousemove", { clientX: 900 }));
  expect(l.sidebarWidth()).toBe(400);
  window.dispatchEvent(new MouseEvent("mouseup", { clientX: 900 }));
  expect(localStorage.getItem(WIDTH_KEY)).toBe("400");
  window.dispatchEvent(new MouseEvent("mousemove", { clientX: 220 }));
  expect(l.sidebarWidth()).toBe(400);
  l.destroy();
  l.el.remove();
});

test("the edge is resizable from the keyboard", () => {
  const l = createLayout();
  const edge = l.el.querySelector<HTMLElement>('[role="separator"]')!;
  edge.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight" }));
  expect(l.sidebarWidth()).toBeGreaterThan(260);
  expect(edge.getAttribute("aria-valuenow")).toBe(String(l.sidebarWidth()));
});

test("showView swaps the main pane for a view; the sidebar and the header controls stay", () => {
  const l = createLayout();
  const view = document.createElement("section");
  l.showView(view);
  expect(l.el.querySelector(".zen-content")!.contains(view)).toBe(true);
  expect(l.slots.main.hidden).toBe(true);
  expect(l.slots.actions.hidden).toBe(true);
  expect(l.slots.breadcrumb.hidden).toBe(true);
  expect(l.el.querySelector("nav")!.hidden).toBe(false);
  expect(l.el.querySelector('button[aria-label="Back"]')).not.toBeNull();
  l.showView(null);
  expect(view.isConnected).toBe(false);
  expect(l.slots.main.hidden).toBe(false);
  expect(l.slots.actions.hidden).toBe(false);
  expect(l.slots.breadcrumb.hidden).toBe(false);
});

test("the cog reports whether Settings is open", () => {
  const l = createLayout();
  const cog = l.el.querySelector<HTMLButtonElement>(".zen-settings-button")!;
  expect(cog.hasAttribute("aria-haspopup")).toBe(false); // it opens a page, not a popup
  l.setSettingsOpen(true);
  expect(cog.getAttribute("aria-expanded")).toBe("true");
  l.setSettingsOpen(false);
  expect(cog.getAttribute("aria-expanded")).toBe("false");
});
