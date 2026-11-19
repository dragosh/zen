// ui/inspector: a bottom panel (ui/panel) under the reading view with three tabs,
// Linting (Vale), File and Page, and the button that toggles it. The page
// feature owns one and feeds it after every render; this file knows nothing
// about loading pages. All text goes in through textContent.
import "./inspector.css";
import type { Alert, KitHost, Page, PageInfo } from "../host";
import { issueList } from "../lint";
import { copyText } from "../shell/clipboard";
import { button, el } from "../shell/dom";
import { createTabbedPanel, panelEmpty, panelProps } from "../panel";
import { INSPECTOR_STATE, INSPECTOR_TOGGLE, inspectorIcon, inspectorLabel, type InspectorState } from "./events";
import { formatDate, formatSize, readingLabel } from "./format";
import { pageStats, type PageStats } from "./stats";

export { pageStats, type PageStats } from "./stats";
export { INSPECTOR_STATE, INSPECTOR_TOGGLE, type InspectorState } from "./events";

export const OPEN_KEY = "zen.inspector.open";
/** The last tab, by id. */
export const TAB_KEY = "zen.inspector.tab";
/** The panel's height, remembered across launches; drag its top edge to change it. */
export const HEIGHT_KEY = "zen.inspector.height";
export const HEIGHT_DEFAULT = 220;
export const HEIGHT_MIN = 120;

export type InspectorOptions = {
  doc: Document;
  /** The calls the File section uses; both are optional, see KitHost. */
  host: Pick<KitHost, "pageInfo" | "revealInFinder">;
  /** A click on an issue in the list. */
  onSelectIssue(alert: Alert, index: number): void;
  /** The panel was opened or closed (button, shortcut or API); not called when nothing changes. */
  onOpenChange?(open: boolean): void;
  /**
   * The sidebar's Inspector button (ui/shell) drives this one: its state goes out as INSPECTOR_STATE
   * events and INSPECTOR_TOGGLE toggles it. `button` is then not used. Without it (the editor)
   * `button` is the toggle and nothing is sent.
   */
  external?: boolean;
};

export type Inspector = {
  /** The toggle for a toolbar (the editor): the icon with the issue count as a badge. */
  button: HTMLButtonElement;
  /** The panel; put it in the content area, before the main pane. */
  el: HTMLElement;
  readonly isOpen: boolean;
  /** There is a page to inspect and the host shows pages now. */
  readonly available: boolean;
  open(): void;
  close(): void;
  toggle(): void;
  /** The page that is shown, with its rendered article; null when there is none. Also called on every refresh of the same page. */
  setPage(path: string | null, page?: Page, root?: ParentNode): void;
  /** false while the main pane shows something else (home, Settings): button and panel hide, nothing is forgotten. */
  setActive(active: boolean): void;
  /** The Vale alerts: null when linting is off or unavailable, "checking" while a result is on its way. */
  setIssues(alerts: Alert[] | null | "checking"): void;
  destroy(): void;
};

/** ⌥⌘I (⌃⌥I elsewhere), no Shift. The key is read from the physical key: ⌥I types "ˆ" on a Mac keyboard. */
export function inspectorKey(e: KeyboardEvent): boolean {
  return (e.metaKey || e.ctrlKey) && e.altKey && !e.shiftKey && (e.code === "KeyI" || e.key.toLowerCase() === "i");
}

const SOURCE_LABEL: Record<string, string> = { builtin: "built-in", folder: "folder", user: "user" };

type FileState =
  | { status: "loading" }
  | { status: "ok"; info: PageInfo }
  | { status: "error" };

let seq = 0;

// Open or closed lasts while the app runs (sessionStorage), so every launch starts with the panel
// closed; the tab is remembered across launches (localStorage).
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

export function createInspector(o: InspectorOptions): Inspector {
  const id = ++seq;
  // The panel is the shared tabbed bottom panel (ui/panel); a later tab (Spelling) is one more entry in
  // TABS and a render function.
  const TABS = [
    { id: "issues", label: "Linting" },
    { id: "file", label: "File" },
    { id: "page", label: "Page" },
  ] as const;
  const frame = createTabbedPanel({
    doc: o.doc,
    label: "Inspector",
    idPrefix: `zen-inspector-${id}`,
    tabs: TABS,
    tabKey: TAB_KEY,
    onClose: () => {
      inspector.close();
      (o.external ? o.doc.querySelector<HTMLElement>(".zen-inspector-button") : toggle)?.focus();
    },
    resize: { key: HEIGHT_KEY, initial: HEIGHT_DEFAULT, min: HEIGHT_MIN },
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

  let isOpen = read(OPEN_KEY) === "1";
  let active = true;
  let path: string | null = null;
  let page: Page | null = null;
  let stats: PageStats | null = null;
  let alerts: Alert[] | null | "checking" = null;
  let file: FileState | null = null;
  let fileFor = "";
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;
  let count = 0;

  const issues = { body: frame.body("issues") };
  const fileSection = { body: frame.body("file") };
  const pageSection = { body: frame.body("page") };

  const rows = panelProps;

  function renderIssues(): void {
    if (alerts === "checking") {
      issues.body.replaceChildren(panelEmpty("Checking…"));
    } else if (alerts === null) {
      issues.body.replaceChildren(panelEmpty("Linting off", "Vale is not available, or the page is empty."));
    } else if (alerts.length === 0) {
      issues.body.replaceChildren(panelEmpty("No issues", "Vale found nothing to flag on this page."));
    } else {
      issues.body.replaceChildren(issueList(alerts, (a, i) => o.onSelectIssue(a, i)));
    }
    count = Array.isArray(alerts) ? alerts.length : 0;
    const tone = Array.isArray(alerts) && alerts.some((a) => a.severity === "error") ? "error" : Array.isArray(alerts) && alerts.some((a) => a.severity === "warning") ? "warning" : "neutral";
    frame.setCount("issues", count, tone);
    badge.hidden = count === 0;
    badge.textContent = String(count);
    toggle.setAttribute("aria-label", inspectorLabel(count));
    announce();
  }

  function announce(): void {
    if (!o.external) return;
    const detail: InspectorState = { available: active && path !== null, open: isOpen, count };
    o.doc.dispatchEvent(new CustomEvent(INSPECTOR_STATE, { detail }));
  }

  function renderFile(): void {
    if (path === null) {
      fileSection.body.replaceChildren();
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
    const nodes: HTMLElement[] = [rows(items)];
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
    fileSection.body.replaceChildren(...nodes);
  }

  function renderPage(): void {
    if (!page || !stats) {
      pageSection.body.replaceChildren();
      return;
    }
    const s = stats;
    const layout = page.layout?.name || "article";
    const source = page.layout?.source ? SOURCE_LABEL[page.layout.source] ?? page.layout.source : "";
    const list = (v: string[]) => (v.length ? v.join(", ") : "none");
    pageSection.body.replaceChildren(rows([
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
    if (!isOpen || path === null || typeof o.host.pageInfo !== "function") return;
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

  function sync(): void {
    const has = path !== null;
    toggle.hidden = !(active && has);
    panel.hidden = !(isOpen && active && has);
    toggle.setAttribute("aria-expanded", String(isOpen));
    announce();
  }

  const inspector: Inspector = {
    button: toggle,
    el: panel,
    get isOpen() { return isOpen; },
    get available() { return active && path !== null; },
    open() {
      if (isOpen) return;
      isOpen = true;
      write(OPEN_KEY, "1");
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
    setPage(next: string | null, p?: Page, root?: ParentNode) {
      if (next === null || !p || !root) {
        path = null;
        page = null;
        stats = null;
        file = null;
        fileFor = "";
        fetchSeq++;
        renderFile();
        renderPage();
        sync();
        return;
      }
      path = next;
      page = p;
      stats = pageStats(root, p);
      renderFile();
      renderPage();
      sync();
      fetchFile();
    },
    setActive(next) {
      active = next;
      sync();
    },
    setIssues(next) {
      alerts = next;
      renderIssues();
    },
    destroy() {
      if (o.external) {
        o.doc.removeEventListener(INSPECTOR_TOGGLE, onExternal);
        o.doc.dispatchEvent(new CustomEvent(INSPECTOR_STATE, { detail: { available: false, open: false, count: 0 } satisfies InspectorState }));
      }
      fetchSeq++;
      clearTimeout(copiedTimer);
      panel.removeEventListener("keydown", onKey);
      toggle.removeEventListener("click", onToggle);
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
  if (o.external) o.doc.addEventListener(INSPECTOR_TOGGLE, onExternal);
  toggle.addEventListener("click", onToggle);
  panel.addEventListener("keydown", onKey);

  renderIssues();
  renderFile();
  renderPage();
  sync();
  return inspector;
}
