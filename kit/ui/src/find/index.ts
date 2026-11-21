// ui/find: Find in page (⌘F). A small bar over the top right of the window with a search field, "n of m",
// previous / next, an "Aa" (match case) toggle and a close button. Used by the reader (the page's text,
// highlighted with the CSS Custom Highlight API, so the page's DOM is not changed) and by the editor (the
// markdown source in its textarea, the match selected). The query is plain text: it is never used as HTML
// and never as a regular expression without escaping.
import { button, el } from "../shell/dom";
import "./find.css";

/** Highlight names; find.css styles them with ::highlight(). */
export const HIGHLIGHT_ALL = "zen-find";
export const HIGHLIGHT_CURRENT = "zen-find-current";
/** Stops a runaway query ("e" in a book) from building a million ranges. */
export const MAX_MATCHES = 5000;

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/** Start offsets of every non-overlapping, literal occurrence of query in text. */
export function findInText(text: string, query: string, matchCase: boolean): number[] {
  if (!query) return [];
  const re = new RegExp(escapeRegExp(query), matchCase ? "gu" : "giu");
  const out: number[] = [];
  for (let m = re.exec(text); m && out.length < MAX_MATCHES; m = re.exec(text)) out.push(m.index);
  return out;
}

/** What the bar needs of a searchable thing: the reader's DOM or the editor's textarea. */
export interface Finder {
  /** Searches again and returns the number of matches. Paints no "current" match. */
  find(query: string, matchCase: boolean): number;
  /** Makes match i the current one. quiet: only repaint, do not scroll, select or open anything. */
  show(i: number, opts?: { quiet?: boolean }): void;
  /** Removes every highlight and selection this finder made. */
  clear(): void;
  /** The text the user has selected, to start a search with. */
  selected(): string;
}

// ---- highlights ----
type Registry = { set(name: string, h: unknown): unknown; delete(name: string): unknown };
/** The CSS Custom Highlight API; tests pass a stub. */
export type HighlightSupport = { registry: Registry; create(ranges: Range[]): unknown };

/** The CSS Custom Highlight API, or null where it is missing; shared by find and the Inspector's problem marks. */
export function highlightApi(): HighlightSupport | null {
  const css = (globalThis as { CSS?: { highlights?: Registry } }).CSS;
  const Ctor = (globalThis as { Highlight?: new (...r: Range[]) => unknown }).Highlight;
  if (!css?.highlights || typeof Ctor !== "function") return null;
  return { registry: css.highlights, create: (ranges) => new Ctor(...ranges) };
}

function detectHighlights(): HighlightSupport | null {
  const api = highlightApi();
  if (!api) return null;
  return {
    registry: api.registry,
    create(ranges) {
      const h = api.create(ranges) as { priority: number };
      h.priority = ranges.length === 1 ? 1 : 0; // the current match is drawn over the others
      return h;
    },
  };
}

// ---- the page's DOM ----
const BLOCKS = new Set([
  "P", "DIV", "LI", "UL", "OL", "DL", "DT", "DD", "TABLE", "THEAD", "TBODY", "TFOOT", "TR", "TD", "TH", "CAPTION",
  "H1", "H2", "H3", "H4", "H5", "H6", "PRE", "BLOCKQUOTE", "SECTION", "ARTICLE", "HEADER", "FOOTER", "ASIDE", "NAV",
  "MAIN", "DETAILS", "SUMMARY", "FIGURE", "FIGCAPTION", "HR", "BR", "FORM", "FIELDSET", "LEGEND", "BUTTON", "SELECT",
  "OPTION", "TEXTAREA", "INPUT",
]);
const NEVER = new Set(["script", "style", "template", "noscript", "svg", "textarea", "select", "head"]);

export type DomFinderOptions = {
  /** Elements whose text is not searched (hidden notes, thumbnails). */
  skip?: (e: Element) => boolean;
  /**
   * Shows a block with the hidden attribute that holds the current match (a slide, a course step).
   * Return true when it did; otherwise the attribute is removed.
   */
  reveal?: (block: HTMLElement) => boolean;
  /** Default: the CSS Custom Highlight API when there is one; null: select the current match instead. */
  highlights?: HighlightSupport | null;
};

type Group = { block: Element; nodes: Text[] };

/** The text of root in reading order, grouped by block so a match can run across inline markup. */
function textGroups(root: Element, skip?: (e: Element) => boolean): Group[] {
  const groups: Group[] = [];
  let force = true;
  const walk = (node: Node, block: Element) => {
    if (node.nodeType === 3) {
      const last = groups[groups.length - 1];
      if (force || !last || last.block !== block) groups.push({ block, nodes: [node as Text] });
      else last.nodes.push(node as Text);
      force = false;
      return;
    }
    if (node.nodeType !== 1) return;
    const e = node as Element;
    if (NEVER.has(e.localName) || skip?.(e)) return;
    const isBlock = BLOCKS.has(e.tagName);
    if (isBlock) force = true;
    for (const c of Array.from(e.childNodes)) walk(c, isBlock ? e : block);
    if (isBlock) force = true;
  };
  for (const c of Array.from(root.childNodes)) walk(c, root);
  return groups;
}

function rangesIn(root: Element, query: string, matchCase: boolean, skip?: (e: Element) => boolean): Range[] {
  const doc = root.ownerDocument;
  const out: Range[] = [];
  for (const g of textGroups(root, skip)) {
    if (out.length >= MAX_MATCHES) break;
    const text = g.nodes.map((n) => n.data).join("");
    for (const start of findInText(text, query, matchCase)) {
      if (out.length >= MAX_MATCHES) break;
      const end = start + query.length;
      const r = doc.createRange();
      let at = 0;
      let started = false;
      for (const n of g.nodes) {
        const len = n.data.length;
        if (!started && start < at + len) { r.setStart(n, start - at); started = true; }
        if (started && end <= at + len) { r.setEnd(n, end - at); break; }
        at += len;
      }
      out.push(r);
    }
  }
  return out;
}

/** Opens every closed <details> and shows every hidden block between the match and the root. */
function revealRange(r: Range, root: Element, reveal?: (b: HTMLElement) => boolean): void {
  let child: Node = r.startContainer;
  for (let n: Node | null = child.parentNode; n && n !== root && n.nodeType === 1; child = n, n = n.parentNode) {
    const e = n as HTMLElement;
    if (e.localName === "details" && !(e as HTMLDetailsElement).open && (child as Element).localName !== "summary") {
      (e as HTMLDetailsElement).open = true;
    }
    if (e.hidden && e.getAttribute("hidden") !== "until-found" && !(reveal?.(e) ?? false)) e.hidden = false;
  }
}

/** A Finder for the text in root(): highlights every match, scrolls to the current one. */
export function domFinder(root: () => HTMLElement | null, opts: DomFinderOptions = {}): Finder {
  const support = opts.highlights === undefined ? detectHighlights() : opts.highlights;
  let ranges: Range[] = [];
  let selecting = false;

  const paint = (current: number) => {
    if (!support) return;
    if (ranges.length) support.registry.set(HIGHLIGHT_ALL, support.create(ranges));
    else support.registry.delete(HIGHLIGHT_ALL);
    if (ranges[current]) support.registry.set(HIGHLIGHT_CURRENT, support.create([ranges[current]]));
    else support.registry.delete(HIGHLIGHT_CURRENT);
  };
  const clear = () => {
    support?.registry.delete(HIGHLIGHT_ALL);
    support?.registry.delete(HIGHLIGHT_CURRENT);
    if (selecting) {
      selecting = false;
      ranges[0]?.startContainer.ownerDocument?.getSelection()?.removeAllRanges();
    }
    ranges = [];
  };
  return {
    find(query, matchCase) {
      clear();
      const r = root();
      if (r && query) ranges = rangesIn(r, query, matchCase, opts.skip);
      paint(-1);
      return ranges.length;
    },
    show(i, o) {
      const range = ranges[i];
      const r = root();
      if (!range || !r) return;
      if (!o?.quiet) revealRange(range, r, opts.reveal);
      paint(i);
      if (!support) {
        const sel = range.startContainer.ownerDocument?.getSelection();
        sel?.removeAllRanges();
        sel?.addRange(range);
        selecting = true;
      }
      if (!o?.quiet) {
        const target = range.startContainer.parentElement;
        target?.scrollIntoView?.({ block: "center", behavior: "auto" });
      }
    },
    clear,
    selected() {
      const r = root();
      const sel = r?.ownerDocument.getSelection();
      if (!r || !sel || sel.isCollapsed || !sel.anchorNode || !r.contains(sel.anchorNode)) return "";
      return sel.toString();
    },
  };
}

// ---- a textarea ----
/** A Finder for the text of a textarea (the editor): the current match is selected in it. */
export function textareaFinder(ta: HTMLTextAreaElement): Finder {
  let at: number[] = [];
  let len = 0;
  return {
    find(query, matchCase) {
      at = findInText(ta.value, query, matchCase);
      len = query.length;
      return at.length;
    },
    show(i, o) {
      if (o?.quiet || at[i] === undefined) return;
      const doc = ta.ownerDocument;
      const before = doc.activeElement as HTMLElement | null;
      ta.setSelectionRange(at[i], at[i] + len);
      // A blurred textarea shows no selection and does not scroll to it: focus it for a moment, then give
      // the focus back (the search field keeps what the user is typing).
      if (before !== ta) {
        ta.focus();
        if (before && before !== doc.body) before.focus({ preventScroll: true });
      }
    },
    clear() { at = []; },
    selected() { return ta.value.slice(ta.selectionStart, ta.selectionEnd); },
  };
}

// ---- the bar ----
export type Find = {
  /** The bar is shown. */
  readonly isOpen: boolean;
  readonly query: string;
  /** Shows the bar with the field focused and its text selected; seed (or the finder's selection) becomes the query. */
  open(seed?: string): void;
  close(): void;
  /** Opens the bar, or closes it when it is open (the top bar's Find button). */
  toggle(): void;
  /** Tells whenever the bar opens or closes (not when nothing changes), for the Find button's aria-pressed. Returns the way to stop. */
  onChange(fn: (open: boolean) => void): () => void;
  /** Next / previous match, wrapping around. With the bar closed they open it. */
  next(): void;
  prev(): void;
  /** The text changed: count again, keep the current match, do not scroll. */
  refresh(): void;
  destroy(): void;
};

export type FindOptions = {
  doc: Document;
  /** Where the bar goes (it is positioned over the top right of the window). */
  host: HTMLElement;
  finder: Finder;
  /** Accessible name of the bar; default "Find in page". */
  label?: string;
};

const seedText = (s: string | undefined): string => (s && !/[\r\n]/.test(s) && s.length <= 200 ? s : "");

export function createFind(o: FindOptions): Find {
  const { doc, host, finder } = o;
  let bar: HTMLElement | null = null;
  let input: HTMLInputElement;
  let countEl: HTMLElement;
  let caseBtn: HTMLButtonElement;
  let query = "";
  let matchCase = false;
  let total = 0;
  let index = -1;
  let previous: HTMLElement | null = null;
  const listeners = new Set<(open: boolean) => void>();
  const changed = (open: boolean) => { for (const fn of Array.from(listeners)) fn(open); };

  const paintCount = () => {
    if (!bar) return;
    countEl.textContent = !query ? "" : total === 0 ? "No results" : `${index + 1} of ${total}${total >= MAX_MATCHES ? "+" : ""}`;
    bar.classList.toggle("is-empty", !!query && total === 0);
  };
  /** Moves to match i without losing the caret of the search field. */
  const go = (i: number, quiet = false) => {
    index = i;
    const [a, b] = [input?.selectionStart ?? null, input?.selectionEnd ?? null];
    finder.show(i, { quiet });
    if (input && doc.activeElement === input && a !== null && b !== null) input.setSelectionRange(a, b);
    paintCount();
  };
  const search = (keep = false) => {
    if (!query) {
      finder.clear();
      total = 0;
      index = -1;
      paintCount();
      return;
    }
    total = finder.find(query, matchCase);
    if (total === 0) {
      index = -1;
      paintCount();
      return;
    }
    go(keep && index >= 0 ? Math.min(index, total - 1) : 0, keep);
  };
  const step = (d: number) => {
    if (total === 0) return;
    go((index + d + total) % total);
  };

  const onDocKey = (e: KeyboardEvent) => {
    if (bar && e.key === "Escape" && !e.defaultPrevented) {
      e.preventDefault();
      api.close();
    }
  };
  const build = () => {
    bar = el("div", "zen-find");
    bar.setAttribute("role", "search");
    bar.setAttribute("aria-label", o.label ?? "Find in page");
    input = el("input", "zen-find-field");
    input.type = "text";
    input.setAttribute("aria-label", "Find");
    input.placeholder = "Find";
    input.autocomplete = "off";
    input.spellcheck = false;
    input.setAttribute("autocapitalize", "off");
    input.setAttribute("autocorrect", "off");
    input.enterKeyHint = "search";
    input.addEventListener("input", () => {
      query = input.value;
      search();
    });
    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.isComposing) {
        e.preventDefault();
        step(e.shiftKey ? -1 : 1);
      }
    });
    countEl = el("span", "zen-find-count");
    countEl.setAttribute("aria-live", "polite");
    countEl.setAttribute("role", "status");
    caseBtn = button("Match case", () => {
      matchCase = !matchCase;
      caseBtn.setAttribute("aria-pressed", String(matchCase));
      search();
      input.focus();
    }, { className: "zen-find-case", text: "Aa", title: "Match case" });
    caseBtn.setAttribute("aria-pressed", String(matchCase));
    const prev = button("Previous match", () => step(-1), { className: "zen-find-step", text: "↑", title: "Previous match (⇧⌘G)" });
    const next = button("Next match", () => step(1), { className: "zen-find-step", text: "↓", title: "Next match (⌘G)" });
    const close = button("Close find", () => api.close(), { className: "zen-find-close", text: "✕", title: "Close (Esc)" });
    bar.append(input, countEl, caseBtn, prev, next, close);
    host.append(bar);
    doc.addEventListener("keydown", onDocKey, true);
  };

  const api: Find = {
    get isOpen() { return bar !== null; },
    get query() { return query; },
    open(seed) {
      const text = seedText(seed ?? finder.selected());
      const opened = !bar;
      if (!bar) {
        const active = doc.activeElement as HTMLElement | null;
        previous = active && active !== doc.body ? active : null;
        build();
        if (text) query = text;
        input.value = query;
        search();
      } else if (text && text !== query) {
        query = text;
        input.value = query;
        search();
      }
      input.focus();
      input.select();
      if (opened) changed(true);
    },
    toggle() {
      if (bar) api.close();
      else api.open();
    },
    onChange(fn) {
      listeners.add(fn);
      return () => void listeners.delete(fn);
    },
    close() {
      if (!bar) return;
      finder.clear();
      bar.remove();
      bar = null;
      doc.removeEventListener("keydown", onDocKey, true);
      total = 0;
      index = -1;
      const back = previous;
      previous = null;
      if (back?.isConnected) back.focus();
      changed(false);
    },
    next() {
      if (!bar) api.open();
      else step(1);
    },
    prev() {
      if (!bar) api.open();
      else step(-1);
    },
    refresh() {
      if (bar) search(true);
    },
    destroy() { api.close(); },
  };
  return api;
}

// ---- ⌘F, ⌘G and the Edit menu ----
/** The menu events Go sends to every window (apps/zenmark/menu.go); the window that has the focus acts. */
export type FindEvents = { "menu:find": void; "menu:find-next": void; "menu:find-previous": void };
export type FindApi = { on(event: keyof FindEvents, fn: () => void): () => void };

export type BindOptions = {
  /** Whether this window is the one in front; default document.hasFocus(). */
  hasFocus?: () => boolean;
  /** Whether there is anything to search right now (the reader shows a page); default always. */
  active?: () => boolean;
  /** Text to start a search with (the selection); default: the finder's own. */
  seed?: () => string | undefined;
};

/** A menu event that follows a key press this soon is the same press (the page and the menu both saw it). */
const SAME_PRESS_MS = 150;

/**
 * ⌘F opens the bar, ⌘G / ⇧⌘G step, from the keyboard and from Edit › Find. The menu accelerators are the
 * same keys, and which of the two the web view sees first is not something to rely on, so both are handled
 * and a menu event right after the same key is ignored.
 */
export function bindFind(doc: Document, api: FindApi, find: Pick<Find, "open" | "next" | "prev">, opts: BindOptions = {}): () => void {
  const focused = opts.hasFocus ?? (() => doc.hasFocus());
  const active = opts.active ?? (() => true);
  const lastKey = new Map<string, number>();
  const run = (cmd: "open" | "next" | "prev") => {
    if (cmd === "open") find.open(opts.seed?.());
    else find[cmd]();
  };
  const onKey = (e: KeyboardEvent) => {
    if (!(e.metaKey || e.ctrlKey) || e.altKey || !active()) return;
    const k = e.key.toLowerCase();
    let cmd: "open" | "next" | "prev";
    if (k === "f" && !e.shiftKey) cmd = "open";
    else if (k === "g") cmd = e.shiftKey ? "prev" : "next";
    else return;
    e.preventDefault();
    lastKey.set(cmd, Date.now());
    run(cmd);
  };
  const fromMenu = (cmd: "open" | "next" | "prev") => () => {
    if (!focused() || !active()) return;
    if (Date.now() - (lastKey.get(cmd) ?? -Infinity) < SAME_PRESS_MS) return;
    run(cmd);
  };
  doc.addEventListener("keydown", onKey);
  const offs = [
    api.on("menu:find", fromMenu("open")),
    api.on("menu:find-next", fromMenu("next")),
    api.on("menu:find-previous", fromMenu("prev")),
  ];
  return () => {
    doc.removeEventListener("keydown", onKey);
    for (const off of offs) off();
  };
}
