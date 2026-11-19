// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { LogEntry } from "../host";
import { installConsoleCapture, type ConsoleCapture } from "./console";
import { createDeveloper, DEV_HEIGHT_KEY, DEV_KEY, developerEnabled, DEV_HEIGHT_DEFAULT, DEV_HEIGHT_MIN, setDeveloperEnabled, type Developer } from "./developer";
import { createLayout, type Layout } from "./layout";
import { INSPECTOR_CLOSE, INSPECTOR_STATE } from "../inspector/events";
import { developerSection } from "./settings";

let cap: ConsoleCapture;
let layout: Layout;
let dev: Developer | null = null;
const host = { logs: async (): Promise<LogEntry[]> => [], onLog: () => () => {} };

beforeEach(() => {
  localStorage.clear();
  document.body.replaceChildren();
  cap = installConsoleCapture(window);
  layout = createLayout({ onDeveloper: () => dev?.toggle() });
  document.body.append(layout.el);
});
afterEach(() => { dev?.destroy(); dev = null; layout.destroy(); cap.uninstall(); });

const button = () => layout.el.querySelector<HTMLButtonElement>(".zen-sidebar-foot .zen-developer-button")!;
const panel = () => layout.el.querySelector<HTMLElement>(".zen-dev");
const key = (init: KeyboardEventInit) => document.dispatchEvent(new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init }));

test("setting: off by default, remembered, and followed live", () => {
  expect(developerEnabled()).toBe(false);
  const seen: boolean[] = [];
  const off = (window as Window).addEventListener("zen:developer", () => seen.push(developerEnabled()));
  void off;
  setDeveloperEnabled(true);
  expect(localStorage.getItem(DEV_KEY)).toBe("1");
  expect(developerEnabled()).toBe(true);
  setDeveloperEnabled(false);
  expect(localStorage.getItem(DEV_KEY)).toBeNull();
  expect(seen).toEqual([true, false]);
});

test("the Settings section has the checkbox, reflects and changes the setting", () => {
  const s = developerSection();
  expect(s.id).toBe("developer");
  const box = s.el.querySelector<HTMLInputElement>("input[type=checkbox]")!;
  expect(s.el.textContent).toContain("Show developer tab");
  s.show!();
  expect(box.checked).toBe(false);
  box.checked = true;
  box.dispatchEvent(new Event("change"));
  expect(developerEnabled()).toBe(true);
  setDeveloperEnabled(false);
  s.show!();
  expect(box.checked).toBe(false);
});

test("the button is in the footer next to the cog, only when the setting is on", () => {
  dev = createDeveloper(layout, host, document);
  expect(button().hidden).toBe(true);
  setDeveloperEnabled(true);
  expect(button().hidden).toBe(false);
  expect(button().title).toBe("Developer ⌥⌘J");
  expect(button().getAttribute("aria-label")).toBe("Developer");
  expect(button().previousElementSibling?.classList.contains("zen-settings-button")).toBe(true);
  setDeveloperEnabled(false);
  expect(button().hidden).toBe(true);
});

test("the button toggles the panel below the page", async () => {
  setDeveloperEnabled(true);
  dev = createDeveloper(layout, host, document);
  expect(panel()).toBeNull();
  button().click();
  expect(panel()).not.toBeNull();
  expect(dev.isOpen()).toBe(true);
  expect(button().getAttribute("aria-expanded")).toBe("true");
  const content = layout.slots.main.parentElement!;
  expect(panel()!.parentElement).toBe(content);
  expect(content.lastElementChild).toBe(panel());
  button().click();
  expect(panel()).toBeNull();
  expect(button().getAttribute("aria-expanded")).toBe("false");
});

test("⌥⌘J toggles only while the setting is on", () => {
  dev = createDeveloper(layout, host, document);
  key({ code: "KeyJ", key: "∆", altKey: true, metaKey: true });
  expect(dev.isOpen()).toBe(false);
  setDeveloperEnabled(true);
  key({ code: "KeyJ", key: "∆", altKey: true, metaKey: true });
  expect(dev.isOpen()).toBe(true);
  key({ code: "KeyJ", key: "j", metaKey: true }); // ⌘J alone does nothing
  expect(dev.isOpen()).toBe(true);
  key({ code: "KeyJ", key: "∆", altKey: true, metaKey: true });
  expect(dev.isOpen()).toBe(false);
});

test("one bottom panel at a time: opening asks the Inspector to close, an open Inspector closes this one", () => {
  setDeveloperEnabled(true);
  dev = createDeveloper(layout, host, document);
  const asked = vi.fn();
  document.addEventListener(INSPECTOR_CLOSE, asked);
  dev.toggle();
  expect(dev.isOpen()).toBe(true);
  expect(asked).toHaveBeenCalledOnce();
  document.dispatchEvent(new CustomEvent(INSPECTOR_STATE, { detail: { available: true, open: false, count: 0 } }));
  expect(dev.isOpen()).toBe(true); // a closed Inspector changes nothing
  document.dispatchEvent(new CustomEvent(INSPECTOR_STATE, { detail: { available: true, open: true, count: 0 } }));
  expect(dev.isOpen()).toBe(false);
  expect(panel()).toBeNull();
  document.removeEventListener(INSPECTOR_CLOSE, asked);
});

test("turning the setting off closes the panel", () => {
  setDeveloperEnabled(true);
  dev = createDeveloper(layout, host, document);
  dev.toggle();
  expect(dev.isOpen()).toBe(true);
  setDeveloperEnabled(false);
  expect(dev.isOpen()).toBe(false);
  expect(panel()).toBeNull();
});

test("height: default, clamped, remembered; keyboard on the handle; ✕ and Esc close", () => {
  setDeveloperEnabled(true);
  dev = createDeveloper(layout, host, document);
  dev.toggle();
  expect(dev.height()).toBe(DEV_HEIGHT_DEFAULT);
  dev.setHeight(10);
  expect(dev.height()).toBe(DEV_HEIGHT_MIN);
  dev.setHeight(300);
  expect(localStorage.getItem(DEV_HEIGHT_KEY)).toBe("300");
  expect(panel()!.style.height).toBe("300px");
  const handle = panel()!.querySelector<HTMLElement>(".zen-dev-resizer")!;
  expect(handle.getAttribute("role")).toBe("separator");
  handle.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp", bubbles: true }));
  expect(dev.height()).toBe(316);
  handle.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
  handle.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
  expect(dev.height()).toBe(284);
  dev.destroy();
  dev = createDeveloper(layout, host, document); // a new window remembers it
  dev.toggle();
  expect(dev.height()).toBe(284);
  panel()!.querySelector<HTMLButtonElement>(".zen-dev-close")!.click();
  expect(panel()).toBeNull();
  dev.toggle();
  panel()!.querySelector<HTMLElement>(".zen-dev-list")!.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  expect(dev.isOpen()).toBe(false);
});

test("a host without logs still works: the Console tab", () => {
  setDeveloperEnabled(true);
  dev = createDeveloper(layout, {}, document);
  dev.toggle();
  expect(Array.from(panel()!.querySelectorAll("[role=tab]")).map((t) => t.textContent)).toEqual(["Console"]);
});

test("destroy removes the button handler's effects and the panel", () => {
  setDeveloperEnabled(true);
  dev = createDeveloper(layout, host, document);
  dev.toggle();
  dev.destroy();
  expect(panel()).toBeNull();
  key({ code: "KeyJ", key: "∆", altKey: true, metaKey: true });
  expect(panel()).toBeNull();
  vi.restoreAllMocks();
});

test("the open panel's height is a CSS variable, so side panels (the Inspector) end above it", () => {
  setDeveloperEnabled(true);
  dev = createDeveloper(layout, host, document);
  dev.toggle();
  dev.setHeight(300);
  expect(layout.el.style.getPropertyValue("--zen-dev-height")).toBe("300px");
  dev.toggle();
  expect(layout.el.style.getPropertyValue("--zen-dev-height")).toBe("");
});
