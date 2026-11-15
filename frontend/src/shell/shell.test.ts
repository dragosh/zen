// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test } from "vitest";
import { createFakeApi, sampleWorkspace, type FakeApi } from "../api/fake";
import { createShell, type Shell } from "./shell";
import type { Ctx, Feature, Slots } from "./types";

let root: HTMLElement;
let shell: Shell | undefined;
let log: string[];

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
const features = () => [recorder("nav"), recorder("page"), recorder("editor")];

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
  createFakeApi({ startup: { workspace: sampleWorkspace(), page, mode } });

beforeEach(() => {
  localStorage.clear();
  root = document.createElement("div");
  document.body.append(root);
  log = [];
});
afterEach(() => {
  shell?.destroy();
  shell = undefined;
  root.remove();
});

test("shows welcome without workspace", async () => {
  const s = boot(createFakeApi());
  await s.start();
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  expect(root.querySelector("h1")?.textContent).toBe("zen");
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
  expect(log).toContain("mount page guides/usage.md");
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
  expect(log).toEqual(["unmount page", "unmount nav", "mount nav README.md", "mount page README.md"]);
  expect(root.querySelectorAll("nav")).toHaveLength(1);
});

test("the main window always shows the page; mode never mounts the editor", async () => {
  const s = boot(withWorkspace("README.md"));
  await s.start();
  expect(log).toEqual(["mount nav README.md", "mount page README.md"]);
  expect(root.querySelector('nav [data-feature="nav"]')).not.toBeNull();
  expect(root.querySelector('main [data-feature="page"]')).not.toBeNull();

  s.store.set({ mode: "edit" });
  expect(log).toHaveLength(2);
  expect(root.querySelector('main [data-feature="editor"]')).toBeNull();

  s.navigate("guides/usage.md"); // nothing is remounted
  expect(log).toHaveLength(2);
});

test("startup in edit mode shows the page in read mode (Go opens the editor window)", async () => {
  const s = boot(withWorkspace("README.md", "edit"));
  await s.start();
  expect(log).toEqual(["mount nav README.md", "mount page README.md"]);
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
  const ws = { ...sampleWorkspace(), name };
  const { features: shipped } = await import("./features");
  const s = boot(createFakeApi({ startup: { workspace: ws, page: "", mode: "read" } }), shipped);
  await s.start();
  expect(root.querySelector("img")).toBeNull();
  expect(root.querySelector("script")).toBeNull();
  expect(root.querySelector("nav")?.textContent).toContain(name);
  expect((window as unknown as { pwned?: number }).pwned).toBeUndefined();
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
  expect(shipped.map((f) => f.id).sort()).toEqual(["editor", "nav", "page"]);
  const s = boot(withWorkspace("README.md"), shipped);
  await s.start();
  expect(root.querySelector("nav")?.textContent).toContain("sample-docs");
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
