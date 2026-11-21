// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, ZENMARK_BUILD } from "../../../../apps/zenmark/frontend/src/api/fake";
import { pageFeature } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

// ui/top-bar-actions: the page's Find and Edit are icon buttons in the top bar's action slot.
let env: Env;
let unmount: (() => void) | undefined;
const SOURCES = { "README.md": "# Sample docs\n\nThe sample is a sample.\n" };

const build = (without: string[] = []) =>
  setup(createFakeApi({ sources: { ...SOURCES }, build: { features: ZENMARK_BUILD.features.filter((f) => !without.includes(f)) } }));
const mount = async () => {
  env.ctx.store.set({ path: "README.md" });
  unmount = pageFeature.mount(env.slots, env.ctx);
  await tick();
};
const buttons = () => Array.from(env.slots.actions.querySelectorAll<HTMLButtonElement>("button"));
const names = () => buttons().map((b) => b.getAttribute("aria-label"));
const key = (init: KeyboardEventInit) => document.dispatchEvent(new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init }));

beforeEach(() => {
  document.head.replaceChildren();
  document.body.replaceChildren();
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
});
afterEach(() => { unmount?.(); unmount = undefined; vi.restoreAllMocks(); });

test("Find then Edit: icons with the name as aria-label, the tooltip as title, no visible text", async () => {
  env = build();
  await mount();
  expect(names()).toEqual(["Find", "Edit"]);
  expect(buttons().map((b) => b.title)).toEqual(["Find (⌘F)", "Edit (⌘E)"]);
  for (const b of buttons()) {
    expect(b.textContent).toBe("");
    expect(b.querySelector("svg")).not.toBeNull();
    expect(b.classList.contains("zen-settings-button")).toBe(true); // the footer's icon button, its size and on style
  }
});

test("each button is missing without its feature, and none is left with neither", async () => {
  env = build(["find"]);
  await mount();
  expect(names()).toEqual(["Edit"]);
  unmount!(); unmount = undefined;
  env = build(["editor"]);
  await mount();
  expect(names()).toEqual(["Find"]);
  unmount!(); unmount = undefined;
  env = build(["editor", "find"]);
  await mount();
  expect(names()).toEqual([]);
});

test("Find toggles the bar, and its aria-pressed follows the bar, however it opens or closes", async () => {
  env = build();
  await mount();
  const [find] = buttons();
  expect(find.getAttribute("aria-pressed")).toBe("false");
  find.click();
  expect(document.querySelector("[role=search]")).not.toBeNull();
  expect(find.getAttribute("aria-pressed")).toBe("true");
  find.click();
  expect(document.querySelector("[role=search]")).toBeNull();
  expect(find.getAttribute("aria-pressed")).toBe("false");
  key({ key: "f", metaKey: true }); // ⌘F
  expect(find.getAttribute("aria-pressed")).toBe("true");
  key({ key: "Escape" });
  expect(find.getAttribute("aria-pressed")).toBe("false");
  vi.spyOn(Date, "now").mockReturnValue(Date.now() + 1000); // the menu event is not the ⌘F press just before
  env.api.emit("menu:find", undefined); // Edit › Find
  expect(find.getAttribute("aria-pressed")).toBe("true");
  (document.querySelector(".zen-find-close") as HTMLButtonElement).click();
  expect(find.getAttribute("aria-pressed")).toBe("false");
});

test("Edit opens the page in the editor, as ⌘E does", async () => {
  env = build();
  await mount();
  buttons()[1].click();
  expect(env.api.callsTo("openEditor")).toEqual([["README.md"]]);
});

test("unmounting takes the buttons away", async () => {
  env = build();
  await mount();
  unmount!(); unmount = undefined;
  expect(env.slots.actions.children).toHaveLength(0);
});
