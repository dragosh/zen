// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, sampleWorkspace, type FakeApi } from "../api/fake";
import { createShell, windowKind, type Shell } from "./shell";
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
      const action = document.createElement("button");
      action.textContent = `${id} action`;
      slots.actions.append(action);
      return () => { log.push(`unmount ${id}`); mark.remove(); action.remove(); };
    },
  };
}
const features = () => [recorder("nav"), recorder("page"), recorder("editor")];
const EDITOR = "?window=editor&path=guides%2Finstall.md";

function boot(api: FakeApi, search = EDITOR, feats: Feature[] = features()): Shell {
  shell = createShell(root, { api, features: feats, search });
  return shell;
}
const withWorkspace = () => createFakeApi({ startup: { workspace: sampleWorkspace(), page: "README.md", mode: "read" } });
const tick = () => new Promise((r) => setTimeout(r, 0));
const key = (k: string) =>
  document.dispatchEvent(new KeyboardEvent("keydown", { key: k, metaKey: true, bubbles: true, cancelable: true }));

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

test("windowKind reads ?window=editor&path=", () => {
  expect(windowKind("")).toEqual({ kind: "main" });
  expect(windowKind("?")).toEqual({ kind: "main" });
  expect(windowKind("?window=main")).toEqual({ kind: "main" });
  expect(windowKind("?window=editor&path=README.md")).toEqual({ kind: "editor", path: "README.md" });
  expect(windowKind("?path=docs%2Fa%20b.md&window=editor")).toEqual({ kind: "editor", path: "docs/a b.md" });
  expect(windowKind("window=editor&path=a+b.md")).toEqual({ kind: "editor", path: "a b.md" });
  expect(windowKind("?window=editor")).toEqual({ kind: "main" }); // nothing to edit
  expect(windowKind("?window=editor&path=")).toEqual({ kind: "main" });
  expect(windowKind("?window=other&path=a.md")).toEqual({ kind: "main" });
});

test("an editor window mounts only the editor, under a header with the page title", async () => {
  const s = boot(withWorkspace());
  await s.start();
  expect(log).toEqual(["mount editor guides/install.md"]);
  expect(s.store.get()).toMatchObject({ path: "guides/install.md", mode: "edit" });
  expect(root.querySelector("nav")).toBeNull();
  expect(root.querySelector(".zen-breadcrumb")).toBeNull();
  expect(Array.from(root.querySelectorAll("button")).map((b) => b.textContent)).toEqual(["editor action"]);
  const header = root.querySelector("header")!;
  expect(header.querySelector(".zen-editor-window-title")?.textContent).toBe("Install");
  expect(header.querySelector(".zen-actions")?.textContent).toBe("editor action");
  expect(root.querySelector('main [data-feature="editor"]')).not.toBeNull();
  expect(document.title).toBe("Install — Edit — zen");
});

test("the title is plain text; unknown pages fall back to the file name", async () => {
  const ws = sampleWorkspace();
  ws.tree[0].title = '<img src=x onerror="window.pwned=1">';
  const api = createFakeApi({ startup: { workspace: ws, page: "", mode: "read" } });
  let s = boot(api, "?window=editor&path=README.md");
  await s.start();
  expect(root.querySelector("img")).toBeNull();
  expect(root.querySelector(".zen-editor-window-title")?.textContent).toBe(ws.tree[0].title);
  s.destroy();

  s = boot(api, "?window=editor&path=notes%2Fnew-page.md");
  await s.start();
  expect(root.querySelector(".zen-editor-window-title")?.textContent).toBe("new-page.md");
});

test("workspace:changed renames, workspace:opened and navigation keys are ignored", async () => {
  const api = withWorkspace();
  const s = boot(api);
  await s.start();
  const ws = sampleWorkspace();
  ws.tree[1].children[0].title = "Installing";
  api.emit("workspace:changed", ws);
  expect(root.querySelector(".zen-editor-window-title")?.textContent).toBe("Installing");
  expect(document.title).toBe("Installing — Edit — zen");

  api.emit("workspace:opened", { workspace: { ...sampleWorkspace(), name: "other" }, page: "README.md", mode: "read" });
  expect(s.store.get().path).toBe("guides/install.md");
  expect(log).toEqual(["mount editor guides/install.md"]);

  key("o");
  key("[");
  key("\\");
  await tick();
  expect(api.callsTo("pickFolder")).toHaveLength(0);
  expect(s.store.get().path).toBe("guides/install.md");
});

test("an editor window without a folder shows the error view", async () => {
  const s = boot(createFakeApi());
  await s.start();
  expect(root.querySelector('[role="alert"]')?.textContent).toContain("No folder is open");
  expect(log).toEqual([]);
});

test("destroy unmounts the editor", async () => {
  const s = boot(withWorkspace());
  await s.start();
  s.destroy();
  shell = undefined;
  expect(log).toEqual(["mount editor guides/install.md", "unmount editor"]);
  expect(root.childElementCount).toBe(0);
});

test("the main window ignores the editor feature and startup mode", async () => {
  const api = createFakeApi({ startup: { workspace: sampleWorkspace(), page: "README.md", mode: "edit" } });
  const s = boot(api, "");
  await s.start();
  expect(log).toEqual(["mount nav README.md", "mount page README.md"]);
  expect(s.store.get().mode).toBe("read");
});

// ---- ⌘W (app/menu): the window closes through the same guard as its close button

function closeRequests(): { count: () => number; off: () => void } {
  let n = 0;
  const on = () => { n++; };
  window.addEventListener("zen:close-window", on);
  return { count: () => n, off: () => window.removeEventListener("zen:close-window", on) };
}

test("⌘W asks before closing a dirty editor: it requests a close, the editor's guard decides", async () => {
  const s = boot(withWorkspace());
  await s.start();
  const closes = closeRequests();
  const e = new KeyboardEvent("keydown", { key: "w", metaKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(e);
  expect(closes.count()).toBe(1);
  expect(e.defaultPrevented).toBe(true);
  closes.off();
});

test("⌘W also closes the main window; other W combos do not", async () => {
  const s = boot(withWorkspace(), "");
  await s.start();
  const closes = closeRequests();
  key("w");
  expect(closes.count()).toBe(1);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "w", metaKey: true, shiftKey: true, bubbles: true }));
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "w", bubbles: true }));
  expect(closes.count()).toBe(1);
  closes.off();
});

test("⌘R does nothing in production builds (the key is swallowed) and in dev is left alone", async () => {
  const s = boot(withWorkspace(), "");
  await s.start();
  const press = () => {
    const e = new KeyboardEvent("keydown", { key: "r", metaKey: true, bubbles: true, cancelable: true });
    document.dispatchEvent(e);
    return e.defaultPrevented;
  };
  expect(press()).toBe(false);
  vi.stubEnv("PROD", true);
  try {
    expect(press()).toBe(true);
  } finally {
    vi.unstubAllEnvs();
  }
});
