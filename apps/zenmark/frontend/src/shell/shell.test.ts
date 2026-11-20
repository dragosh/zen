// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, sampleWorkspace, ZENMARK_BUILD, type FakeApi } from "../api/fake";
import { createShell, type Shell } from "./shell";
import type { Ctx, Feature, Slots } from "@zenmd/kit";

let root: HTMLElement;
let shell: Shell | undefined;
let log: string[];

const fullBuild = () => expect(ZENMARK_BUILD.features).toEqual(["reader", "nav", "find", "editor", "inspector", "print", "export", "newsite", "developer"]);

function recorder(id: Feature["id"]): Feature {
  return {
    id,
    mount(slots: Slots, ctx: Ctx) {
      log.push(`mount ${id} ${ctx.store.get().path}`);
      const mark = document.createElement("p");
      mark.dataset.feature = id;
      (id === "nav" ? slots.nav : slots.main).append(mark);
      return () => { log.push(`unmount ${id}`); mark.remove(); };
    },
  };
}
const features = () => [recorder("nav"), recorder("reader"), recorder("editor")];
const ALL = ZENMARK_BUILD.features;
const without = (...ids: string[]) => ALL.filter((f) => !ids.includes(f));

function boot(api: FakeApi, feats: Feature[] = features()): Shell {
  shell = createShell(root, { api, features: feats, search: "" });
  return shell;
}
const key = (k: string) =>
  document.dispatchEvent(new KeyboardEvent("keydown", { key: k, metaKey: true, bubbles: true, cancelable: true }));
const tick = () => new Promise((r) => setTimeout(r, 0));
const button = (label: string) =>
  Array.from(root.querySelectorAll("button")).find((b) => b.textContent === label || b.getAttribute("aria-label") === label);
const withWorkspace = (page = "", mode: "read" | "edit" = "read") =>
  createFakeApi({ startup: { workspace: sampleWorkspace(), page, mode }, layouts: [{ id: "course", version: "1.0.0", description: "Steps", source: "builtin", status: "on" }] });

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  root = document.createElement("div");
  document.body.append(root);
  log = [];
});
afterEach(() => {
  shell?.destroy();
  shell = undefined;
  root.remove();
});

test("welcome has the logo, tagline and footer", async () => {
  const s = boot(createFakeApi());
  await s.start();
  const img = root.querySelector<HTMLImageElement>(".zen-welcome img.zen-welcome-logo")!;
  expect(img).not.toBeNull();
  expect(img.getAttribute("alt")).toBe("");
  expect(root.querySelector(".zen-welcome-tagline")?.textContent).toBe("Markdown folders as sites, documents and courses");
  expect(root.querySelector(".zen-welcome-foot")).toBeNull();
  expect(root.querySelector(".zen-welcome")?.textContent).not.toContain("Also File");
});

test("welcome has a hint pointing up to the menu bar, and Settings stays reachable", async () => {
  const s = boot(createFakeApi());
  await s.start();
  const hint = root.querySelector(".zen-welcome-menuhint")!;
  expect(hint).not.toBeNull();
  expect(hint.textContent).toContain("↖");
  expect(hint.textContent).toContain("File menu");
  expect(hint.textContent).toContain("Open Folder");
  expect(hint.textContent).toContain("New Site");
  expect(hint.textContent).toContain("⇧⌘N");
  expect(hint.querySelector("button.zen-welcome-settings")?.textContent).toContain("Settings");
});

test("shows welcome without workspace", async () => {
  const s = boot(createFakeApi());
  await s.start();
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  expect(root.querySelector("h1")?.textContent).toBe("Zenmark");
  expect(button("Open a folder")?.classList.contains("primary")).toBe(true);
  expect(root.textContent).toContain("⌘O");
  expect(root.querySelector("nav")).toBeNull();
  expect(log).toEqual([]);
});

test("shows frame with workspace", async () => {
  const s = boot(withWorkspace("guides/install.md"));
  await s.start();
  expect(root.querySelector(".zen-welcome")).toBeNull();
  expect(root.querySelector("nav")).not.toBeNull();
  expect(root.querySelector("header")).not.toBeNull();
  expect(root.querySelector("main")).not.toBeNull();
  expect(s.store.get().workspace?.name).toBe("sample-docs");
  expect(s.store.get().path).toBe("guides/install.md");
});

test("a reload keeps the page you were reading (same folder only)", async () => {
  const s1 = boot(withWorkspace());
  await s1.start();
  s1.navigate("guides/install.md", "steps");
  s1.destroy();
  // ⌘R: a new shell in the same window session, Go still says README.md
  const s2 = boot(withWorkspace());
  await s2.start();
  expect(s2.store.get().path).toBe("guides/install.md");
  expect(s2.store.get().anchor).toBe("steps");
  s2.destroy();
  // another folder ignores it
  const other = { ...sampleWorkspace(), root: "/elsewhere" };
  const s3 = boot(createFakeApi({ startup: { workspace: other, page: "", mode: "read" } }));
  await s3.start();
  expect(s3.store.get().path).toBe("README.md");
  s3.destroy();
  // a page that is gone falls back to the startup page
  sessionStorage.setItem("zen.shell.last", JSON.stringify({ root: sampleWorkspace().root, path: "gone.md", anchor: "" }));
  const s4 = boot(withWorkspace());
  await s4.start();
  expect(s4.store.get().path).toBe("README.md");
});

test("startup without a page goes to the workspace home", async () => {
  const s = boot(withWorkspace());
  await s.start();
  expect(s.store.get().path).toBe("README.md");
});

test("no page and no home shows a hint in the page slot", async () => {
  const ws = { ...sampleWorkspace(), home: "" };
  const s = boot(createFakeApi({ startup: { workspace: ws, page: "", mode: "read" } }));
  await s.start();
  expect(root.querySelector("main")?.textContent).toBe("Select a page from the sidebar");
  expect(log).toEqual(["mount nav "]);
  s.navigate("guides/usage.md");
  expect(root.querySelector("main")?.textContent).not.toContain("Select a page");
  expect(log).toContain("mount reader guides/usage.md");
});

test("the launch folder that did not open: the welcome screen says why", async () => {
  const s = boot(createFakeApi({ startup: { workspace: null, page: "", mode: "read", error: "opening big took longer than 30s." } }));
  await s.start();
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  expect(root.querySelector(".zen-welcome-status")?.textContent).toBe("Could not open the folder: opening big took longer than 30s.");
});

test("opening a folder shows the loader with Cancel; a failure shows why", async () => {
  const api = createFakeApi();
  const s = boot(api);
  await s.start();
  const loader = () => document.querySelector<HTMLElement>(".zen-opening")!;
  api.emit("workspace:opening", { name: "big", phase: "indexing", done: 3, total: 10 });
  await new Promise((r) => setTimeout(r, 300));
  expect(loader().hidden).toBe(false);
  expect(loader().textContent).toContain("Indexing pages · 3 of 10");
  Array.from(loader().querySelectorAll("button")).find((b) => b.textContent === "Cancel")!.click();
  expect(api.callsTo("cancelOpen")).toHaveLength(1);
  api.emit("workspace:opening", { name: "big", phase: "failed", done: 0, total: 0 });
  expect(loader().hidden).toBe(true);

  // The Open button in the frame: an error shows in the loader.
  api.failNext("pickFolder", new Error("opening big took longer than 30s."));
  s.openFolder();
  await tick();
  expect(loader().hidden).toBe(false);
  expect(loader().textContent).toContain("took longer than 30s.");
  s.destroy();
  expect(document.querySelector(".zen-opening")).toBeNull();
  shell = undefined;
});

test("open folder", async () => {
  const api = createFakeApi({ pick: null });
  const s = boot(api);
  await s.start();

  button("Open a folder")!.click();
  await tick();
  expect(api.callsTo("pickFolder")).toHaveLength(1);
  expect(root.querySelector(".zen-welcome")).not.toBeNull(); // cancelled: nothing changes

  key("o");
  await tick();
  expect(api.callsTo("pickFolder")).toHaveLength(2);
  expect(root.querySelector(".zen-welcome")).not.toBeNull();

  api.pick = sampleWorkspace();
  key("o");
  await tick();
  expect(root.querySelector(".zen-welcome")).toBeNull();
  expect(root.querySelector("nav")).not.toBeNull();
  expect(s.store.get().path).toBe("README.md");
});

test("open folder from the frame: cancel keeps the page, a new folder replaces it", async () => {
  const api = withWorkspace("guides/usage.md");
  api.pick = null;
  const s = boot(api);
  await s.start();
  await s.openFolder();
  expect(s.store.get().path).toBe("guides/usage.md");

  api.pick = { ...sampleWorkspace(), root: "/other", name: "other" };
  log.length = 0;
  await s.openFolder();
  expect(s.store.get().workspace?.name).toBe("other");
  expect(s.store.get().path).toBe("README.md");
  expect(log).toEqual(["unmount reader", "unmount nav", "mount nav README.md", "mount reader README.md"]);
  expect(root.querySelectorAll("nav")).toHaveLength(1);
});

test("the main window always shows the page; mode never mounts the editor", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  expect(log).toEqual(["mount nav README.md", "mount reader README.md"]);
  expect(root.querySelector('nav [data-feature="nav"]')).not.toBeNull();
  expect(root.querySelector('main [data-feature="reader"]')).not.toBeNull();

  s.store.set({ mode: "edit" });
  expect(log).toHaveLength(2);
  expect(root.querySelector('main [data-feature="editor"]')).toBeNull();

  s.navigate("guides/usage.md"); // nothing is remounted
  expect(log).toHaveLength(2);
});

test("startup in edit mode shows the page in read mode (Go opens the editor window)", async () => {
  const s = boot(withWorkspace("README.md", "edit"));
  await s.start();
  expect(log).toEqual(["mount nav README.md", "mount reader README.md"]);
  expect(s.store.get().mode).toBe("read");
});

test("a feature gets api, store and navigate", async () => {
  let got: Ctx | undefined;
  const api = withWorkspace("README.md");
  const s = boot(api, [{ id: "nav", mount: (_s, ctx) => { got = ctx; return () => {}; } }]);
  await s.start();
  expect(got?.api).toBe(api);
  expect(got?.store).toBe(s.store);
  got!.navigate("guides/install.md", "from-a-terminal");
  expect(s.store.get()).toMatchObject({ path: "guides/install.md", anchor: "from-a-terminal" });
});

test("navigate, back and forward, by key and by button", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  expect(button("Back")?.disabled).toBe(true);
  s.navigate("guides/install.md", "x");
  s.navigate("guides/usage.md");
  expect(s.store.get().anchor).toBe("");

  key("[");
  expect(s.store.get()).toMatchObject({ path: "guides/install.md", anchor: "x" });
  button("Back")!.click();
  expect(s.store.get().path).toBe("README.md");
  expect(button("Back")?.disabled).toBe(true);
  key("[");
  expect(s.store.get().path).toBe("README.md");

  key("]");
  expect(s.store.get().path).toBe("guides/install.md");
  button("Forward")!.click();
  expect(s.store.get().path).toBe("guides/usage.md");
  expect(button("Forward")?.disabled).toBe(true);
});

test("⌘\\ toggles the sidebar", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  key("\\");
  expect(root.querySelector("nav")?.hidden).toBe(true);
  key("\\");
  expect(root.querySelector("nav")?.hidden).toBe(false);
});

test("shortcuts need the command key and are ignored after destroy", async () => {
  const api = createFakeApi({ pick: null });
  const s = boot(api);
  await s.start();
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "o" }));
  await tick();
  expect(api.callsTo("pickFolder")).toHaveLength(0);
  s.destroy();
  key("o");
  await tick();
  expect(api.callsTo("pickFolder")).toHaveLength(0);
});

test("workspace:changed updates the store", async () => {
  const api = withWorkspace("README.md");
  const s = boot(api);
  await s.start();
  const next = { ...sampleWorkspace(), tree: [] };
  api.emit("workspace:changed", next);
  expect(s.store.get().workspace).toBe(next);
  expect(s.store.get().path).toBe("README.md");
});

test("startup error", async () => {
  const api = withWorkspace("README.md");
  api.failNext("startup", new Error("config is <b>broken</b>"));
  const s = boot(api);
  await s.start();
  const alert = root.querySelector('[role="alert"]')!;
  expect(alert.textContent).toContain("config is <b>broken</b>");
  expect(alert.querySelector("b")).toBeNull();
  expect(root.querySelector("nav")).toBeNull();

  button("Retry")!.click();
  await tick();
  expect(api.callsTo("startup")).toHaveLength(2);
  expect(root.querySelector('[role="alert"]')).toBeNull();
  expect(root.querySelector("nav")).not.toBeNull();
});

test("escapes workspace name", async () => {
  const name = '<img src=x onerror="window.pwned=1"><script>window.pwned=1</script>';
  const ws = { ...sampleWorkspace(), name, title: name };
  const { features: shipped } = await import("./features");
  const s = boot(createFakeApi({ startup: { workspace: ws, page: "", mode: "read" } }), shipped);
  await s.start();
  expect(root.querySelector("img")).toBeNull();
  expect(root.querySelector("script")).toBeNull();
  expect(root.querySelector("nav")?.textContent).toContain(name);
  expect((window as unknown as { pwned?: number }).pwned).toBeUndefined();
});

test("the sidebar header falls back to the workspace name without a title, and it is the only heading", async () => {
  const ws = { ...sampleWorkspace(), name: "my-folder", title: undefined };
  const s = boot(createFakeApi({ startup: { workspace: ws, page: "", mode: "read" } }));
  await s.start();
  expect(root.querySelector(".zen-sidebar-title")?.textContent).toBe("my-folder");
  expect(root.querySelector(".zen-nav-ws")).toBeNull();
});

test("shell sources never use innerHTML", async () => {
  const files = import.meta.glob(["./*.ts", "!./*.test.ts"], { query: "?raw", import: "default", eager: true });
  expect(Object.keys(files).length).toBeGreaterThan(3);
  for (const [name, src] of Object.entries(files)) {
    expect(src as string, name).not.toMatch(/innerHTML|outerHTML|insertAdjacentHTML|document\.write/);
  }
});

test("shipped features make the app usable", async () => {
  const { features: shipped } = await import("./features");
  expect(shipped.map((f) => f.id).sort()).toEqual(["editor", "nav", "reader"]);
  const s = boot(withWorkspace("README.md"), shipped);
  await s.start();
  expect(root.querySelector("nav")?.textContent).toContain("Sample docs");
  const settle = () => new Promise((r) => setTimeout(r, 0));
  await settle();
  expect(root.querySelector("main .zen-page")).not.toBeNull();
  expect(root.querySelector("main textarea")).toBeNull();
  s.destroy();
  shell = undefined;
  expect(root.childElementCount).toBe(0);
});

test("workspace:opened switches to that folder, also from the welcome screen", async () => {
  const api = createFakeApi();
  const s = boot(api);
  await s.start();
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  const ws = { ...sampleWorkspace(), name: "other" };
  api.emit("workspace:opened", { workspace: ws, page: "guides/install.md", mode: "edit" });
  expect(root.querySelector(".zen-welcome")).toBeNull();
  expect(s.store.get()).toMatchObject({ workspace: ws, path: "guides/install.md", mode: "read" });
});

test("File › New Page (menu or ⌘N) asks, creates, shows and opens the editor", async () => {
  const api = withWorkspace("guides/install.md");
  const s = boot(api);
  await s.start();
  api.emit("menu:new-page", undefined);
  const dialog = () => document.querySelector<HTMLElement>(".zen-newpage-backdrop");
  expect(dialog()).not.toBeNull();
  expect(document.querySelector<HTMLInputElement>(".zen-newpage-name")!.value).toBe("guides/untitled");
  key("n"); // already open: not a second one
  expect(document.querySelectorAll(".zen-newpage-backdrop")).toHaveLength(1);
  for (let i = 0; i < 5; i++) await tick();
  document.querySelector<HTMLElement>(".zen-newpage-layout[data-layout=course]")!.click();
  document.querySelector<HTMLFormElement>(".zen-newpage")!.requestSubmit();
  for (let i = 0; i < 5; i++) await tick();
  expect(dialog()).toBeNull();
  expect(api.callsTo("createPage")).toEqual([["guides/untitled", ""]]);
  expect(await api.source("guides/untitled.md")).toBe("---\nlayout: course\n---\n# Title");
  expect(s.store.get().path).toBe("guides/untitled.md");
  expect(api.callsTo("openEditor")).toEqual([["guides/untitled.md"]]);
  key("n");
  expect(dialog()).not.toBeNull();
  s.destroy();
  shell = undefined;
  expect(dialog()).toBeNull();
});

test("read-only: no new page", async () => {
  const api = createFakeApi({
    startup: { workspace: sampleWorkspace(), page: "guides/install.md", mode: "read" }, build: { features: without("editor") },
  });
  let newPageOffered: boolean | undefined;
  const probe: Feature = { id: "nav", mount(_s: Slots, ctx: Ctx) { newPageOffered = !!ctx.newPage; return () => {}; } };
  const s = boot(api, [probe, recorder("reader"), recorder("editor")]);
  await s.start();
  expect(newPageOffered).toBe(false); // the sidebar menu has no New Page Here…
  api.emit("menu:new-page", undefined);
  key("n");
  expect(document.querySelector(".zen-newpage-backdrop")).toBeNull();
});

test("no New Page dialog on the welcome screen", async () => {
  const api = createFakeApi();
  const s = boot(api);
  await s.start();
  api.emit("menu:new-page", undefined);
  key("n");
  expect(document.querySelector(".zen-newpage-backdrop")).toBeNull();
});

// ---- Settings is a page in the main pane

const esc = () => document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
const settingsPage = () => root.querySelector<HTMLElement>(".zen-settings-page");
const cog = () => root.querySelector<HTMLButtonElement>(".zen-settings-button")!;
const pageMain = () => root.querySelector<HTMLElement>("main.zen-main")!;

test("the cog opens Settings in the main pane: the page is kept, the sidebar stays", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  expect(settingsPage()).toBeNull();
  cog().click();
  const view = settingsPage()!;
  expect(view).not.toBeNull();
  expect(view.isConnected).toBe(true);
  expect(root.querySelector(".zen-content")!.contains(view)).toBe(true);
  expect(pageMain().hidden).toBe(true);
  expect(root.querySelector<HTMLElement>("nav.zen-sidebar")!.hidden).toBe(false);
  expect(cog().getAttribute("aria-expanded")).toBe("true");
  expect(root.querySelector(".zen-settings-popover, [role=dialog]")).toBeNull();
  expect(log).not.toContain("unmount reader"); // the page you were reading stays mounted
  expect(s.store.get().path).toBe("README.md");
});

test("the cog and ⌘, toggle Settings", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  key(",");
  expect(settingsPage()).not.toBeNull();
  key(",");
  expect(settingsPage()).toBeNull();
  expect(pageMain().hidden).toBe(false);
  cog().click();
  expect(settingsPage()).not.toBeNull();
  cog().click();
  expect(settingsPage()).toBeNull();
  expect(cog().getAttribute("aria-expanded")).toBe("false");
});

test("Esc returns to the page; Esc elsewhere does nothing", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  const prevented = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
  document.dispatchEvent(prevented);
  expect(prevented.defaultPrevented).toBe(false);
  cog().click();
  esc();
  expect(settingsPage()).toBeNull();
  expect(pageMain().hidden).toBe(false);
  expect(s.store.get().path).toBe("README.md");
});

test("Back (⌘[ and the Back button) returns to the page and does not touch the history", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  s.navigate("guides/install.md");
  cog().click();
  expect(button("Back")!.disabled).toBe(false);
  key("[");
  expect(settingsPage()).toBeNull();
  expect(s.store.get().path).toBe("guides/install.md");
  cog().click();
  button("Back")!.click();
  expect(settingsPage()).toBeNull();
  expect(s.store.get().path).toBe("guides/install.md");
  key("[");
  expect(s.store.get().path).toBe("README.md"); // with Settings closed, Back goes back again
});

test("Back is available in Settings even at the start of the history, and goes disabled again after", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  expect(button("Back")!.disabled).toBe(true);
  cog().click();
  expect(button("Back")!.disabled).toBe(false);
  esc();
  expect(button("Back")!.disabled).toBe(true);
});

test("choosing a page in the sidebar (navigate) leaves Settings for that page", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  cog().click();
  s.navigate("guides/usage.md");
  expect(settingsPage()).toBeNull();
  expect(pageMain().hidden).toBe(false);
  expect(s.store.get().path).toBe("guides/usage.md");
});

test("a new folder closes Settings", async () => {
  const api = withWorkspace("README.md");
  const s = boot(api);
  await s.start();
  cog().click();
  api.emit("workspace:opened", { workspace: sampleWorkspace(), page: "guides/install.md", mode: "read" });
  expect(settingsPage()).toBeNull();
});

test("on the home screen ⌘, (and the Settings link) show Settings as a page; Back and Esc return home", async () => {
  const api = createFakeApi();
  const s = boot(api);
  await s.start();
  key(",");
  await tick();
  expect(root.querySelector(".zen-home-settings .zen-settings-page")).not.toBeNull();
  expect(root.querySelector(".zen-welcome")).toBeNull();
  // No folder: the folder's own config file cannot be opened.
  expect(root.querySelectorAll(".zen-settings-section").length).toBeGreaterThan(0);
  root.querySelector<HTMLButtonElement>(".zen-home-settings-top button")!.click();
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  root.querySelector<HTMLButtonElement>(".zen-welcome-settings")!.click();
  expect(root.querySelector(".zen-home-settings")).not.toBeNull();
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  api.emit("menu:settings", undefined);
  expect(root.querySelector(".zen-home-settings")).not.toBeNull();
});

test("File › Close Folder closes the folder and shows the home screen", async () => {
  const api = withWorkspace("README.md");
  const s = boot(api);
  await s.start();
  expect(root.querySelector(".zen-sidebar")).not.toBeNull();
  api.emit("menu:close-folder", undefined);
  await tick();
  expect(api.callsTo("closeFolder")).toHaveLength(1);
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  expect(root.querySelector(".zen-sidebar")).toBeNull();
  expect(s.store.get().workspace).toBeNull();
  // On the home screen it does nothing.
  api.emit("menu:close-folder", undefined);
  await tick();
  expect(api.callsTo("closeFolder")).toHaveLength(1);
});

test("Close Folder keeps the folder when an editor window keeps its changes", async () => {
  const api = withWorkspace("README.md");
  api.closeFolder = async () => false;
  const s = boot(api);
  await s.start();
  api.emit("menu:close-folder", undefined);
  await tick();
  expect(root.querySelector(".zen-sidebar")).not.toBeNull();
  expect(s.store.get().workspace).not.toBeNull();
});

test("Settings shows the plugins and its sections", async () => {
  const api = createFakeApi({ startup: { workspace: sampleWorkspace(), page: "README.md", mode: "read" } });
  const s = boot(api);
  await s.start();
  cog().click();
  await tick();
  expect(Array.from(settingsPage()!.querySelectorAll(".zen-settings-section")).map((b) => b.textContent)).toEqual(["Appearance", "Accessibility", "Configuration", "Markdown", "Layouts", "Plugins", "Templates", "Export", "Developer"]);
  expect(api.callsTo("plugins")).toHaveLength(0); // a list is read when its section is chosen
});

// ---- app/menu: Zen › Settings… (⌘,) works from any window

test("menu:settings opens Settings", async () => {
  const api = withWorkspace("README.md");
  const handlers: Array<() => void> = [];
  const realOn = api.on.bind(api) as (e: string, fn: (p?: unknown) => void) => () => void;
  (api as { on: unknown }).on = (e: string, fn: () => void) => {
    if (e === "menu:settings") { handlers.push(fn); return () => handlers.splice(handlers.indexOf(fn), 1); }
    return realOn(e, fn);
  };
  const s = boot(api);
  await s.start();
  expect(handlers).toHaveLength(1);
  expect(settingsPage()).toBeNull();
  handlers[0]();
  expect(settingsPage()).not.toBeNull();
  handlers[0](); // already open: stays open
  expect(settingsPage()).not.toBeNull();
  s.destroy();
  shell = undefined;
  expect(handlers).toHaveLength(0);
});

// ---- the welcome screen: Start from a template (core/init)

type SiteTpl = { id: string; name: string; description: string; version: string; ask: { key: string; label: string; default: string }[]; builtin: boolean; update: string; error: string };
const siteTpl = (id: string, over: Partial<SiteTpl> = {}): SiteTpl => ({ id, name: `Name ${id}`, description: `About ${id}`, version: "1.0.0", ask: [{ key: "title", label: "Site title", default: "{{folder}}" }], builtin: true, update: "", error: "", ...over });

/** The fake api plus core/init's methods and event, which api/ gets when the lead merges it. */
function withTemplates(api: FakeApi, templates: SiteTpl[]) {
  const siteCalls: unknown[][] = [];
  const listeners = new Set<() => void>();
  const origOn = api.on.bind(api) as (e: string, f: (x: never) => void) => () => void;
  return Object.assign(api, {
    siteCalls,
    emitNewSite: () => listeners.forEach((f) => f()),
    siteTemplates: async () => { siteCalls.push(["siteTemplates"]); return templates; },
    pickSiteFolder: async () => "/tmp/x/Docs",
    createSite: async (...a: unknown[]) => { siteCalls.push(["createSite", ...a]); return { workspace: sampleWorkspace(), page: "README.md", mode: "read" as const }; },
    revealTemplates: async () => undefined,
    on: (event: string, fn: (x: never) => void) => {
      if (event === "menu:new-site") { listeners.add(fn as () => void); return () => listeners.delete(fn as () => void); }
      return origOn(event, fn);
    },
  });
}

test("welcome lists templates: a card for each (six at most), invalid ones left out, and Show all…", async () => {
  const many = ["a", "b", "c", "d", "e", "f", "g"].map((id) => siteTpl(id));
  many.splice(2, 0, siteTpl("bad", { error: "broken" }));
  const api = withTemplates(createFakeApi(), many);
  const s = boot(api);
  await s.start();
  await tick();
  await tick();
  const section = root.querySelector(".zen-welcome-templates")!;
  expect(section.querySelector("h2")?.textContent).toBe("Start from a template");
  const cards = Array.from(section.querySelectorAll<HTMLButtonElement>(".zen-welcome-card"));
  expect(cards.map((c) => c.dataset.template)).toEqual(["a", "b", "c", "d", "e", "f"]);
  expect(cards[0].querySelector(".zen-welcome-card-name")?.textContent).toBe("Name a");
  expect(cards[0].querySelector(".zen-welcome-card-hint")?.textContent).toBe("About a");
  // Open a folder stays first.
  expect(root.querySelector(".zen-welcome .primary")?.textContent).toBe("Open a folder");
  expect(root.querySelector(".zen-welcome")!.innerHTML.indexOf("Open a folder")).toBeLessThan(root.querySelector(".zen-welcome")!.innerHTML.indexOf("Start from a template"));
  // A card opens the New Site dialog with that template chosen.
  cards[1].click();
  await tick();
  await tick();
  expect(document.querySelector(".zen-newsite")).not.toBeNull();
  expect(document.querySelector('.zen-newsite-template[aria-checked="true"]')?.getAttribute("data-template")).toBe("b");
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  // Show all… opens it with the first one chosen.
  Array.from(section.querySelectorAll("button")).find((b) => b.textContent === "Show all…")!.click();
  await tick();
  await tick();
  expect(document.querySelectorAll(".zen-newsite-template")).toHaveLength(8);
  expect(document.querySelector('.zen-newsite-template[aria-checked="true"]')?.getAttribute("data-template")).toBe("a");
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
});

test("welcome shows no template section when there are none or when listing fails", async () => {
  let s = boot(withTemplates(createFakeApi(), []));
  await s.start();
  await tick();
  expect(root.querySelector(".zen-welcome-templates")).toBeNull();
  s.destroy();
  const failing = withTemplates(createFakeApi(), []);
  failing.siteTemplates = async () => { throw new Error("down"); };
  s = boot(failing);
  await s.start();
  await tick();
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  expect(root.querySelector(".zen-welcome-templates")).toBeNull();
});

test("creating a site from the welcome screen opens it", async () => {
  const api = withTemplates(createFakeApi(), [siteTpl("a")]);
  const s = boot(api);
  await s.start();
  await tick();
  await tick();
  root.querySelector<HTMLButtonElement>(".zen-welcome-card")!.click();
  await tick();
  await tick();
  Array.from(document.querySelectorAll("button")).find((b) => b.textContent === "Choose folder…")!.click();
  await tick();
  document.querySelector<HTMLFormElement>(".zen-newsite")!.requestSubmit();
  await tick();
  await tick();
  expect(api.siteCalls.find((c) => c[0] === "createSite")).toEqual(["createSite", "a", "/tmp/x/Docs", { title: "Docs" }]);
  expect(root.querySelector(".zen-welcome")).toBeNull();
  expect(s.store.get().workspace?.root).toBe("/sample-docs");
});

// ---- Console in the Inspector (ui/inspector, ui/developer-tab)

const jKey = () => document.dispatchEvent(new KeyboardEvent("keydown", { code: "KeyJ", key: "∆", altKey: true, metaKey: true, bubbles: true, cancelable: true }));

test("Console: there is no Developer button; the tab shows only with the setting on, and ⌥⌘J opens the Inspector on it", async () => {
  const { features: shipped } = await import("./features");
  const s = boot(withWorkspace("README.md"), shipped);
  await s.start();
  await tick();
  await tick();
  expect(root.querySelector(".zen-developer-button")).toBeNull();
  expect(root.querySelector(".zen-dev")).toBeNull();
  const consoleTab = () => root.querySelector<HTMLElement>('.zen-inspector [data-tab="console"]')!;
  expect(consoleTab().hidden).toBe(true);
  jKey(); // the setting is off: nothing
  expect(root.querySelector<HTMLElement>(".zen-inspector")!.hidden).toBe(true);
  s.destroy();
  shell = undefined;

  localStorage.setItem("zen.developer", "1");
  const s2 = boot(withWorkspace("README.md"), shipped);
  await s2.start();
  await tick();
  await tick();
  expect(consoleTab().hidden).toBe(false);
  jKey();
  await tick();
  await tick();
  expect(root.querySelector<HTMLElement>(".zen-inspector")!.hidden).toBe(false);
  expect(consoleTab().getAttribute("aria-selected")).toBe("true");
  expect(root.querySelectorAll(".zen-dev-row").length).toBeGreaterThanOrEqual(4); // the fake's Go-side entries
  jKey();
  expect(root.querySelector<HTMLElement>(".zen-inspector")!.hidden).toBe(true);
});

test("the theme of the build is on the root element (data-app-theme)", async () => {
  delete document.documentElement.dataset.appTheme;
  const api = createFakeApi({ build: { theme: "default" } });
  const s = boot(api);
  await s.start();
  expect(document.documentElement.dataset.appTheme).toBe("default");
  s.destroy();
  shell = undefined;
  // a host without a build sets none
  delete document.documentElement.dataset.appTheme;
  const s2 = boot(createFakeApi({ build: null }));
  await s2.start();
  expect(document.documentElement.dataset.appTheme).toBeUndefined();
});

// ---- kit/shell-features: the bare shell and the features an app can leave out

const mod = (k: string, init: KeyboardEventInit = {}) =>
  document.dispatchEvent(new KeyboardEvent("keydown", { key: k, metaKey: true, bubbles: true, cancelable: true, ...init }));
const sectionNames = () => Array.from(root.querySelectorAll(".zen-settings-section")).map((b) => b.textContent);
const buildWith = (features: string[], page = "README.md") =>
  createFakeApi({ startup: { workspace: sampleWorkspace(), page, mode: "read" }, build: { features } });

test("the fake build is a full one (features: all)", () => {
  fullBuild();
});

test("bare shell", async () => {
  const { features: shipped } = await import("./features");
  // the home screen: open a folder, Settings; no templates, no New Site hint
  const home = createFakeApi({ build: { features: [] } });
  let s = boot(home, shipped);
  await s.start();
  await tick();
  expect(button("Open a folder")).toBeDefined();
  expect(root.querySelector(".zen-welcome-templates")).toBeNull();
  expect(root.querySelector(".zen-welcome-menuhint")!.textContent).not.toContain("New Site");
  expect(home.callsTo("siteTemplates")).toHaveLength(0);
  s.destroy();

  // a folder: the quiet card, no sidebar, no page, no navigation
  const api = buildWith([]);
  s = boot(api, shipped);
  await s.start();
  await tick();
  const card = root.querySelector(".zen-bare")!;
  expect(card).not.toBeNull();
  expect(card.querySelector("h1")!.textContent).toBe("sample-docs");
  expect(card.textContent).toContain("3 pages");
  expect(card.textContent).toContain("This build shows no pages.");
  expect(root.querySelector(".zen-nav-tree")).toBeNull();
  expect(root.querySelector(".zen-page")).toBeNull();
  expect(root.querySelector<HTMLElement>(".zen-sidebar")!.hidden).toBe(true);
  expect(root.querySelector('[data-action="toggle-sidebar"]')).toBeNull();
  expect(api.callsTo("page")).toHaveLength(0);
  expect(s.store.get().path).toBe("");
  // one page says "1 page"
  s.destroy();
  s = boot(createFakeApi({ startup: { workspace: { ...sampleWorkspace(), tree: [sampleWorkspace().tree[0]] }, page: "", mode: "read" }, build: { features: [] } }), shipped);
  await s.start();
  expect(root.querySelector(".zen-bare")!.textContent).toContain("1 page");
  expect(root.querySelector(".zen-bare")!.textContent).not.toContain("1 pages");
  s.destroy();

  // Settings is reachable (⌘,) with Appearance and This build only
  const bare = buildWith([]);
  s = boot(bare, shipped);
  await s.start();
  key(",");
  expect(sectionNames()).toEqual(["Appearance", "Accessibility"]);
  expect(root.querySelector('[role="radiogroup"][aria-label="Navigation"]')).toBeNull(); // no nav: no side to choose
  // Zenmark › About is the standard macOS panel: the web page has no About sheet and does not listen for menu:about
  esc();
  expect(document.querySelector(".zen-about")).toBeNull();
  // shortcuts of missing features do nothing; Open Folder and Settings stay
  esc();
  key("n");
  mod("n", { shiftKey: true });
  mod("f");
  mod("p");
  mod("e");
  mod("\\");
  expect(document.querySelector(".zen-newpage-backdrop, .zen-newsite, .zen-find")).toBeNull();
  expect(bare.callsTo("printWindow")).toHaveLength(0);
  key("o");
  await tick();
  expect(bare.callsTo("pickFolder")).toHaveLength(1);
});

test("developer alone in a bare shell gives an Inspector with the Console tab only, opened by ⌥⌘J", async () => {
  const { features: shipped } = await import("./features");
  localStorage.setItem("zen.developer", "1");
  const s = boot(buildWith(["developer"]), shipped);
  await s.start();
  await tick();
  expect(root.querySelector(".zen-bare")).not.toBeNull();
  expect(root.querySelector(".zen-inspector-button")).toBeNull();
  expect(root.querySelector(".zen-developer-button")).toBeNull();
  jKey();
  await tick();
  await tick();
  const panel = root.querySelector<HTMLElement>(".zen-inspector")!;
  expect(panel.hidden).toBe(false);
  expect(Array.from(panel.querySelectorAll<HTMLElement>('[role="tab"]')).filter((t) => !t.hidden).map((t) => t.dataset.tab)).toEqual(["console", "build"]);
  expect(root.querySelectorAll(".zen-dev-row").length).toBeGreaterThanOrEqual(4);
  mod("i", { altKey: true }); // ⌥⌘I: no Inspector in this build
  expect(panel.querySelector('[data-tab="info"]')).toBeNull();
  key(",");
  expect(sectionNames()).toEqual(["Appearance", "Accessibility", "Developer"]);
});

test("the Console follows the setting in a developer-only build: switched off, the panel goes", async () => {
  const { features: shipped } = await import("./features");
  localStorage.setItem("zen.developer", "1");
  const s = boot(buildWith(["developer"]), shipped);
  await s.start();
  jKey();
  expect(root.querySelector(".zen-inspector")).not.toBeNull();
  localStorage.removeItem("zen.developer");
  window.dispatchEvent(new CustomEvent("zen:developer"));
  expect(root.querySelector(".zen-inspector")).toBeNull();
});

// Each feature a build leaves out has no menu-free trace in the window: no button, section, dialog or shortcut.
const MISSING: { feature: string; check(api: FakeApi): Promise<void> | void }[] = [
  { feature: "nav", check: (api) => {
    expect(root.querySelector(".zen-nav-tree, .zen-nav-crumbs")).toBeNull();
    expect(root.querySelector<HTMLElement>(".zen-sidebar")!.hidden).toBe(true);
    expect(root.querySelector('[data-action="toggle-sidebar"]')).toBeNull();
    expect(root.querySelector(".zen-page")).not.toBeNull(); // the reader stays
    expect(api.callsTo("revealInFinder")).toHaveLength(0);
  } },
  { feature: "find", check: () => {
    mod("f");
    expect(document.querySelector(".zen-find")).toBeNull();
  } },
  { feature: "editor", check: () => {
    expect(root.querySelector('[title="Edit (⌘E)"]')).toBeNull();
    key("n");
    expect(document.querySelector(".zen-newpage-backdrop")).toBeNull();
  } },
  { feature: "inspector", check: async (api) => {
    expect(root.querySelector(".zen-inspector-button")).toBeNull();
    expect(root.querySelector(".zen-inspector")).toBeNull();
    document.dispatchEvent(new KeyboardEvent("keydown", { code: "KeyI", key: "i", altKey: true, metaKey: true, bubbles: true, cancelable: true }));
    expect(root.querySelector(".zen-inspector")).toBeNull();
    expect(api.callsTo("pageInfo")).toHaveLength(0);
  } },
  { feature: "print", check: async (api) => {
    const e = new KeyboardEvent("keydown", { key: "p", metaKey: true, bubbles: true, cancelable: true });
    document.dispatchEvent(e);
    expect(e.defaultPrevented).toBe(false);
    api.emit("menu:print-page", undefined);
    await tick();
    expect(api.callsTo("printWindow")).toHaveLength(0);
  } },
  { feature: "export", check: async (api) => {
    api.emit("menu:build-website", undefined);
    await tick();
    expect(document.querySelector(".zen-buildsite")).toBeNull();
    key(",");
    expect(sectionNames()).not.toContain("Export");
  } },
  { feature: "newsite", check: async (api) => {
    mod("n", { shiftKey: true });
    api.emit("menu:new-site", undefined);
    await tick();
    expect(document.querySelector(".zen-newsite")).toBeNull();
  } },
  { feature: "developer", check: async () => {
    localStorage.setItem("zen.developer", "1");
    expect(root.querySelector(".zen-developer-button")).toBeNull();
    jKey();
    expect(root.querySelector(".zen-dev")).toBeNull();
    expect(root.querySelector('.zen-inspector [data-tab="console"]')).toBeNull(); // the tab is not there at all
    key(",");
    expect(sectionNames()).not.toContain("Developer");
  } },
];

test("each missing feature leaves no trace", async () => {
  const { features: shipped } = await import("./features");
  for (const { feature, check } of MISSING) {
    sessionStorage.setItem("zen.inspector.open", "1"); // a remembered panel is not opened when its feature is missing
    const api = buildWith(without(feature));
    const s = boot(api, shipped);
    await s.start();
    await tick();
    await tick();
    try {
      await check(api);
    } catch (err) {
      throw new Error(`without ${feature}: ${(err as Error).message}`);
    }
    s.destroy();
    shell = undefined;
    localStorage.clear();
    sessionStorage.clear();
  }
});

test("with every feature the same controls are there", async () => {
  const { features: shipped } = await import("./features");
  const api = buildWith(ALL);
  const s = boot(api, shipped);
  await s.start();
  await tick();
  await tick();
  expect(root.querySelector(".zen-nav-tree")).not.toBeNull();
  expect(root.querySelector('[title="Edit (⌘E)"]')).not.toBeNull();
  expect(root.querySelector(".zen-inspector-button")).not.toBeNull();
  expect(root.querySelector(".zen-developer-button")).toBeNull(); // the Console is a tab of the Inspector
  expect(root.querySelector('[data-action="toggle-sidebar"]')).not.toBeNull();
  mod("f");
  expect(document.querySelector(".zen-find")).not.toBeNull();
  const e = new KeyboardEvent("keydown", { key: "p", metaKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(true);
});

// Intro screen (zenmark-org/zenmd#23)
test("welcome: the logo comes first, the rest follows in order (CSS delays)", async () => {
  await boot(createFakeApi()).start();
  const view = root.querySelector<HTMLElement>(".zen-welcome")!;
  const logo = view.querySelector(".zen-welcome-logo")!;
  expect(logo.classList.contains("zen-welcome-first")).toBe(true);
  for (const sel of ["h1", ".zen-welcome-tagline", ".zen-welcome-open", ".zen-welcome-menuhint"]) {
    expect(view.querySelector(sel)!.classList.contains("zen-welcome-later"), sel).toBe(true);
  }
  expect(logo.classList.contains("zen-welcome-later")).toBe(false);
});

test("welcome: the background is one of three images picked by the injected random", async () => {
  const urls: string[] = [];
  for (const r of [0, 0.5, 0.99]) {
    const s = createShell(root, { api: createFakeApi(), features: features(), search: "", random: () => r });
    shell = s;
    await s.start();
    const bg = root.querySelector<HTMLElement>(".zen-welcome .zen-welcome-bg")!;
    expect(bg).not.toBeNull();
    expect(bg.getAttribute("aria-hidden")).toBe("true");
    urls.push(bg.style.backgroundImage);
    s.destroy();
  }
  expect(new Set(urls).size).toBe(3);
  for (const u of urls) expect(u).toMatch(/intro-[123]/);
});

test("welcome: it picks again each time the home screen opens", async () => {
  const picks = [0, 0.99];
  const s = createShell(root, { api: createFakeApi(), features: features(), search: "", random: () => picks.shift() ?? 0 });
  shell = s;
  await s.start();
  const first = root.querySelector<HTMLElement>(".zen-welcome-bg")!.style.backgroundImage;
  key(","); // Settings from home, then Back to the home screen
  button("Back to the home screen")!.click();
  const second = root.querySelector<HTMLElement>(".zen-welcome-bg")!.style.backgroundImage;
  expect(second).not.toBe(first);
});

test("the background belongs to the home screen only", async () => {
  await boot(withWorkspace()).start();
  expect(root.querySelector(".zen-welcome-bg")).toBeNull();
  expect(root.querySelector(".zen-welcome")).toBeNull();
});

test("home screen: folder-only shortcuts and menu events do nothing", async () => {
  const api = createFakeApi();
  const s = boot(api);
  await s.start();
  const before = root.innerHTML;
  const fire = (k: string, extra: KeyboardEventInit = {}) => {
    const e = new KeyboardEvent("keydown", { key: k, metaKey: true, bubbles: true, cancelable: true, ...extra });
    document.dispatchEvent(e);
    return e;
  };
  for (const k of ["n", "\\", "[", "]", "f", "g", "p"]) fire(k);
  fire("f", { altKey: true });
  fire("i", { altKey: true });
  fire("b", { shiftKey: true });
  fire("w", { shiftKey: true });
  for (const ev of ["menu:new-page", "menu:find", "menu:find-next", "menu:find-previous", "menu:print-page", "menu:build-website", "menu:close-folder"] as const) {
    api.emit(ev, undefined);
  }
  await tick();
  expect(api.callsTo("printWindow")).toHaveLength(0);
  expect(api.callsTo("closeFolder")).toHaveLength(0);
  expect(api.callsTo("openEditor")).toHaveLength(0);
  expect(document.querySelector(".zen-newpage, .zen-buildsite, .zen-inspector")).toBeNull();
  expect(root.innerHTML).toBe(before);
});

test("⌘P on the home screen is swallowed, not printed", async () => {
  const api = createFakeApi();
  await boot(api).start();
  const e = new KeyboardEvent("keydown", { key: "p", metaKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(e);
  await tick();
  expect(api.callsTo("printWindow")).toHaveLength(0);
  expect(e.defaultPrevented).toBe(true);
});

test("app:ready is sent once the first screen is drawn, home or folder", async () => {
  const home = createFakeApi();
  shell = createShell(root, { api: home, features: features(), search: "", assetWaitMs: 0 }); // happy-dom never loads images
  await shell.start();
  await tick();
  expect(home.callsTo("appReady")).toHaveLength(1);
  shell?.destroy();
  const folder = withWorkspace();
  await boot(folder).start();
  await tick();
  expect(folder.callsTo("appReady")).toHaveLength(1);
});

test("app:ready is also sent when startup fails, so the error is seen", async () => {
  const api = createFakeApi();
  api.failNext("startup", new Error("boom"));
  await boot(api).start();
  await tick();
  expect(root.querySelector(".zen-error")).not.toBeNull();
  expect(api.callsTo("appReady")).toHaveLength(1);
});

test("the welcome fade waits until the window is shown (appReady resolves), then plays", async () => {
  const api = createFakeApi();
  let shown!: () => void;
  api.appReady = () => new Promise<void>((r) => { shown = r; });
  shell = createShell(root, { api, features: features(), search: "", assetWaitMs: 0 });
  await shell.start();
  await tick();
  const view = root.querySelector(".zen-welcome")!;
  expect(view.classList.contains("zen-welcome-play")).toBe(false);
  shown();
  await tick();
  expect(root.querySelector(".zen-welcome")!.classList.contains("zen-welcome-play")).toBe(true);
});

// ---- ui/settings-layout: Open Console, File › Export › Website…

test("there is no About sheet in the web page: Zenmark › About is the standard macOS panel", async () => {
  const api = createFakeApi({ startup: { workspace: sampleWorkspace(), page: "README.md", mode: "read" }, build: { version: "1.2.3" } });
  const s = boot(api);
  await s.start();
  expect(document.querySelector(".zen-about")).toBeNull();
  expect(document.body.textContent).not.toContain("Version 1.2.3");
});

test("File › Export › Website… (menu:build-website) opens Settings › Export › Website, with no modal", async () => {
  const api = withWorkspace("README.md");
  const s = boot(api);
  await s.start();
  api.emit("menu:build-website", undefined);
  await tick();
  await tick();
  expect(document.querySelector(".zen-buildsite-backdrop")).toBeNull();
  expect(root.querySelector(".zen-buildform")!.closest("[role=dialog]")).toBeNull();
  expect(settingsPage()).not.toBeNull();
  expect(root.querySelector('.zen-settings-section[data-section="export"]')!.getAttribute("aria-current")).toBe("true");
  expect(root.querySelector('[role=option][data-item="website"]')!.getAttribute("aria-selected")).toBe("true");
  expect(root.querySelector(".zen-buildform")).not.toBeNull();
  api.emit("menu:build-website", undefined); // already open: stays on it
  await tick();
  expect(root.querySelectorAll(".zen-settings-page")).toHaveLength(1);
});

test("Developer › Open Console asks the Inspector for its Console tab", async () => {
  localStorage.setItem("zen.developer", "1");
  const s = boot(withWorkspace("README.md"));
  await s.start();
  const asked = vi.fn();
  document.addEventListener("zen:inspector-console", asked);
  key(",");
  root.querySelector<HTMLElement>('.zen-settings-section[data-section="developer"]')!.click();
  await tick();
  await tick();
  const open = Array.from(root.querySelectorAll<HTMLButtonElement>(".zen-settings-detail button")).find((b) => b.textContent === "Open Console")!;
  expect(open.disabled).toBe(false);
  open.click();
  expect(asked).toHaveBeenCalledTimes(1);
  document.removeEventListener("zen:inspector-console", asked);
});

// ---- ui/nav-panel: the sidebar's header and footer, and its side (#24)

const sidebar = () => root.querySelector<HTMLElement>(".zen-sidebar")!;
const navRow = () => root.querySelector<HTMLElement>('[role="radiogroup"][aria-label="Navigation"]');

test("sidebar header: the folder's title and logo, replaced live, with the name when there is no title", async () => {
  const api = createFakeApi({ startup: { workspace: { ...sampleWorkspace(), title: "My Course", logo: "/_ws/img/logo.svg" }, page: "README.md", mode: "read" } });
  await boot(api).start();
  const title = () => sidebar().querySelector<HTMLElement>(".zen-sidebar-head .zen-sidebar-title")!;
  expect(title().textContent).toBe("My Course");
  expect(sidebar().querySelector(".zen-sidebar-head img")!.getAttribute("src")).toBe("/_ws/img/logo.svg");
  api.emit("workspace:changed", { ...sampleWorkspace(), title: "Renamed", logo: "" });
  expect(title().textContent).toBe("Renamed");
  expect(sidebar().querySelector(".zen-sidebar-head img")).toBeNull();
  api.emit("workspace:changed", { ...sampleWorkspace(), title: "", logo: "" });
  expect(title().textContent).toBe("sample-docs"); // the folder's name
});

test("sidebar close: the × closes the folder like File › Close Folder", async () => {
  const api = withWorkspace("README.md");
  const s = boot(api);
  await s.start();
  const x = button("Close folder")!;
  expect(x).toBeDefined();
  expect(sidebar().contains(x)).toBe(true);
  expect(x.title).toBe("Close Folder ⇧⌘W");
  x.click();
  await tick();
  expect(api.callsTo("closeFolder")).toHaveLength(1);
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  expect(s.store.get().workspace).toBeNull();
});

test("sidebar close: the folder stays when an editor window keeps its changes", async () => {
  const api = withWorkspace("README.md");
  api.closeFolder = async () => false;
  const s = boot(api);
  await s.start();
  button("Close folder")!.click();
  await tick();
  expect(sidebar()).not.toBeNull();
  expect(s.store.get().workspace).not.toBeNull();
});

test("footer buttons per feature: only those the build has, in order, and never a Developer button", async () => {
  const { features: shipped } = await import("./features");
  const labels = () => Array.from(sidebar().querySelectorAll(".zen-sidebar-foot button")).map((b) => b.getAttribute("aria-label"));
  let s = boot(buildWith(ALL), shipped);
  await s.start();
  expect(labels()).toEqual(["Settings", "Export", "Inspector"]);
  expect(root.querySelector(".zen-developer-button")).toBeNull();
  s.destroy();
  s = boot(buildWith(without("export")), shipped);
  await s.start();
  expect(labels()).toEqual(["Settings", "Inspector"]);
  s.destroy();
  s = boot(buildWith(without("inspector")), shipped);
  await s.start();
  expect(labels()).toEqual(["Settings", "Export"]);
});

test("export button opens Settings › Export", async () => {
  const api = withWorkspace("README.md");
  await boot(api).start();
  button("Export")!.click();
  await tick();
  await tick();
  expect(settingsPage()).not.toBeNull();
  expect(root.querySelector('.zen-settings-section[data-section="export"]')!.getAttribute("aria-current")).toBe("true");
  button("Export")!.click(); // already open: stays on it
  await tick();
  expect(root.querySelectorAll(".zen-settings-page")).toHaveLength(1);
});

test("Navigation row in Appearance: Left or Right, applied at once to the sidebar and kept", async () => {
  const api = withWorkspace("README.md");
  await boot(api).start();
  const frame = () => root.querySelector<HTMLElement>(".zen-frame")!;
  expect(frame().dataset.navSide ?? "left").toBe("left");
  key(",");
  await tick();
  root.querySelector<HTMLElement>('[role=option][data-item="extra-0"]')!.click();
  const row = navRow()!;
  expect(row).not.toBeNull();
  const radios = Array.from(row.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
  expect(radios.map((r) => r.checked)).toEqual([true, false]);
  radios[1].click();
  expect(frame().dataset.navSide).toBe("right");
  expect(localStorage.getItem("zen.navSide")).toBe("right");
  radios[0].click();
  expect(frame().dataset.navSide ?? "left").toBe("left");
});

test("Navigation row in Appearance: a stored Right is applied when the folder opens", async () => {
  localStorage.setItem("zen.navSide", "right");
  await boot(withWorkspace("README.md")).start();
  expect(root.querySelector<HTMLElement>(".zen-frame")!.dataset.navSide).toBe("right");
});

test("Navigation is below Theme on the home screen too (a setting of this Mac, applied when a folder opens)", async () => {
  await boot(createFakeApi()).start();
  key(",");
  await tick();
  expect(root.querySelector(".zen-home-settings")).not.toBeNull();
  const items = Array.from(root.querySelectorAll<HTMLElement>('[data-section="appearance"] [role=option], [role=option][data-item]'))
    .filter((o) => o.closest(".zen-home-settings"))
    .map((o) => o.textContent?.trim());
  expect(items.slice(0, 2)).toEqual(["Theme", "Navigation"]);
  root.querySelector<HTMLElement>('[role=option][data-item="extra-0"]')!.click();
  const radios = Array.from(navRow()!.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
  radios[1].click();
  expect(localStorage.getItem("zen.navSide")).toBe("right");
});

// ui/accessibility 13: the options are on <html> before the shell's first render, so no window shows the defaults first.
test("accessibility applied before mount", async () => {
  localStorage.setItem("zen.a11y.contrast", "more");
  localStorage.setItem("zen.a11y.reduceMotion", "on");
  localStorage.setItem("zen.a11y.textSize", "22");
  const root$ = document.documentElement;
  delete root$.dataset.contrast;
  delete root$.dataset.reduceMotion;
  root$.style.removeProperty("--zen-text-size");
  const seen: string[] = [];
  const probe: Feature = {
    id: "reader",
    mount() { seen.push(`${root$.dataset.contrast}/${root$.dataset.reduceMotion}/${root$.style.getPropertyValue("--zen-text-size")}`); return () => {}; },
  };
  const s = boot(withWorkspace("README.md"), [probe]);
  // before start(): createShell itself has applied them
  expect(root$.dataset.contrast).toBe("more");
  await s.start();
  expect(seen.length).toBeGreaterThan(0);
  expect(seen[0]).toBe("more/on/22px");
  delete root$.dataset.contrast;
  delete root$.dataset.reduceMotion;
  root$.style.removeProperty("--zen-text-size");
});

test("the Export button is on while Settings shows Export, and off when it closes or switches section", async () => {
  await boot(withWorkspace("README.md")).start();
  const exp = () => root.querySelector<HTMLElement>(".zen-export-button")!;
  const section = (id: string) => root.querySelector<HTMLElement>(`.zen-settings-section[data-section="${id}"]`)!;
  expect(exp().getAttribute("aria-pressed")).toBe("false");
  exp().click();
  await tick();
  expect(exp().getAttribute("aria-pressed")).toBe("true");
  expect(root.querySelector(".zen-settings-button")!.getAttribute("aria-expanded")).toBe("true");
  section("appearance").click();
  expect(exp().getAttribute("aria-pressed")).toBe("false");
  section("export").click();
  expect(exp().getAttribute("aria-pressed")).toBe("true");
  key(",");
  await tick();
  expect(exp().getAttribute("aria-pressed")).toBe("false");
});
