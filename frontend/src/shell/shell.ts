// The window frame: decides between welcome, error and frame views, owns the
// store and the history, and mounts the registered features into the slots.
// An editor window (?window=editor&path=…, ui/editor-window) gets only the
// editor under a small header.
import type { Api, NavNode, Workspace } from "../api";
import { button, el, logo } from "./dom";
import { createHistory } from "./history";
import { createLayout, type Layout } from "./layout";
import { openNewPage, type NewPageDialog } from "./newpage";
import { createSettingsView, type SettingsView } from "./settings";
import { createStore } from "./store";
import type { Ctx, Feature, Store } from "./types";

export type ShellOptions = {
  api: Api;
  features: Feature[];
  /** The window URL's query string; read once. Default: location.search. */
  search?: string;
};

export type WindowKind = { kind: "main" } | { kind: "editor"; path: string };

/** Which window this is: the main window, or the editor window for one page. */
export function windowKind(search: string): WindowKind {
  const q = new URLSearchParams(search);
  const path = q.get("path") ?? "";
  return q.get("window") === "editor" && path ? { kind: "editor", path } : { kind: "main" };
}

/** Editor-window titles for the settings files (see editor CONFIG_PREFIX). */
const CONFIG_TITLES: Record<string, string> = {
  "zen:config/global": "config.yml — settings for all folders",
  "zen:config/workspace": ".zen.yml — settings for this folder",
};

function findTitle(nodes: NavNode[], path: string): string {
  for (const n of nodes) {
    if (n.kind === "page" && n.path === path) return n.title;
    const t = findTitle(n.children, path);
    if (t) return t;
  }
  return "";
}

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

export function createShell(root: HTMLElement, { api, features, search }: ShellOptions): Shell {
  const doc = root.ownerDocument;
  const kind = windowKind(search ?? doc.defaultView?.location.search ?? "");
  const store = createStore();
  const history = createHistory();
  const feature = (id: Feature["id"]) => features.find((f) => f.id === id);

  let layout: Layout | null = null;
  let unmountNav: (() => void) | null = null;
  let unmountMain: (() => void) | null = null;
  let mainId: "page" | null = null;
  let editorTitle: HTMLElement | null = null;
  let unsubscribeStore: (() => void) | null = null;
  let destroyed = false;

  const ctx: Ctx = { api, store, navigate };

  function navigate(path: string, anchor = ""): void {
    closeSettings(); // choosing a page leaves Settings
    history.push({ path, anchor });
    store.set({ path, anchor });
  }
  function back(): void {
    // In Settings, Back returns to the page you were reading; the history is not touched.
    if (settings) return closeSettings();
    const entry = history.back();
    if (entry) store.set(entry);
  }
  /** Back is available while Settings shows, even at the start of the history. */
  function syncHistoryButtons(): void {
    layout?.setHistoryState(history.canBack() || settings !== null, history.canForward());
  }
  function forward(): void {
    closeSettings();
    const entry = history.forward();
    if (entry) store.set(entry);
  }

  /** Mounts the page in `main` once there is a path (editing has its own window). */
  function syncMain(): void {
    if (!layout) return;
    const { path } = store.get();
    const want = path === "" ? null : "page";
    syncHistoryButtons();
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
    settings?.destroy();
    settings = null;
    newPage?.close();
    layout?.destroy();
    layout = null;
    editorTitle = null;
  }

  let newPage: NewPageDialog | null = null;
  /** File › New Page… (⌘N): ask for a name and layout, then open the new page in the editor. */
  function showNewPage(): void {
    if (kind.kind !== "main" || !layout || newPage) return;
    const path = store.get().path;
    const folder = path.includes("/") ? path.slice(0, path.lastIndexOf("/") + 1) : "";
    newPage = openNewPage({
      api, doc, folder,
      onCreated: (created) => {
        navigate(created);
        void api.openEditor(created).catch((err) => console.error("zen: opening the editor failed", err));
      },
      onClose: () => { newPage = null; },
    });
  }

  let settings: SettingsView | null = null;
  /**
   * The cog at the bottom of the sidebar (and ⌘,) shows Settings in the main pane, in the place of
   * the page (which stays mounted); the sidebar stays. Back, Esc, the cog and choosing a page leave it.
   */
  function openSettings(): void {
    if (kind.kind !== "main" || !layout || settings) return;
    settings = createSettingsView({ api, hasWorkspace: () => store.get().workspace !== null });
    layout.showView(settings.el);
    layout.setSettingsOpen(true);
    syncHistoryButtons();
    settings.show();
  }
  function closeSettings(): void {
    if (!settings) return;
    const inside = settings.el.contains(doc.activeElement);
    settings.destroy();
    settings = null;
    layout?.showView(null);
    layout?.setSettingsOpen(false);
    syncHistoryButtons();
    if (inside) layout?.el.querySelector<HTMLElement>(".zen-settings-button")?.focus();
  }
  function toggleSettings(): void {
    if (settings) closeSettings();
    else openSettings();
  }

  function showFrame(workspace: Workspace, page: string): void {
    closeFrame();
    history.reset();
    store.set({ workspace, path: "", anchor: "", mode: "read", page: undefined, dirty: false });
    const path = page || workspace.home;
    if (path) navigate(path);

    layout = createLayout({ onBack: back, onForward: forward, onOpenFolder: () => void openFolder(), onSettings: () => toggleSettings() });
    root.replaceChildren(layout.el);
    doc.title = `${workspace.name} — zen`;
    unmountNav = feature("nav")?.mount(layout.slots, ctx) ?? null;
    syncMain();
    unsubscribeStore = store.subscribe(syncMain);
  }

  function titleEditor(workspace: Workspace, path: string): void {
    const title = CONFIG_TITLES[path] || findTitle(workspace.tree, path) || path.split("/").pop() || path;
    if (editorTitle) editorTitle.textContent = title;
    doc.title = `${title} — Edit — zen`;
  }

  /** The editor window: header (title + editor actions) over the editor. */
  function showEditor(workspace: Workspace, path: string): void {
    closeFrame();
    store.set({ workspace, path, anchor: "", mode: "edit", page: undefined, dirty: false });
    const frame = el("div", "zen-editor-window");
    const header = el("header", "zen-top");
    editorTitle = el("h1", "zen-editor-window-title");
    const actions = el("div", "zen-actions");
    header.append(editorTitle, actions);
    const main = el("main", "zen-main");
    frame.append(header, main);
    root.replaceChildren(frame);
    titleEditor(workspace, path);
    // nav and breadcrumb exist for the Slots type; nothing shows them.
    const slots = { nav: el("nav"), breadcrumb: el("div"), actions, main };
    unmountMain = feature("editor")?.mount(slots, ctx) ?? null;
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
    if (kind.kind === "editor") {
      if (startup.workspace) showEditor(startup.workspace, kind.path);
      else showError(new Error("No folder is open"));
    } else if (startup.workspace) showFrame(startup.workspace, startup.page);
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
    showFrame(workspace, "");
  }

  const onKey = (e: KeyboardEvent) => {
    if (kind.kind !== "main") return; // an editor window has no folder or history
    if (e.key === "Escape" && settings && !newPage && !e.defaultPrevented) {
      e.preventDefault();
      closeSettings();
      return;
    }
    if (!(e.metaKey || e.ctrlKey) || e.altKey || e.shiftKey) return;
    const inFrame = layout !== null;
    if (e.key === "o" || e.key === "O") void openFolder();
    else if ((e.key === "n" || e.key === "N") && inFrame) showNewPage();
    else if (e.key === "," && inFrame) toggleSettings();
    else if (e.key === "\\" && inFrame) layout!.toggleSidebar();
    else if (e.key === "[" && inFrame) back();
    else if (e.key === "]" && inFrame) forward();
    else return;
    e.preventDefault();
  };
  doc.addEventListener("keydown", onKey);

  const offWorkspace = api.on("workspace:changed", (workspace) => {
    if (!store.get().workspace) return;
    store.set({ workspace });
    if (editorTitle) titleEditor(workspace, store.get().path);
  });
  // Go closes the editor windows before it opens another folder.
  const offNewPage = api.on("menu:new-page", showNewPage);
  const offOpened = api.on("workspace:opened", (s) => {
    if (!destroyed && s.workspace && kind.kind === "main") showFrame(s.workspace, s.page);
  });

  return {
    start, store, navigate, back, forward,
    openFolder: () => openFolder(),
    toggleSidebar: () => layout?.toggleSidebar(),
    destroy() {
      destroyed = true;
      doc.removeEventListener("keydown", onKey);
      offWorkspace();
      offNewPage();
      offOpened();
      closeFrame();
      root.replaceChildren();
    },
  };
}
