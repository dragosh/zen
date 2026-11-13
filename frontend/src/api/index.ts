// The typed client for the Go services. Types and the Api interface mirror
// docs/PLAN.md "Service API" exactly; UI features import only from here.
import { createFakeApi } from "./fake";

export type NavNode = { title: string; path: string; kind: "page" | "group"; children: NavNode[] };
export type Workspace = { root: string; name: string; home: string; tree: NavNode[] }; // home: "" or "README.md"
export type Crumb = { title: string; path: string };
export type Page = {
  path: string; title: string; html: string;
  frontmatter: Record<string, unknown>; breadcrumb: Crumb[];
  assets: { css: string[]; js: string[] }; warnings: string[];
};
export type Startup = { workspace: Workspace | null; page: string; mode: "read" | "edit" };

export interface Api {
  startup(): Promise<Startup>;
  pickFolder(): Promise<Workspace | null>; // native folder dialog
  page(path: string): Promise<Page>;
  source(path: string): Promise<string>;
  preview(path: string, content: string): Promise<Page>; // render unsaved text
  save(path: string, content: string): Promise<void>;
  openExternal(url: string): Promise<boolean>; // Go shows the confirm dialog
  on(event: "workspace:changed", fn: (w: Workspace) => void): () => void;
  on(event: "page:changed", fn: (e: { path: string }) => void): () => void;
  /** The app opened another folder (Finder "Open With", File > Open Folder). */
  on(event: "workspace:opened", fn: (s: Startup) => void): () => void;
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
  return createFakeApi();
}
