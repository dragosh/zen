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
  const alert = { line: 3, col: 1, endCol: 1, severity: "error" as const, check: "C", message: "m", match: "A" };
  const withAlerts = createFakeApi({ lint: { available: true, alerts: [alert] } });
  const r = await withAlerts.lint("README.md", "");
  expect(r).toEqual({ available: true, alerts: [alert] });
  r.alerts.length = 0; // results are copies
  expect((await withAlerts.lint("README.md", "")).alerts).toHaveLength(1);
  withAlerts.lintResult = (path, content) => ({ available: true, alerts: content ? [{ ...alert, match: content }] : [] });
  expect((await withAlerts.lint("README.md", "zz")).alerts[0].match).toBe("zz");
  expect(withAlerts.callsTo("lint")).toEqual([["README.md", ""], ["README.md", ""], ["README.md", "zz"]]);
});
