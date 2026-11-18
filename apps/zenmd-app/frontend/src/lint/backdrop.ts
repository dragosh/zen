// The editor's backdrop: a layer behind the (transparent) textarea that
// mirrors its text, wrapping, padding and scroll, and underlines alert ranges.
// The caret and selection stay the textarea's own.
import type { Alert } from "../api";
import { makeMark } from "./mark";

export type Backdrop = {
  /** The wrapper now holding the mirror and the textarea, where the textarea was. */
  el: HTMLElement;
  mirror: HTMLElement;
  /** Draws alerts computed for `text` (default: the current value), moved onto the current value. */
  update(alerts: Alert[], text?: string): void;
  /** Re-reads the textarea's value after a change made without an input event. */
  refresh(): void;
  /** Copies the textarea's typography and box to the mirror (runs on resize too). */
  sync(): void;
  /** Current [start, end) of alert i, null when it is gone or was never placed. */
  range(i: number): [number, number] | null;
  mark(i: number): HTMLElement | null;
  /** Focuses the textarea, selects alert i and scrolls it into view. */
  select(i: number): void;
  destroy(): void;
};

/** Offset in s after `runes` code points starting at `from`. */
function advance(s: string, from: number, runes: number, limit: number): number {
  let i = from;
  for (let n = 0; n < runes && i < limit; n++) {
    const c = s.charCodeAt(i);
    i += c >= 0xd800 && c <= 0xdbff && i + 1 < limit ? 2 : 1;
  }
  return i;
}

/** The [start, end) offsets in text of an alert's 1-based line and rune columns (endCol inclusive). */
export function alertRange(text: string, a: Alert): [number, number] | null {
  if (a.line < 1 || a.col < 1) return null;
  let start = 0;
  for (let l = 1; l < a.line; l++) {
    const nl = text.indexOf("\n", start);
    if (nl < 0) return null;
    start = nl + 1;
  }
  let end = text.indexOf("\n", start);
  if (end < 0) end = text.length;
  if (end > start && text[end - 1] === "\r") end--;
  const s = advance(text, start, a.col - 1, end);
  const e = advance(text, s, Math.max(1, a.endCol - a.col + 1), end);
  return e > s ? [s, e] : null;
}

/** Properties that decide where text lands. */
const MIRRORED = [
  "fontFamily", "fontSize", "fontWeight", "fontStyle", "fontStretch", "fontVariant", "fontFeatureSettings", "fontKerning",
  "lineHeight", "letterSpacing", "wordSpacing", "textIndent", "textTransform", "textRendering",
  "tabSize", "whiteSpace", "wordBreak", "overflowWrap", "direction", "hyphens",
  "paddingTop", "paddingRight", "paddingBottom", "paddingLeft",
  "borderTopWidth", "borderRightWidth", "borderBottomWidth", "borderLeftWidth", "boxSizing",
] as const;

type Range = { start: number; end: number; alerts: number[] };

/** A highlighted range of the text (editor/highlight.ts). */
export type Decoration = { start: number; end: number; cls: string };

export type BackdropOptions = {
  /**
   * Syntax highlighting: the mirror draws the text visibly with these classes
   * (the editor makes the textarea's own text transparent).
   */
  highlight?: (text: string) => Decoration[];
  /** One row per line with its number in the left padding (class zen-ed-line, data-n). */
  lineNumbers?: boolean;
};

export function backdrop(ta: HTMLTextAreaElement, opts: BackdropOptions = {}): Backdrop {
  const doc = ta.ownerDocument;
  const wrap = doc.createElement("div");
  wrap.className = "zen-lint-wrap";
  const mirror = doc.createElement("div");
  mirror.className = "zen-lint-mirror";
  mirror.setAttribute("aria-hidden", "true");
  const parent = ta.parentNode;
  const next = ta.nextSibling;
  if (parent) parent.insertBefore(wrap, ta);
  wrap.append(mirror, ta);

  let alerts: Alert[] = [];
  let ranges: Range[] = [];
  let text = ta.value;
  let marks: HTMLElement[] = [];

  /** Moves ranges computed for `from` onto `to`; ranges touching the edit are dropped. */
  function remap(from: string, to: string, list: Range[]): Range[] {
    if (from === to) return list;
    let p = 0;
    while (p < from.length && p < to.length && from[p] === to[p]) p++;
    let q = 0;
    while (q < from.length - p && q < to.length - p && from[from.length - 1 - q] === to[to.length - 1 - q]) q++;
    const oldEnd = from.length - q;
    const delta = to.length - from.length;
    const out: Range[] = [];
    for (const r of list) {
      if (r.end <= p) out.push(r);
      else if (r.start >= oldEnd) out.push({ ...r, start: r.start + delta, end: r.end + delta });
    }
    return out;
  }

  const decorated = Boolean(opts.highlight || opts.lineNumbers);
  if (opts.highlight) mirror.classList.add("is-highlighted");
  if (opts.lineNumbers) mirror.classList.add("has-line-numbers");

  function render(): void {
    ensureSynced();
    marks = [];
    if (!ranges.length && !decorated) {
      mirror.replaceChildren();
      return;
    }
    const toks = opts.highlight ? opts.highlight(text) : [];
    // Every place where the class, the mark or the line changes.
    const cuts = new Set<number>([0, text.length]);
    for (const t of toks) { cuts.add(t.start); cuts.add(t.end); }
    for (const r of ranges) { cuts.add(r.start); cuts.add(r.end); }
    if (opts.lineNumbers) {
      for (let i = text.indexOf("\n"); i >= 0; i = text.indexOf("\n", i + 1)) { cuts.add(i); cuts.add(i + 1); }
    }
    const points = Array.from(cuts).filter((n) => n >= 0 && n <= text.length).sort((a, b) => a - b);

    const out: Node[] = [];
    let lineNo = 1;
    let line: HTMLElement | null = null;
    const newLine = () => {
      line = doc.createElement("div");
      line.className = "zen-ed-line";
      line.dataset.n = String(lineNo++);
      out.push(line);
    };
    const endLine = () => {
      // An empty row needs content to keep its height.
      if (line && !line.firstChild) line.append(doc.createTextNode("\u200b"));
    };
    if (opts.lineNumbers) newLine();
    const put = (n: Node) => (line ? line.append(n) : out.push(n));

    let ti = 0;
    let ri = 0;
    for (let k = 0; k + 1 < points.length; k++) {
      const a = points[k];
      const b = points[k + 1];
      if (b <= a) continue;
      const piece = text.slice(a, b);
      if (opts.lineNumbers && piece === "\n") {
        endLine();
        newLine();
        continue;
      }
      while (ti < toks.length && toks[ti].end <= a) ti++;
      while (ri < ranges.length && ranges[ri].end <= a) ri++;
      const tok = ti < toks.length && toks[ti].start <= a ? toks[ti] : null;
      const r = ri < ranges.length && ranges[ri].start <= a ? ranges[ri] : null;
      let node: Node = doc.createTextNode(piece);
      if (tok) {
        const span = doc.createElement("span");
        span.className = tok.cls;
        span.append(node);
        node = span;
      }
      if (r) {
        const m = makeMark(doc, r.alerts.map((i) => alerts[i]));
        m.append(node);
        for (const i of r.alerts) if (!marks[i]) marks[i] = m;
        node = m;
      }
      put(node);
    }
    if (opts.lineNumbers) endLine();
    // The placeholder keeps a trailing empty line as tall as in the textarea.
    else out.push(doc.createTextNode("\u200b"));
    mirror.replaceChildren(...out);
    syncScroll();
  }

  function refresh(): void {
    const now = ta.value;
    if (now === text) return;
    ranges = remap(text, now, ranges);
    text = now;
    render();
  }

  function syncScroll(): void {
    mirror.scrollTop = ta.scrollTop;
    mirror.scrollLeft = ta.scrollLeft;
  }

  /** What the last copy was taken from; a change means the copy is stale. */
  let synced = "";
  function shape(cs: CSSStyleDeclaration): string {
    return [cs.font, cs.lineHeight, cs.paddingLeft, cs.paddingRight, ta.clientWidth, ta.offsetWidth].join("|");
  }

  /**
   * Copies the textarea's typography and box to the mirror. Does nothing
   * (and stays due) while the textarea is not in the page: a detached
   * element has no computed style, which left the mirror in the UI font with
   * no gutter, so the caret drifted from the drawn text.
   */
  function sync(): void {
    if (!ta.isConnected) {
      synced = "";
      return;
    }
    const view = doc.defaultView;
    const cs = view?.getComputedStyle(ta);
    if (!cs || !cs.font && !cs.fontFamily) return;
    for (const p of MIRRORED) {
      const v = cs[p] || ta.style[p];
      if (v) mirror.style[p] = v;
    }
    // A scrollbar narrows the textarea's text; give the mirror the same width.
    const bars = ta.offsetWidth - ta.clientWidth - (parseFloat(cs.borderLeftWidth) || 0) - (parseFloat(cs.borderRightWidth) || 0);
    if (bars > 0) mirror.style.paddingRight = `${(parseFloat(cs.paddingRight) || 0) + bars}px`;
    synced = shape(cs);
    syncScroll();
  }

  /** Re-syncs when the copy is due or the textarea's shape changed since. */
  function ensureSynced(): void {
    if (!ta.isConnected) return;
    const cs = doc.defaultView?.getComputedStyle(ta);
    if (!synced || (cs && shape(cs) !== synced)) sync();
  }

  const onInput = () => refresh();
  ta.addEventListener("input", onInput);
  ta.addEventListener("scroll", syncScroll);
  const ro = typeof ResizeObserver === "function" ? new ResizeObserver(() => sync()) : null;
  ro?.observe(ta);
  // Not only the observer: a window created hidden may not deliver it.
  const onResize = () => ensureSynced();
  doc.defaultView?.addEventListener("resize", onResize);
  void (doc as Document & { fonts?: { ready?: Promise<unknown> } }).fonts?.ready?.then(() => sync());
  sync();
  if (decorated) render();

  return {
    el: wrap,
    mirror,
    update(next, forText = ta.value) {
      alerts = next.slice();
      const byPos = new Map<string, Range>();
      for (let i = 0; i < alerts.length; i++) {
        const r = alertRange(forText, alerts[i]);
        if (!r) continue;
        const key = `${r[0]}:${r[1]}`;
        const have = byPos.get(key);
        if (have) have.alerts.push(i);
        else byPos.set(key, { start: r[0], end: r[1], alerts: [i] });
      }
      // Overlaps join the earlier (wider first) range.
      const sorted = Array.from(byPos.values()).sort((a, b) => a.start - b.start || b.end - a.end);
      const merged: Range[] = [];
      for (const r of sorted) {
        const last = merged[merged.length - 1];
        if (last && r.start < last.end) {
          last.end = Math.max(last.end, r.end);
          last.alerts.push(...r.alerts);
        } else merged.push({ ...r, alerts: [...r.alerts] });
      }
      text = ta.value;
      ranges = remap(forText, text, merged);
      render();
    },
    refresh,
    sync,
    range(i) {
      const r = ranges.find((x) => x.alerts.includes(i));
      return r ? [r.start, r.end] : null;
    },
    mark: (i) => marks[i] ?? null,
    select(i) {
      refresh();
      const r = ranges.find((x) => x.alerts.includes(i));
      if (!r) return;
      ta.focus();
      ta.setSelectionRange(r.start, r.end);
      const m = marks[i];
      if (m && ta.clientHeight > 0) {
        const top = m.offsetTop;
        if (top < ta.scrollTop || top > ta.scrollTop + ta.clientHeight - m.offsetHeight) {
          ta.scrollTop = Math.max(0, top - ta.clientHeight / 3);
          syncScroll();
        }
      }
    },
    destroy() {
      ta.removeEventListener("input", onInput);
      ta.removeEventListener("scroll", syncScroll);
      ro?.disconnect();
      doc.defaultView?.removeEventListener("resize", onResize);
      if (wrap.parentNode) wrap.parentNode.insertBefore(ta, wrap);
      else if (parent) parent.insertBefore(ta, next);
      wrap.remove();
    },
  };
}
