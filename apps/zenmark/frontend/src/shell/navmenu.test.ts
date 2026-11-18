// @vitest-environment happy-dom
// The sidebar's context menu and reordering, through the real shell and the fake api (ui/sidebar-menu).
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import { createShell, type Shell } from "./shell";
import type { Feature, NavNode, Workspace } from "@zenmd/kit";
import { navFeature } from "@zenmd/kit";
import { item, visible } from "@zenmd/kit/nav/fixtures";

const p = (title: string, path: string, children: NavNode[] = []): NavNode => ({ title, path, kind: "page", children });
const ws = (): Workspace => ({
  root: "/docs", name: "docs", home: "README.md",
  tree: [p("Home", "README.md", [p("Alpha", "guides/alpha.md"), p("Beta", "guides/beta.md"), p("Gamma", "gamma.md")])],
});

let root: HTMLElement;
let shell: Shell;
let api: FakeApi;
const stub = (id: Feature["id"]): Feature => ({ id, mount: () => () => {} });
const tick = () => new Promise((r) => setTimeout(r, 0));
const settle = async () => { for (let i = 0; i < 4; i++) await tick(); };
const nav = () => root.querySelector<HTMLElement>("[role=tree]")!.parentElement!;
const row = (path: string) => item(nav(), path).querySelector<HTMLElement>(".zen-nav-row")!;
const menu = () => document.querySelector<HTMLElement>("[role=menu]");
const labels = () => Array.from(document.querySelectorAll("[role=menuitem]")).map((i) => i.textContent);
const choose = (label: string) => Array.from(document.querySelectorAll<HTMLElement>("[role=menuitem]")).find((i) => i.textContent === label)!.click();
const rightClick = (path: string) => row(path).dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 30, clientY: 40 }));
const status = () => root.querySelector<HTMLElement>(".zen-nav-status")!;
const order = () => visible(nav()).filter((x) => x !== "README.md");

async function boot(page = "README.md", mutate?: (a: FakeApi) => void) {
  api = createFakeApi({ startup: { workspace: ws(), page, mode: "read" }, sources: { "README.md": "# Home", "guides/alpha.md": "# Alpha", "guides/beta.md": "# Beta", "gamma.md": "# Gamma" } });
  mutate?.(api);
  shell = createShell(root, { api, features: [navFeature, stub("page"), stub("editor")] });
  await shell.start();
  await tick();
}
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); document.body.replaceChildren(); root = document.createElement("div"); document.body.append(root); });
afterEach(() => { shell?.destroy(); root.remove(); vi.restoreAllMocks(); });

test("right-click shows the menu with every item the api supports; the page is not opened", async () => {
  await boot();
  rightClick("guides/beta.md");
  expect(menu()).not.toBeNull();
  expect(labels()).toEqual(["Open", "Open in Editor", "New Page Here…", "Move Up", "Move Down", "Copy Path", "Copy Link", "Reveal in Finder", "Hide from Sidebar"]);
  expect(shell.store.get().path).toBe("README.md");
});

test("Open and Open in Editor", async () => {
  await boot();
  rightClick("gamma.md"); choose("Open");
  expect(shell.store.get().path).toBe("gamma.md");
  rightClick("gamma.md"); choose("Open in Editor");
  expect(api.callsTo("openEditor")).toEqual([["gamma.md"]]);
});

test("New Page Here… opens the New Page dialog in the page's folder", async () => {
  await boot();
  rightClick("guides/beta.md"); choose("New Page Here…");
  const name = document.querySelector<HTMLInputElement>(".zen-newpage-name")!;
  expect(name.value).toBe("guides/untitled");
});

test("Copy Path and Copy Link write to the clipboard", async () => {
  const write = vi.fn(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText: write }, configurable: true });
  await boot("guides/alpha.md");
  rightClick("guides/beta.md"); choose("Copy Path");
  await settle();
  expect(write).toHaveBeenLastCalledWith("guides/beta.md");
  rightClick("guides/beta.md"); choose("Copy Link");
  await settle();
  expect(write).toHaveBeenLastCalledWith("[Beta](beta.md)");
  expect(status().textContent).toContain("Copied");
});

test("Reveal in Finder calls the api with the path", async () => {
  await boot();
  rightClick("gamma.md"); choose("Reveal in Finder");
  expect(api.callsTo("revealInFinder")).toEqual([["gamma.md"]]);
});

test("Move Down reorders through the api; the tree follows the workspace:changed event and the page stays focused", async () => {
  await boot();
  expect(order()).toEqual(["guides/alpha.md", "guides/beta.md", "gamma.md"]);
  rightClick("guides/alpha.md"); choose("Move Down");
  await settle();
  expect(api.callsTo("reorderNav")).toEqual([["README.md", ["guides/beta.md", "guides/alpha.md", "gamma.md"]]]);
  expect(order()).toEqual(["guides/beta.md", "guides/alpha.md", "gamma.md"]);
  expect((document.activeElement as HTMLElement).closest<HTMLElement>("[role=treeitem]")!.dataset.path).toBe("guides/alpha.md");
  expect(status().textContent).toContain("Alpha");
});

test("Move Up is disabled for the first page and does nothing", async () => {
  await boot();
  rightClick("guides/alpha.md");
  expect(document.querySelector("[role=menuitem][aria-disabled=true]")!.textContent).toBe("Move Up");
  choose("Move Up");
  expect(api.callsTo("reorderNav")).toEqual([]);
});

test("Alt+Down on the focused row is Move Down", async () => {
  await boot();
  row("guides/beta.md").focus();
  row("guides/beta.md").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", altKey: true, bubbles: true, cancelable: true }));
  await settle();
  expect(order()).toEqual(["guides/alpha.md", "gamma.md", "guides/beta.md"]);
});

test("a failed reorder says why and leaves the tree as it was", async () => {
  await boot();
  api.failNext("reorderNav", new Error("a.md: permission denied"));
  rightClick("guides/alpha.md"); choose("Move Down");
  await settle();
  expect(status().textContent).toContain("permission denied");
  expect(status().getAttribute("role")).toBe("alert");
  expect(order()).toEqual(["guides/alpha.md", "guides/beta.md", "gamma.md"]);
});

test("Hide from Sidebar asks first; Cancel changes nothing, Hide writes and the page leaves the tree", async () => {
  await boot();
  rightClick("gamma.md"); choose("Hide from Sidebar");
  expect(document.querySelector("[role=alertdialog]")!.textContent).toContain("Gamma");
  Array.from(document.querySelectorAll("button")).find((b) => b.textContent === "Cancel")!.click();
  expect(api.callsTo("setNavHidden")).toEqual([]);

  rightClick("gamma.md"); choose("Hide from Sidebar");
  Array.from(document.querySelectorAll("button")).find((b) => b.textContent === "Hide")!.click();
  await settle();
  expect(api.callsTo("setNavHidden")).toEqual([["gamma.md", true]]);
  expect(order()).toEqual(["guides/alpha.md", "guides/beta.md"]);
});

test("items the api does not have are not offered", async () => {
  await boot("README.md", (a) => {
    const x = a as unknown as Record<string, unknown>;
    delete x.reorderNav; delete x.setNavHidden; delete x.revealInFinder;
  });
  rightClick("gamma.md");
  expect(labels()).toEqual(["Open", "Open in Editor", "New Page Here…", "Copy Path", "Copy Link"]);
});

test("the menu closes when the workspace is replaced under it", async () => {
  await boot();
  rightClick("gamma.md");
  expect(menu()).not.toBeNull();
  shell.destroy();
  expect(menu()).toBeNull();
});
