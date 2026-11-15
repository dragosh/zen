// @vitest-environment happy-dom
import { beforeEach, expect, test } from "vitest";
import { createFakeApi } from "../api/fake";
import { createSettings } from "./settings";
import { followTheme, storedTheme, THEME_KEY } from "./theme";

beforeEach(() => {
  localStorage.clear();
  document.body.replaceChildren();
  delete document.documentElement.dataset.theme;
});

function setup(hasWorkspace = true) {
  const api = createFakeApi();
  const cog = document.createElement("button");
  document.body.append(cog);
  const s = createSettings({ api, anchor: cog, hasWorkspace: () => hasWorkspace });
  document.body.append(s.el);
  return { api, cog, s };
}
const radio = (s: { el: HTMLElement }, id: string) => s.el.querySelector<HTMLButtonElement>(`[role=radio][data-theme="${id}"]`)!;

test("opens and closes, reflecting state on the cog; Esc and outside clicks close", () => {
  const { cog, s } = setup();
  expect(s.el.hidden).toBe(true);
  s.toggle();
  expect(s.el.hidden).toBe(false);
  expect(cog.getAttribute("aria-expanded")).toBe("true");
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(s.isOpen()).toBe(false);
  expect(document.activeElement).toBe(cog);
  s.open();
  document.body.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  expect(s.isOpen()).toBe(false);
});

test("theme: system by default, choosing applies and remembers it", () => {
  const { s } = setup();
  s.open();
  expect(radio(s, "system").getAttribute("aria-checked")).toBe("true");
  radio(s, "dark").click();
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(localStorage.getItem(THEME_KEY)).toBe("dark");
  expect(radio(s, "dark").getAttribute("aria-checked")).toBe("true");
  radio(s, "system").click();
  expect(document.documentElement.dataset.theme).toBeUndefined();
  expect(storedTheme()).toBe("system");
});

test("arrow keys move through the theme options", () => {
  const { s } = setup();
  s.open();
  const seg = s.el.querySelector<HTMLElement>("[role=radiogroup]")!;
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(storedTheme()).toBe("light");
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
  expect(storedTheme()).toBe("dark");
});

test("config files open through the API; folder file disabled without a folder; errors shown as text", async () => {
  const { api, s } = setup();
  s.open();
  s.el.querySelector<HTMLButtonElement>('[aria-label="Open config.yml"]')!.click();
  await Promise.resolve(); await Promise.resolve();
  expect(api.callsTo("openConfig")).toEqual([["global"]]);
  expect(s.el.querySelector("[role=status]")!.textContent).toContain("config.yml opened");
  api.failNext("openConfig", new Error("<b>nope</b>"));
  s.el.querySelector<HTMLButtonElement>('[aria-label="Open .zen.yml"]')!.click();
  await Promise.resolve(); await Promise.resolve(); await Promise.resolve();
  expect(s.el.querySelector("[role=status]")!.textContent).toContain("<b>nope</b>");
  expect(s.el.querySelector("b")).toBeNull();

  const none = setup(false);
  none.s.open();
  expect(none.s.el.querySelector<HTMLButtonElement>('[aria-label="Open .zen.yml"]')!.disabled).toBe(true);
});

test("followTheme applies the stored theme and follows other windows", () => {
  localStorage.setItem(THEME_KEY, "light");
  const off = followTheme();
  expect(document.documentElement.dataset.theme).toBe("light");
  localStorage.setItem(THEME_KEY, "dark");
  window.dispatchEvent(new StorageEvent("storage", { key: THEME_KEY }));
  expect(document.documentElement.dataset.theme).toBe("dark");
  off();
});
