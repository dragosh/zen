// The window frame: decides between welcome, error and frame views, owns the
// store and the history, and mounts the registered features into the slots.
// An editor window (?window=editor&path=…, ui/editor-window) gets only the
// editor under a small header.
import type { Api, NavNode, Workspace } from "../api";
import {
  applyAppTheme, button, closeWindowKey, createHistory, createInspector, INSPECTOR_CONSOLE, createLayout, createOpening, createStore, developerEnabled, el, followDeveloper, hasFeature, mountPreview, reloadKey, shellAction,
  type Ctx, type Feature, type Inspector, type Layout, type Store,
} from "@zenmd/kit";
import { navSideRow } from "@zenmd/kit/shell/layout";
import { applyAccessibility, storedAccessibility } from "@zenmd/kit/shell/a11y";
import { BRAND } from "../brand";
// ?no-inline: a file of the app, not a data: URL (the CSP allows images from 'self' only).
import logoUrl from "../assets/logo.svg?no-inline";
import intro1 from "../assets/intro/intro-1.jpg?no-inline";
import intro2 from "../assets/intro/intro-2.jpg?no-inline";
import intro3 from "../assets/intro/intro-3.jpg?no-inline";
import { registerBuildSite } from "./buildsite";
import { installPagePrint } from "@zenmd/runtime/print/page";
import { openNewPage, type NewPageDialog } from "./newpage";
import { openNewSite, type NewSiteDialog, type SiteApi, type SiteTemplate } from "./newsite";
import { createSettingsView, type SettingsView } from "./settings";

export type ShellOptions = {
  api: Api;
  features: Feature[];
  /** The window URL's query string; read once. Default: location.search. */
  search?: string;
  /** Picks the home screen's background (0 up to, not including, 1); default Math.random. */
  random?: () => number;
  /** The most the window waits for its images and fonts before it asks to be shown; default 1000 ms. */
  assetWaitMs?: number;
};

/** The home screen's backgrounds; one is picked each time it opens. */
const INTRO_IMAGES = [intro1, intro2, intro3];

/** Resolves when the image has loaded or failed; at once when it already has. */
function imageSettled(img: HTMLImageElement): Promise<void> {
  if (img.complete && img.naturalWidth > 0) return Promise.resolve();
  return new Promise((resolve) => {
    img.addEventListener("load", () => resolve(), { once: true });
    img.addEventListener("error", () => resolve(), { once: true });
  });
}

/** ⌘P, no other modifiers. */
const printKey = (e: KeyboardEvent): boolean =>
  (e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "p";

export type WindowKind = { kind: "main" } | { kind: "editor"; path: string } | { kind: "print" };

/** Which window this is: the main window, the editor window for one page, or Print Preview (ui/print). */
export function windowKind(search: string): WindowKind {
  const q = new URLSearchParams(search);
  const path = q.get("path") ?? "";
  if (q.get("window") === "print") return { kind: "print" };
  return q.get("window") === "editor" && path ? { kind: "editor", path } : { kind: "main" };
}

/** Editor-window titles for the settings files (see editor CONFIG_PREFIX). */
const CONFIG_TITLES: Record<string, string> = {
  "zen:config/global": "config.yml — settings for all folders",
  "zen:config/workspace": ".zenmd.yml — settings for this folder",
};

/** What the sidebar header shows: the folder's logo and its title (the folder's name when it has none). */
const identity = (w: Workspace) => ({ logo: w.logo ?? "", title: w.title || w.name });

function findTitle(nodes: NavNode[], path: string): string {
  for (const n of nodes) {
    if (n.kind === "page" && n.path === path) return n.title;
    const t = findTitle(n.children, path);
    if (t) return t;
  }
  return "";
}

/**
 * The page the main window shows, kept for this window's session so that a
 * reload (⌘R) comes back to it instead of the page zen was started with.
 */
const LAST_KEY = "zen.shell.last";
type Last = { root: string; path: string; anchor: string };

function rememberPage(last: Last): void {
  try {
    sessionStorage.setItem(LAST_KEY, JSON.stringify(last));
  } catch {
    // no storage: a reload shows the startup page
  }
}

/** The page to show again after a reload of this folder, if it still exists. */
function lastPage(workspace: Workspace): { path: string; anchor: string } | null {
  try {
    const v = JSON.parse(sessionStorage.getItem(LAST_KEY) ?? "null") as Partial<Last> | null;
    if (!v || v.root !== workspace.root || typeof v.path !== "string" || !findTitle(workspace.tree, v.path)) return null;
    return { path: v.path, anchor: typeof v.anchor === "string" ? v.anchor : "" };
  } catch {
    return null;
  }
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

/** The pages of a folder's tree, groups not counted. */
function countPages(nodes: NavNode[]): number {
  return nodes.reduce((n, node) => n + (node.kind === "page" ? 1 : 0) + countPages(node.children), 0);
}

const EMPTY_HINT = "Select a page from the sidebar";

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

export function createShell(root: HTMLElement, { api, features, search, random = Math.random, assetWaitMs = 1000 }: ShellOptions): Shell {
  const doc = root.ownerDocument;
  applyAccessibility(storedAccessibility(), doc); // the options are on <html> before anything is rendered (ui/accessibility)
  const kind = windowKind(search ?? doc.defaultView?.location.search ?? "");
  const store = createStore();
  const history = createHistory();
  // A feature the build leaves out is not mounted (kit/features, kit/shell-features): not in the main window,
  // and an editor window is refused. The build is known once startup has answered, so this is looked up when mounting.
  const feature = (id: Feature["id"]) => (hasFeature(api, id) ? features.find((f) => f.id === id) : undefined);
  const has = (id: string) => hasFeature(api, id);

  let layout: Layout | null = null;
  let homeSettings = false; // Settings shown on the home screen (no folder open)
  // The Console panel of a build that has `developer` but not `inspector` (the page feature owns the Inspector otherwise).
  let consolePanel: Inspector | null = null;
  let stopFollowConsole: (() => void) | null = null;
  let unmountNav: (() => void) | null = null;
  let unmountMain: (() => void) | null = null;
  let mainId: "reader" | null = null;
  let editorTitle: HTMLElement | null = null;
  let unsubscribeStore: (() => void) | null = null;
  let destroyed = false;

  // Read-only apps have no New Page: the sidebar menu's New Page Here… is left out, and so are the menu event and ⌘N.
  // The build is known only once startup has answered, so newPage is looked up when the menu asks.
  const ctx: Ctx = {
    api, store, navigate, brand: BRAND,
    get newPage() { return hasFeature(api, "editor") ? (folder: string) => showNewPage(folder) : undefined; },
  };

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
    syncHistoryButtons();
    if (!has("reader")) { // a bare shell: the folder is open, and this build shows no pages
      showBare(layout);
      return;
    }
    const { path } = store.get();
    const want = path === "" ? null : "reader";
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

  /** The quiet card of a build without the reader: the folder's name, its page count and a line saying why there is nothing to read. */
  function showBare(l: Layout): void {
    const ws = store.get().workspace;
    if (!ws) return;
    mainId = null;
    const pages = countPages(ws.tree);
    const card = el("div", "zen-welcome zen-bare");
    card.append(el("h1", "", ws.name), el("p", "", `${pages} ${pages === 1 ? "page" : "pages"}`), el("p", "", "This build shows no pages."));
    l.slots.actions.replaceChildren();
    l.slots.main.replaceChildren(card);
  }

  function closeFrame(): void {
    unsubscribeStore?.();
    unsubscribeStore = null;
    unmountMain?.();
    unmountMain = null;
    mainId = null;
    unmountNav?.();
    unmountNav = null;
    stopFollowConsole?.();
    stopFollowConsole = null;
    consolePanel?.destroy();
    consolePanel = null;
    settings?.destroy();
    settings = null;
    homeSettings = false;
    newPage?.close();
    layout?.destroy();
    layout = null;
    editorTitle = null;
  }

  let newPage: NewPageDialog | null = null;
  /** File › New Page… (⌘N): ask for a name and layout, then open the new page in the editor. */
  function showNewPage(inFolder?: string): void {
    if (kind.kind !== "main" || !layout || newPage || !hasFeature(api, "editor")) return;
    const path = store.get().path;
    const folder = inFolder ?? (path.includes("/") ? path.slice(0, path.lastIndexOf("/") + 1) : "");
    newPage = openNewPage({
      api, doc, folder,
      onCreated: (created) => {
        navigate(created);
        void api.openEditor(created).catch((err) => console.error("zen: opening the editor failed", err));
      },
      onClose: () => { newPage = null; },
    });
  }

  /**
   * File › New Site from Template… (⇧⌘N) and the welcome cards. The api methods come from core/init;
   * an api that does not have them (an older one, a test) simply has no templates.
   */
  const siteApi: SiteApi | null = api;
  let newSite: NewSiteDialog | null = null;
  function showNewSite(preselect?: string): void {
    if (kind.kind !== "main" || !siteApi || newSite || !has("newsite")) return;
    newSite = openNewSite({
      api: siteApi, doc, preselect,
      onCreated: (s) => { if (!destroyed && s.workspace) showFrame(s.workspace, s.page); },
      onClose: () => { newSite = null; },
    });
  }

  let settings: SettingsView | null = null;
  /** What Settings asks of the shell: Developer › Open Console (the Inspector's Console tab), a page named by a build warning, unsaved pages. */
  const settingsWiring = {
    openConsole: () => doc.dispatchEvent(new CustomEvent(INSPECTOR_CONSOLE)),
    onOpenPage: (p: string) => navigate(p),
    unsaved: () => Array.from(drafts),
  };
  /**
   * The cog at the bottom of the sidebar (and ⌘,) shows Settings in the main pane, in the place of
   * the page (which stays mounted); the sidebar stays. Back, Esc, the cog and choosing a page leave it.
   */
  function openSettings(): void {
    if (kind.kind !== "main" || settings) return;
    if (!layout) {
      openHomeSettings();
      return;
    }
    settings = createSettingsView({
      api, hasWorkspace: () => store.get().workspace !== null,
      build: api, root: () => store.get().workspace?.root ?? "", configFile: () => store.get().workspace?.configFile ?? "", ...settingsWiring,
      appearanceExtra: has("nav") ? [navSideRow(doc)] : [],
    });
    layout.showView(settings.el);
    layout.setSettingsOpen(true);
    // The Export button is on while Settings shows Export (the page says when its section changes).
    const view = settings;
    // One footer button is on (the footer group in layout.ts): Export while Settings shows Export, the cog otherwise.
    view.el.addEventListener("zen-settings-section", () => layout?.setActive(view.current() === "export" ? "export" : "settings", true));
    syncHistoryButtons();
    settings.show();
  }
  /** Settings with no folder open (⌘, or Zenmark › Settings… on the home screen): a page of its own, Back returns home. */
  function openHomeSettings(): void {
    settings = createSettingsView({
      api, hasWorkspace: () => false,
      build: api, root: () => "", ...settingsWiring,
      // The sidebar's side is a setting of this Mac, chosen here too and applied when a folder opens.
      appearanceExtra: has("nav") ? [navSideRow(doc)] : [],
    });
    homeSettings = true;
    const view = el("main", "zen-home-settings");
    const top = el("div", "zen-home-settings-top");
    top.append(button("Back to the home screen", () => closeSettings(), { className: "ghost", text: "‹ Back", title: "Back (Esc)" }));
    view.append(top, settings.el);
    root.replaceChildren(view);
    settings.show();
  }
  function closeSettings(): void {
    if (!settings) return;
    if (homeSettings) {
      homeSettings = false;
      settings.destroy();
      settings = null;
      showWelcome();
      return;
    }
    const inside = settings.el.contains(doc.activeElement);
    settings.destroy();
    settings = null;
    layout?.showView(null);
    layout?.setSettingsOpen(false);
    layout?.setActive("export", false);
    syncHistoryButtons();
    if (inside) layout?.el.querySelector<HTMLElement>(".zen-settings-button")?.focus();
  }
  /** The Export button of the sidebar (⇧⌘B): Settings › Export. */
  function openExport(): void {
    if (kind.kind !== "main" || !has("export")) return;
    openSettings();
    settings?.open("export");
  }
  function toggleSettings(): void {
    if (settings) closeSettings();
    else openSettings();
  }

  function showFrame(workspace: Workspace, page: string, anchor = ""): void {
    closeFrame();
    history.reset();
    store.set({ workspace, path: "", anchor: "", mode: "read", page: undefined, dirty: false });
    const path = page || workspace.home;
    if (path && has("reader")) navigate(path, page ? anchor : "");

    layout = createLayout({
      onBack: back, onForward: forward, onOpenFolder: () => void openFolder(), onSettings: () => toggleSettings(),
      onClose: () => void closeFolder(), onExport: () => openExport(),
      // Another footer button was turned on (the Inspector): Settings closes, so the two never overlap.
      onCloseView: (v) => { if (v === "settings") closeSettings(); },
    });
    layout.setIdentity(identity(workspace));
    root.replaceChildren(layout.el);
    // What the build leaves out leaves no trace: no sidebar without nav, no Inspector button without the inspector feature.
    if (!has("nav")) {
      layout.toggleSidebar();
      layout.el.querySelector('[data-action="toggle-sidebar"]')?.remove();
    }
    if (!has("inspector")) layout.el.querySelector(".zen-inspector-button")?.remove();
    if (!has("export")) layout.el.querySelector(".zen-export-button")?.remove();
    // The Console is a tab of the Inspector, which the page feature owns. A build with `developer` but not `inspector`
    // has a panel of its own with the Console tab only (⌥⌘J, no button), there while the setting is on (ui/inspector).
    if (has("developer") && !has("inspector")) {
      const l = layout;
      const syncConsole = () => {
        const want = developerEnabled();
        if (want && !consolePanel) {
          consolePanel = createInspector({ doc, host: api, external: true, onSelectIssue: () => {} });
          l.setBottomPanel(consolePanel.el);
        } else if (!want && consolePanel) {
          consolePanel.destroy();
          consolePanel = null;
          l.setBottomPanel(null);
        }
      };
      stopFollowConsole = followDeveloper(syncConsole, doc.defaultView ?? window);
      syncConsole();
    }
    doc.title = `${workspace.name} — ${BRAND}`;
    unmountNav = feature("nav")?.mount(layout.slots, ctx) ?? null;
    syncMain();
    unsubscribeStore = store.subscribe((st) => {
      syncMain();
      if (st.workspace && st.path) rememberPage({ root: st.workspace.root, path: st.path, anchor: st.anchor });
    });
  }

  function titleEditor(workspace: Workspace, path: string): void {
    const title = CONFIG_TITLES[path] || findTitle(workspace.tree, path) || path.split("/").pop() || path;
    if (editorTitle) editorTitle.textContent = title;
    doc.title = `${title} — Edit — ${BRAND}`;
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

  /** What the home screen loads (logo, background); the window is shown when these have loaded or failed. */
  let welcomeAssets: HTMLImageElement[] = [];

  function showWelcome(error = ""): void {
    closeFrame();
    store.set({ workspace: null, path: "", anchor: "", page: undefined, dirty: false });
    // The fade (shell.css) plays only once the window is on screen; it would finish unseen in the hidden window.
    // The fade is the app starting: once the window is on screen, a later home screen (Close Folder) shows at once.
    const view = el("main", windowShown ? "zen-welcome zen-welcome-still" : "zen-welcome");
    // One of the backgrounds, behind a scrim (shell.css); it is not in the page view.
    const pick = Math.min(INTRO_IMAGES.length - 1, Math.max(0, Math.floor(random() * INTRO_IMAGES.length)));
    const bg = el("div", "zen-welcome-bg");
    bg.setAttribute("aria-hidden", "true");
    bg.style.backgroundImage = `url("${INTRO_IMAGES[pick]}")`;
    const bgImage = new Image();
    bgImage.src = INTRO_IMAGES[pick];
    welcomeAssets = [bgImage];
    // Points up to the menu bar (top-left of the screen on macOS).
    const menuHint = el("p", "zen-welcome-menuhint zen-welcome-later");
    menuHint.append(
      el("span", "", `↖ File menu: Open Folder… · ${has("newsite") ? "New Site… (⇧⌘N) · " : ""}`),
      button("Settings", () => openSettings(), { className: "link zen-welcome-settings", text: "Settings ⌘," }),
    );
    const status = el("p", "zen-welcome-status");
    status.setAttribute("role", "status");
    const mark = el("img", "zen-welcome-logo zen-welcome-first");
    welcomeAssets.push(mark);
    mark.src = logoUrl;
    mark.alt = "";
    mark.setAttribute("aria-hidden", "true");
    mark.draggable = false;
    const open = el("div", "zen-welcome-open zen-welcome-later");
    open.append(
      button("Open a folder", () => void openFolder(status), { className: "primary" }),
      el("p", "zen-hint", "⌘O"),
    );
    view.append(
      bg,
      menuHint,
      mark,
      el("h1", "zen-welcome-later", BRAND),
      el("p", "zen-welcome-tagline zen-welcome-later", "Markdown folders as sites, documents and courses"),
      open,
      status,
    );
    if (error) status.textContent = `Could not open the folder: ${error}`;
    root.replaceChildren(view);
    doc.title = BRAND;
    void showTemplates(view);
  }

  /** The most cards the welcome screen shows; Show all… has the rest. */
  const WELCOME_CARDS = 6;

  /** "Start from a template": a card for each valid template, below Open a folder. */
  async function showTemplates(view: HTMLElement): Promise<void> {
    if (!siteApi || !has("newsite")) return;
    let all: SiteTemplate[];
    try {
      all = (await siteApi.siteTemplates()).filter((t) => !t.error);
    } catch (err) {
      console.error("zen: listing the site templates failed", err);
      return;
    }
    if (destroyed || !view.isConnected || all.length === 0) return;
    const section = el("section", "zen-welcome-templates zen-welcome-later");
    section.setAttribute("aria-labelledby", "zen-welcome-templates-title");
    const heading = el("h2", "", "Start from a template");
    heading.id = "zen-welcome-templates-title";
    const grid = el("div", "zen-welcome-cards");
    for (const t of all.slice(0, WELCOME_CARDS)) {
      const card = el("button", "zen-welcome-card");
      card.type = "button";
      card.dataset.template = t.id;
      card.append(el("span", "zen-welcome-card-name", t.name), el("span", "zen-welcome-card-hint", t.description));
      card.addEventListener("click", () => showNewSite(t.id));
      grid.append(card);
    }
    section.append(heading, grid, button("Show all…", () => showNewSite()));
    view.append(section);
  }

  function showError(err: unknown): void {
    closeFrame();
    const view = el("main", "zen-error");
    view.setAttribute("role", "alert");
    view.append(
      el("h1", "", `${BRAND} could not start`),
      el("p", "zen-error-message", message(err)),
      button("Retry", () => void start(), { className: "primary" }),
    );
    root.replaceChildren(view);
  }

  /**
   * Tells Go the first screen is drawn so it shows the hidden main window (no flash on launch). It waits for
   * the fonts and the home screen's images, to load or fail, for a second at most; Go shows the window after
   * two seconds anyway. Sent once; a failed start sends it too, so the error screen is seen.
   */
  let readySent = false;
  /** The main window is on screen (appReady resolved); until then the welcome fade waits. */
  let windowShown = kind.kind !== "main";
  async function sendReady(): Promise<void> {
    if (readySent || kind.kind !== "main") return;
    readySent = true;
    const loading: Promise<unknown>[] = welcomeAssets.map(imageSettled);
    const fonts = (doc as Document & { fonts?: { ready: Promise<unknown> } }).fonts;
    if (fonts) loading.push(fonts.ready.catch(() => undefined));
    let timer: ReturnType<typeof setTimeout> | undefined;
    await Promise.race([Promise.all(loading), new Promise((r) => { timer = setTimeout(r, assetWaitMs); })]);
    clearTimeout(timer);
    if (destroyed) return;
    try {
      await api.appReady();
    } catch (err) {
      console.error("zen: could not tell the window it is ready", err);
    }
    if (destroyed) return;
    windowShown = true;
    root.querySelector(".zen-welcome")?.classList.add("zen-welcome-play");
  }

  let stopPreview: (() => void) | null = null;
  async function start(): Promise<void> {
    try {
      await startScreen();
    } finally {
      void sendReady();
    }
  }
  async function startScreen(): Promise<void> {
    if (kind.kind === "print") {
      // Print Preview (ui/print) needs no folder view: it fetches the whole site as one document.
      stopPreview?.();
      stopPreview = mountPreview(root, api, doc);
      return;
    }
    let startup;
    try {
      startup = await api.startup();
    } catch (err) {
      if (!destroyed) showError(err);
      return;
    }
    if (destroyed) return;
    applyAppTheme(doc, api.build);
    // ⌘P and the print menu event, only in a build with print (kit/shell-features).
    if (has("print") && !offPagePrint) {
      // Print Page waits for a folder: File › Print Page… is disabled without one, and so is this.
      const printApi = {
        on: (ev: "menu:print-page", fn: () => void) => api.on(ev, () => { if (kind.kind !== "main" || store.get().workspace) fn(); }),
        printWindow: api.printWindow?.bind(api),
      };
      offPagePrint = installPagePrint(doc, printApi);
    }
    if (kind.kind === "editor") {
      if (!hasFeature(api, "editor")) showError(new Error("This app is read-only"));
      else if (startup.workspace) showEditor(startup.workspace, kind.path);
      else showError(new Error("No folder is open"));
    } else if (startup.workspace) {
      const last = has("reader") ? lastPage(startup.workspace) : null;
      if (last) showFrame(startup.workspace, last.path, last.anchor);
      else showFrame(startup.workspace, startup.page);
    }
    else showWelcome(startup.error ?? "");
  }

  async function openFolder(status?: HTMLElement): Promise<void> {
    let workspace: Workspace | null;
    try {
      workspace = await api.pickFolder();
    } catch (err) {
      console.error("zen: opening a folder failed", err);
      opening.hide();
      if (status?.isConnected) status.textContent = `Could not open the folder: ${message(err)}`;
      else if (!destroyed) opening.fail(message(err));
      return;
    }
    if (destroyed || !workspace) return; // cancelled
    showFrame(workspace, "");
  }

  const onKey = (e: KeyboardEvent) => {
    // ⌘W closes this window, main or editor, through the close handshake (an
    // editor with unsaved changes asks first). ⌘R does nothing in production.
    if (closeWindowKey(e)) {
      e.preventDefault();
      window.dispatchEvent(new CustomEvent("zen:close-window"));
      return;
    }
    if (reloadKey(e) && import.meta.env.PROD) {
      e.preventDefault();
      return;
    }
    if (kind.kind !== "main") return; // an editor window has no folder or history
    // With no folder open there is nothing to print: ⌘P would print the home screen.
    if (!store.get().workspace && !homeSettings && printKey(e)) {
      e.preventDefault();
      e.stopImmediatePropagation();
      return;
    }
    if (e.key === "Escape" && settings && !newPage && !e.defaultPrevented) {
      e.preventDefault();
      closeSettings();
      return;
    }
    // The shortcuts of features the build leaves out do nothing (keys.ts).
    const action = shellAction(e, has);
    if (action === "new-site") {
      e.preventDefault();
      showNewSite();
      return;
    }
    if (!action) return;
    const inFrame = layout !== null;
    if (action === "open-folder") void openFolder();
    else if (action === "new-page" && inFrame) showNewPage();
    else if (action === "settings") toggleSettings();
    else if (action === "toggle-sidebar" && inFrame) layout!.toggleSidebar();
    else if (action === "back" && inFrame) back();
    else if (action === "forward" && inFrame) forward();
    else return;
    e.preventDefault();
  };
  doc.addEventListener("keydown", onKey);

  const offWorkspace = api.on("workspace:changed", (workspace) => {
    if (!store.get().workspace) return;
    store.set({ workspace });
    layout?.setIdentity(identity(workspace));
    if (editorTitle) titleEditor(workspace, store.get().path);
  });
  // Go closes the editor windows before it opens another folder.
  const offNewPage = api.on("menu:new-page", () => showNewPage());
  // File › New Site from Template… works from the welcome screen and from an open folder.
  const offNewSite = siteApi ? siteApi.on("menu:new-site", () => showNewSite()) : () => {};
  // Zen › Settings… (⌘,) from any window: Go shows the main window first.
  const offSettings = api.on("menu:settings", openSettings);
  // File › Close Folder (⇧⌘W): Go closes the editor windows first (each may keep its changes), then the folder.
  let closing = false;
  async function closeFolder(): Promise<void> {
    if (kind.kind !== "main" || !layout || closing) return;
    closing = true;
    try {
      if (!(await api.closeFolder()) || destroyed) return;
      showWelcome(); // closes the frame and empties the store
    } catch (err) {
      console.error("zen: closing the folder failed", err);
    } finally {
      closing = false;
    }
  }
  const offCloseFolder = api.on("menu:close-folder", () => void closeFolder());
  // File › Build Website… (⇧⌘B) and Settings › Website (ui/static-build). Pages with unsaved text
  // in an editor window are named in the dialog: the website uses the saved files.
  const drafts = new Set<string>();
  const offDrafts = api.on("page:draft", (d) => { if (d.content === null) drafts.delete(d.path); else drafts.add(d.path); });
  const buildSite = registerBuildSite({
    api, doc,
    // File › Export › Website… opens Settings › Export › Website (no modal).
    showExport: () => {
      if (kind.kind !== "main") return;
      openSettings();
      settings?.open("export", "website");
    },
    root: () => (kind.kind === "main" && has("export") ? store.get().workspace?.root ?? "" : ""),
    name: () => store.get().workspace?.name ?? "",
    onOpenPage: (p) => navigate(p),
    unsaved: () => Array.from(drafts),
  });
  // Print Page (⌘P, File › Print Page…) in the main and editor windows (ui/print): installed by start(), once the build is known.
  let offPagePrint: (() => void) | null = null;

  // While a folder opens (the Open button, the menu, Open With, the launch folder): a loader with Cancel.
  const opening = createOpening(doc, { onCancel: () => void api.cancelOpen().catch((err) => console.error("zen: cancel failed", err)) });
  const offOpening = kind.kind === "main" ? api.on("workspace:opening", (p) => opening.update(p)) : () => {};
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
      offNewSite();
      newSite?.close();
      offSettings();
      offCloseFolder();
      offOpened();
      offOpening();
      opening.destroy();
      offDrafts();
      buildSite.destroy();
      offPagePrint?.();
      stopPreview?.();
      closeFrame();
      root.replaceChildren();
    },
  };
}

