// The typed client for the Go services. Types and the Api interface mirror
// docs/PLAN.md "Service API" exactly; UI features import only from here.
import type { Alert, Draft, KitHost, LayoutSource, LintResult, OpenProgress, Page, PluginStreamEvent, Workspace } from "@zenmd/kit";
import { createFakeApi, sampleWorkspace, type FakeOptions } from "./fake";

/** One built-in or external plugin as Settings lists it (core/extplugins). */
export type PluginSnippet = { title: string; markdown: string };
/** One option a plugin reads from the config or a page's frontmatter. */
export type PluginOption = { name: string; type: string; default: unknown; description: string };
export type PluginInfo = {
  id: string; version: string; description: string;
  source: "builtin" | "folder" | "user";
  status: "on" | "off" | "blocked" | "undecided" | "invalid";
  /** What it runs, as lines for people: a backend command line, "scripts in the page". */
  runs: string[];
  /** Why an invalid plugin was rejected, "" otherwise. */
  problem: string;
  /** What it is made of: Go built into zen, a program zen runs, scripts and styles only, or one Tengo file zen runs itself. */
  kind: "program" | "page" | "script" | "builtin";
  /** What the plugin says about itself: a summary and snippets to copy into a page (Settings › Plugins). */
  help: { summary: string; snippets: PluginSnippet[] };
  /** What it adds: block, inline and layout names, and its options. */
  blocks: string[]; inlines: string[]; layouts: string[];
  options: PluginOption[];
};
/** A frontmatter field a layout's template reads (core/layouts). */
export type LayoutField = { name: string; type: string; description: string };
/** One built-in or external layout as Settings and New Page list it (core/layouts). */
export type LayoutInfo = {
  id: string; version: string; description: string;
  source: LayoutSource;
  status: "on" | "off" | "blocked" | "undecided" | "invalid";
  kind: "layout";
  /** It has an HTML template / a page script (what it can do decides the trust prompt). */
  hasTemplate: boolean; hasScript: boolean;
  fields: LayoutField[]; options: PluginOption[];
  help: { summary: string; snippets: PluginSnippet[] };
  /** Why an invalid layout was rejected, "" otherwise. */
  problem: string;
};
/** error: the folder given at launch did not open (workspace is then null). */
export type Startup = { workspace: Workspace | null; page: string; mode: "read" | "edit"; error?: string };
/** A site template (core/init): a folder of preset files in the templates folder. */
export type SiteTemplate = {
  id: string; name: string; description: string; version: string;
  ask: { key: string; label: string; default: string }[];
  builtin: boolean; update: string; error: string;
};
/** Build Website (ui/static-build). */
export type BuildOptions = { out: string; portable: boolean; drafts: boolean; clean: boolean; print: boolean };
export type BuildWarning = { path: string; message: string };
export type BuildResult = { out: string; pages: number; files: number; warnings: BuildWarning[]; millis: number; cancelled: boolean };
export type BuildProgress = { done: number; total: number };
/** The layout id File › New Page passes: "" is a plain page. */
export type PageLayout = string;

/** The data types the kit declares (kit/ui/src/host.ts); the app's api speaks them. */
export type { Alert, Crumb, Draft, LayoutSource, LintResult, NavNode, OpenProgress, Page, PageLayoutRef, PluginStreamEvent, Workspace } from "@zenmd/kit";

/** The page, source, preview, save, lint, draft and openEditor calls are KitHost (the kit's side); the storage, invoke, stream, cancel and openExternal calls are RuntimeHost, the runtime's side. */
export interface Api extends KitHost {
  startup(): Promise<Startup>;
  pickFolder(): Promise<Workspace | null>; // native folder dialog; null also when the opening was cancelled
  /** Stops the folder that is opening; the one open before stays. */
  cancelOpen(): Promise<void>;
  // Site templates (core/init)
  siteTemplates(): Promise<SiteTemplate[]>;
  /** The native folder panel with New Folder enabled; "" when cancelled. */
  pickSiteFolder(): Promise<string>;
  /** Creates the site in dir (nothing is overwritten) and opens it. */
  createSite(id: string, dir: string, values: Record<string, string>): Promise<Startup>;
  revealTemplates(): Promise<void>;
  /** The templates folder, "" when there is none. */
  templatesDir(): Promise<string>;
  // Build Website (ui/static-build)
  buildOptions(): Promise<BuildOptions>;
  buildWebsite(o: BuildOptions): Promise<BuildResult>;
  cancelBuild(): Promise<void>;
  /** Without confirm: the preview text, nothing written. With it: writes .zen.yml. */
  saveBuildDefaults(o: BuildOptions, confirm: boolean): Promise<string>;
  pickBuildFolder(): Promise<string>;
  openBuiltSite(out: string): Promise<void>;
  revealBuiltSite(out: string): Promise<void>;
  // Print (ui/print)
  printPage(): Promise<void>;
  printSite(): Promise<void>;
  /** Opens the print panel for the calling window (window.print() does nothing in the macOS web view). */
  printWindow(title: string): Promise<void>;
  page(path: string): Promise<Page>;
  source(path: string): Promise<string>;
  preview(path: string, content: string): Promise<Page>; // render unsaved text
  save(path: string, content: string): Promise<void>;
  /** Opens the page in its own editor window, or focuses the one already open (ui/editor-window). */
  openEditor(path: string): Promise<void>;
  /** Vale alerts for path: the unsaved content, or the file on disk when content is "" (ui/lint). */
  lint(path: string, content: string): Promise<LintResult>;
  /**
   * Shares an editor's unsaved text with the other windows so the reader
   * follows it live; null when the draft is gone (saved, discarded, closed).
   */
  draft(path: string, content: string | null): void;
  /**
   * Opens the global config.yml or the folder's .zen.yml in zen's editor
   * window (as page zen:config/global or zen:config/workspace), creating it
   * from a commented template if missing.
   */
  openConfig(which: "global" | "workspace"): Promise<void>;
  /**
   * File › New Page: creates name (".md" added when missing; folders inside
   * the workspace are created) from the layout's template and resolves to its
   * path. Rejects when the file exists or the name is not allowed.
   */
  createPage(name: string, layout: PageLayout): Promise<string>;
  /** Every built-in and external plugin, for Settings › Plugins (ui/plugins). */
  plugins(): Promise<PluginInfo[]>;
  /** Every built-in and external layout, for Settings › Layouts and File › New Page (core/layouts). */
  layouts(): Promise<LayoutInfo[]>;
  /** Records the user's Allow (true) or Block (false) for an external layout and reloads. */
  layoutTrust(id: string, allow: boolean): Promise<void>;
  /** Records the user's Allow (true) or Block (false) for an external plugin and reloads. */
  pluginTrust(id: string, allow: boolean): Promise<void>;
  on(event: "workspace:changed", fn: (w: Workspace) => void): () => void;
  /** Also delivered when another window saved the page ("editor:saved"). */
  on(event: "page:changed", fn: (e: { path: string }) => void): () => void;
  /** The app opened another folder (Finder "Open With", File > Open Folder). */
  on(event: "workspace:opened", fn: (s: Startup) => void): () => void;
  /** Chunks and the end of plugin streams, of every window (call ids are random; see plugins/runtime.ts). */
  /** The external plugin set or a trust decision changed (or a folder was opened): reload plugin scripts and styles. */
  on(event: "plugins:changed", fn: () => void): () => void;
  /** The layout set or a layout trust decision changed: render the page again (core/layouts). */
  on(event: "layouts:changed", fn: () => void): () => void;
  on(event: "plugin:stream", fn: (e: PluginStreamEvent) => void): () => void;
  /** Another window's unsaved text for a page (see draft); content null = draft gone. */
  on(event: "page:draft", fn: (d: Draft) => void): () => void;
  /** File › New Page… (⌘N) in the app menu: show the New Page dialog. */
  on(event: "menu:new-page", fn: () => void): () => void;
  /** Zen › Settings… (⌘,): show Settings in the main window. */
  on(event: "menu:settings", fn: () => void): () => void;
  on(event: "workspace:opening", fn: (p: OpenProgress) => void): () => void;
  on(event: "menu:new-site", fn: () => void): () => void;
  on(event: "menu:build-website", fn: () => void): () => void;
  on(event: "build:progress", fn: (p: BuildProgress) => void): () => void;
  on(event: "menu:print-page", fn: () => void): () => void;
}

/**
 * What `api/wails.ts` (owned by app/services) may export; the first one found
 * is used: a default export, `wailsApi`, `api`, or a `createWailsApi()` factory.
 */
type WailsModule = { default?: Api; wailsApi?: Api; api?: Api; createWailsApi?: () => Api };

// A glob instead of an import so the build works while wails.ts does not exist,
// and lazy so the generated bindings are only loaded inside the Wails webview.
const wailsModules = import.meta.glob<WailsModule>("./wails.ts");

/** The Wails implementation inside the app, the in-memory fake elsewhere. */
export async function getApi(): Promise<Api> {
  const load = wailsModules["./wails.ts"];
  if (load && typeof window !== "undefined" && "_wails" in window) {
    const m = await load();
    const api = m.default ?? m.wailsApi ?? m.api ?? m.createWailsApi?.();
    if (api) return api;
  }
  return createFakeApi(devOptions());
}

/**
 * Browser development (vite, no Go): an editor window starts in the sample
 * workspace, and a tiny stand-in for Vale flags a few misspellings so the
 * lint marks and console can be seen. Tests build their own fakes.
 */
function devOptions(): FakeOptions {
  const q = new URLSearchParams(typeof location === "undefined" ? "" : location.search);
  const typos: Record<string, string> = { teh: "the", sentnce: "sentence", speling: "spelling", applicaton: "application" };
  const lint = (_path: string, content: string): LintResult => {
    const alerts: Alert[] = [];
    content.split("\n").forEach((line, i) => {
      for (const m of Array.from(line.matchAll(/[A-Za-z]+/g))) {
        const fix = typos[m[0].toLowerCase()];
        if (!fix) continue;
        const col = Array.from(line.slice(0, m.index)).length + 1;
        alerts.push({
          line: i + 1, col, endCol: col + m[0].length - 1, severity: "error", check: "Vale.Spelling",
          message: `Did you really mean '${m[0]}'?`, match: m[0],
          description: `Probably a typo for "${fix}". (Development stand-in for Vale.)`, link: "https://vale.sh/docs/checks/spelling",
        });
      }
      if (/\bvery\b/i.test(line)) {
        const at = line.toLowerCase().indexOf("very");
        alerts.push({ line: i + 1, col: at + 1, endCol: at + 4, severity: "warning", check: "Dev.Weasel", message: "Avoid 'very'.", match: line.slice(at, at + 4), description: "", link: "" });
      }
    });
    return { available: true, alerts };
  };
  const layout = (id: string, source: "builtin" | "folder" | "user", status: LayoutInfo["status"], description: string, more: Partial<LayoutInfo> = {}): LayoutInfo => ({
    id, version: "1.0.0", description, source, status, kind: "layout", hasTemplate: false, hasScript: false, problem: "", fields: [], options: [],
    help: { summary: "", snippets: [] }, ...more,
  });
  const frontmatter = (id: string, extra = "") => `---\nlayout: ${id}\n${extra}---\n# Title\n\nText.`;
  const layouts: LayoutInfo[] = [
    layout("article", "builtin", "on", "The reading view: a single column.", { help: { summary: "The default look: a reading column.", snippets: [{ title: "A plain page", markdown: frontmatter("article") }] } }),
    layout("docs", "builtin", "on", "Documentation: contents rail, Prev / Next.", { hasTemplate: true, help: { summary: "A docs page with an on-page contents rail.", snippets: [{ title: "A docs page", markdown: frontmatter("docs") }] } }),
    layout("post", "builtin", "on", "A blog post with cover, date and author.", {
      hasTemplate: true,
      fields: [{ name: "date", type: "date", description: "Publication date" }, { name: "author", type: "string", description: "" }, { name: "cover", type: "image", description: "Workspace image shown on top" }, { name: "tags", type: "list", description: "" }],
      options: [{ name: "toc", type: "bool", default: true, description: "Show an on-page contents list." }],
      help: { summary: "Cover, title, byline and tags above the text.", snippets: [{ title: "A post", markdown: frontmatter("post", "date: 2026-11-17\nauthor: Ada\n") }] },
    }),
    layout("slides", "builtin", "on", "A presentation: --- starts the next slide.", { hasScript: true, help: { summary: "Turns a page into slides.", snippets: [{ title: "A deck", markdown: frontmatter("slides") }] } }),
    layout("memo", "folder", "undecided", "A memo with to, from and date.", {
      hasTemplate: true, fields: [{ name: "to", type: "string", description: "Recipient" }, { name: "from", type: "string", description: "" }, { name: "date", type: "date", description: "" }],
      help: { summary: "A short memo.", snippets: [{ title: "A memo", markdown: frontmatter("memo", "to: Ann\nfrom: Bo\n") }] },
    }),
    layout("zine", "folder", "on", "Two columns and a big title.", { hasTemplate: true, hasScript: true }),
  ];
  const noHelp = { help: { summary: "", snippets: [] }, blocks: [], inlines: [], layouts: [], options: [] };
  return {
    layouts,
    plugins: [
      {
        id: "codeblock", version: "0.1.0", description: "Code blocks: syntax colours and Copy and Wrap buttons.", source: "builtin", status: "on", runs: [], problem: "", kind: "builtin",
        help: { summary: "Colours fenced code blocks by language and gives each a Copy and a Wrap button.", snippets: [{ title: "A coloured block", markdown: "```go\nfmt.Println(1)\n```" }, { title: "Line numbers", markdown: "---\nplugins:\n  codeblock: { lineNumbers: true }\n---\n# A page" }] },
        blocks: [], inlines: [], layouts: [],
        options: [{ name: "lineNumbers", type: "bool", default: false, description: "Show a line-number gutter." }, { name: "style", type: "string", default: "zen", description: "Highlight style." }],
      },
      {
        id: "diagram", version: "0.1.0", description: "Mermaid diagrams", source: "builtin", status: "on", runs: [], problem: "", kind: "builtin",
        help: { summary: "Draws mermaid diagrams.", snippets: [{ title: "A flowchart", markdown: "```mermaid\ngraph LR\n  A --> B\n```" }] },
        blocks: ["mermaid", "diagram"], inlines: [], layouts: [], options: [{ name: "theme", type: "string", default: "neutral", description: "mermaid theme: neutral, dark or default" }],
      },
      {
        id: "note", version: "0.1.0", description: "Callout boxes", source: "folder", status: "undecided", runs: ["./note.sh"], problem: "", kind: "program",
        help: { summary: "Callout boxes.", snippets: [{ title: "A note", markdown: ":::note{title=\"Remember\"}\nHello.\n:::" }] },
        blocks: ["note", "warning"], inlines: [], layouts: [], options: [],
      },
      { id: "calc", version: "0.1.0", description: "", source: "folder", status: "on", runs: ["bc -l"], problem: "", kind: "program", ...noHelp },
    ],
    lint: { available: true, alerts: [] },
    devLint: lint,
    ...(q.get("window") === "editor" ? { startup: { workspace: sampleWorkspace(), page: q.get("path") ?? "", mode: "edit" as const } } : {}),
  };
}
