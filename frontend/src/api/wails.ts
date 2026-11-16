// The Api over the generated Wails bindings (app/services). Loaded by
// getApi() only inside the Wails webview.
import { Events, Window } from "@wailsio/runtime";
import { Zen } from "../../bindings/github.com/dragosh/zen/internal/app/index.js";
import type { Alert, Api, Crumb, LintResult, NavNode, Page, PluginInfo, Startup, Workspace } from "./index";

type Raw = Record<string, any> | null | undefined;

function nodes(list: Raw[] | null | undefined): NavNode[] {
  return (list ?? []).map((n) => ({
    title: n?.title ?? "",
    path: n?.path ?? "",
    kind: n?.kind === "group" ? "group" : "page",
    children: nodes(n?.children),
  }));
}

function workspace(w: Raw): Workspace | null {
  if (!w) return null;
  return { root: w.root ?? "", name: w.name ?? "", home: w.home ?? "", tree: nodes(w.tree) };
}

function page(p: Raw): Page {
  return {
    path: p?.path ?? "",
    title: p?.title ?? "",
    html: p?.html ?? "",
    frontmatter: p?.frontmatter ?? {},
    breadcrumb: ((p?.breadcrumb ?? []) as Crumb[]).map((c) => ({ title: c.title, path: c.path })),
    assets: { css: p?.assets?.css ?? [], js: p?.assets?.js ?? [] },
    warnings: p?.warnings ?? [],
    layout: typeof p?.layout === "string" ? p.layout : "",
  };
}

const SOURCES: PluginInfo["source"][] = ["builtin", "folder", "user"];
const STATUSES: PluginInfo["status"][] = ["on", "off", "blocked", "undecided", "invalid"];

/** Unknown values fall to the careful side: an external plugin that is not in use. */
function pluginInfo(p: Record<string, any>): PluginInfo {
  return {
    id: String(p.id ?? ""),
    version: String(p.version ?? ""),
    description: String(p.description ?? ""),
    source: SOURCES.includes(p.source) ? p.source : "folder",
    status: STATUSES.includes(p.status) ? p.status : "invalid",
    runs: Array.isArray(p.runs) ? p.runs.map(String) : [],
    problem: String(p.problem ?? ""),
  };
}

const SEVERITIES: Alert["severity"][] = ["error", "warning", "suggestion"];

function lintResult(r: Raw): LintResult {
  const alerts = ((r?.alerts ?? []) as Raw[]).map((a): Alert => ({
    line: Number(a?.line) || 0,
    col: Number(a?.col) || 0,
    endCol: Number(a?.endCol) || 0,
    severity: SEVERITIES.includes(a?.severity) ? a?.severity : "suggestion",
    check: String(a?.check ?? ""),
    message: String(a?.message ?? ""),
    match: String(a?.match ?? ""),
    description: String(a?.description ?? ""),
    link: typeof a?.link === "string" && a.link.startsWith("https://") ? a.link : "",
  }));
  return { available: Boolean(r?.available), alerts };
}

/** Rejections from the runtime become Errors carrying the Go message. */
async function call<T>(p: PromiseLike<T>): Promise<T> {
  try {
    return await p;
  } catch (err) {
    if (err instanceof Error) throw err;
    const msg = typeof err === "string" ? err : (err as { message?: string })?.message ?? String(err);
    throw new Error(msg);
  }
}

/**
 * This window's name as Go knows it (internal/app/windows.go): "main", or
 * "editor:<path>" for an editor window (shell windowKind reads the same URL).
 */
function ownWindowName(): string {
  const q = new URLSearchParams(typeof location === "undefined" ? "" : location.search);
  const path = q.get("path") ?? "";
  return q.get("window") === "editor" && path ? `editor:${path}` : "main";
}
const ownName = ownWindowName();

/** Single-argument Emit data arrives as is; several arguments as an array. */
function payload(data: unknown): unknown {
  return Array.isArray(data) && data.length === 1 ? data[0] : data;
}

const api: Api = {
  async startup(): Promise<Startup> {
    const s = (await call(Zen.Startup())) as Raw;
    return { workspace: workspace(s?.workspace), page: s?.page ?? "", mode: s?.mode === "edit" ? "edit" : "read" };
  },
  async pickFolder() {
    return workspace((await call(Zen.PickFolder())) as Raw);
  },
  async page(path) {
    return page((await call(Zen.Page(path))) as Raw);
  },
  async source(path) {
    return (await call(Zen.Source(path))) ?? "";
  },
  async preview(path, content) {
    return page((await call(Zen.Preview(path, content))) as Raw);
  },
  async save(path, content) {
    await call(Zen.Save(path, content));
    // The main window re-renders even when file watching is unavailable.
    void Events.Emit("editor:saved", { path });
  },
  async openExternal(url) {
    return Boolean(await call(Zen.OpenExternal(url)));
  },
  async openConfig(which) {
    await call(Zen.OpenConfig(which));
  },
  async createPage(name, layout) {
    return String((await call(Zen.CreatePage(name, layout))) ?? "");
  },
  async openEditor(path) {
    await call(Zen.OpenEditor(path));
  },
  draft(path, content) {
    void Events.Emit("editor:draft", { path, content });
  },
  async lint(path, content) {
    return lintResult((await call(Zen.Lint(path, content))) as Raw);
  },
  async plugins() {
    const list = (await call(Zen.Plugins())) as Raw[] | null;
    return (list ?? []).filter((p): p is Record<string, any> => Boolean(p)).map(pluginInfo);
  },
  async pluginTrust(id, allow) {
    await call(Zen.PluginTrust(id, allow));
  },
  async pluginStorageGet(id, key) {
    return (await call(Zen.PluginStorageGet(id, key))) || "null";
  },
  async pluginStorageSet(id, key, json) {
    await call(Zen.PluginStorageSet(id, key, json));
  },
  async pluginInvoke(id, page, json) {
    return (await call(Zen.PluginInvoke(id, page, json))) ?? "";
  },
  async pluginStorageKeys(id, prefix) {
    const list = (await call(Zen.PluginStorageKeys(id, prefix))) as string[] | null;
    return (list ?? []).map(String);
  },
  async pluginStream(id, page, json) {
    return String((await call(Zen.PluginStream(id, page, json))) ?? "");
  },
  async pluginCancel(callId) {
    await call(Zen.PluginCancel(callId));
  },
  on(event: "workspace:changed" | "page:changed" | "workspace:opened" | "page:draft" | "plugin:stream" | "plugins:changed" | "menu:new-page", fn: (p: any) => void): () => void {
    if (event === "menu:new-page") return Events.On("menu:new-page", () => fn(undefined));
    if (event === "plugins:changed") return Events.On("plugins:changed", () => fn(undefined));
    if (event === "plugin:stream") {
      return Events.On("plugin:stream", (e: { data: unknown }) => {
        const d = payload(e.data) as { call?: unknown; chunk?: unknown; done?: unknown; error?: unknown } | null;
        if (typeof d?.call !== "string" || !d.call) return;
        fn({ call: d.call, chunk: typeof d.chunk === "string" ? d.chunk : "", done: d.done === true, error: typeof d.error === "string" ? d.error : "" });
      });
    }
    if (event === "page:draft") {
      return Events.On("editor:draft", (e: { data: unknown; sender?: string }) => {
        if (e.sender === ownName) return;
        const d = payload(e.data) as { path?: unknown; content?: unknown } | null;
        if (typeof d?.path !== "string" || !d.path) return;
        fn({ path: d.path, content: typeof d.content === "string" ? d.content : null });
      });
    }
    if (event === "page:changed") {
      const deliver = (e: { data: unknown }) => fn({ path: (payload(e.data) as { path?: string } | null)?.path ?? "" });
      const offChanged = Events.On("page:changed", deliver);
      const offSaved = Events.On("editor:saved", (e: { data: unknown; sender?: string }) => {
        if (e.sender !== ownName) deliver(e);
      });
      return () => {
        offChanged();
        offSaved();
      };
    }
    return Events.On(event, (e: { data: unknown }) => {
      const d = payload(e.data);
      if (event === "workspace:opened") {
        const s = d as { workspace?: Raw; page?: string; mode?: string } | null;
        const w = s?.workspace ? workspace(s.workspace) : null;
        if (w) fn({ workspace: w, page: s?.page ?? "", mode: s?.mode === "edit" ? "edit" : "read" });
      } else {
        const w = workspace(d as Raw);
        if (w) fn(w);
      }
    });
  },
};

/**
 * Window close handshake (app/services behaviour 11, per window since
 * ui/editor-window). Go emits app:before-close naming the window; the named
 * window dispatches a cancelable "zen:before-close" DOM event (ui/editor
 * prevents it while dirty and calls detail.close() after Save/Discard or
 * detail.cancel() on Cancel). Nobody objecting closes at once; otherwise Go is
 * told the user is being asked. Wails sets each reply's sender to this window.
 */
Events.On("app:before-close", (e: { data: unknown }) => {
  const target = (payload(e.data) as { window?: string } | null)?.window;
  if (target && target !== ownName) return;
  let answered = false;
  const reply = (name: string) => () => {
    if (answered) return;
    answered = true;
    void Events.Emit(name, null);
  };
  const close = reply("app:close");
  const ev = new CustomEvent("zen:before-close", { cancelable: true, detail: { close, cancel: reply("app:close-cancel") } });
  if (window.dispatchEvent(ev)) close();
  else void Events.Emit("app:close-pending", null);
});

/** Done in an editor window: close it through the handshake above. */
window.addEventListener("zen:close-window", () => void Window.Close());

export default api;
