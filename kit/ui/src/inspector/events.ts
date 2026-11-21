import type { ProblemKind } from "./problems";

// How the top bar's Inspector button and the page's inspector find each other without knowing
// each other: two DOM events on the document. The inspector announces its state; the button
// asks for a toggle. Also the accessible name both buttons share.

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

/** "Inspector", or "Inspector, 3 issues". */
export function inspectorLabel(count: number): string {
  return count > 0 ? `Inspector, ${count} ${count === 1 ? "issue" : "issues"}` : "Inspector";
}
