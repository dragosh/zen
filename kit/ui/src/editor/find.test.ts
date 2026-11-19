// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import { createStore } from "../shell/store";
import type { Ctx, Slots } from "../shell/types";
import { editorFeature } from "./index";

// ui/find in the editor: ⌘F searches the markdown source and selects the match in the textarea.
const tick = () => new Promise((r) => setTimeout(r, 0));
let api: FakeApi;
let unmount: () => void;
let ta: HTMLTextAreaElement;

beforeEach(async () => {
  localStorage.clear();
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  api = createFakeApi({ sources: { "README.md": "# Title\n\nSome text, some Text.\n\n```\na.b(x)\n```\n" } });
  const store = createStore({ path: "README.md", mode: "edit" });
  const slots: Slots = {
    nav: document.createElement("nav"), breadcrumb: document.createElement("div"),
    actions: document.createElement("div"), main: document.createElement("main"),
  };
  document.body.append(slots.actions, slots.main);
  const ctx: Ctx = { api, store, brand: "Zenmark", navigate: () => {} };
  unmount = editorFeature.mount(slots, ctx);
  await tick();
  ta = slots.main.querySelector("textarea")!;
});
afterEach(() => { unmount(); document.body.replaceChildren(); vi.restoreAllMocks(); });

const bar = () => document.querySelector<HTMLElement>("[role=search]");
const field = () => bar()!.querySelector("input")!;
const count = () => bar()!.querySelector("[aria-live]")!.textContent;
const type = (v: string) => { field().value = v; field().dispatchEvent(new Event("input", { bubbles: true })); };
const selected = () => ta.value.slice(ta.selectionStart, ta.selectionEnd);

test("⌘F opens the bar, the match is selected in the source", () => {
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "f", metaKey: true, bubbles: true, cancelable: true }));
  expect(bar()).not.toBeNull();
  type("text");
  expect(count()).toBe("1 of 2");
  expect(selected()).toBe("text");
  expect(ta.selectionStart).toBe(ta.value.indexOf("text"));
  field().dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true }));
  expect(count()).toBe("2 of 2");
  expect(ta.selectionStart).toBe(ta.value.lastIndexOf("Text"));
});

test("the selection of the source is the starting query", () => {
  ta.setSelectionRange(ta.value.indexOf("Title"), ta.value.indexOf("Title") + 5);
  api.emit("menu:find", undefined);
  expect(field().value).toBe("Title");
});

test("regex characters are searched literally in the source", () => {
  api.emit("menu:find", undefined);
  type("a.b(");
  expect(count()).toBe("1 of 1");
  expect(selected()).toBe("a.b(");
});

test("Esc closes the bar and does not leave the editor", () => {
  api.emit("menu:find", undefined);
  field().dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  expect(bar()).toBeNull();
  expect(api.callsTo("save")).toEqual([]);
});

test("Find Next from the menu works with the bar open", () => {
  api.emit("menu:find", undefined);
  type("some");
  api.emit("menu:find-next", undefined);
  expect(count()).toBe("2 of 2");
});
