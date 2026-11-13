// The window frame: decides between welcome, error and frame views, owns the
// store and the history, and mounts the registered features into the slots.
import type { Api, Workspace } from "../api";
import { button, el, logo } from "./dom";
import { createHistory } from "./history";
import { createLayout, type Layout } from "./layout";
import { createStore } from "./store";
import type { Ctx, Feature, Store } from "./types";

export type ShellOptions = { api: Api; features: Feature[] };

export type Shell = {
  /** Calls api.startup() and shows the matching view. Also the Retry action. */
  start(): Promise<void>;
  store: Store;
  navigate(path: string, anchor?: string): void;
  back(): void;
  forward(): void;
  /** Shows the native folder dialog; a cancelled dialog changes nothing. */
  openFolder(): Promise<void>;
  toggleSidebar(): void;
  destroy(): void;
};

const EMPTY_HINT = "Select a page from the sidebar";

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

export function createShell(root: HTMLElement, { api, features }: ShellOptions): Shell {
  const doc = root.ownerDocument;
  const store = createStore();
  const history = createHistory();
  const feature = (id: Feature["id"]) => features.find((f) => f.id === id);

  let layout: Layout | null = null;
  let unmountNav: (() => void) | null = null;
  let unmountMain: (() => void) | null = null;
  let mainId: "page" | "editor" | null = null;
  let unsubscribeStore: (() => void) | null = null;
  let destroyed = false;

  const ctx: Ctx = { api, store, navigate };

  function navigate(path: string, anchor = ""): void {
    history.push({ path, anchor });
    store.set({ path, anchor });
  }
  function back(): void {
    const entry = history.back();
    if (entry) store.set(entry);
  }
  function forward(): void {
    const entry = history.forward();
    if (entry) store.set(entry);
  }

  /** Mounts in `main` what the state asks for: page (read) or editor (edit). */
  function syncMain(): void {
    if (!layout) return;
    const { path, mode } = store.get();
    const want = path === "" ? null : mode === "edit" ? "editor" : "page";
    layout.setHistoryState(history.canBack(), history.canForward());
    if (want === mainId && (want !== null || layout.slots.main.childNodes.length > 0)) return;

    unmountMain?.();
    unmountMain = null;
    mainId = want;
    // Whatever the feature left behind goes with it.
    layout.slots.main.replaceChildren();
    layout.slots.actions.replaceChildren();
    if (want === null) {
      layout.slots.main.append(el("p", "zen-empty", EMPTY_HINT));
      return;
    }
    unmountMain = feature(want)?.mount(layout.slots, ctx) ?? null;
  }

  function closeFrame(): void {
    unsubscribeStore?.();
    unsubscribeStore = null;
    unmountMain?.();
    unmountMain = null;
    mainId = null;
    unmountNav?.();
    unmountNav = null;
    layout?.destroy();
    layout = null;
  }

  function showFrame(workspace: Workspace, page: string, mode: "read" | "edit"): void {
    closeFrame();
    history.reset();
    store.set({ workspace, path: "", anchor: "", mode, page: undefined, dirty: false });
    const path = page || workspace.home;
    if (path) navigate(path);

    layout = createLayout({ onBack: back, onForward: forward, onOpenFolder: () => void openFolder() });
    root.replaceChildren(layout.el);
    doc.title = `${workspace.name} — zen`;
    unmountNav = feature("nav")?.mount(layout.slots, ctx) ?? null;
    syncMain();
    unsubscribeStore = store.subscribe(syncMain);
  }

  function showWelcome(): void {
    closeFrame();
    store.set({ workspace: null, path: "", anchor: "", page: undefined, dirty: false });
    const view = el("main", "zen-welcome");
    const status = el("p", "zen-welcome-status");
    status.setAttribute("role", "status");
    view.append(
      logo("zen-logo"),
      el("h1", "", "zen"),
      button("Open a folder", () => void openFolder(status), { className: "primary" }),
      el("p", "zen-hint", "⌘O"),
      status,
    );
    root.replaceChildren(view);
    doc.title = "zen";
  }

  function showError(err: unknown): void {
    closeFrame();
    const view = el("main", "zen-error");
    view.setAttribute("role", "alert");
    view.append(
      el("h1", "", "zen could not start"),
      el("p", "zen-error-message", message(err)),
      button("Retry", () => void start(), { className: "primary" }),
    );
    root.replaceChildren(view);
  }

  async function start(): Promise<void> {
    let startup;
    try {
      startup = await api.startup();
    } catch (err) {
      if (!destroyed) showError(err);
      return;
    }
    if (destroyed) return;
    if (startup.workspace) showFrame(startup.workspace, startup.page, startup.mode);
    else showWelcome();
  }

  async function openFolder(status?: HTMLElement): Promise<void> {
    let workspace: Workspace | null;
    try {
      workspace = await api.pickFolder();
    } catch (err) {
      console.error("zen: opening a folder failed", err);
      if (status?.isConnected) status.textContent = `Could not open the folder: ${message(err)}`;
      return;
    }
    if (destroyed || !workspace) return; // cancelled
    showFrame(workspace, "", "read");
  }

  const onKey = (e: KeyboardEvent) => {
    if (!(e.metaKey || e.ctrlKey) || e.altKey || e.shiftKey) return;
    const inFrame = layout !== null;
    if (e.key === "o" || e.key === "O") void openFolder();
    else if (e.key === "\\" && inFrame) layout!.toggleSidebar();
    else if (e.key === "[" && inFrame) back();
    else if (e.key === "]" && inFrame) forward();
    else return;
    e.preventDefault();
  };
  doc.addEventListener("keydown", onKey);

  const offWorkspace = api.on("workspace:changed", (workspace) => {
    if (store.get().workspace) store.set({ workspace });
  });
  const offOpened = api.on("workspace:opened", (s) => {
    if (!destroyed && s.workspace) showFrame(s.workspace, s.page, s.mode);
  });

  return {
    start, store, navigate, back, forward,
    openFolder: () => openFolder(),
    toggleSidebar: () => layout?.toggleSidebar(),
    destroy() {
      destroyed = true;
      doc.removeEventListener("keydown", onKey);
      offWorkspace();
      offOpened();
      closeFrame();
      root.replaceChildren();
    },
  };
}
