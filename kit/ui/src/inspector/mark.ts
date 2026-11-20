// A problem shown in the page (ui/inspector-details): the reader gets INSPECTOR_MARK from the Inspector,
// finds the link, image or word it names, scrolls to it and marks it the way the Vale marks look: a wavy
// underline in the problem's lint colour, plus a brief background flash. The marks are CSS Custom Highlights
// (as find's), so the page's DOM is not changed; a missing image gets an outline through an attribute.
// Without the Highlight API the element is still scrolled to and focused. Nothing here builds a selector from
// a problem's text: attribute values and text are compared in JS.
import type { HighlightSupport } from "../find";
import type { ProblemTarget } from "./events";

/** The base highlight (the flash) and the prefix of the one that carries the severity's underline colour. */
export const HIGHLIGHT_PROBLEM = "zen-problem";
export const FLASH_MS = 1200;
/** The attribute that outlines a missing image (inspector.css). */
export const IMAGE_ATTR = "data-zen-problem";

/** The CSS Custom Highlight API; read on each call so a page without it (or a test stub) is honoured. */
function highlights(): HighlightSupport | null {
  const css = (globalThis as { CSS?: { highlights?: HighlightSupport["registry"] } }).CSS;
  const Ctor = (globalThis as { Highlight?: new (...r: Range[]) => unknown }).Highlight;
  if (!css?.highlights || typeof Ctor !== "function") return null;
  return { registry: css.highlights, create: (ranges) => new Ctor(...ranges) };
}

const SKIP = new Set(["SCRIPT", "STYLE", "TEMPLATE", "NOSCRIPT", "TEXTAREA"]);
const text = (e: Element): string => (e.textContent ?? "").replace(/\s+/g, " ").trim();

/** The nth element of `all` for which `same` holds (n = offset, 0 when none is given), else the first. */
function nth<T>(all: T[], same: (e: T) => boolean, offset: number | undefined): T | null {
  const hits = all.filter(same);
  return hits[offset !== undefined && offset >= 0 && offset < hits.length ? offset : 0] ?? null;
}

export type Resolved = { el: HTMLElement; range: Range | null };

/** The first text of the article that holds `needle`, as a range; skips code of the page's scripts and styles. */
function findText(root: HTMLElement, needle: string): Range | null {
  const doc = root.ownerDocument;
  const walk = (n: Node): Range | null => {
    for (let c = n.firstChild; c; c = c.nextSibling) {
      if (c.nodeType === 3) {
        const at = (c as Text).data.indexOf(needle);
        if (at >= 0) {
          const r = doc.createRange();
          r.setStart(c, at);
          r.setEnd(c, at + needle.length);
          return r;
        }
      } else if (c.nodeType === 1 && !SKIP.has((c as Element).tagName)) {
        const r = walk(c);
        if (r) return r;
      }
    }
    return null;
  };
  return needle ? walk(root) : null;
}

/** Where a problem is in the article, or null (a page warning, or a link or word that is gone). `marks` are the lint marks by alert index. */
export function resolveTarget(root: HTMLElement, t: ProblemTarget, marks: readonly (HTMLElement | null)[] = []): Resolved | null {
  const doc = root.ownerDocument;
  const around = (el: HTMLElement): Resolved => {
    const range = doc.createRange();
    range.selectNodeContents(el);
    return { el, range };
  };
  if (t.kind === "link") {
    const links = Array.from(root.querySelectorAll<HTMLElement>("a")).filter((a) => a.getAttribute("data-zen-link") === "broken");
    const a = nth(links, (e) => (e.getAttribute("href") ?? "") === (t.href ?? "") && (t.text === undefined || text(e) === t.text), t.offset);
    return a ? around(a) : null;
  }
  if (t.kind === "image") {
    const imgs = Array.from(root.querySelectorAll<HTMLElement>("img"));
    const img = nth(imgs, (e) => (e.getAttribute("src") ?? "") === (t.src ?? ""), t.offset);
    return img ? { el: img, range: null } : null;
  }
  if (t.kind === "spelling") {
    const m = t.offset !== undefined ? marks[t.offset] : null;
    if (m?.isConnected && root.contains(m)) return around(m);
    const range = t.text ? findText(root, t.text) : null;
    const host = range?.startContainer.parentElement;
    return range && host ? { el: host, range } : null;
  }
  return null;
}

export type ProblemMark = {
  /** Shows the problem (scroll and mark); null, or a target with nothing in the page, clears. */
  show(t: ProblemTarget | null): void;
  clear(): void;
  destroy(): void;
};

export function createProblemMark(o: {
  root(): HTMLElement | null;
  /** The Vale marks by alert index, as markText returned them. */
  marks(): readonly (HTMLElement | null)[];
  reduced(): boolean;
}): ProblemMark {
  let names: string[] = [];
  let outlined: HTMLElement | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;

  function clear(): void {
    clearTimeout(timer);
    const h = highlights();
    for (const n of names) h?.registry.delete(n);
    names = [];
    outlined?.removeAttribute(IMAGE_ATTR);
    outlined = null;
  }

  function show(t: ProblemTarget | null): void {
    clear();
    const root = o.root();
    if (!t || !root) return;
    const found = resolveTarget(root, t, o.marks());
    if (!found?.el.isConnected) return;
    const { el, range } = found;
    el.scrollIntoView?.({ behavior: o.reduced() ? "auto" : "smooth", block: "center" });
    const h = highlights();
    if (t.kind === "image") {
      el.setAttribute(IMAGE_ATTR, t.severity);
      outlined = el;
    }
    if (h && range) {
      const sev = `${HIGHLIGHT_PROBLEM}-${t.severity}`;
      h.registry.set(sev, h.create([range]));
      h.registry.set(HIGHLIGHT_PROBLEM, h.create([range]));
      names = [sev, HIGHLIGHT_PROBLEM];
      // The flash is the background only: it fades after a moment, and stays as it is when Reduce motion is on.
      if (!o.reduced()) {
        timer = setTimeout(() => {
          h.registry.delete(HIGHLIGHT_PROBLEM);
          names = names.filter((n) => n !== HIGHLIGHT_PROBLEM);
        }, FLASH_MS);
      }
    } else if (!h) {
      // No Highlight API: the element itself takes the focus, as the Show button used to give it.
      if (!el.hasAttribute("tabindex")) el.setAttribute("tabindex", "-1"); // a broken link has no href and an image is not focusable
      el.focus({ preventScroll: true });
    }
  }

  return { show, clear, destroy: clear };
}
