// The Api over the generated Wails bindings (app/services). Loaded by
// getApi() only inside the Wails webview.
import { Events, Window } from "@wailsio/runtime";
import { Zen } from "../../bindings/github.com/zenmark-org/zenmd/apps/zenmark/index.js";
// The headless service is unexported, so the generated index leaves it out.
import * as HeadlessService from "../../bindings/github.com/zenmark-org/zenmd/apps/zenmark/headlessservice.js";
import type { LogEntry } from "@zenmd/kit";
import type { Alert, Api, BuildOptions, Crumb, LayoutField, LayoutInfo, LintResult, NavNode, Page, PluginInfo, PluginOption, PluginSnippet, Startup, Workspace } from "./index";

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

const LAYOUT_SOURCES = ["builtin", "folder", "user"] as const;

/** {name, source}; a plain string (an older Go side) is a name; nothing is the article layout. */
function pageLayout(v: unknown): Page["layout"] {
  if (typeof v === "string") return { name: v || "article", source: "" };
  const l = (v ?? {}) as Record<string, unknown>;
  const source = (LAYOUT_SOURCES as readonly unknown[]).includes(l.source) ? (l.source as Page["layout"]["source"]) : "";
  return { name: typeof l.name === "string" && l.name ? l.name : "article", source };
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
    layout: pageLayout(p?.layout),
  };
}

const SOURCES: PluginInfo["source"][] = ["builtin", "folder", "user"];
const STATUSES: PluginInfo["status"][] = ["on", "off", "blocked", "undecided", "invalid"];
const KINDS: PluginInfo["kind"][] = ["program", "page", "script", "builtin"];

const strings = (v: unknown): string[] => (Array.isArray(v) ? v.map(String) : []);

function snippets(v: unknown): PluginSnippet[] {
  return (Array.isArray(v) ? v : []).filter((x): x is Record<string, any> => Boolean(x) && typeof x === "object")
    .map((x) => ({ title: String(x.title ?? ""), markdown: String(x.markdown ?? "") }));
}

function options(v: unknown): PluginOption[] {
  return (Array.isArray(v) ? v : []).filter((x): x is Record<string, any> => Boolean(x) && typeof x === "object")
    .map((x) => ({ name: String(x.name ?? ""), type: String(x.type ?? ""), default: x.default ?? null, description: String(x.description ?? "") }));
}

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
    kind: KINDS.includes(p.kind) ? p.kind : p.source === "builtin" ? "builtin" : "program",
    help: { summary: String(p.help?.summary ?? ""), snippets: snippets(p.help?.snippets) },
    blocks: strings(p.blocks),
    inlines: strings(p.inlines),
    layouts: strings(p.layouts),
    options: options(p.options),
  };
}

function fields(v: unknown): LayoutField[] {
  return (Array.isArray(v) ? v : []).filter((x): x is Record<string, any> => Boolean(x) && typeof x === "object")
    .map((x) => ({ name: String(x.name ?? ""), type: String(x.type ?? ""), description: String(x.description ?? "") }));
}

function layoutInfo(l: Record<string, any>): LayoutInfo {
  return {
    id: String(l.id ?? ""),
    version: String(l.version ?? ""),
    description: String(l.description ?? ""),
    source: SOURCES.includes(l.source) ? l.source : "folder",
    status: STATUSES.includes(l.status) ? l.status : "invalid",
    kind: "layout",
    hasTemplate: l.hasTemplate === true,
    hasScript: l.hasScript === true,
    problem: String(l.problem ?? ""),
    fields: fields(l.fields),
    options: options(l.options),
    help: { summary: String(l.help?.summary ?? ""), snippets: snippets(l.help?.snippets) },
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
 * This window's name as Go knows it (apps/zenmark/windows.go): "main", or
 * "editor:<path>" for an editor window (shell windowKind reads the same URL).
 */
function ownWindowName(): string {
  const q = new URLSearchParams(typeof location === "undefined" ? "" : location.search);
  const path = q.get("path") ?? "";
  return q.get("window") === "editor" && path ? `editor:${path}` : "main";
}
const ownName = ownWindowName();

/** Single-argument Emit data arrives as is; several arguments as an array. */
const LOG_LEVELS = ["debug", "info", "warn", "error"];
const LOG_SOURCES = ["app", "plugin"];

/** Log entries from the Go side; anything that is not an entry is dropped. */
function logEntries(list: unknown): LogEntry[] {
  if (!Array.isArray(list)) return [];
  return list.filter((e): e is Record<string, any> => Boolean(e) && typeof e === "object" && typeof e.seq === "number").map((e) => ({
    seq: e.seq,
    time: typeof e.time === "number" ? e.time : 0,
    level: LOG_LEVELS.includes(e.level) ? e.level : "info",
    source: LOG_SOURCES.includes(e.source) ? e.source : "app",
    message: String(e.message ?? ""),
  }));
}

function payload(data: unknown): unknown {
  return Array.isArray(data) && data.length === 1 ? data[0] : data;
}

function buildOptions(o: Raw): BuildOptions {
  return { out: String(o?.out ?? ""), portable: o?.portable !== false, drafts: o?.drafts === true, clean: o?.clean === true, print: o?.print !== false };
}

const api: Api = {
  async startup(): Promise<Startup> {
    const s = (await call(Zen.Startup())) as Raw;
    const out: Startup = { workspace: workspace(s?.workspace), page: s?.page ?? "", mode: s?.mode === "edit" ? "edit" : "read" };
    if (typeof s?.error === "string" && s.error) out.error = s.error;
    return out;
  },
  async pickFolder() {
    return workspace((await call(Zen.PickFolder())) as Raw);
  },
  async cancelOpen() {
    await call(Zen.CancelOpen());
  },
  async closeFolder() {
    const r = (await call(Zen.CloseFolder())) as unknown;
    return Array.isArray(r) ? Boolean(r[0]) : Boolean(r);
  },
  async siteTemplates() {
    const list = ((await call(Zen.SiteTemplates())) ?? []) as Raw[];
    return list.map((t) => ({
      id: String(t?.id ?? ""), name: String(t?.name ?? ""), description: String(t?.description ?? ""), version: String(t?.version ?? ""),
      ask: (Array.isArray(t?.ask) ? t.ask : []).map((a: Raw) => ({ key: String(a?.key ?? ""), label: String(a?.label ?? ""), default: String(a?.default ?? "") })),
      builtin: t?.builtin === true, update: String(t?.update ?? ""), error: String(t?.error ?? ""),
    }));
  },
  async pickSiteFolder() {
    return String((await call(Zen.PickSiteFolder())) ?? "");
  },
  async createSite(id, dir, values) {
    const s = (await call(Zen.CreateSite(id, dir, values))) as Raw;
    return { workspace: workspace(s?.workspace), page: s?.page ?? "", mode: s?.mode === "edit" ? "edit" : "read" };
  },
  async revealTemplates() {
    await call(Zen.RevealTemplates());
  },
  async templatesDir() {
    return String((await call(Zen.TemplatesDir())) ?? "");
  },
  async buildOptions() {
    return buildOptions((await call(Zen.BuildOptions())) as Raw);
  },
  async buildWebsite(o) {
    const r = (await call(Zen.BuildWebsite(o))) as Raw;
    return {
      out: String(r?.out ?? ""), pages: Number(r?.pages ?? 0), files: Number(r?.files ?? 0), millis: Number(r?.millis ?? 0), cancelled: r?.cancelled === true,
      warnings: (Array.isArray(r?.warnings) ? r.warnings : []).map((w: Raw) => ({ path: String(w?.path ?? ""), message: String(w?.message ?? "") })),
    };
  },
  async cancelBuild() {
    await call(Zen.CancelBuild());
  },
  async saveBuildDefaults(o, confirm) {
    return String((await call(Zen.SaveBuildDefaults(o, confirm))) ?? "");
  },
  async pickBuildFolder() {
    return String((await call(Zen.PickBuildFolder())) ?? "");
  },
  async openBuiltSite(out) {
    await call(Zen.OpenBuiltSite(out));
  },
  async revealBuiltSite(out) {
    await call(Zen.RevealBuiltSite(out));
  },
  async printPage() {
    await call(Zen.PrintPage());
  },
  async printSite() {
    await call(Zen.PrintSite());
  },
  async printWindow(title) {
    await call(Zen.PrintWindow(title));
  },
  async exportPdf(title) {
    return String((await call(Zen.ExportPDF(title))) ?? "");
  },
  async revealExport() {
    await call(Zen.RevealExport());
  },
  async printReady(late) {
    await call(HeadlessService.PrintReady(late));
  },
  async reorderNav(parent, paths) {
    await call(Zen.ReorderNav(parent, paths));
  },
  async setNavHidden(path, hidden) {
    await call(Zen.SetNavHidden(path, hidden));
  },
  async revealInFinder(path) {
    await call(Zen.RevealInFinder(path));
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
  async pageInfo(path) {
    const r: Raw = ((await call(Zen.PageInfo(path))) as Raw | null) ?? {};
    return { path: String(r.path ?? path), absPath: String(r.absPath ?? ""), size: Number(r.size) || 0, modTime: Number(r.modTime) || 0, birthTime: Number(r.birthTime) || 0 };
  },
  async plugins() {
    const list = (await call(Zen.Plugins())) as Raw[] | null;
    return (list ?? []).filter((p): p is Record<string, any> => Boolean(p)).map(pluginInfo);
  },
  async layouts() {
    const list = (await call(Zen.Layouts())) as Raw[] | null;
    return (list ?? []).filter((l): l is Record<string, any> => Boolean(l)).map(layoutInfo);
  },
  async layoutTrust(id, allow) {
    await call(Zen.LayoutTrust(id, allow));
  },
  async layoutStorageGet(id, key) {
    return (await call(Zen.LayoutStorageGet(id, key))) || "null";
  },
  async layoutStorageSet(id, key, json) {
    await call(Zen.LayoutStorageSet(id, key, json));
  },
  async layoutStorageKeys(id, prefix) {
    const list = (await call(Zen.LayoutStorageKeys(id, prefix))) as string[] | null;
    return (list ?? []).map(String);
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
  async logs(since) {
    return logEntries((await call(Zen.Logs(since))) as Raw[] | null);
  },
  onLog(fn) {
    // The Go side sends a batch (an array); the runtime hands a one-entry batch over unwrapped.
    return Events.On("log:entry", (e: { data: unknown }) => {
      const d = e.data;
      const list = logEntries(Array.isArray(d) && d.length === 1 && Array.isArray(d[0]) ? d[0] : Array.isArray(d) ? d : d ? [d] : []);
      if (list.length) fn(list);
    });
  },
  on(event: "workspace:changed" | "page:changed" | "workspace:opened" | "page:draft" | "plugin:stream" | "plugins:changed" | "layouts:changed" | "menu:new-page" | "menu:settings" | "menu:close-folder" | "workspace:opening" | "menu:new-site" | "menu:build-website" | "build:progress" | "menu:print-page" | "menu:find" | "menu:find-next" | "menu:find-previous", fn: (p: any) => void): () => void {
    if (event === "menu:new-page") return Events.On("menu:new-page", () => fn(undefined));
    if (event === "menu:settings") return Events.On("menu:settings", () => fn(undefined));
    if (event === "menu:close-folder") return Events.On("menu:close-folder", () => fn(undefined));
    if (event === "menu:new-site") return Events.On("menu:new-site", () => fn(undefined));
    if (event === "menu:build-website") return Events.On("menu:build-website", () => fn(undefined));
    if (event === "menu:print-page") return Events.On("menu:print-page", () => fn(undefined));
    if (event === "menu:find" || event === "menu:find-next" || event === "menu:find-previous") return Events.On(event, () => fn(undefined));
    if (event === "build:progress") {
      return Events.On("build:progress", (e: { data: unknown }) => {
        const d = payload(e.data) as { done?: unknown; total?: unknown } | null;
        fn({ done: typeof d?.done === "number" ? d.done : 0, total: typeof d?.total === "number" ? d.total : 0 });
      });
    }
    if (event === "layouts:changed") return Events.On("layouts:changed", () => fn(undefined));
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
    if (event === "workspace:opening") {
      const phases = ["reading", "indexing", "done", "failed"];
      return Events.On("workspace:opening", (e: { data: unknown }) => {
        const d = payload(e.data) as { name?: unknown; phase?: unknown; done?: unknown; total?: unknown } | null;
        if (typeof d?.phase !== "string" || !phases.includes(d.phase)) return;
        fn({
          name: typeof d.name === "string" ? d.name : "",
          phase: d.phase,
          done: typeof d.done === "number" ? d.done : 0,
          total: typeof d.total === "number" ? d.total : 0,
        });
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
