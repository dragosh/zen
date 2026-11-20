// @vitest-environment happy-dom
// The sidebar of a read-only app (kit/features): no editing items, no reordering.
import { afterEach, beforeEach, expect, test } from "vitest";
import { readFileSync } from "node:fs";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import type { NavNode, Workspace } from "../host";
import { createStore } from "../shell/store";
import type { Ctx, Slots } from "../shell/types";
import { item } from "./fixtures";
import { navFeature } from "./index";

const p = (title: string, path: string, children: NavNode[] = []): NavNode => ({ title, path, kind: "page", children });
const ws = (): Workspace => ({
  root: "/docs", name: "docs", home: "README.md",
  tree: [p("Home", "README.md", [p("Alpha", "guides/alpha.md"), p("Beta", "guides/beta.md")])],
});

let unmount: (() => void) | undefined;
afterEach(() => { unmount?.(); unmount = undefined; });
beforeEach(() => { localStorage.clear(); document.body.replaceChildren(); });

function mount(api: FakeApi) {
  const store = createStore({ workspace: ws(), path: "README.md" });
  const ctx: Ctx = { api, store, brand: "Zenmark", navigate: (path) => store.set({ path }), newPage: () => {} };
  const slot = () => document.body.appendChild(document.createElement("div"));
  const slots: Slots = { nav: slot(), breadcrumb: slot(), actions: slot(), main: slot() };
  unmount = navFeature.mount(slots, ctx);
  return slots.nav;
}
const row = (nav: HTMLElement, path: string) => item(nav, path).querySelector<HTMLElement>(".zen-nav-row")!;
const labels = () => Array.from(document.querySelectorAll("[role=menuitem]")).map((i) => i.textContent);

test("read-only sidebar", () => {
  const api = createFakeApi({ build: { features: [] } });
  const nav = mount(api);
  row(nav, "guides/beta.md").dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 30, clientY: 40 }));
  // only what reads: no Open in Editor, New Page Here, Move Up/Down, Hide
  expect(labels()).toEqual(["Open", "Copy Path", "Copy Link", "Reveal in Finder"]);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));

  // Alt+↓ does not move a page
  const target = row(nav, "guides/alpha.md");
  target.focus();
  item(nav, "guides/alpha.md").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", altKey: true, bubbles: true, cancelable: true }));
  // dragging does not reorder: press, move far, release
  target.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true, cancelable: true, clientX: 50, clientY: 10, button: 0 }));
  document.dispatchEvent(new MouseEvent("pointermove", { bubbles: true, clientX: 50, clientY: 80 }));
  expect(nav.querySelector(".zen-nav-dragging, .zen-nav-drop")).toBeNull();
  document.dispatchEvent(new MouseEvent("pointerup", { bubbles: true, clientX: 50, clientY: 80 }));
  expect(api.callsTo("reorderNav")).toHaveLength(0);
  expect(api.callsTo("setNavHidden")).toHaveLength(0);
});

test("with the editor the menu has every item", () => {
  const api = createFakeApi();
  const nav = mount(api);
  row(nav, "guides/beta.md").dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 30, clientY: 40 }));
  expect(labels()).toEqual(["Open", "Open in Editor", "New Page Here…", "Move Up", "Move Down", "Copy Path", "Copy Link", "Reveal in Finder", "Hide from Sidebar"]);
});

test("one heading", () => {
  const api = createFakeApi();
  const nav = mount(api);
  // the sidebar header (layout) is the only heading: the old folder row is gone, and so is its CSS
  expect(nav.querySelector(".zen-nav-ws, .zen-nav-ws-name")).toBeNull();
  expect(nav.textContent).not.toContain("docs");
  expect(readFileSync(new URL("./nav.css", import.meta.url), "utf8")).not.toContain("zen-nav-ws");
  // the status under the tree is still there, below the tree
  const status = nav.querySelector<HTMLElement>(".zen-nav-status")!;
  expect(status.getAttribute("role")).toBe("status");
  expect(nav.querySelector(".zen-nav-tree")!.nextElementSibling).toBe(status);
});
