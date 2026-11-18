// @vitest-environment happy-dom
import { expect, test, vi } from "vitest";
import { getApi, type Api, type NavNode } from "./index";
import { createFakeApi, sampleWorkspace } from "./fake";

function pagePaths(nodes: NavNode[]): string[] {
  return nodes.flatMap((n) => [...(n.kind === "page" ? [n.path] : []), ...pagePaths(n.children)]);
}

test("fake implements Api with a three-page sample workspace", async () => {
  const api: Api = createFakeApi();
  const ws = sampleWorkspace();
  const paths = pagePaths(ws.tree);
  expect(paths).toHaveLength(3);
  expect(paths).toContain(ws.home);

  for (const path of paths) {
    const page = await api.page(path);
    expect(page.path).toBe(path);
    expect(page.title).not.toBe("");
    expect(page.html).toContain("<h1>");
    expect(page.breadcrumb.at(-1)?.path).toBe(path);
    expect(await api.source(path)).toContain("# ");
  }
});

test("startup and pickFolder are configurable", async () => {
  const empty = createFakeApi();
  expect((await empty.startup()).workspace).toBeNull();
  expect((await empty.pickFolder())?.name).toBe(sampleWorkspace().name);

  const ws = sampleWorkspace();
  const api = createFakeApi({ startup: { workspace: ws, page: "guides/usage.md", mode: "edit" }, pick: null });
  expect(await api.startup()).toEqual({ workspace: ws, page: "guides/usage.md", mode: "edit" });
  expect(await api.pickFolder()).toBeNull();
});

test("unknown page rejects", async () => {
  const api = createFakeApi();
  await expect(api.page("nope.md")).rejects.toThrow("nope.md");
  await expect(api.source("nope.md")).rejects.toThrow("nope.md");
});

test("save changes source and page, preview does not", async () => {
  const api = createFakeApi();
  const preview = await api.preview("README.md", "# Draft\n\nText <b>");
  expect(preview.title).toBe("Draft");
  expect(preview.html).toContain("&lt;b&gt;");
  expect(await api.source("README.md")).not.toContain("Draft");

  await api.save("README.md", "# Saved\n\nBody");
  expect(await api.source("README.md")).toBe("# Saved\n\nBody");
  expect((await api.page("README.md")).title).toBe("Saved");
});

test("records calls", async () => {
  const api = createFakeApi();
  await api.startup();
  await api.page("README.md");
  await api.openExternal("https://example.com");
  expect(api.calls).toEqual([
    { method: "startup", args: [] },
    { method: "page", args: ["README.md"] },
    { method: "openExternal", args: ["https://example.com"] },
  ]);
  expect(api.callsTo("page")).toEqual([["README.md"]]);
});

test("emit delivers events until unsubscribed", () => {
  const api = createFakeApi();
  const onPage = vi.fn();
  const onWs = vi.fn();
  const off = api.on("page:changed", onPage);
  api.on("workspace:changed", onWs);

  api.emit("page:changed", { path: "README.md" });
  expect(onPage).toHaveBeenCalledWith({ path: "README.md" });
  expect(onWs).not.toHaveBeenCalled();

  off();
  api.emit("page:changed", { path: "README.md" });
  expect(onPage).toHaveBeenCalledTimes(1);

  const ws = sampleWorkspace();
  api.emit("workspace:changed", ws);
  expect(onWs).toHaveBeenCalledWith(ws);
});

test("failNext rejects one call", async () => {
  const api = createFakeApi();
  api.failNext("startup", new Error("boom"));
  await expect(api.startup()).rejects.toThrow("boom");
  await expect(api.startup()).resolves.toBeTruthy();
});

test("getApi falls back to the fake outside Wails", async () => {
  const api = await getApi();
  expect((await api.startup()).workspace).toBeNull();
  expect(typeof api.on).toBe("function");
});

test("openEditor records the call", async () => {
  const api = createFakeApi();
  await expect(api.openEditor("guides/usage.md")).resolves.toBeUndefined();
  expect(api.callsTo("openEditor")).toEqual([["guides/usage.md"]]);
});

test("save broadcasts editor:saved to the other windows", async () => {
  const api = createFakeApi();
  await api.save("README.md", "# x");
  expect(api.broadcasts).toEqual([{ event: "editor:saved", payload: { path: "README.md" } }]);
  api.failNext("save", new Error("disk full"));
  await expect(api.save("README.md", "# y")).rejects.toThrow("disk full");
  expect(api.broadcasts).toHaveLength(1);
});

test("lint is configurable and recorded", async () => {
  const api = createFakeApi();
  expect(await api.lint("README.md", "")).toEqual({ available: false, alerts: [] });
  const alert = { line: 3, col: 1, endCol: 1, severity: "error" as const, check: "C", message: "m", match: "A", description: "", link: "" };
  const withAlerts = createFakeApi({ lint: { available: true, alerts: [alert] } });
  const r = await withAlerts.lint("README.md", "");
  expect(r).toEqual({ available: true, alerts: [alert] });
  r.alerts.length = 0; // results are copies
  expect((await withAlerts.lint("README.md", "")).alerts).toHaveLength(1);
  withAlerts.lintResult = (path, content) => ({ available: true, alerts: content ? [{ ...alert, match: content }] : [] });
  expect((await withAlerts.lint("README.md", "zz")).alerts[0].match).toBe("zz");
  expect(withAlerts.callsTo("lint")).toEqual([["README.md", ""], ["README.md", ""], ["README.md", "zz"]]);
});

test("plugins: the list is settable and pluginTrust updates the status", async () => {
  const api = createFakeApi({
    plugins: [
      { id: "highlight", version: "0.1.0", description: "", source: "builtin", status: "on", runs: [], problem: "", kind: "builtin" },
      { id: "note", version: "0.1.0", description: "Callouts", source: "folder", status: "undecided", runs: ["./note.sh"], problem: "", kind: "program" },
    ],
  });
  const list = await api.plugins();
  expect(list.map((p) => p.id)).toEqual(["highlight", "note"]);
  list[1].status = "on"; // a copy: the fake's own list is untouched
  expect((await api.plugins())[1].status).toBe("undecided");

  await api.pluginTrust("note", true);
  expect((await api.plugins())[1].status).toBe("on");
  await api.pluginTrust("note", false);
  expect((await api.plugins())[1].status).toBe("blocked");
  expect(api.callsTo("pluginTrust")).toEqual([["note", true], ["note", false]]);
  await expect(api.pluginTrust("highlight", false)).rejects.toThrow("highlight");
  await expect(api.pluginTrust("nope", true)).rejects.toThrow("nope");

  api.pluginList = [];
  expect(await createFakeApi().plugins()).toEqual([]);
  expect(await api.plugins()).toEqual([]);
});

test("plugin storage is kept in memory per plugin; invoke answers are settable", async () => {
  const api = createFakeApi();
  expect(await api.pluginStorageGet("quiz", "score")).toBe("null");
  await api.pluginStorageSet("quiz", "score", "{\"n\":3}");
  expect(await api.pluginStorageGet("quiz", "score")).toBe("{\"n\":3}");
  expect(await api.pluginStorageGet("other", "score")).toBe("null");
  await expect(api.pluginStorageSet("quiz", "bad key!", "1")).rejects.toThrow("key");
  await expect(api.pluginStorageSet("quiz", "k", "{nope")).rejects.toThrow("JSON");
  expect(api.callsTo("pluginStorageSet")[0]).toEqual(["quiz", "score", "{\"n\":3}"]);

  expect(await api.pluginInvoke("quiz", "a.md", "{\"a\":1}")).toBe("null");
  api.invokeResult = { ok: true };
  expect(await api.pluginInvoke("quiz", "a.md", "1")).toBe("{\"ok\":true}");
  api.invokeResult = (id: string, payload: unknown) => ({ id, echo: payload });
  expect(JSON.parse(await api.pluginInvoke("quiz", "a.md", "{\"a\":1}"))).toEqual({ id: "quiz", echo: { a: 1 } });
  expect(api.callsTo("pluginInvoke")).toHaveLength(3);
  expect(api.callsTo("pluginInvoke")[0]).toEqual(["quiz", "a.md", "{\"a\":1}"]); // the page is recorded
  api.invokeResult = (_id: string, _payload: unknown, page: string) => ({ page });
  expect(JSON.parse(await api.pluginInvoke("quiz", "z.md", "1"))).toEqual({ page: "z.md" });
});

test("pages are article by default; decorate shapes what page() and preview() return", async () => {
  const api = createFakeApi();
  expect((await api.page("README.md")).layout).toEqual({ name: "article", source: "" });
  api.decorate = (p) => ({ ...p, layout: { name: "slides", source: "builtin" }, html: `<div data-zen-layout="slides" data-zen-layout-source="builtin">${p.html}</div>` });
  const p = await api.page("README.md");
  expect(p.layout).toEqual({ name: "slides", source: "builtin" });
  expect(p.html).toContain("data-zen-layout");
  expect((await api.preview("README.md", "# X")).layout.name).toBe("slides");
});

test("plugin streams: call ids, recorded calls, events through on(), cancel", async () => {
  const api = createFakeApi();
  const events: unknown[] = [];
  const off = api.on("plugin:stream", (e) => events.push(e));
  const a = await api.pluginStream("sandbox", "a.md", "{\"cmd\":\"ls\"}");
  const b = await api.pluginStream("sandbox", "b.md", "{}");
  expect(a).not.toBe(b);
  expect(api.callsTo("pluginStream")).toEqual([["sandbox", "a.md", "{\"cmd\":\"ls\"}"], ["sandbox", "b.md", "{}"]]);
  api.emit("plugin:stream", { call: a, chunk: "x", done: false, error: "" });
  expect(events).toEqual([{ call: a, chunk: "x", done: false, error: "" }]);
  await api.pluginCancel(a);
  expect(api.callsTo("pluginCancel")).toEqual([[a]]);
  off();
  api.emit("plugin:stream", { call: a, chunk: "y", done: false, error: "" });
  expect(events).toHaveLength(1);
  api.failNext("pluginStream", new Error("busy"));
  await expect(api.pluginStream("sandbox", "a.md", "1")).rejects.toThrow("busy");
});

test("pluginStorageKeys lists a plugin's keys sorted, filtered by prefix", async () => {
  const api = createFakeApi();
  expect(await api.pluginStorageKeys("quiz", "")).toEqual([]);
  for (const k of ["q.b", "q.a", "z"]) await api.pluginStorageSet("quiz", k, "1");
  await api.pluginStorageSet("other", "q.c", "1");
  expect(await api.pluginStorageKeys("quiz", "q.")).toEqual(["q.a", "q.b"]);
  expect(await api.pluginStorageKeys("quiz", "")).toEqual(["q.a", "q.b", "z"]);
  expect(api.callsTo("pluginStorageKeys")).toHaveLength(3);
});

test("plugins:changed is an event like the others", () => {
  const api = createFakeApi();
  let n = 0;
  const off = api.on("plugins:changed", () => { n++; });
  api.emit("plugins:changed", undefined);
  off();
  api.emit("plugins:changed", undefined);
  expect(n).toBe(1);
});

test("plugins: seeds may leave out the help and what a plugin adds; the list always carries them", async () => {
  const api = createFakeApi({
    plugins: [
      { id: "a", version: "1.0.0", description: "", source: "builtin", status: "on", runs: [], problem: "", kind: "builtin" },
      { id: "b", version: "1.0.0", description: "", source: "folder", status: "on", runs: [], problem: "", kind: "program", help: { summary: "S", snippets: [{ title: "T", markdown: "M" }] }, blocks: ["b"] },
    ],
  });
  const [a, b] = await api.plugins();
  expect(a.help).toEqual({ summary: "", snippets: [] });
  expect([a.blocks, a.inlines, a.layouts, a.options]).toEqual([[], [], [], []]);
  expect(b.help.snippets).toEqual([{ title: "T", markdown: "M" }]);
  expect(b.blocks).toEqual(["b"]);
});

test("layouts: seeds fill in the defaults; layoutTrust updates external ones only", async () => {
  const api = createFakeApi({
    layouts: [
      { id: "article", version: "1.0.0", description: "Reading view", source: "builtin", status: "on" },
      { id: "memo", version: "0.1.0", description: "A memo", source: "folder", status: "undecided", hasTemplate: true, fields: [{ name: "to", type: "string", description: "" }], help: { summary: "S", snippets: [{ title: "T", markdown: "M" }] } },
    ],
  });
  const [a, m] = await api.layouts();
  expect(a).toMatchObject({ kind: "layout", hasTemplate: false, hasScript: false, fields: [], options: [], problem: "", help: { summary: "", snippets: [] } });
  expect(m.fields).toEqual([{ name: "to", type: "string", description: "" }]);
  expect(m.help.snippets).toEqual([{ title: "T", markdown: "M" }]);
  await api.layoutTrust("memo", true);
  expect((await api.layouts())[1].status).toBe("on");
  await api.layoutTrust("memo", false);
  expect((await api.layouts())[1].status).toBe("blocked");
  await expect(api.layoutTrust("article", false)).rejects.toThrow("not an external layout");
  await expect(api.layoutTrust("nope", true)).rejects.toThrow();
  expect(api.callsTo("layoutTrust")).toEqual([["memo", true], ["memo", false], ["article", false], ["nope", true]]);
  api.layoutList[0].description = "changed"; // the list is copied on the way out
  expect((await api.layouts())[0].description).toBe("changed");
});

test("layout storage is its own namespace, apart from plugin storage", async () => {
  const api = createFakeApi();
  expect(await api.layoutStorageGet("post", "k")).toBe("null");
  await api.layoutStorageSet("post", "k", "{\"a\":1}");
  await api.layoutStorageSet("post", "j", "2");
  expect(await api.layoutStorageGet("post", "k")).toBe("{\"a\":1}");
  expect(await api.pluginStorageGet("post", "k")).toBe("null");
  expect(await api.layoutStorageKeys("post", "")).toEqual(["j", "k"]);
  expect(await api.layoutStorageKeys("post", "k")).toEqual(["k"]);
  expect(await api.layoutStorageKeys("other", "")).toEqual([]);
  await expect(api.layoutStorageSet("post", "bad key", "1")).rejects.toThrow("invalid storage key");
  await expect(api.layoutStorageSet("post", "k", "{")).rejects.toThrow("JSON");
});

test("layouts:changed is an event like the others", () => {
  const api = createFakeApi();
  let n = 0;
  const off = api.on("layouts:changed", () => { n++; });
  api.emit("layouts:changed", undefined);
  off();
  api.emit("layouts:changed", undefined);
  expect(n).toBe(1);
});

test("fake logs(since) and onLog feed the Developer panel", async () => {
  const api = createFakeApi();
  expect((await api.logs!(0)).length).toBe(api.logList.length);
  expect((await api.logs!(3)).map((e) => e.seq)).toEqual([4]);
  const got: number[] = [];
  const off = api.onLog!((b) => got.push(...b.map((e) => e.seq)));
  api.emitLog([{ seq: 5, time: 1, level: "info", source: "app", message: "m" }]);
  off();
  api.emitLog([{ seq: 6, time: 1, level: "info", source: "app", message: "m" }]);
  expect(got).toEqual([5]);
});
