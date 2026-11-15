// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from "vitest";

const calls: { name: string; args: unknown[] }[] = [];
const results: Record<string, unknown> = {};
const listeners = new Map<string, ((e: { name: string; data: unknown }) => void)[]>();
const emitted: { name: string; data: unknown }[] = [];

function binding(name: string) {
  return (...args: unknown[]) => {
    calls.push({ name, args });
    const r = results[name];
    return r instanceof Error ? Promise.reject(r) : Promise.resolve(r);
  };
}

vi.mock("../../bindings/github.com/dragosh/zen/internal/app/index.js", () => ({
  Zen: {
    Startup: binding("Startup"),
    PickFolder: binding("PickFolder"),
    Page: binding("Page"),
    Source: binding("Source"),
    Preview: binding("Preview"),
    Save: binding("Save"),
    OpenExternal: binding("OpenExternal"),
    OpenEditor: binding("OpenEditor"),
  },
}));

const windowCalls: string[] = [];

vi.mock("@wailsio/runtime", () => ({
  Events: {
    On(name: string, fn: (e: { name: string; data: unknown }) => void) {
      const l = listeners.get(name) ?? [];
      l.push(fn);
      listeners.set(name, l);
      return () => listeners.set(name, (listeners.get(name) ?? []).filter((f) => f !== fn));
    },
    Emit(name: string, data: unknown) {
      emitted.push({ name, data });
      return Promise.resolve(true);
    },
  },
  Window: {
    Close() {
      windowCalls.push("Close");
      return Promise.resolve();
    },
  },
}));

function fire(name: string, data?: unknown, sender?: string) {
  for (const fn of listeners.get(name) ?? []) fn({ name, data, ...(sender === undefined ? {} : { sender }) } as { name: string; data: unknown });
}

const ws = { root: "/r", name: "r", home: "README.md", tree: [{ title: "Home", path: "README.md", kind: "page", children: null }] };
const page = { path: "a.md", title: "A", html: "<p>a</p>", frontmatter: null, breadcrumb: null, assets: { css: null, js: null }, warnings: null };

describe("wails api", () => {
  beforeEach(() => {
    calls.length = 0;
    emitted.length = 0;
    windowCalls.length = 0;
    for (const k of Object.keys(results)) delete results[k];
  });

  it("is the default export and maps every call", async () => {
    const api = (await import("./wails")).default;
    results.Startup = { workspace: ws, page: "README.md", mode: "edit" };
    const s = await api.startup();
    expect(s).toEqual({ workspace: { ...ws, tree: [{ title: "Home", path: "README.md", kind: "page", children: [] }] }, page: "README.md", mode: "edit" });

    results.Startup = { workspace: null, page: "", mode: "" };
    expect(await api.startup()).toEqual({ workspace: null, page: "", mode: "read" });

    results.PickFolder = null;
    expect(await api.pickFolder()).toBeNull();
    results.PickFolder = ws;
    expect((await api.pickFolder())?.tree[0].children).toEqual([]);

    results.Page = page;
    const p = await api.page("a.md");
    expect(p).toEqual({ ...page, frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [] });

    results.Source = "# a";
    expect(await api.source("a.md")).toBe("# a");
    results.Preview = page;
    expect((await api.preview("a.md", "# b")).title).toBe("A");
    results.Save = undefined;
    await expect(api.save("a.md", "# b")).resolves.toBeUndefined();
    results.OpenExternal = true;
    expect(await api.openExternal("https://x")).toBe(true);
    results.OpenEditor = null;
    await expect(api.openEditor("a.md")).resolves.toBeUndefined();

    expect(calls).toEqual([
      { name: "Startup", args: [] },
      { name: "Startup", args: [] },
      { name: "PickFolder", args: [] },
      { name: "PickFolder", args: [] },
      { name: "Page", args: ["a.md"] },
      { name: "Source", args: ["a.md"] },
      { name: "Preview", args: ["a.md", "# b"] },
      { name: "Save", args: ["a.md", "# b"] },
      { name: "OpenExternal", args: ["https://x"] },
      { name: "OpenEditor", args: ["a.md"] },
    ]);
  });

  it("turns failures into Errors with the Go message", async () => {
    const api = (await import("./wails")).default;
    results.Page = Object.assign(new Error("ignored"), { message: "missing.md: not found" });
    await expect(api.page("missing.md")).rejects.toThrow("missing.md: not found");
    results.Save = "plain string failure";
    calls.length = 0;
    // a string rejection from the runtime still becomes an Error
    results.Save = new Error("docs/a.md: not a markdown file");
    await expect(api.save("docs/a.md", "x")).rejects.toBeInstanceOf(Error);
  });

  it("subscribes to events and unsubscribes", async () => {
    const api = (await import("./wails")).default;
    const got: unknown[] = [];
    const off1 = api.on("workspace:changed", (w) => got.push(w.tree[0].children));
    const off2 = api.on("page:changed", (e) => got.push(e.path));
    fire("workspace:changed", ws);
    fire("page:changed", { path: "a.md" });
    // Wails may deliver payloads wrapped in an array when emitted with several args
    fire("page:changed", [{ path: "b.md" }]);
    off1();
    off2();
    fire("workspace:changed", ws);
    fire("page:changed", { path: "c.md" });
    expect(got).toEqual([[], "a.md", "b.md"]);
  });

  it("answers app:before-close: closes when nothing objects", async () => {
    await import("./wails");
    fire("app:before-close");
    expect(emitted.map((e) => e.name)).toEqual(["app:close"]);
  });

  it("answers app:before-close: waits for a listener that prevents it", async () => {
    await import("./wails");
    let close: (() => void) | undefined;
    const onClose = (e: Event) => {
      e.preventDefault();
      close = (e as CustomEvent<{ close: () => void }>).detail.close;
    };
    window.addEventListener("zen:before-close", onClose);
    fire("app:before-close");
    window.removeEventListener("zen:before-close", onClose);
    expect(emitted.map((e) => e.name)).toEqual(["app:close-pending"]);
    close?.();
    expect(emitted.map((e) => e.name)).toEqual(["app:close-pending", "app:close"]);
  });

  it("openEditor failures carry the Go message", async () => {
    const api = (await import("./wails")).default;
    results.OpenEditor = new Error("notes.txt: not a markdown file");
    await expect(api.openEditor("notes.txt")).rejects.toThrow("not a markdown file");
  });

  it("a successful save tells the other windows with editor:saved", async () => {
    const api = (await import("./wails")).default;
    await api.save("docs/a.md", "x");
    expect(emitted).toEqual([{ name: "editor:saved", data: { path: "docs/a.md" } }]);
    results.Save = new Error("disk full");
    await expect(api.save("docs/a.md", "y")).rejects.toThrow("disk full");
    expect(emitted).toHaveLength(1);
  });

  it("page:changed listeners also get editor:saved from other windows", async () => {
    const api = (await import("./wails")).default;
    const got: string[] = [];
    const off = api.on("page:changed", (e) => got.push(e.path));
    fire("editor:saved", { path: "a.md" }, "editor:a.md");
    fire("editor:saved", [{ path: "b.md" }], "editor:b.md");
    fire("editor:saved", { path: "c.md" }, "main"); // this window's own save
    off();
    fire("editor:saved", { path: "d.md" }, "editor:d.md");
    expect(got).toEqual(["a.md", "b.md"]);
  });

  it("app:before-close for another window is ignored", async () => {
    await import("./wails");
    fire("app:before-close", { window: "editor:a.md" });
    expect(emitted).toEqual([]);
    fire("app:before-close", { window: "main" });
    expect(emitted.map((e) => e.name)).toEqual(["app:close"]);
  });

  it("answers app:before-close: a cancelled prompt is reported", async () => {
    await import("./wails");
    let cancel: (() => void) | undefined;
    const onClose = (e: Event) => {
      e.preventDefault();
      cancel = (e as CustomEvent<{ cancel: () => void }>).detail.cancel;
    };
    window.addEventListener("zen:before-close", onClose);
    fire("app:before-close", { window: "main" });
    window.removeEventListener("zen:before-close", onClose);
    cancel?.();
    cancel?.(); // answered once
    expect(emitted.map((e) => e.name)).toEqual(["app:close-pending", "app:close-cancel"]);
  });

  it("zen:close-window closes this native window", async () => {
    await import("./wails");
    window.dispatchEvent(new CustomEvent("zen:close-window"));
    expect(windowCalls).toEqual(["Close"]);
  });

  it("in an editor window it answers for that window only (keep last: reloads the module)", async () => {
    vi.resetModules();
    listeners.clear();
    history.replaceState(null, "", "/?window=editor&path=docs%2Fa.md");
    try {
      const api = (await import("./wails")).default;
      fire("app:before-close", { window: "main" });
      expect(emitted).toEqual([]);
      fire("app:before-close", { window: "editor:docs/a.md" });
      expect(emitted.map((e) => e.name)).toEqual(["app:close"]);
      const got: string[] = [];
      api.on("page:changed", (e) => got.push(e.path));
      fire("editor:saved", { path: "docs/a.md" }, "editor:docs/a.md");
      expect(got).toEqual([]);
    } finally {
      history.replaceState(null, "", "/");
    }
  });
});
