// ui/inspector: a panel on the right of the reading view with three sections,
// Issues (Vale), File and Page, and the button that toggles it. The page
// feature owns one and feeds it after every render; this file knows nothing
// about loading pages. All text goes in through textContent.
import "./inspector.css";
import type { Alert, KitHost, Page, PageInfo } from "../host";
import { issueList } from "../lint";
import { copyText } from "../shell/clipboard";
import { button, el } from "../shell/dom";
import { formatDate, formatSize, readingLabel } from "./format";
import { pageStats, type PageStats } from "./stats";

export { pageStats, type PageStats } from "./stats";

export const OPEN_KEY = "zen.inspector.open";
export const COLLAPSED_KEY = "zen.inspector.collapsed";

export type InspectorOptions = {
  doc: Document;
  /** The calls the File section uses; both are optional, see KitHost. */
  host: Pick<KitHost, "pageInfo" | "revealInFinder">;
  /** A click on an issue in the list. */
  onSelectIssue(alert: Alert, index: number): void;
  /** The panel was opened or closed (button, shortcut or API); not called when nothing changes. */
  onOpenChange?(open: boolean): void;
};

export type Inspector = {
  /** The toggle for the page actions: "Inspector" with the issue count. */
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

function read(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}
function write(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // not remembered; it still applies for this session
  }
}

function storedCollapsed(): Set<string> {
  try {
    const v = JSON.parse(read(COLLAPSED_KEY) ?? "[]");
    return new Set(Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : []);
  } catch {
    return new Set();
  }
}

export function createInspector(o: InspectorOptions): Inspector {
  const id = ++seq;
  const panel = el("aside", "zen-inspector");
  panel.id = `zen-inspector-${id}`;
  panel.setAttribute("aria-label", "Inspector");
  panel.hidden = true;

  const toggle = el("button", "zen-inspector-toggle ghost");
  toggle.type = "button";
  toggle.title = "Inspector (⌥⌘I)";
  toggle.setAttribute("aria-controls", panel.id);
  toggle.setAttribute("aria-expanded", "false");
  toggle.hidden = true;
  const badge = el("span", "zen-inspector-badge");
  badge.hidden = true;
  toggle.append(el("span", "zen-inspector-label", "Inspector"), badge);

  let isOpen = read(OPEN_KEY) === "1";
  let active = true;
  let path: string | null = null;
  let page: Page | null = null;
  let stats: PageStats | null = null;
  let alerts: Alert[] | null | "checking" = null;
  let file: FileState | null = null;
  let fileFor = "";
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;
  const collapsed = storedCollapsed();

  // ---- sections

  type Section = { root: HTMLElement; head: HTMLButtonElement; body: HTMLElement };
  const makeSection = (key: string, title: string): Section => {
    const root = el("section", "zen-inspector-section");
    root.dataset.section = key;
    root.setAttribute("role", "region");
    const head = el("button", "zen-inspector-head", title);
    head.type = "button";
    head.id = `${panel.id}-${key}`;
    root.setAttribute("aria-labelledby", head.id);
    const body = el("div", "zen-inspector-body");
    root.append(head, body);
    const apply = () => {
      const c = collapsed.has(key);
      root.dataset.collapsed = String(c);
      head.setAttribute("aria-expanded", String(!c));
      body.hidden = c;
    };
    head.addEventListener("click", () => {
      if (!collapsed.delete(key)) collapsed.add(key);
      write(COLLAPSED_KEY, JSON.stringify(Array.from(collapsed)));
      apply();
    });
    apply();
    return { root, head, body };
  };
  const issues = makeSection("issues", "Issues");
  const fileSection = makeSection("file", "File");
  const pageSection = makeSection("page", "Page");
  panel.append(issues.root, fileSection.root, pageSection.root);

  const rows = (items: [label: string, field: string, value: string, mono?: boolean][]): HTMLElement => {
    const dl = el("dl", "zen-inspector-rows");
    for (const [label, field, value, mono] of items) {
      const dd = el("dd", mono ? "zen-inspector-mono" : "", value);
      dd.dataset.field = field;
      dl.append(el("dt", "", label), dd);
    }
    return dl;
  };

  function renderIssues(): void {
    if (alerts === "checking") {
      issues.body.replaceChildren(el("p", "zen-inspector-empty", "Checking…"));
    } else if (alerts === null) {
      issues.body.replaceChildren(el("p", "zen-inspector-empty", "Linting off"));
    } else if (alerts.length === 0) {
      issues.body.replaceChildren(el("p", "zen-inspector-empty", "No issues"));
    } else {
      issues.body.replaceChildren(issueList(alerts, (a, i) => o.onSelectIssue(a, i)));
    }
    const n = Array.isArray(alerts) ? alerts.length : 0;
    badge.hidden = n === 0;
    badge.textContent = String(n);
    if (n > 0) toggle.setAttribute("aria-label", `Inspector, ${n} ${n === 1 ? "issue" : "issues"}`);
    else toggle.removeAttribute("aria-label");
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
      fetchSeq++;
      clearTimeout(copiedTimer);
      panel.removeEventListener("keydown", onKey);
      toggle.removeEventListener("click", onToggle);
      panel.remove();
      toggle.remove();
    },
  };

  const onToggle = () => inspector.toggle();
  const onKey = (e: KeyboardEvent) => {
    if (e.key !== "Escape" || !isOpen || e.defaultPrevented) return;
    e.preventDefault();
    e.stopPropagation();
    inspector.close();
    toggle.focus();
  };
  toggle.addEventListener("click", onToggle);
  panel.addEventListener("keydown", onKey);

  renderIssues();
  renderFile();
  renderPage();
  sync();
  return inspector;
}
