import type { ProblemKind } from "./problems";

// How the sidebar's Inspector button and the page's inspector find each other without knowing
// each other: two DOM events on the document. The inspector announces its state; the button
// asks for a toggle. Also the icon and the accessible name both buttons share.

/** inspector -> button. detail: InspectorState. */
export const INSPECTOR_STATE = "zen:inspector-state";
/** button -> inspector. No detail; ignored while no page is inspectable. */
export const INSPECTOR_TOGGLE = "zen:inspector-toggle";
/** anyone -> inspector: close the panel. */
export const INSPECTOR_CLOSE = "zen:inspector-close";
/** anyone (the Develop menu) -> inspector: open on the Console tab and focus it; ignored while the Console is not there. */
export const INSPECTOR_CONSOLE = "zen:inspector-console";

/**
 * Inspector -> page: show this problem in the page (scroll to it and mark it); null clears the mark.
 * The page finds the element or text by comparing values in JS, never with a selector built from them.
 * link: `href` and `text` (the link text) of a broken link, `offset` the nth link with the same two;
 * image: `src` and `text` (alt), `offset` the nth with the same src; spelling: `text` the word, `offset`
 * the index of the Vale alert (the page's lint marks share it) and `length` the word's length;
 * a page warning names no element and has neither.
 */
export const INSPECTOR_MARK = "zen:inspector-mark"; // detail: ProblemTarget | null

export type ProblemTarget = { kind: ProblemKind; severity: "error" | "warning" | "suggestion"; href?: string; src?: string; text?: string; offset?: number; length?: number };

export type InspectorState = { available: boolean; open: boolean; count: number };

const SVG = "http://www.w3.org/2000/svg";

/** A magnifier over a page: the Inspector's icon, same stroke as the cog and the terminal. */
export function inspectorIcon(doc: Document): SVGSVGElement {
  const svg = doc.createElementNS(SVG, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("width", "16");
  svg.setAttribute("height", "16");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("class", "zen-icon");
  const page = doc.createElementNS(SVG, "path");
  page.setAttribute("d", "M12 20.5H6.5a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2h7l4.5 4.5V10");
  const lens = doc.createElementNS(SVG, "circle");
  lens.setAttribute("cx", "16");
  lens.setAttribute("cy", "16.5");
  lens.setAttribute("r", "3.2");
  const handle = doc.createElementNS(SVG, "path");
  handle.setAttribute("d", "M18.4 18.9l2.3 2.3");
  for (const n of [page, lens, handle]) {
    n.setAttribute("fill", "none");
    n.setAttribute("stroke", "currentColor");
    n.setAttribute("stroke-width", "1.6");
    n.setAttribute("stroke-linejoin", "round");
    n.setAttribute("stroke-linecap", "round");
    svg.append(n);
  }
  return svg;
}

/** "Inspector", or "Inspector, 3 issues". */
export function inspectorLabel(count: number): string {
  return count > 0 ? `Inspector, ${count} ${count === 1 ? "issue" : "issues"}` : "Inspector";
}
