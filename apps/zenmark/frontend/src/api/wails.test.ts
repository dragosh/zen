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

vi.mock("../../bindings/github.com/zenmark-org/zenmd/apps/zenmark/index.js", () => ({
  Zen: {
    Startup: binding("Startup"),
    PickFolder: binding("PickFolder"),
    Page: binding("Page"),
    Source: binding("Source"),
    Preview: binding("Preview"),
    Save: binding("Save"),
    OpenExternal: binding("OpenExternal"),
    OpenEditor: binding("OpenEditor"),
    Lint: binding("Lint"),
    PageInfo: binding("PageInfo"),
    RevealInFinder: binding("RevealInFinder"),
    OpenConfig: binding("OpenConfig"),
    Plugins: binding("Plugins"),
    PluginTrust: binding("PluginTrust"),
    PluginStorageGet: binding("PluginStorageGet"),
    PluginStorageSet: binding("PluginStorageSet"),
    PluginInvoke: binding("PluginInvoke"),
    PluginStream: binding("PluginStream"),
    PluginCancel: binding("PluginCancel"),
    PluginStorageKeys: binding("PluginStorageKeys"),
    ReorderNav: binding("ReorderNav"),
    SetNavHidden: binding("SetNavHidden"),
    Layouts: binding("Layouts"),
    LayoutTrust: binding("LayoutTrust"),
    LayoutStorageGet: binding("LayoutStorageGet"),
    LayoutStorageSet: binding("LayoutStorageSet"),
    LayoutStorageKeys: binding("LayoutStorageKeys"),
    Logs: binding("Logs"),
    Dialects: binding("Dialects"),
    SetDialect: binding("SetDialect"),
    DialectHint: binding("DialectHint"),
    DismissDialectHint: binding("DismissDialectHint"),
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

  it("startup records the add-ons of the build on the api (and leaves out bad entries)", async () => {
    const api = (await import("./wails")).default;
    expect(api.addons).toBeUndefined();
    results.Startup = { workspace: null, page: "", mode: "read", addons: [{ id: "vale", name: "Vale linting", summary: "s", extra: 1 }, { id: 3 }, null] };
    const s = await api.startup();
    expect(s.addons).toEqual([{ id: "vale", name: "Vale linting", summary: "s" }]);
    expect(api.addons).toEqual([{ id: "vale", name: "Vale linting", summary: "s" }]);
    results.Startup = { workspace: null, page: "", mode: "read" };
    expect((await api.startup()).addons).toBeUndefined();
    expect(api.addons).toEqual([]);
  });

  it("startup carries the build (the assembly) and leaves out what is not text", async () => {
    const api = (await import("./wails")).default;
    results.Startup = {
      workspace: null, page: "", mode: "read",
      addons: [{ id: "vale", name: "Vale linting", summary: "s" }],
      build: { app: "zenmark", features: ["editor", 4], addons: [{ id: "vale", name: "Vale linting", summary: "s" }], plugins: ["codeblock", 3, null, "quiz"], layouts: ["article"], templates: null, dialects: ["myst", 2], theme: "default", extra: 1 },
    };
    const s = await api.startup();
    const want = { app: "zenmark", features: ["editor"], addons: [{ id: "vale", name: "Vale linting", summary: "s" }], plugins: ["codeblock", "quiz"], layouts: ["article"], templates: [], dialects: ["myst"], theme: "default" };
    expect(s.build).toEqual(want);
    expect(api.build).toEqual(want);
    results.Startup = { workspace: null, page: "", mode: "read" };
    expect((await api.startup()).build).toBeUndefined();
    expect(api.build).toBeUndefined();
  });

  it("a build that does not list its features has them all; an empty list is the bare shell", async () => {
    const api = (await import("./wails")).default;
    results.Startup = { workspace: null, page: "", mode: "read", build: { app: "old", plugins: [], layouts: [], templates: [], theme: "default" } };
    expect((await api.startup()).build?.features).toEqual(["reader", "nav", "find", "editor", "inspector", "print", "export", "newsite", "developer"]);
    results.Startup = { workspace: null, page: "", mode: "read", build: { app: "bare", features: [], plugins: [], layouts: [], templates: [], theme: "default" } };
    expect((await api.startup()).build?.features).toEqual([]);
  });

  it("maps the dialect calls and keeps only well-formed dialects", async () => {
    const api = (await import("./wails")).default;
    results.Dialects = [{ id: "myst", name: "MyST", summary: "s", on: true, extra: 1 }, { id: 3 }, null, { id: "x", name: "X", summary: "y" }];
    expect(await api.dialects()).toEqual([{ id: "myst", name: "MyST", summary: "s", on: true }, { id: "x", name: "X", summary: "y", on: false }]);
    results.Dialects = null;
    expect(await api.dialects()).toEqual([]);
    await api.setDialect("myst", true);
    await api.dismissDialectHint("myst");
    results.DialectHint = "myst";
    expect(await api.dialectHint()).toBe("myst");
    results.DialectHint = undefined;
    expect(await api.dialectHint()).toBe("");
    expect(calls.filter((c) => c.name !== "Dialects" && c.name !== "DialectHint")).toEqual([
      { name: "SetDialect", args: ["myst", true] }, { name: "DismissDialectHint", args: ["myst"] },
    ]);
    results.SetDialect = new Error("this app is read-only");
    await expect(api.setDialect("myst", true)).rejects.toThrow("read-only");
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
    expect(p).toEqual({ ...page, frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [], layout: { name: "article", source: "" } });
    results.Page = { ...page, layout: { name: "post", source: "folder" } };
    expect((await api.page("a.md")).layout).toEqual({ name: "post", source: "folder" });
    results.Page = { ...page, layout: { name: "x", source: "elsewhere" } };
    expect((await api.page("a.md")).layout).toEqual({ name: "x", source: "" });
    results.Page = { ...page, layout: "slides" }; // an older Go side: a plain name
    expect((await api.page("a.md")).layout).toEqual({ name: "slides", source: "" });
    calls.splice(-3); // the extra Page calls above are not part of the list below

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

  it("menu:about (Zenmark > About Zenmark) is delivered without a payload and unsubscribes", async () => {
    const api = (await import("./wails")).default;
    let n = 0;
    const off = api.on("menu:about", () => { n++; });
    fire("menu:about");
    off();
    fire("menu:about");
    expect(n).toBe(1);
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

  it("lint maps the result and its alerts", async () => {
    const api = (await import("./wails")).default;
    results.Lint = { available: true, alerts: [
      { line: 2, col: 1, endCol: 3, severity: "bogus", check: "Vale.Spelling", message: "m", match: "Teh", description: "Checks spelling.", link: "https://vale.sh/x" },
      { line: 3, col: 1, endCol: 1, severity: "warning", check: "C", message: "n", match: "A", link: "javascript:alert(1)" },
    ] };
    expect(await api.lint("a.md", "")).toEqual({
      available: true,
      alerts: [
        { line: 2, col: 1, endCol: 3, severity: "suggestion", check: "Vale.Spelling", message: "m", match: "Teh", description: "Checks spelling.", link: "https://vale.sh/x" },
        { line: 3, col: 1, endCol: 1, severity: "warning", check: "C", message: "n", match: "A", description: "", link: "" },
      ],
    });
    expect(calls.at(-1)).toEqual({ name: "Lint", args: ["a.md", ""] });
    results.Lint = { available: false, alerts: null };
    expect(await api.lint("a.md", "x")).toEqual({ available: false, alerts: [] });
    results.Lint = null;
    expect(await api.lint("a.md", "x")).toEqual({ available: false, alerts: [] });
  });

  it("pageInfo maps the file details and RevealInFinder is called with the page path", async () => {
    const api = (await import("./wails")).default;
    results.PageInfo = { path: "a.md", absPath: "/f/a.md", size: 12, modTime: 1700000000000, birthTime: 1600000000000 };
    expect(await api.pageInfo("a.md")).toEqual({ path: "a.md", absPath: "/f/a.md", size: 12, modTime: 1700000000000, birthTime: 1600000000000 });
    expect(calls.at(-1)).toEqual({ name: "PageInfo", args: ["a.md"] });
    results.PageInfo = { path: "a.md", absPath: 3, size: "x" };
    expect(await api.pageInfo("a.md")).toEqual({ path: "a.md", absPath: "3", size: 0, modTime: 0, birthTime: 0 });
    results.PageInfo = new Error("a.md: not found");
    await expect(api.pageInfo("a.md")).rejects.toThrow("not found");
    results.RevealInFinder = undefined;
    await api.revealInFinder("a.md");
    expect(calls.at(-1)).toEqual({ name: "RevealInFinder", args: ["a.md"] });
  });

  it("draft is broadcast and other windows' drafts are delivered as page:draft", async () => {
    const api = (await import("./wails")).default;
    const got: unknown[] = [];
    const off = api.on("page:draft", (d) => got.push(d));
    api.draft("a.md", "hello");
    expect(emitted.at(-1)).toEqual({ name: "editor:draft", data: { path: "a.md", content: "hello" } });
    fire("editor:draft", { path: "a.md", content: "x" }, "editor:a.md");
    fire("editor:draft", { path: "a.md", content: null }, "editor:a.md");
    fire("editor:draft", { path: "", content: "x" }, "editor:a.md");
    expect(got).toEqual([{ path: "a.md", content: "x" }, { path: "a.md", content: null }]);
    off();
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

  it("plugins: maps the list defensively and passes the other calls through", async () => {
    const api = (await import("./wails")).default;
    results.Plugins = [
      {
        id: "note", version: "0.1.0", description: "Callouts", source: "folder", status: "undecided", runs: ["./note.sh"], problem: "", kind: "script",
        help: { summary: "Boxes.", snippets: [{ title: "Basic", markdown: ":::note\nhi\n:::" }, null, { title: 5 }] },
        blocks: ["note", 7], inlines: ["badge"], layouts: null,
        options: [{ name: "icon", type: "bool", default: true, description: "Show an icon" }, { name: "x" }, null],
      },
      { id: "x", source: "elsewhere", status: "weird", runs: null, kind: "teapot", help: "nope", options: "nope" },
      { id: "core", source: "builtin", status: "on", runs: [] },
      null,
    ];
    const none = { help: { summary: "", snippets: [] }, blocks: [], inlines: [], layouts: [], options: [] };
    expect(await api.plugins()).toEqual([
      {
        id: "note", version: "0.1.0", description: "Callouts", source: "folder", status: "undecided", runs: ["./note.sh"], problem: "", kind: "script",
        help: { summary: "Boxes.", snippets: [{ title: "Basic", markdown: ":::note\nhi\n:::" }, { title: "5", markdown: "" }] },
        blocks: ["note", "7"], inlines: ["badge"], layouts: [],
        options: [{ name: "icon", type: "bool", default: true, description: "Show an icon" }, { name: "x", type: "", default: null, description: "" }],
      },
      { id: "x", version: "", description: "", source: "folder", status: "invalid", runs: [], problem: "", kind: "program", ...none },
      { id: "core", version: "", description: "", source: "builtin", status: "on", runs: [], problem: "", kind: "builtin", ...none },
    ]);
    results.Plugins = null;
    expect(await api.plugins()).toEqual([]);

    await expect(api.pluginTrust("note", true)).resolves.toBeUndefined();
    results.PluginStorageGet = "{\"n\":1}";
    expect(await api.pluginStorageGet("note", "k")).toBe("{\"n\":1}");
    results.PluginStorageGet = "";
    expect(await api.pluginStorageGet("note", "k")).toBe("null");
    await expect(api.pluginStorageSet("note", "k", "2")).resolves.toBeUndefined();
    results.PluginInvoke = "[1]";
    expect(await api.pluginInvoke("note", "a.md", "{}")).toBe("[1]");
    results.PluginInvoke = new Error("plugin \"note\" has no action hook");
    await expect(api.pluginInvoke("note", "a.md", "{}")).rejects.toThrow("no action hook");

    expect(calls.map((c) => [c.name, ...c.args])).toEqual([
      ["Plugins"], ["Plugins"],
      ["PluginTrust", "note", true],
      ["PluginStorageGet", "note", "k"], ["PluginStorageGet", "note", "k"],
      ["PluginStorageSet", "note", "k", "2"],
      ["PluginInvoke", "note", "a.md", "{}"], ["PluginInvoke", "note", "a.md", "{}"],
    ]);
  });

  it("plugin streams: start, cancel, keys, and the plugin:stream event", async () => {
    const api = (await import("./wails")).default;
    results.PluginStream = "abc123";
    expect(await api.pluginStream("sandbox", "p.md", "{\"a\":1}")).toBe("abc123");
    await expect(api.pluginCancel("abc123")).resolves.toBeUndefined();
    results.PluginStorageKeys = ["a", "b"];
    expect(await api.pluginStorageKeys("quiz", "")).toEqual(["a", "b"]);
    results.PluginStorageKeys = null;
    expect(await api.pluginStorageKeys("quiz", "x")).toEqual([]);
    results.PluginStream = new Error("at most 4 streams run at once");
    await expect(api.pluginStream("sandbox", "p.md", "1")).rejects.toThrow("at most 4");
    expect(calls.map((c) => [c.name, ...c.args])).toEqual([
      ["PluginStream", "sandbox", "p.md", "{\"a\":1}"], ["PluginCancel", "abc123"],
      ["PluginStorageKeys", "quiz", ""], ["PluginStorageKeys", "quiz", "x"], ["PluginStream", "sandbox", "p.md", "1"],
    ]);

    const got: unknown[] = [];
    const off = api.on("plugin:stream", (e) => got.push(e));
    fire("plugin:stream", { call: "abc123", chunk: "line", done: false, error: "" });
    fire("plugin:stream", [{ call: "abc123", chunk: "", done: true, error: "cancelled" }]); // single arguments may arrive wrapped
    fire("plugin:stream", { call: "abc123" });                                                 // missing fields are defaulted
    fire("plugin:stream", { chunk: "no call" });                                               // not a stream event: dropped
    fire("plugin:stream", null);
    expect(got).toEqual([
      { call: "abc123", chunk: "line", done: false, error: "" },
      { call: "abc123", chunk: "", done: true, error: "cancelled" },
      { call: "abc123", chunk: "", done: false, error: "" },
    ]);
    off();
    fire("plugin:stream", { call: "abc123", chunk: "late", done: false, error: "" });
    expect(got).toHaveLength(3);
  });

  it("logs(since) and the log:entry batches (ui/developer-tab)", async () => {
    const api = (await import("./wails")).default;
    results.Logs = [
      { seq: 5, time: 1000, level: "warn", source: "plugin", message: "calc failed" },
      { seq: 6, time: 1001, level: "weird", source: "other" },
      { nope: true },
    ];
    expect(await api.logs!(4)).toEqual([
      { seq: 5, time: 1000, level: "warn", source: "plugin", message: "calc failed" },
      { seq: 6, time: 1001, level: "info", source: "app", message: "" },
    ]);
    results.Logs = null;
    expect(await api.logs!(0)).toEqual([]);
    expect(calls.map((c) => [c.name, ...c.args]).slice(-2)).toEqual([["Logs", 4], ["Logs", 0]]);

    const got: unknown[][] = [];
    const off = api.onLog!((b) => got.push(b.map((e) => e.seq)));
    const entry = (seq: number) => ({ seq, time: 1, level: "info", source: "app", message: "m" });
    fire("log:entry", [entry(1), entry(2)]);   // a batch
    fire("log:entry", [entry(3)]);             // a batch of one
    fire("log:entry", [[entry(4)]]);           // a batch of one, wrapped
    fire("log:entry", []);                     // nothing: no call
    expect(got).toEqual([[1, 2], [3], [4]]);
    off();
    fire("log:entry", [entry(9)]);
    expect(got).toHaveLength(3);
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

  it("plugins:changed arrives without a payload", async () => {
    const api = (await import("./wails")).default;
    let n = 0;
    const off = api.on("plugins:changed", () => { n++; });
    for (const fn of listeners.get("plugins:changed") ?? []) fn({ name: "plugins:changed", data: null });
    off();
    for (const fn of listeners.get("plugins:changed") ?? []) fn({ name: "plugins:changed", data: null });
    expect(n).toBe(1);
  });

  it("layouts: maps the list defensively and passes trust and storage through", async () => {
    const api = (await import("./wails")).default;
    results.Layouts = [
      {
        id: "memo", version: "0.1.0", description: "A memo", source: "folder", status: "undecided", kind: "layout", hasTemplate: true, hasScript: false, problem: "",
        fields: [{ name: "to", type: "string", description: "Who" }, { name: "x" }, null],
        options: [{ name: "toc", type: "bool", default: true, description: "Rail" }, null],
        help: { summary: "S", snippets: [{ title: "A memo", markdown: "---\nlayout: memo\n---" }, null] },
      },
      { id: "x", source: "elsewhere", status: "weird", fields: "nope", options: 3, help: "nope" },
      { id: "article", source: "builtin", status: "on" },
      null,
    ];
    const rest = { kind: "layout", hasTemplate: false, hasScript: false, problem: "", fields: [], options: [], help: { summary: "", snippets: [] } };
    expect(await api.layouts()).toEqual([
      {
        id: "memo", version: "0.1.0", description: "A memo", source: "folder", status: "undecided", kind: "layout", hasTemplate: true, hasScript: false, problem: "",
        fields: [{ name: "to", type: "string", description: "Who" }, { name: "x", type: "", description: "" }],
        options: [{ name: "toc", type: "bool", default: true, description: "Rail" }],
        help: { summary: "S", snippets: [{ title: "A memo", markdown: "---\nlayout: memo\n---" }] },
      },
      { id: "x", version: "", description: "", source: "folder", status: "invalid", ...rest },
      { id: "article", version: "", description: "", source: "builtin", status: "on", ...rest },
    ]);
    results.Layouts = null;
    expect(await api.layouts()).toEqual([]);
    calls.length = 0;
    await expect(api.layoutTrust("memo", true)).resolves.toBeUndefined();
    results.LayoutStorageGet = "{\"n\":1}";
    expect(await api.layoutStorageGet("memo", "k")).toBe("{\"n\":1}");
    results.LayoutStorageGet = "";
    expect(await api.layoutStorageGet("memo", "k")).toBe("null");
    await expect(api.layoutStorageSet("memo", "k", "2")).resolves.toBeUndefined();
    results.LayoutStorageKeys = ["a", "b"];
    expect(await api.layoutStorageKeys("memo", "")).toEqual(["a", "b"]);
    results.LayoutStorageKeys = null;
    expect(await api.layoutStorageKeys("memo", "z")).toEqual([]);
    expect(calls.map((c) => [c.name, ...c.args])).toEqual([
      ["LayoutTrust", "memo", true], ["LayoutStorageGet", "memo", "k"], ["LayoutStorageGet", "memo", "k"],
      ["LayoutStorageSet", "memo", "k", "2"], ["LayoutStorageKeys", "memo", ""], ["LayoutStorageKeys", "memo", "z"],
    ]);
    results.LayoutTrust = new Error("not external");
    await expect(api.layoutTrust("article", true)).rejects.toThrow("not external");
  });

  it("layouts:changed arrives without a payload", async () => {
    const api = (await import("./wails")).default;
    let n = 0;
    const off = api.on("layouts:changed", () => { n++; });
    for (const fn of listeners.get("layouts:changed") ?? []) fn({ name: "layouts:changed", data: null });
    off();
    for (const fn of listeners.get("layouts:changed") ?? []) fn({ name: "layouts:changed", data: null });
    expect(n).toBe(1);
  });

  it("maps the sidebar menu calls and keeps Go's messages", async () => {
    const api = (await import("./wails")).default;
    await api.reorderNav("README.md", ["b.md", "a.md"]);
    await api.setNavHidden("a.md", true);
    await api.revealInFinder("a.md");
    expect(calls).toEqual([
      { name: "ReorderNav", args: ["README.md", ["b.md", "a.md"]] },
      { name: "SetNavHidden", args: ["a.md", true] },
      { name: "RevealInFinder", args: ["a.md"] },
    ]);
    results.ReorderNav = new Error("a.md: permission denied");
    await expect(api.reorderNav("", ["a.md"])).rejects.toThrow("permission denied");
  });
});
