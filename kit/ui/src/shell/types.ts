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
  /** Shows the New Page dialog for a new page in `folder` ("" or ending in "/"); the sidebar menu's New Page Here… is left out without it. */
  newPage?(folder: string): void;
};
/** The features an app can have (kit/assembly's catalog, in its order). `reader`, `nav` and `editor` are mounted as features; the others are gated where they act. */
export const FEATURE_IDS = ["reader", "nav", "find", "editor", "inspector", "print", "export", "newsite", "developer"] as const;
export type FeatureId = (typeof FEATURE_IDS)[number];
export type Feature = { id: FeatureId; mount(slots: Slots, ctx: Ctx): () => void }; // returns unmount
