// ui/inspector: the one bottom panel (ui/panel) under the reading view, with tabs Page Info, Problems and
// Console, each in two columns (the list or main info on the left, details on the right), and the button
// that toggles it. Page Info and Problems come with the `inspector` feature, the Console with `developer`
// (and the Settings switch); the panel exists when the build has either. Spelling problems come from the
// vale add-on (kit/addons). The page feature owns one and feeds it after every render; this file knows
// nothing about loading pages. All text goes in through textContent.
import "./inspector.css";
import { hasAddon, hasFeature, type Alert, type KitHost, type Page, type PageInfo } from "../host";
import { reducedMotion } from "../shell/a11y";
import { installConsoleCapture } from "../shell/console";
import { copyText } from "../shell/clipboard";
import { developerEnabled, developerKey, followDeveloper } from "../shell/developer";
import { createConsoleTab, type ConsoleTab } from "../shell/devpanel";
import { buildSection } from "../shell/addons";
import { button, el } from "../shell/dom";
import { BOTTOM_PANEL_HEIGHT, bindColumns, createTabbedPanel, PANEL_COLLAPSED_KEY, panelColumns, panelEmpty, panelProps } from "../panel";
import { INSPECTOR_CLOSE, INSPECTOR_CONSOLE, INSPECTOR_STATE, INSPECTOR_TOGGLE, inspectorIcon, inspectorLabel, type InspectorState } from "./events";
import { formatDate, formatSize, readingLabel } from "./format";
import { brokenLinks, failedAlready, isCheck, missingImage, sameProblem, spellingProblems, warningProblems, type Filter, type Problem } from "./problems";
import { pageStats, type PageStats } from "./stats";

export { pageStats, type PageStats } from "./stats";
export { INSPECTOR_CLOSE, INSPECTOR_CONSOLE, INSPECTOR_STATE, INSPECTOR_TOGGLE, type InspectorState } from "./events";

export const OPEN_KEY = "zen.inspector.open";
/** The last tab, by id. */
export const TAB_KEY = "zen.inspector.tab";

export type InspectorOptions = {
  doc: Document;
  /**
   * The calls the Page Info tab uses (both optional, see KitHost), the add-ons of the build (Spelling needs "vale")
   * and the build's features and log calls (Page Info and Problems need `inspector`, the Console `developer`).
   */
  host: Pick<KitHost, "pageInfo" | "revealInFinder" | "addons" | "build" | "logs" | "onLog">;
  /** Show on a spelling problem: scroll to its mark and select it. */
  onSelectIssue(alert: Alert, index: number): void;
  /** The panel was opened or closed (button, shortcut or API); not called when nothing changes. */
  onOpenChange?(open: boolean): void;
  /**
   * The sidebar's Inspector button (ui/shell) drives this one: its state goes out as INSPECTOR_STATE
   * events and INSPECTOR_TOGGLE toggles it. `button` is then not used. Without it (the editor)
   * `button` is the toggle and nothing is sent.
   */
  external?: boolean;
  /** List the page's render warnings (`Page.warnings`) under Problems. The main window does; the editor's preview does not (the editor has its own console). */
  pageWarnings?: boolean;
};

export type Inspector = {
  /** The toggle for a toolbar (the editor): the icon with the problem count as a badge. */
  button: HTMLButtonElement;
  /** The panel; put it in the content area, before the main pane. */
  el: HTMLElement;
  readonly isOpen: boolean;
  /** There is a page to inspect (Page Info, Problems) and the host shows pages now: what the Inspector button needs. */
  readonly available: boolean;
  open(): void;
  close(): void;
  toggle(): void;
  /** The page that is shown, with its rendered article; null when there is none. Also called on every refresh of the same page. */
  setPage(path: string | null, page?: Page, root?: ParentNode): void;
  /** false while the main pane shows something else (home, Settings): button and panel hide, nothing is forgotten. */
  setActive(active: boolean): void;
  /** The Vale alerts (the Spelling problems): null when linting is off or unavailable, "checking" while a result is on its way. */
  setIssues(alerts: Alert[] | null | "checking"): void;
  destroy(): void;
};

/** What createInspector returns: the Inspector with its panel's collapse and the Console. */
export type InspectorPanel = Inspector & {
  /** Only the tab bar shows. */
  readonly collapsed: boolean;
  setCollapsed(collapsed: boolean): void;
  /** Opens on the Console tab, expanded, and focuses it; nothing while the Console is not there (no `developer` feature, or the switch is off). */
  showConsole(): void;
};

/** ⌥⌘I (⌃⌥I elsewhere), no Shift. The key is read from the physical key: ⌥I types "ˆ" on a Mac keyboard. */
export function inspectorKey(e: KeyboardEvent): boolean {
  return (e.metaKey || e.ctrlKey) && e.altKey && !e.shiftKey && (e.code === "KeyI" || e.key.toLowerCase() === "i");
}

const SOURCE_LABEL: Record<string, string> = { builtin: "built-in", folder: "folder", user: "user" };
const KIND_LABEL: Record<Problem["kind"], string> = { link: "Link", image: "Image", spelling: "Spelling", warning: "Warning" };

type FileState =
  | { status: "loading" }
  | { status: "ok"; info: PageInfo }
  | { status: "error" };

let seq = 0;

// Open or closed lasts while the app runs (sessionStorage), so every launch starts with the panel
// closed; the tab is remembered across launches (localStorage), and so are the height and collapsed state (ui/panel).
const store = (key: string): Storage => (key === OPEN_KEY ? sessionStorage : localStorage);

function read(key: string): string | null {
  try {
    return store(key).getItem(key);
  } catch {
    return null;
  }
}
function write(key: string, value: string): void {
  try {
    store(key).setItem(key, value);
  } catch {
    // not remembered; it still applies for this session
  }
}

export function createInspector(o: InspectorOptions): InspectorPanel {
  const id = ++seq;
  const win = (o.doc.defaultView ?? window) as Window & typeof globalThis;
  // The panel is the shared tabbed bottom panel (ui/panel). A tab whose feature the build leaves out is not
  // there at all; one more tab is one more entry in TABS and a render function. Spelling comes with the vale add-on.
  const lintOn = hasAddon(o.host, "vale");
  const pageTabs = hasFeature(o.host, "inspector");
  const consoleFeature = hasFeature(o.host, "developer");
  const TABS = [
    ...(pageTabs ? [{ id: "info", label: "Page Info" }, { id: "problems", label: "Problems" }] : []),
    ...(consoleFeature ? [{ id: "console", label: "Console" }, { id: "build", label: "Build" }] : []),
  ];
  const frame = createTabbedPanel({
    doc: o.doc,
    label: "Inspector",
    idPrefix: `zen-inspector-${id}`,
    tabs: TABS.length ? TABS : [{ id: "info", label: "Page Info" }], // a build with neither feature has no Inspector; this one is never shown
    tabKey: TAB_KEY,
    collapseKey: PANEL_COLLAPSED_KEY,
    onClose: () => {
      inspector.close();
      (o.external ? o.doc.querySelector<HTMLElement>(".zen-inspector-button") : toggle)?.focus();
    },
    resize: BOTTOM_PANEL_HEIGHT,
  });
  const panel = frame.el;
  panel.classList.add("zen-inspector");
  panel.id = `zen-inspector-${id}`;
  panel.hidden = true;

  const toggle = el("button", "zen-inspector-toggle zen-settings-button ghost");
  toggle.type = "button";
  toggle.title = "Inspector (⌥⌘I)";
  toggle.setAttribute("aria-controls", panel.id);
  toggle.setAttribute("aria-expanded", "false");
  toggle.hidden = true;
  const badge = el("span", "zen-inspector-badge");
  badge.hidden = true;
  toggle.append(inspectorIcon(o.doc), badge);
  toggle.setAttribute("aria-label", inspectorLabel(0));

  const consoleOn = () => consoleFeature && developerEnabled();
  let isOpen = read(OPEN_KEY) === "1" && (pageTabs || consoleOn()); // a remembered open panel stays shut when it has no tab to show
  let active = true;
  let path: string | null = null;
  let page: Page | null = null;
  let stats: PageStats | null = null;
  let alerts: Alert[] | null | "checking" = null;
  let file: FileState | null = null;
  let fileFor = "";
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;
  let count = 0;
  let filter: Filter = "all";
  let selected: Problem | null = null;
  let checkRoot: ParentNode | null = null;
  let failed = new Set<HTMLImageElement>();

  // ---- Page Info: the page on the left, the file on the right
  const info = pageTabs ? panelColumns({ id: "info", doc: o.doc }) : null;
  if (info) frame.body("info").append(info.el);
  const pageBody = info?.left ?? el("div");
  const fileBody = info?.right ?? el("div");

  // ---- Problems: the list on the left, the selected problem on the right
  const problems = pageTabs ? panelColumns({ id: "problems", doc: o.doc }) : null;
  if (problems) frame.body("problems").append(problems.el);
  const problemList = problems?.left ?? el("div");
  const problemDetails = problems?.right ?? el("div");
  if (problems) {
    const select = el("select", "zen-problems-filter");
    select.setAttribute("aria-label", "Show problems");
    for (const [v, label] of [["all", "All"], ["checks", "Checks"], ...(lintOn ? [["spelling", "Spelling"]] : [])]) {
      const opt = el("option", "", label);
      opt.value = v;
      select.append(opt);
    }
    select.addEventListener("change", () => { filter = select.value as Filter; renderProblems(); });
    frame.toolsFor("problems").append(select);
  }

  // ---- Console: the log on the left, the entry on the right
  let consoleTab: ConsoleTab | null = null;
  let consoleStarted = false;
  if (consoleFeature) {
    consoleTab = createConsoleTab({ doc: o.doc, host: o.host, capture: installConsoleCapture(win), tools: frame.toolsFor("console") });
    frame.body("console").append(consoleTab.el);
  }

  // ---- Build: the version and the features on the left, what else the build contains on the right (the same content as Settings › This build)
  let buildTab: { show(): void } | null = null;
  if (consoleFeature) {
    const cols = panelColumns({ id: "build", doc: o.doc });
    const section = buildSection(o.host);
    const draw = () => {
      section.show?.();
      const groups = Array.from(section.el?.children ?? []) as HTMLElement[];
      const features = groups.filter((g) => g.dataset.group === "features");
      cols.left.replaceChildren(el("p", "zen-build-version", `Version ${o.host.build?.version || "dev"}`), ...features);
      cols.right.replaceChildren(...groups.filter((g) => g.dataset.group !== "features"));
    };
    draw();
    frame.body("build").append(cols.el);
    buildTab = { show: draw };
  }
  // The Console's columns come from the Console tab; they take their split from this tab's id.
  if (consoleFeature) bindColumns(frame.body("console").querySelector(".zen-panel-cols"), "console");

  const rows = panelProps;
  const heading = (t: string) => el("h3", "zen-panel-heading", t);

  // ---- problems

  const checks = (): Problem[] => {
    if (!checkRoot) return [];
    const imgs = Array.from(checkRoot.querySelectorAll("img")).filter((i) => failed.has(i)).map(missingImage);
    return [...brokenLinks(checkRoot), ...imgs];
  };
  const warnings = (): Problem[] => (o.pageWarnings && page ? warningProblems(page.warnings) : []);
  const spelling = (): Problem[] => (lintOn && Array.isArray(alerts) ? spellingProblems(alerts) : []);
  let all: Problem[] = [];

  function showProblem(p: Problem): void {
    if (p.alert) {
      o.onSelectIssue(p.alert, p.index ?? 0);
      return;
    }
    const t = p.el;
    if (!t?.isConnected) return;
    const reduced = reducedMotion(o.doc);
    t.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "center" });
    if (!t.hasAttribute("tabindex")) t.setAttribute("tabindex", "-1"); // a broken link has no href and an image is not focusable
    t.focus({ preventScroll: true });
  }

  function renderProblems(): void {
    if (!problems) return;
    all = [...warnings(), ...checks(), ...spelling()];
    if (selected) selected = all.find((p) => sameProblem(p, selected!)) ?? null;
    const shown = all.filter((p) => filter === "all" || (filter === "checks") === isCheck(p));
    if (shown.length === 0) {
      problemList.replaceChildren(alerts === "checking" && filter !== "checks" && lintOn ? panelEmpty("Checking…") : panelEmpty("No problems on this page."));
    } else {
      const list = el("div", "zen-problem-list");
      list.setAttribute("role", "listbox");
      list.setAttribute("aria-label", "Problems");
      for (const p of shown) {
        const r = el("button", "zen-problem");
        r.type = "button";
        r.setAttribute("role", "option");
        r.setAttribute("aria-selected", String(p === selected));
        r.dataset.kind = p.kind;
        r.dataset.severity = p.severity;
        const dot = el("span", "zen-problem-dot");
        dot.setAttribute("aria-hidden", "true");
        r.append(dot, el("span", "zen-problem-kind", KIND_LABEL[p.kind]), el("span", "zen-problem-label", p.label), el("span", "zen-problem-where", p.where));
        r.title = p.message;
        r.addEventListener("click", () => { selected = p; renderProblems(); });
        list.append(r);
      }
      problemList.replaceChildren(list);
    }
    renderProblemDetails();
    count = all.length;
    const tone = all.some((p) => p.severity === "error") ? "error" : all.length ? "warning" : "neutral";
    frame.setCount("problems", count, tone);
    badge.hidden = count === 0;
    badge.textContent = String(count);
    toggle.setAttribute("aria-label", inspectorLabel(count));
    announce();
  }

  function renderProblemDetails(): void {
    const p = selected;
    if (!p) {
      problemDetails.replaceChildren(el("div", "zen-problem-details", "Select a problem"));
      return;
    }
    const box = el("div", "zen-problem-details");
    box.append(rows([["Message", "message", p.message], ...(p.fix ? [["Fix", "fix", p.fix] as [string, string, string]] : []), ["Where", "where", p.where], ["Rule", "rule", p.rule, true]]));
    if (p.kind !== "warning") { // a page warning is about the page as a whole: there is nothing to scroll to
      const show = button("Show", () => showProblem(p), { className: "zen-problem-show", title: "Scroll the page to it" });
      show.dataset.action = "show";
      const actions = el("div", "zen-inspector-actions");
      actions.append(show);
      box.append(actions);
    }
    problemDetails.replaceChildren(box);
  }

  // The images of the shown page: one that fails to load is a missing image.
  const onImageError = (e: Event) => {
    const t = e.target;
    if (!(t instanceof win.HTMLImageElement) || failed.has(t)) return;
    failed.add(t);
    renderProblems();
  };
  function watch(root: ParentNode | null): void {
    checkRoot?.removeEventListener("error", onImageError, true);
    checkRoot = root;
    failed = new Set(root ? Array.from(root.querySelectorAll("img")).filter(failedAlready) : []);
    root?.addEventListener("error", onImageError, true);
  }

  function announce(): void {
    if (!o.external) return;
    const detail: InspectorState = { available: inspector.available, open: isOpen, count };
    o.doc.dispatchEvent(new CustomEvent(INSPECTOR_STATE, { detail }));
  }

  function renderFile(): void {
    if (path === null) {
      fileBody.replaceChildren();
      return;
    }
    const items: [string, string, string, boolean?][] = [["Path", "path", path]];
    const hasInfo = typeof o.host.pageInfo === "function";
    if (hasInfo) {
      if (file?.status === "ok") {
        const i = file.info;
        items.push(["Full path", "abs", i.absPath, true], ["Size", "size", formatSize(i.size)], ["Modified", "modified", formatDate(i.modTime)]);
        if (i.birthTime > 0) items.push(["Created", "created", formatDate(i.birthTime)]);
      } else {
        items.push(["Size", "size", file?.status === "error" ? "Not available" : "…"]);
      }
    }
    const nodes: HTMLElement[] = [heading("File"), rows(items)];
    const actions = el("div", "zen-inspector-actions");
    if (file?.status === "ok") {
      const abs = file.info.absPath;
      const copy = button("Copy path", () => {
        void copyText(o.doc, abs).then((ok) => {
          if (!ok) return;
          copy.textContent = "Copied";
          clearTimeout(copiedTimer);
          copiedTimer = setTimeout(() => { copy.textContent = "Copy path"; }, 1500);
        });
      });
      copy.dataset.action = "copy";
      actions.append(copy);
    }
    if (typeof o.host.revealInFinder === "function" && path !== null) {
      const target = path;
      const reveal = button("Reveal in Finder", () => {
        o.host.revealInFinder?.(target).catch((err) => console.warn(`zen: could not reveal ${target}`, err));
      });
      reveal.dataset.action = "reveal";
      actions.append(reveal);
    }
    if (actions.childElementCount) nodes.push(actions);
    fileBody.replaceChildren(...nodes);
  }

  function renderPage(): void {
    if (!page || !stats) {
      pageBody.replaceChildren();
      return;
    }
    const s = stats;
    const layout = page.layout?.name || "article";
    const source = page.layout?.source ? SOURCE_LABEL[page.layout.source] ?? page.layout.source : "";
    const list = (v: string[]) => (v.length ? v.join(", ") : "none");
    pageBody.replaceChildren(heading("Page"), rows([
      ["Title", "title", page.title],
      ["Layout", "layout", source ? `${layout} · ${source}` : layout],
      ["Words", "words", String(s.words)],
      ["Reading time", "reading", readingLabel(s.minutes)],
      ["Headings", "headings", String(s.headings)],
      ["Links", "links", `${s.links.internal} internal · ${s.links.external} external`],
      ["Images", "images", String(s.images)],
      ["Plugins", "plugins", list(s.plugins)],
      ["Front matter", "frontmatter", list(s.frontmatter)],
    ]));
  }

  // ---- file data: fetched while the panel is open, never for a stale page

  let fetchSeq = 0;
  function fetchFile(): void {
    const mine = ++fetchSeq;
    if (!pageTabs || !isOpen || path === null || typeof o.host.pageInfo !== "function") return;
    const asked = path;
    if (fileFor !== asked || !file) {
      file = { status: "loading" };
      fileFor = asked;
      renderFile();
    }
    o.host.pageInfo(asked).then(
      (info) => {
        if (mine !== fetchSeq) return;
        file = { status: "ok", info };
        renderFile();
      },
      (err) => {
        if (mine !== fetchSeq) return;
        file = { status: "error" };
        console.warn(`zen: could not read the details of ${asked}`, err);
        renderFile();
      },
    );
  }

  // ---- visibility

  /** The tabs that are there now: the page's need a page, the Console needs the switch. */
  function syncTabs(): void {
    frame.setTabVisible("info", pageTabs && path !== null);
    frame.setTabVisible("problems", pageTabs && path !== null);
    frame.setTabVisible("console", consoleOn());
    frame.setTabVisible("build", consoleOn());
  }

  function sync(): void {
    syncTabs();
    const has = path !== null;
    toggle.hidden = !(active && pageTabs && has);
    panel.hidden = !(isOpen && active && frame.visibleTabs.length > 0 && (pageTabs ? has || consoleOn() : consoleOn()));
    toggle.setAttribute("aria-expanded", String(isOpen));
    if (isOpen && !panel.hidden && consoleOn() && consoleTab && !consoleStarted) {
      consoleStarted = true;
      void consoleTab.start();
    }
    if (isOpen && !panel.hidden && consoleOn()) buildTab?.show();
    announce();
  }

  const inspector: InspectorPanel = {
    button: toggle,
    el: panel,
    get isOpen() { return isOpen; },
    get available() { return active && pageTabs && path !== null; },
    get collapsed() { return frame.collapsed; },
    setCollapsed: (c) => frame.setCollapsed(c),
    open() {
      if (isOpen) return;
      isOpen = true;
      write(OPEN_KEY, "1");
      frame.reload(); // the height may have been changed meanwhile
      sync();
      fetchFile();
      o.onOpenChange?.(true);
    },
    close() {
      if (!isOpen) return;
      isOpen = false;
      write(OPEN_KEY, "0");
      fetchSeq++;
      sync();
      o.onOpenChange?.(false);
    },
    toggle() {
      if (isOpen) inspector.close();
      else inspector.open();
    },
    showConsole() {
      if (!consoleOn() || !active) return;
      syncTabs();
      inspector.open();
      frame.setCollapsed(false);
      frame.select("console");
      sync();
      panel.querySelector<HTMLElement>('[role="tab"][data-tab="console"]')?.focus();
    },
    setPage(next: string | null, p?: Page, root?: ParentNode) {
      if (next === null || !p || !root) {
        path = null;
        page = null;
        stats = null;
        file = null;
        fileFor = "";
        fetchSeq++;
        watch(null);
        renderFile();
        renderPage();
        renderProblems();
        sync();
        return;
      }
      path = next;
      page = p;
      stats = pageStats(root, p);
      watch(root);
      renderFile();
      renderPage();
      renderProblems();
      sync();
      fetchFile();
    },
    setActive(next) {
      active = next;
      sync();
    },
    setIssues(next) {
      if (!lintOn) return;
      alerts = next;
      renderProblems();
    },
    destroy() {
      if (o.external) {
        o.doc.removeEventListener(INSPECTOR_TOGGLE, onExternal);
        o.doc.removeEventListener(INSPECTOR_CLOSE, onCloseRequest);
        o.doc.dispatchEvent(new CustomEvent(INSPECTOR_STATE, { detail: { available: false, open: false, count: 0 } satisfies InspectorState }));
      }
      o.doc.removeEventListener(INSPECTOR_CONSOLE, onConsole);
      o.doc.removeEventListener("keydown", onDevKey);
      stopFollow();
      watch(null);
      fetchSeq++;
      clearTimeout(copiedTimer);
      panel.removeEventListener("keydown", onKey);
      toggle.removeEventListener("click", onToggle);
      consoleTab?.destroy();
      frame.destroy();
      toggle.remove();
    },
  };

  const onToggle = () => inspector.toggle();
  const onKey = (e: KeyboardEvent) => {
    if (e.key !== "Escape" || !isOpen || e.defaultPrevented) return;
    e.preventDefault();
    e.stopPropagation();
    inspector.close();
    (o.external ? o.doc.querySelector<HTMLElement>(".zen-inspector-button") : toggle)?.focus();
  };
  const onExternal = () => { if (inspector.available) inspector.toggle(); };
  const onCloseRequest = () => inspector.close();
  // ⌥⌘J and the Develop menu: the Console, with the switch on. With it off they do nothing.
  const onDevKey = (e: KeyboardEvent) => {
    if (!developerKey(e) || !consoleOn() || !active) return;
    e.preventDefault();
    if (isOpen && !panel.hidden && !frame.collapsed && frame.selected === "console") inspector.close();
    else inspector.showConsole();
  };
  const onConsole = () => inspector.showConsole();
  // The switch turned off takes the Console tab away; a panel left with no tab closes.
  const stopFollow = followDeveloper(() => {
    if (!pageTabs && !consoleOn()) inspector.close();
    sync();
  }, win);
  if (o.external) {
    o.doc.addEventListener(INSPECTOR_TOGGLE, onExternal);
    o.doc.addEventListener(INSPECTOR_CLOSE, onCloseRequest);
  }
  o.doc.addEventListener(INSPECTOR_CONSOLE, onConsole);
  o.doc.addEventListener("keydown", onDevKey);
  toggle.addEventListener("click", onToggle);
  panel.addEventListener("keydown", onKey);

  renderProblems();
  renderFile();
  renderPage();
  sync();
  return inspector;
}
