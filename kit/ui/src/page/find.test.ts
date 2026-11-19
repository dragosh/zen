// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import { pageFeature } from "./index";
import { setup, tick, type Env } from "./helpers.test-util";

// ui/find in the reader: ⌘F and Edit › Find open a find bar over the page.
let env: Env;
let unmount: (() => void) | undefined;

beforeEach(() => {
  document.head.replaceChildren();
  document.body.replaceChildren();
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  env = setup(createFakeApi({ sources: { "README.md": "# Sample docs\n\nThe sample is a sample.\n\n## Usage\n\nBody." } }));
});
afterEach(() => { unmount?.(); unmount = undefined; vi.restoreAllMocks(); });

const mount = async () => {
  env.ctx.store.set({ path: "README.md" });
  unmount = pageFeature.mount(env.slots, env.ctx);
  await tick();
};
const bar = () => document.querySelector<HTMLElement>("[role=search]");
const field = () => bar()!.querySelector("input")!;
const count = () => bar()!.querySelector("[aria-live]")!.textContent;
const type = (v: string) => { field().value = v; field().dispatchEvent(new Event("input", { bubbles: true })); };

test("⌘F opens the find bar, typing counts matches in the page", async () => {
  await mount();
  expect(bar()).toBeNull();
  const e = new KeyboardEvent("keydown", { key: "f", metaKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(true);
  expect(bar()).not.toBeNull();
  expect(document.activeElement).toBe(field());
  type("sample");
  expect(count()).toBe("1 of 3");
});

test("the menu event opens it, only in the window that has the focus", async () => {
  await mount();
  vi.mocked(document.hasFocus).mockReturnValue(false);
  env.api.emit("menu:find", undefined);
  expect(bar()).toBeNull();
  vi.mocked(document.hasFocus).mockReturnValue(true);
  env.api.emit("menu:find", undefined);
  expect(bar()).not.toBeNull();
});

test("Edit › Find Next and Previous step through the matches", async () => {
  await mount();
  env.api.emit("menu:find", undefined);
  type("sample");
  env.api.emit("menu:find-next", undefined);
  expect(count()).toBe("2 of 3");
  env.api.emit("menu:find-previous", undefined);
  env.api.emit("menu:find-previous", undefined);
  expect(count()).toBe("3 of 3");
});

test("a page that is shown again is searched again", async () => {
  await mount();
  env.api.emit("menu:find", undefined);
  type("sample");
  env.ctx.navigate("b.md");
  await tick();
  expect(count()).toBe("No results");
});

test("unmounting removes the bar and the listeners", async () => {
  await mount();
  env.api.emit("menu:find", undefined);
  unmount!(); unmount = undefined;
  expect(bar()).toBeNull();
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "f", metaKey: true, bubbles: true }));
  expect(bar()).toBeNull();
});
