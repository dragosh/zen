// What the kit needs from an app: the data types of a folder session and the
// host interface the shared UI calls. An app's own Api (apps/zenmd-app/frontend/src/api)
// extends KitHost with its own methods; the kit imports nothing from an app.
import type { PluginStreamEvent, RuntimeHost } from "@zenmd/runtime";

export type NavNode = { title: string; path: string; kind: "page" | "group"; children: NavNode[] };
export type Workspace = { root: string; name: string; home: string; tree: NavNode[] }; // home: "" or "README.md"
export type Crumb = { title: string; path: string };
export type LayoutSource = "builtin" | "folder" | "user";
export type PageLayoutRef = { name: string; source: LayoutSource | "" };
export type Page = {
  path: string; title: string; html: string;
  frontmatter: Record<string, unknown>; breadcrumb: Crumb[];
  assets: { css: string[]; js: string[] }; warnings: string[];
  /** The layout the page is shown with (core/layouts): "article" when it has none; source "" for article. */
  layout: PageLayoutRef;
};
/** workspace:opening — progress while a folder opens; the last one has phase "done" or "failed". */
export type OpenProgress = { name: string; phase: "reading" | "indexing" | "done" | "failed"; done: number; total: number };
/** One Vale finding (ui/lint). line is 1-based; col and endCol are 1-based rune columns, endCol inclusive. */
export type Alert = {
  line: number; col: number; endCol: number;
  severity: "error" | "warning" | "suggestion";
  check: string; message: string; match: string;
  /** Why the check flagged it ("" when Vale gives none) and an https link to read more (or ""). */
  description: string; link: string;
};
/** available is false when Vale is missing or linting is switched off: show nothing. */
export type LintResult = { available: boolean; alerts: Alert[] };
/** An editor's unsaved text shared with other windows; content null = draft gone. */
export type Draft = { path: string; content: string | null };
export type { PluginStreamEvent };

/** The storage, invoke, stream, cancel and openExternal calls (and the plugin events of `on`) are RuntimeHost, the runtime's side of this API. */
export interface KitHost extends RuntimeHost {
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
  /** Opens the print panel for the calling window (window.print() does nothing in the macOS web view). */
  printWindow?(title: string): Promise<void>;
  /** Also delivered when another window saved the page ("editor:saved"). */
  on(event: "page:changed", fn: (e: { path: string }) => void): () => void;
  /** Another window's unsaved text for a page (see draft); content null = draft gone. */
  on(event: "page:draft", fn: (d: Draft) => void): () => void;
  /** The external plugin set or a trust decision changed (or a folder was opened): reload plugin scripts and styles. */
  on(event: "plugins:changed", fn: () => void): () => void;
  /** The layout set or a layout trust decision changed: render the page again (core/layouts). */
  on(event: "layouts:changed", fn: () => void): () => void;
  on(event: "plugin:stream", fn: (e: PluginStreamEvent) => void): () => void;
  /** Zen › Print Page… in the app menu. */
  on(event: "menu:print-page", fn: () => void): () => void;
}
