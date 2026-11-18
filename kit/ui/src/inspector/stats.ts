// What the Inspector counts in a rendered page. Pure: it reads the article's
// DOM and the Page, nothing else.
import type { Page } from "../host";

export type PageStats = {
  words: number;
  /** Reading time at 200 words a minute, rounded; 0 for an empty page, at least 1 otherwise. */
  minutes: number;
  headings: number;
  links: { internal: number; external: number };
  images: number;
  /** Ids of the plugins that drew something in the page, sorted. */
  plugins: string[];
  /** The page's front matter keys, in file order. */
  frontmatter: string[];
};

export const WORDS_PER_MINUTE = 200;

export const readingMinutes = (words: number): number => (words > 0 ? Math.max(1, Math.round(words / WORDS_PER_MINUTE)) : 0);

const EXTERNAL = /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i; // http:, https:, mailto: ... anything with a scheme

const BLOCKS = new Set(["ADDRESS", "ARTICLE", "ASIDE", "BLOCKQUOTE", "DD", "DIV", "DL", "DT", "FIGCAPTION", "FIGURE", "H1", "H2", "H3", "H4", "H5", "H6",
  "LI", "OL", "P", "PRE", "SECTION", "TABLE", "TD", "TH", "TR", "UL"]);

/** The nearest block element around a text node: text in different blocks is never one word, even without whitespace between the tags. */
function blockOf(n: Node): Node | null {
  for (let p = n.parentNode; p; p = p.parentNode) if (BLOCKS.has((p as Element).tagName)) return p;
  return null;
}

/** Whitespace-separated words of the text, counted per block so that "</h1><p>" is a break. */
export function countWords(root: Node): number {
  const walker = (root.ownerDocument ?? (root as Document)).createTreeWalker(root, 4 /* NodeFilter.SHOW_TEXT */);
  let words = 0;
  let carry = ""; // a word that continues across inline elements ("fo<b>o</b>")
  let block: Node | null | undefined;
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const b = blockOf(n);
    if (block !== undefined && b !== block) {
      if (carry) words++;
      carry = "";
    }
    block = b;
    const text = n.nodeValue ?? "";
    const parts = text.split(/\s+/);
    // parts[0] continues the carried word unless the text starts with whitespace.
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      if (i > 0 && carry) {
        words++;
        carry = "";
      }
      if (part) carry += part;
    }
  }
  return carry ? words + 1 : words;
}

export function pageStats(root: ParentNode, page: Page): PageStats {
  const words = countWords(root as Node);
  let internal = 0;
  let external = 0;
  for (const a of Array.from(root.querySelectorAll("a[href]"))) {
    if (EXTERNAL.test(a.getAttribute("href") ?? "")) external++;
    else internal++;
  }
  const plugins = new Set<string>();
  for (const n of Array.from(root.querySelectorAll("[data-zen-plugin]"))) {
    const id = n.getAttribute("data-zen-plugin");
    if (id) plugins.add(id);
  }
  return {
    words,
    minutes: readingMinutes(words),
    headings: root.querySelectorAll("h1, h2, h3, h4, h5, h6").length,
    links: { internal, external },
    images: root.querySelectorAll("img").length,
    plugins: Array.from(plugins).sort(),
    frontmatter: Object.keys(page.frontmatter ?? {}),
  };
}
