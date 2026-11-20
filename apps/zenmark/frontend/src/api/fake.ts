// In-memory Api for development in a browser and for tests: a three-page
// sample workspace, emit() to simulate Go events, and a record of every call.
import { FEATURE_IDS, type LogEntry } from "@zenmd/kit";
import type { AddonInfo, Api, BuildInfo, BuildProgress, Crumb, DialectInfo, Draft, SiteTemplate, LayoutInfo, LintResult, NavNode, OpenProgress, Page, PluginInfo, PluginStreamEvent, Startup, Workspace } from "./index";

type Method = "startup" | "appReady" | "pickFolder" | "cancelOpen" | "closeFolder"
  | "siteTemplates" | "pickSiteFolder" | "createSite" | "revealTemplates" | "templatesDir"
  | "buildOptions" | "buildWebsite" | "cancelBuild" | "saveBuildDefaults" | "pickBuildFolder" | "openBuiltSite" | "revealBuiltSite"
  | "printPage" | "printSite" | "printWindow" | "exportPdf" | "revealExport" | "printReady" | "printFailed" | "page" | "source" | "preview" | "save" | "openExternal" | "openEditor" | "lint" | "pageInfo" | "revealInFinder" | "openConfig" | "createPage"
  | "plugins" | "pluginTrust" | "pluginStorageGet" | "pluginStorageSet" | "pluginInvoke" | "pluginStream" | "pluginCancel" | "pluginStorageKeys"
  | "reorderNav" | "setNavHidden" | "dialects" | "setDialect" | "dialectHint" | "dismissDialectHint"
  | "logs" | "layouts" | "layoutTrust" | "layoutStorageGet" | "layoutStorageSet" | "layoutStorageKeys";
type Events = { "workspace:changed": Workspace; "page:changed": { path: string }; "workspace:opened": Startup; "page:draft": Draft; "plugin:stream": PluginStreamEvent; "plugins:changed": undefined; "layouts:changed": undefined; "menu:new-page": undefined; "menu:settings": undefined; "menu:close-folder": undefined; "workspace:opening": OpenProgress; "menu:new-site": undefined; "menu:build-website": undefined; "build:progress": BuildProgress; "menu:print-page": undefined; "menu:find": undefined; "menu:find-next": undefined; "menu:find-previous": undefined };

/** A plugin as tests write it: the help and what it adds default to nothing. */
export type PluginSeed = Omit<PluginInfo, "help" | "blocks" | "inlines" | "layouts" | "options"> & Partial<Pick<PluginInfo, "help" | "blocks" | "inlines" | "layouts" | "options">>;

const plugin = (p: PluginSeed): PluginInfo => ({
  ...p,
  runs: [...p.runs],
  help: { summary: p.help?.summary ?? "", snippets: (p.help?.snippets ?? []).map((x) => ({ ...x })) },
  blocks: [...(p.blocks ?? [])], inlines: [...(p.inlines ?? [])], layouts: [...(p.layouts ?? [])],
  options: (p.options ?? []).map((o) => ({ ...o })),
});

/** A layout as tests write it: everything but who and what it is defaults. */
export type LayoutSeed = Pick<LayoutInfo, "id" | "version" | "description" | "source" | "status"> & Partial<Omit<LayoutInfo, "kind">>;

const layoutInfo = (l: LayoutSeed): LayoutInfo => ({
  hasTemplate: false, hasScript: false, problem: "",
  ...l,
  kind: "layout",
  fields: (l.fields ?? []).map((f) => ({ ...f })),
  options: (l.options ?? []).map((o) => ({ ...o })),
  help: { summary: l.help?.summary ?? "", snippets: (l.help?.snippets ?? []).map((x) => ({ ...x })) },
});

export type FakeOptions = {
  /** What startup() resolves to. Default: no workspace (welcome screen). */
  startup?: Startup;
  /** What pickFolder() resolves to; null means cancelled. Default: the sample workspace. */
  pick?: Workspace | null;
  /** Markdown sources by path. Default: the sample pages. */
  sources?: Record<string, string>;
  /** The add-ons of the build (api.addons). Default: the Vale add-on, so lint, Problems and the Linting tab work; [] is a build without add-ons. */
  addons?: AddonInfo[];
  /** The assembly of the build (api.build): what is given is laid over the Zenmark default (everything built in, Vale, the default theme); null is a host that sends none. */
  build?: Partial<BuildInfo> | null;
  /** The markdown dialects of the build (api.dialects()). Default: MyST, off; [] is a build without dialects. */
  dialects?: DialectInfo[];
  /** The dialect id dialectHint() answers: the folder looks written for it. Default: none. */
  dialectHint?: string;
  /** What lint() resolves to. Default: Vale unavailable. */
  lint?: LintResult;
  /** Development only: computes lint() from the content instead (api/index.ts). */
  devLint?: (path: string, content: string) => LintResult;
  /** What plugins() lists. Default: none. The help and what a plugin adds may be left out. */
  plugins?: PluginSeed[];
  /** What layouts() lists. Default: none. */
  layouts?: LayoutSeed[];
  /** What siteTemplates() lists. Default: the three built-in presets. */
  templates?: SiteTemplate[];
};

const SAMPLE_TEMPLATES: SiteTemplate[] = [
  { id: "techdocs", name: "Technical documentation", description: "Tutorials, how-to guides, reference and explanation (Diátaxis)", version: "1.0.0", ask: [{ key: "title", label: "Site title", default: "{{folder}}" }, { key: "author", label: "Author", default: "the team" }], builtin: true, update: "", error: "" },
  { id: "learning", name: "Learning path", description: "A course with steps, a quiz and exercises", version: "1.0.0", ask: [{ key: "title", label: "Site title", default: "{{folder}}" }], builtin: true, update: "", error: "" },
  { id: "planner", name: "Planner", description: "Roadmap, weekly plans, decisions and meeting notes", version: "1.0.0", ask: [{ key: "title", label: "Site title", default: "{{folder}}" }], builtin: true, update: "", error: "" },
];

/** What Zenmark is built from (apps/zenmark/app-assembly.yml). */
export const ZENMARK_BUILD: BuildInfo = {
  app: "zenmark", features: [...FEATURE_IDS], addons: [],
  plugins: ["codeblock", "diagram", "check", "quiz", "sandbox"],
  layouts: ["article", "docs", "post", "landing", "wide", "handout", "slides", "course"],
  templates: ["techdocs", "learning", "planner"],
  dialects: ["myst"],
  theme: "default",
};

/** The MyST dialect as the Go side registers it (builtins/dialects/myst). */
export const MYST_DIALECT: DialectInfo = { id: "myst", name: "MyST", summary: "Directives such as ```{note} and :::{note}, as written for MyST, Jupyter Book and Sphinx.", on: false };

/** The Vale add-on as the Go side registers it (addons/vale). */
export const VALE_ADDON: AddonInfo = { id: "vale", name: "Vale linting", summary: "Spelling and style checks for your pages, by the Vale prose linter." };

type LintAnswer = LintResult | ((path: string, content: string) => LintResult);

export type FakeApi = Api & {
  /** Every call in order. */
  calls: { method: Method; args: unknown[] }[];
  /** Arguments of every call to one method. */
  callsTo(method: Method): unknown[][];
  /** What logs() returns, and a way to push new entries to onLog() listeners (the Developer panel). */
  logList: LogEntry[];
  emitLog(entries: LogEntry[]): void;
  /** Delivers an event to the listeners registered with on(). */
  emit<E extends keyof Events>(event: E, payload: Events[E]): void;
  /** Makes the next call to `method` reject with `error`. */
  failNext(method: Method, error: Error): void;
  /** Result of the next pickFolder(); assignable between calls. */
  pick: Workspace | null;
  /** What exportPdf() resolves to: a path, "" for a cancelled save panel, or a function that answers later. */
  exportPath: string | (() => string | Promise<string>);
  /** What openExternal() resolves to (the user's answer). Default true. */
  confirmExternal: boolean;
  /**
   * Events the real client sends to the other windows: a successful save()
   * broadcasts "editor:saved" (the main window receives it as page:changed).
   */
  broadcasts: ({ event: "editor:saved"; payload: { path: string } } | { event: "editor:draft"; payload: Draft })[];
  /** What lint() resolves to (a copy); assignable between calls. */
  lintResult: LintAnswer;
  /** What plugins() lists (copies); pluginTrust() updates the status. Assignable. */
  pluginList: PluginInfo[];
  /** What layouts() lists (copies); layoutTrust() updates the status. Assignable. */
  layoutList: LayoutInfo[];
  /** What pluginInvoke() answers: a JSON value, or a function of the plugin id and the parsed payload. Default null. */
  invokeResult: unknown | ((id: string, payload: unknown, page: string) => unknown);
  /** Shapes every page() and preview() result, e.g. to add plugin wrappers or a layout. */
  decorate?: (page: Page) => Page;
};

const STORAGE_KEY = /^[A-Za-z0-9._-]{1,64}$/;

const SOURCES: Record<string, string> = {
  "README.md": "# Sample docs\n\nA small workspace served by the fake API.\n\nStart with [Install](guides/install.md), then read [Usage](guides/usage.md).\n",
  "guides/install.md": "# Install\n\nzen is a single application. Open a folder and it turns the markdown inside into a very small site.\n\n```go\nfunc main() {\n\t// say teh hello\n\tfmt.Println(\"hello\", 42)\n}\n```\n\nSee [Usage](usage.md) for the next step.\n",
  "guides/usage.md": "# Usage\n\nPick a page in the sidebar to read it. Press Edit to change it.\n\nBack to the [README](../README.md) or the [zen site](https://example.com).\n",
};

export function sampleWorkspace(): Workspace {
  return {
    root: "/sample-docs",
    name: "sample-docs",
    home: "README.md",
    tree: [
      { title: "Sample docs", path: "README.md", kind: "page", children: [] },
      {
        title: "Guides", path: "guides", kind: "group",
        children: [
          { title: "Install", path: "guides/install.md", kind: "page", children: [] },
          { title: "Usage", path: "guides/usage.md", kind: "page", children: [] },
        ],
      },
    ],
  };
}

/** The sibling list that holds the page at `path`, and the path of the node that owns it ("" at the top). */
function listOf(nodes: NavNode[], path: string, owner = ""): { list: NavNode[]; owner: string } | null {
  if (nodes.some((n) => n.kind === "page" && n.path === path)) return { list: nodes, owner };
  for (const n of nodes) {
    const below = listOf(n.children, path, n.path);
    if (below) return below;
  }
  return null;
}

const copyTree = (nodes: NavNode[]): NavNode[] => nodes.map((n) => ({ ...n, children: copyTree(n.children) }));

const escape = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

function resolve(from: string, href: string): string {
  const parts = from.split("/").slice(0, -1);
  for (const seg of href.split("/")) {
    if (seg === "..") parts.pop();
    else if (seg !== "." && seg !== "") parts.push(seg);
  }
  return parts.join("/");
}

/** Just enough markdown for the samples: `# ` headings, paragraphs and links. */
function renderFake(path: string, source: string, known: (p: string) => boolean): { title: string; html: string } {
  let title = "";
  const blocks = source.split(/\n{2,}/).map((b) => b.trim()).filter(Boolean);
  const html = blocks.map((block) => {
    const h = block.match(/^(#{1,6})\s+(.*)$/);
    if (h) {
      if (!title && h[1].length === 1) title = h[2];
      return `<h${h[1].length}>${escape(h[2])}</h${h[1].length}>`;
    }
    const text = escape(block).replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_m, label: string, href: string) => {
      if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return `<a data-zen-link="external" href="${href}">${label}</a>`;
      const [file, anchor = ""] = href.split("#");
      const target = resolve(path, file);
      if (!known(target)) return `<a data-zen-link="broken">${label}</a>`;
      return `<a data-zen-link="internal" data-path="${target}" data-anchor="${anchor}" href="#">${label}</a>`;
    });
    return `<p>${text}</p>`;
  }).join("\n");
  return { title: title || path.split("/").pop()!.replace(/\.md$/, ""), html };
}

function crumbs(tree: NavNode[], path: string, trail: Crumb[] = []): Crumb[] | null {
  for (const n of tree) {
    const here = [...trail, { title: n.title, path: n.path }];
    if (n.path === path) return here;
    const below = crumbs(n.children, path, here);
    if (below) return below;
  }
  return null;
}

export function createFakeApi(options: FakeOptions = {}): FakeApi {
  const sources: Record<string, string> = { ...(options.sources ?? SOURCES) };
  const startup: Startup = options.startup ?? { workspace: null, page: "", mode: "read" };
  const listeners: { [E in keyof Events]: Set<(p: Events[E]) => void> } = {
    "workspace:changed": new Set(),
    "page:changed": new Set(),
    "workspace:opened": new Set(),
    "page:draft": new Set(),
    "plugin:stream": new Set(),
    "plugins:changed": new Set(),
    "layouts:changed": new Set(),
    "menu:new-page": new Set(),
    "menu:settings": new Set(),
    "menu:close-folder": new Set(),
    "workspace:opening": new Set(),
    "menu:new-site": new Set(),
    "menu:build-website": new Set(),
    "build:progress": new Set(),
    "menu:print-page": new Set(),
    "menu:find": new Set(),
    "menu:find-next": new Set(),
    "menu:find-previous": new Set(),
  };
  const logListeners = new Set<(e: LogEntry[]) => void>();
  const t0 = Date.now() - 60_000;
  let streamCount = 0;
  const failures = new Map<Method, Error>();
  let base: Workspace = startup.workspace ?? sampleWorkspace();
  let tree: NavNode[] = copyTree(base.tree); // reorderNav and setNavHidden change this copy

  /** Sends the tree as the Go side does after a write: a new workspace object, so the sidebar draws it again. */
  const changed = () => {
    api.emit("workspace:changed", { ...base, tree: copyTree(tree) });
  };

  /** Plugin storage: plugin id -> key -> JSON text. */
  const storage = new Map<string, Map<string, string>>();
  /** Layout storage: layout id -> key -> JSON text (apart from the plugins'). */
  const layoutStorage = new Map<string, Map<string, string>>();

  const render = (path: string, content: string): Page => {
    const { title, html } = renderFake(path, content, (p) => p in sources);
    const page: Page = {
      path, title, html, frontmatter: {},
      breadcrumb: crumbs(tree, path) ?? [{ title, path }],
      assets: { css: [], js: [] }, warnings: [], layout: { name: "article", source: "" },
    };
    return api.decorate ? api.decorate(page) : page;
  };
  const need = (path: string): string => {
    if (!(path in sources)) throw new Error(`page not found: ${path}`);
    return sources[path];
  };
  /** Records the call, then runs it asynchronously so rejections are promises. */
  const call = <T>(method: Method, args: unknown[], run: () => T): Promise<T> => {
    api.calls.push({ method, args });
    const failure = failures.get(method);
    if (failure) {
      failures.delete(method);
      return Promise.reject(failure);
    }
    return new Promise((res) => res(run()));
  };

  let dialectList = (options.dialects ?? [MYST_DIALECT]).map((d) => ({ ...d }));
  let hint = options.dialectHint ?? "";
  let lastPage = "";
  const addons = (options.addons ?? [VALE_ADDON]).map((a) => ({ ...a }));
  const api: FakeApi = {
    calls: [],
    pick: options.pick === undefined ? sampleWorkspace() : options.pick,
    confirmExternal: true,
    exportPath: "/Users/me/Documents/sample-docs.pdf",
    broadcasts: [],
    addons,
    build: options.build === null ? undefined : { ...ZENMARK_BUILD, addons: addons.map((a) => ({ ...a })), ...options.build },
    lintResult: options.devLint ?? options.lint ?? { available: false, alerts: [] },
    pluginList: (options.plugins ?? []).map(plugin),
    layoutList: (options.layouts ?? []).map(layoutInfo),
    invokeResult: null,
    logList: [
      { seq: 1, time: t0, level: "info", source: "app", message: "opened folder=docs" },
      { seq: 2, time: t0 + 1200, level: "debug", source: "app", message: "watching 12 pages" },
      { seq: 3, time: t0 + 5400, level: "warn", source: "plugin", message: "plugin calc: exit code 2: division by zero" },
      { seq: 4, time: t0 + 9000, level: "error", source: "plugin", message: "plugin progress: stream failed: timed out after 30s" },
    ],
    emitLog(entries) {
      for (const fn of Array.from(logListeners)) fn(entries);
    },
    callsTo: (method) => api.calls.filter((c) => c.method === method).map((c) => c.args),
    emit(event, payload) {
      for (const fn of Array.from(listeners[event])) fn(payload);
    },
    failNext(method, error) {
      failures.set(method, error);
    },

    startup: () => call("startup", [], () => startup),
    pickFolder: () => call("pickFolder", [], () => {
      if (api.pick) { base = api.pick; tree = copyTree(base.tree); }
      return api.pick;
    }),
    appReady: () => call("appReady", [], () => undefined),
    cancelOpen: () => call("cancelOpen", [], () => undefined),
    closeFolder: () => call("closeFolder", [], () => { startup.workspace = null; startup.page = ""; return true; }),
    siteTemplates: () => call("siteTemplates", [], () => (options.templates ?? SAMPLE_TEMPLATES).map((t) => ({ ...t, ask: t.ask.map((a) => ({ ...a })) }))),
    pickSiteFolder: () => call("pickSiteFolder", [], () => "/Users/me/new-site"),
    createSite: (id, dir, values) => call("createSite", [id, dir, values], () => {
      const ws = sampleWorkspace();
      base = ws;
      tree = copyTree(ws.tree);
      return { workspace: ws, page: ws.home, mode: "read" as const };
    }),
    revealTemplates: () => call("revealTemplates", [], () => undefined),
    templatesDir: () => call("templatesDir", [], () => "/Users/me/Library/Application Support/zen/templates"),
    buildOptions: () => call("buildOptions", [], () => ({ out: "/Users/me/sample-docs/_site", portable: true, drafts: false, clean: false, print: true })),
    buildWebsite: (o) => call("buildWebsite", [o], () => ({ out: o.out, pages: 3, files: 2, warnings: [], millis: 42, cancelled: false })),
    cancelBuild: () => call("cancelBuild", [], () => undefined),
    saveBuildDefaults: (o, confirm) => call("saveBuildDefaults", [o, confirm], () => "Nothing to change"),
    pickBuildFolder: () => call("pickBuildFolder", [], () => ""),
    openBuiltSite: (out) => call("openBuiltSite", [out], () => undefined),
    revealBuiltSite: (out) => call("revealBuiltSite", [out], () => undefined),
    printPage: () => call("printPage", [], () => undefined),
    printSite: () => call("printSite", [], () => undefined),
    printWindow: (title) => call("printWindow", [title], () => { (globalThis as { print?: () => void }).print?.(); }),
    exportPdf: (title) => call("exportPdf", [title], () => (typeof api.exportPath === "function" ? api.exportPath() : api.exportPath)).then((p) => p),
    revealExport: () => call("revealExport", [], () => undefined),
    printReady: (late) => call("printReady", [late], () => undefined),
    printFailed: (message) => call("printFailed", [message], () => undefined),
    dialects: () => call("dialects", [], () => dialectList.map((d) => ({ ...d }))),
    setDialect: (id, on) => call("setDialect", [id, on], () => {
      if (api.build && !api.build.features.includes("editor")) throw new Error("this app is read-only");
      const d = dialectList.find((x) => x.id === id);
      if (!d) throw new Error(`dialect ${id} is not in this build`);
      d.on = on;
      if (on && hint === id) hint = "";
      if (lastPage) api.emit("page:changed", { path: lastPage }); // the Go side renders the open pages again
    }),
    dialectHint: () => call("dialectHint", [], () => {
      if (api.build && !api.build.features.includes("editor")) return "";
      const d = dialectList.find((x) => x.id === hint);
      return d && !d.on ? hint : "";
    }),
    dismissDialectHint: (id) => call("dismissDialectHint", [id], () => {
      if (hint === id) hint = "";
    }),
    page: (path) => call("page", [path], () => { lastPage = path; return render(path, need(path)); }),
    source: (path) => call("source", [path], () => need(path)),
    preview: (path, content) => call("preview", [path, content], () => render(path, content)),
    save: (path, content) => call("save", [path, content], () => {
      need(path);
      sources[path] = content;
      api.broadcasts.push({ event: "editor:saved", payload: { path } });
    }),
    openExternal: (url) => call("openExternal", [url], () => api.confirmExternal),
    openEditor: (path) => call("openEditor", [path], () => undefined),
    openConfig: (which) => call("openConfig", [which], () => undefined),
    createPage: (name, layout) => call("createPage", [name, layout], () => {
      let path = name.trim().replace(/^\/+/, "");
      if (!path || path.endsWith("/")) throw new Error("give the page a name");
      if (!/\.(md|markdown)$/i.test(path)) path += ".md";
      if (path.split("/").some((p) => p === ".." || p.startsWith("."))) throw new Error("the page must be inside the open folder");
      if (path in sources) throw new Error("a page with that name already exists");
      const title = path.split("/").pop()!.replace(/\.[^.]+$/, "").replace(/[-_]/g, " ");
      sources[path] = `---\ntitle: ${title}\n${layout ? `layout: ${layout}\n` : ""}---\n# ${title}\n`;
      return path;
    }),
    reorderNav: (parent, paths) => call("reorderNav", [parent, paths], () => {
      const found = paths.length ? listOf(tree, paths[0]) : null;
      if (!found) throw new Error(`${paths[0] ?? ""}: not a page in the sidebar`);
      if (found.owner !== parent) throw new Error(`${paths[0]}: not under "${parent}"`);
      const pages = found.list.filter((n) => n.kind === "page");
      for (const p of paths) {
        if (!pages.some((n) => n.path === p)) throw new Error(`${p}: not a page next to ${paths[0]}`);
      }
      if (new Set(paths).size !== paths.length || paths.length !== pages.length) throw new Error("give every page of the group, in the new order");
      // Like the engine: the numbered pages come first, folders after them.
      const ordered = paths.map((p) => pages.find((n) => n.path === p)!);
      found.list.splice(0, found.list.length, ...ordered, ...found.list.filter((n) => n.kind !== "page"));
      changed();
    }),
    setNavHidden: (path, hidden) => call("setNavHidden", [path, hidden], () => {
      const found = listOf(tree, path);
      if (!found) throw new Error(`${path}: not a page in the sidebar`);
      if (hidden) found.list.splice(found.list.findIndex((n) => n.path === path), 1);
      changed();
    }),
    revealInFinder: (path) => call("revealInFinder", [path], () => undefined),
    draft(path, content) {
      api.broadcasts.push({ event: "editor:draft", payload: { path, content } });
    },
    lint: (path, content) => call("lint", [path, content], () => {
      const r = typeof api.lintResult === "function" ? api.lintResult(path, content) : api.lintResult;
      return { available: r.available, alerts: r.alerts.map((a) => ({ ...a })) };
    }),
    pageInfo: (path) => call("pageInfo", [path], () => ({
      path, absPath: `${(startup.workspace ?? sampleWorkspace()).root}/${path}`, size: new TextEncoder().encode(need(path)).length,
      modTime: Date.UTC(2026, 10, 18, 9, 30), birthTime: Date.UTC(2026, 10, 1, 8, 0),
    })),
    plugins: () => call("plugins", [], () => api.pluginList.map(plugin)),
    pluginTrust: (id, allow) => call("pluginTrust", [id, allow], () => {
      const p = api.pluginList.find((x) => x.id === id);
      if (!p || p.source === "builtin") throw new Error(`plugin "${id}" is not an external plugin`);
      if (p.status !== "invalid") p.status = allow ? "on" : "blocked";
    }),
    layouts: () => call("layouts", [], () => api.layoutList.map(layoutInfo)),
    layoutTrust: (id, allow) => call("layoutTrust", [id, allow], () => {
      const l = api.layoutList.find((x) => x.id === id);
      if (!l || l.source === "builtin") throw new Error(`layout "${id}" is not an external layout`);
      if (l.status !== "invalid") l.status = allow ? "on" : "blocked";
    }),
    layoutStorageGet: (id, key) => call("layoutStorageGet", [id, key], () => layoutStorage.get(id)?.get(key) ?? "null"),
    layoutStorageSet: (id, key, json) => call("layoutStorageSet", [id, key, json], () => {
      if (!STORAGE_KEY.test(key)) throw new Error(`invalid storage key "${key}"`);
      try {
        JSON.parse(json);
      } catch {
        throw new Error("value is not JSON");
      }
      if (!layoutStorage.has(id)) layoutStorage.set(id, new Map());
      layoutStorage.get(id)!.set(key, json);
    }),
    layoutStorageKeys: (id, prefix) => call("layoutStorageKeys", [id, prefix], () =>
      Array.from(layoutStorage.get(id)?.keys() ?? []).filter((k) => k.startsWith(prefix)).sort()),
    pluginStorageGet: (id, key) => call("pluginStorageGet", [id, key], () => storage.get(id)?.get(key) ?? "null"),
    pluginStorageSet: (id, key, json) => call("pluginStorageSet", [id, key, json], () => {
      if (!STORAGE_KEY.test(key)) throw new Error(`invalid storage key "${key}"`);
      try {
        JSON.parse(json);
      } catch {
        throw new Error("value is not JSON");
      }
      if (!storage.has(id)) storage.set(id, new Map());
      storage.get(id)!.set(key, json);
    }),
    pluginInvoke: (id, page, json) => call("pluginInvoke", [id, page, json], () => {
      const r = typeof api.invokeResult === "function" ? (api.invokeResult as (id: string, payload: unknown, page: string) => unknown)(id, JSON.parse(json), page) : api.invokeResult;
      return JSON.stringify(r ?? null);
    }),
    pluginStorageKeys: (id, prefix) => call("pluginStorageKeys", [id, prefix], () =>
      Array.from(storage.get(id)?.keys() ?? []).filter((k) => k.startsWith(prefix)).sort()),
    pluginStream: (id, page, json) => call("pluginStream", [id, page, json], () => `call-${++streamCount}`),
    pluginCancel: (callId) => call("pluginCancel", [callId], () => undefined),
    logs: (since) => call("logs", [since], () => api.logList.filter((e) => e.seq > since)),
    onLog(fn) {
      logListeners.add(fn);
      return () => void logListeners.delete(fn);
    },
    on(event: keyof Events, fn: (p: never) => void) {
      const set = listeners[event] as Set<(p: never) => void>;
      set.add(fn);
      return () => void set.delete(fn);
    },
  };
  return api;
}
