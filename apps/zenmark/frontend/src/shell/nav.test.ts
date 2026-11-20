// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import { createShell, type Shell } from "./shell";
import type { Feature } from "@zenmd/kit";
import { navFeature } from "@zenmd/kit";
import { deepTree, deepWorkspace, item, visible } from "@zenmd/kit/nav/fixtures";

let root: HTMLElement;
let shell: Shell;
let api: FakeApi;
const stub = (id: Feature["id"]): Feature => ({ id, mount: () => () => {} });
const tick = () => new Promise((r) => setTimeout(r, 0));
const nav = () => root.querySelector<HTMLElement>("[role=tree]")!.parentElement!;

async function boot(page = "README.md", mode: "read" | "edit" = "read") {
  api = createFakeApi({ startup: { workspace: deepWorkspace(), page, mode } });
  shell = createShell(root, { api, features: [navFeature, stub("reader"), stub("editor")] });
  await shell.start();
  await tick();
}
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); root = document.createElement("div"); document.body.append(root); });
afterEach(() => { shell?.destroy(); root.remove(); });

test("shows the tree, with no second heading; first-level nodes start expanded", async () => {
  await boot();
  expect(root.querySelector(".zen-nav-ws")).toBeNull();
  expect(visible(nav())).toEqual(["README.md", "guides", "guides/install.md", "guides/adv", "api.md", "api/auth.md"]);
});

test("clicking a page navigates via the shell", async () => {
  await boot();
  item(nav(), "guides/install.md").querySelector<HTMLElement>(".zen-nav-row")!.click();
  expect(shell.store.get().path).toBe("guides/install.md");
  expect(item(nav(), "guides/install.md").getAttribute("aria-current")).toBe("page");
});

test("expansion persists per workspace root", async () => {
  await boot();
  item(nav(), "guides").querySelector<HTMLElement>(".zen-nav-row")!.click();
  expect(visible(nav())).not.toContain("guides/install.md");
  shell.destroy();
  root.replaceChildren();
  await boot();
  expect(visible(nav())).not.toContain("guides/install.md");
  expect(localStorage.length).toBeGreaterThan(0);
});

test("workspace:changed keeps expansion and focus", async () => {
  await boot();
  item(nav(), "api.md").querySelector<HTMLElement>(".zen-nav-row")!.click(); // page with children: navigates
  item(nav(), "guides").querySelector<HTMLElement>(".zen-nav-row")!.click(); // collapse guides
  item(nav(), "api.md").querySelector<HTMLElement>("[tabindex]")!.focus();
  const tree = deepTree();
  tree.push({ title: "New", path: "new.md", kind: "page", children: [] });
  api.emit("workspace:changed", deepWorkspace(tree));
  await tick();
  expect(visible(nav())).toContain("new.md");
  expect(visible(nav())).not.toContain("guides/install.md");
  expect((document.activeElement as HTMLElement).closest<HTMLElement>("[role=treeitem]")!.dataset.path).toBe("api.md");
});

test("breadcrumb follows store page and dirty flag in edit mode", async () => {
  // The shell no longer enters edit mode from startup (ui/editor-window); set it directly.
  await boot("guides/install.md");
  shell.store.set({ mode: "edit" });
  shell.store.set({ page: { path: "guides/install.md", title: "Install", html: "", frontmatter: {}, warnings: [], assets: { css: [], js: [] }, layout: { name: "article", source: "" },
    breadcrumb: [{ title: "Guides", path: "guides" }, { title: "Install", path: "guides/install.md" }] } });
  const bc = root.querySelector<HTMLElement>(".zen-breadcrumb")!;
  expect(bc.textContent).toContain("Guides");
  expect(bc.querySelector('[title="Unsaved changes"]')).toBeNull();
  shell.store.set({ dirty: true });
  expect(bc.querySelector('[title="Unsaved changes"]')).not.toBeNull();
  shell.store.set({ mode: "read" });
  expect(bc.querySelector('[title="Unsaved changes"]')).toBeNull();
});

test("badges in the store are shown after the title and follow changes", async () => {
  await boot();
  const badge = (path: string) => item(nav(), path).querySelector<HTMLElement>(".zen-nav-badge");
  expect(badge("guides/install.md")).toBeNull();
  shell.store.set({ badges: { "guides/install.md": "3/5" } });
  expect(badge("guides/install.md")!.textContent).toBe("3/5");
  shell.store.set({ badges: { "guides/install.md": "4/5", "README.md": "new" } });
  expect(badge("guides/install.md")!.textContent).toBe("4/5");
  expect(badge("README.md")!.textContent).toBe("new");
  shell.store.set({ badges: {} });
  expect(badge("guides/install.md")).toBeNull();
});
