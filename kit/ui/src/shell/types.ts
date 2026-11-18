import type { KitHost, Page, Workspace } from "../host";

export type AppState = {
  workspace: Workspace | null; path: string; anchor: string;
  /** "edit" only in an editor window (ui/editor-window); the main window always reads. */
  mode: "read" | "edit";
  page?: Page;       // set by ui/page after a load
  dirty?: boolean;   // set by ui/editor
  /** Short labels plugins put after a page's title in the sidebar (ctx.badge), by page path. */
  badges?: Record<string, string>;
};
export type Store = { get(): AppState; set(p: Partial<AppState>): void; subscribe(fn: (s: AppState) => void): () => void };
export type Slots = { nav: HTMLElement; breadcrumb: HTMLElement; actions: HTMLElement; main: HTMLElement };
export type Ctx = {
  api: KitHost; store: Store; navigate(path: string, anchor?: string): void;
  /** The product's name: the suffix of the window title ("Page — Zenmark"). */
  brand: string;
};
export type Feature = { id: "nav" | "page" | "editor"; mount(slots: Slots, ctx: Ctx): () => void }; // returns unmount
