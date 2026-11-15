import type { Api, Page, Workspace } from "../api";

export type AppState = {
  workspace: Workspace | null; path: string; anchor: string;
  /** "edit" only in an editor window (ui/editor-window); the main window always reads. */
  mode: "read" | "edit";
  page?: Page;       // set by ui/page after a load
  dirty?: boolean;   // set by ui/editor
};
export type Store = { get(): AppState; set(p: Partial<AppState>): void; subscribe(fn: (s: AppState) => void): () => void };
export type Slots = { nav: HTMLElement; breadcrumb: HTMLElement; actions: HTMLElement; main: HTMLElement };
export type Ctx = { api: Api; store: Store; navigate(path: string, anchor?: string): void };
export type Feature = { id: "nav" | "page" | "editor"; mount(slots: Slots, ctx: Ctx): () => void }; // returns unmount
