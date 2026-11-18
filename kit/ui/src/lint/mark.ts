// Marks Vale alerts in rendered HTML (the reader). Only text nodes outside
// code, pre and plugin blocks are touched; marks are built with DOM APIs and
// Vale's text only ever reaches attributes and textContent.
import type { Alert } from "../host";

export type Severity = Alert["severity"];

const RANK: Record<Severity, number> = { error: 3, warning: 2, suggestion: 1 };

/** Vale's severity, anything unknown as "suggestion" (it ends up in a class name). */
export function severityOf(s: string): Severity {
  return s === "error" || s === "warning" ? s : "suggestion";
}

/** "<Check>: <Message>". */
export function titleOf(a: Alert): string {
  return a.check ? `${a.check}: ${a.message}` : a.message;
}

/** The most severe of the alerts. */
export function worst(alerts: Alert[]): Severity {
  return alerts.map((a) => severityOf(a.severity)).reduce((w, s) => (RANK[s] > RANK[w] ? s : w), "suggestion" as Severity);
}

/** A mark for alerts; text is added by the caller. */
export function makeMark(doc: Document, alerts: Alert[]): HTMLElement {
  const m = doc.createElement("mark");
  m.className = `zen-lint zen-lint-${worst(alerts)}`;
  m.setAttribute("title", alerts.map(titleOf).join("\n"));
  return m;
}

const SKIP = new Set(["CODE", "PRE", "KBD", "SAMP", "SCRIPT", "STYLE", "TEXTAREA", "svg", "SVG", "MATH"]);
const BLOCK = new Set([
  "ADDRESS", "ARTICLE", "ASIDE", "BLOCKQUOTE", "BR", "DD", "DETAILS", "DIV", "DL", "DT", "FIGCAPTION", "FIGURE", "FOOTER",
  "H1", "H2", "H3", "H4", "H5", "H6", "HEADER", "HR", "LI", "MAIN", "NAV", "OL", "P", "SECTION", "SUMMARY",
  "TABLE", "TBODY", "TD", "TFOOT", "TH", "THEAD", "TR", "UL", "IMG",
]);
/** Never part of a match: separates blocks and skipped elements. */
const GAP = "\u0000";

type Seg = { node: Text; start: number };

function skipped(e: Element): boolean {
  return SKIP.has(e.tagName) || e.hasAttribute("data-zen-plugin") || e.classList.contains("zen-plugin-error");
}

/** The article's markable text, with a GAP wherever a match must not run across. */
function collect(root: Element): { text: string; segs: Seg[] } {
  let text = "";
  const segs: Seg[] = [];
  const gap = () => { if (text && !text.endsWith(GAP)) text += GAP; };
  const visit = (n: Node) => {
    for (let c = n.firstChild; c; c = c.nextSibling) {
      if (c.nodeType === Node.TEXT_NODE) {
        const t = c as Text;
        if (t.data) {
          segs.push({ node: t, start: text.length });
          text += t.data;
        }
      } else if (c.nodeType === Node.ELEMENT_NODE) {
        const e = c as Element;
        if (skipped(e)) { gap(); continue; }
        const block = BLOCK.has(e.tagName);
        if (block) gap();
        visit(e);
        if (block) gap();
      }
    }
  };
  visit(root);
  return { text, segs };
}

const WORD = new RegExp("[\\p{L}\\p{N}_]", "u");
const isWord = (ch: string | undefined) => ch !== undefined && WORD.test(ch);

type Placed = { start: number; end: number; alerts: number[] };

/** Removes the marks markText added. */
export function clearMarks(root: Element): void {
  const parents = new Set<Node>();
  for (const m of Array.from(root.querySelectorAll("mark.zen-lint"))) {
    const p = m.parentNode;
    if (!p) continue;
    while (m.firstChild) p.insertBefore(m.firstChild, m);
    m.remove();
    parents.add(p);
  }
  parents.forEach((p) => p.normalize());
}

/**
 * Wraps each alert's Match in <mark class="zen-lint zen-lint-<severity>">.
 * The rendered page has no source positions, so lines are walked in order:
 * the k-th alert for a word on a line takes the k-th occurrence after where
 * the previous line's marks ended (falling back to any free occurrence).
 * Returns, per alert, its (first) mark, or null when it could not be placed.
 */
export function markText(root: HTMLElement, alerts: Alert[]): (HTMLElement | null)[] {
  clearMarks(root);
  const { text, segs } = collect(root);
  const placed: Placed[] = [];
  const at = new Map<string, Placed>();

  const free = (s: number, e: number) => placed.every((p) => e <= p.start || s >= p.end || (p.start === s && p.end === e));
  const find = (m: string, from: number): number => {
    for (let i = text.indexOf(m, from); i >= 0; i = text.indexOf(m, i + 1)) {
      const e = i + m.length;
      if (isWord(m[0]) && isWord(text[i - 1])) continue;
      if (isWord(m[m.length - 1]) && isWord(text[e])) continue;
      if (free(i, e)) return i;
    }
    return -1;
  };
  const place = (i: number, s: number, e: number): Placed => {
    let p = placed.find((x) => x.start === s && x.end === e);
    if (!p) placed.push((p = { start: s, end: e, alerts: [] }));
    p.alerts.push(i);
    return p;
  };

  const order = alerts.map((_, i) => i).sort((a, b) => alerts[a].line - alerts[b].line || alerts[a].col - alerts[b].col);
  let cursor = 0;
  for (let k = 0; k < order.length;) {
    const line = alerts[order[k]].line;
    const from = new Map<string, number>();
    let lineEnd = cursor;
    for (; k < order.length && alerts[order[k]].line === line; k++) {
      const i = order[k];
      const a = alerts[i];
      const m = a.match;
      if (!m || m.includes(GAP)) continue;
      const key = `${a.line}:${a.col}:${a.endCol}:${m}`;
      const same = at.get(key);
      if (same) { place(i, same.start, same.end); continue; }
      let s = find(m, from.get(m) ?? cursor);
      if (s >= 0) {
        from.set(m, s + m.length);
        lineEnd = Math.max(lineEnd, s + m.length);
      } else {
        s = find(m, 0);
        if (s < 0) continue;
      }
      at.set(key, place(i, s, s + m.length));
    }
    cursor = lineEnd;
  }

  const result: (HTMLElement | null)[] = alerts.map(() => null);
  const doc = root.ownerDocument;
  placed.sort((a, b) => b.start - a.start);
  for (const p of placed) {
    let first: HTMLElement | null = null;
    for (let j = segs.length - 1; j >= 0; j--) {
      const seg = segs[j];
      const ls = Math.max(p.start, seg.start) - seg.start;
      const le = Math.min(p.end, seg.start + seg.node.data.length) - seg.start;
      if (le <= ls) continue;
      // Splitting keeps seg.node as the part before ls, so earlier offsets stay valid.
      if (le < seg.node.data.length) seg.node.splitText(le);
      const piece = ls > 0 ? seg.node.splitText(ls) : seg.node;
      const mark = makeMark(doc, p.alerts.map((i) => alerts[i]));
      piece.parentNode!.insertBefore(mark, piece);
      mark.append(piece);
      first = mark;
    }
    if (first) for (const i of p.alerts) result[i] = first;
  }
  return result;
}

/** Briefly highlights a mark (class zen-lint-flash). */
export function flash(mark: HTMLElement, ms = 1200): void {
  mark.classList.add("zen-lint-flash");
  setTimeout(() => mark.classList.remove("zen-lint-flash"), ms);
}
