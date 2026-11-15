// The typed client for the Go services. Types and the Api interface mirror
// docs/PLAN.md "Service API" exactly; UI features import only from here.
import { createFakeApi, sampleWorkspace, type FakeOptions } from "./fake";

export type NavNode = { title: string; path: string; kind: "page" | "group"; children: NavNode[] };
export type Workspace = { root: string; name: string; home: string; tree: NavNode[] }; // home: "" or "README.md"
export type Crumb = { title: string; path: string };
export type Page = {
  path: string; title: string; html: string;
  frontmatter: Record<string, unknown>; breadcrumb: Crumb[];
  assets: { css: string[]; js: string[] }; warnings: string[];
};
export type Startup = { workspace: Workspace | null; page: string; mode: "read" | "edit" };
/** One Vale finding (ui/lint). line is 1-based; col and endCol are 1-based rune columns, endCol inclusive. */
export type Alert = {
  line: number; col: number; endCol: number;
  severity: "error" | "warning" | "suggestion";
  check: string; message: string; match: string;
  /** Why the check flagged it ("" when Vale gives none) and an https link to read more (or ""). */
  description: string; link: string;
};
/** An editor's unsaved text shared with other windows; content null = draft gone. */
export type Draft = { path: string; content: string | null };
/** available is false when Vale is missing or linting is switched off: show nothing. */
export type LintResult = { available: boolean; alerts: Alert[] };

export interface Api {
  startup(): Promise<Startup>;
  pickFolder(): Promise<Workspace | null>; // native folder dialog
  page(path: string): Promise<Page>;
  source(path: string): Promise<string>;
  preview(path: string, content: string): Promise<Page>; // render unsaved text
  save(path: string, content: string): Promise<void>;
  openExternal(url: string): Promise<boolean>; // Go shows the confirm dialog
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
   * Opens the global config.yml or the folder's .zen.yml in the system text
   * editor, creating it from a commented template if missing.
   */
  openConfig(which: "global" | "workspace"): Promise<void>;
  on(event: "workspace:changed", fn: (w: Workspace) => void): () => void;
  /** Also delivered when another window saved the page ("editor:saved"). */
  on(event: "page:changed", fn: (e: { path: string }) => void): () => void;
  /** The app opened another folder (Finder "Open With", File > Open Folder). */
  on(event: "workspace:opened", fn: (s: Startup) => void): () => void;
  /** Another window's unsaved text for a page (see draft); content null = draft gone. */
  on(event: "page:draft", fn: (d: Draft) => void): () => void;
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
  return {
    lint: { available: true, alerts: [] },
    devLint: lint,
    ...(q.get("window") === "editor" ? { startup: { workspace: sampleWorkspace(), page: q.get("path") ?? "", mode: "edit" as const } } : {}),
  };
}
