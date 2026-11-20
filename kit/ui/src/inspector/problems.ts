// The problems of a page the Inspector lists (ui/inspector): Checks, found in the rendered article, and
// Spelling, the Vale alerts. Pure: it reads the DOM it is given and builds plain data; no DOM is changed.
//
// Broken links: the renderer marks them itself, `a[data-zen-link="broken"]` (engine/render/nodes.go;
// runtime page.css mutes them with a dotted underline). Missing images: the renderer does not mark them
// (an image whose file is not in the folder becomes its alt text; a /_ws/ file that is gone fails in the
// webview), so an <img> that failed to load is a missing image: an `error` event while the page is shown, or
// one that has already finished with no pixels.
import type { Alert } from "../host";
import { severityOf } from "../lint";

export type ProblemKind = "link" | "image" | "spelling";
export type ProblemSeverity = "error" | "warning" | "suggestion";

export type Problem = {
  kind: ProblemKind;
  severity: ProblemSeverity;
  /** The short text of the list row. */
  label: string;
  message: string;
  /** "Line 3, column 5", or the element ("Link: Gone page"). */
  where: string;
  /** broken-link, missing-image, or the Vale check ("Vale.Spelling"). */
  rule: string;
  /** Checks: the element in the page. */
  el?: HTMLElement;
  /** Spelling: the alert and its index in the list the host gave. */
  alert?: Alert;
  index?: number;
};

export type Filter = "all" | "checks" | "spelling";
export const isCheck = (p: Problem): boolean => p.kind !== "spelling";

const text = (e: Element): string => (e.textContent ?? "").replace(/\s+/g, " ").trim();

export function brokenLinks(root: ParentNode): Problem[] {
  return Array.from(root.querySelectorAll<HTMLElement>('a[data-zen-link="broken"]')).map((a) => {
    const label = text(a) || a.getAttribute("href") || "(empty link)";
    return {
      kind: "link", severity: "warning", label, el: a, rule: "broken-link",
      message: "Broken link: the target is not a page or file of this folder, or its address is not allowed.",
      where: `Link: ${label}`,
    };
  });
}

/** An image that has already finished loading with nothing to show (SVG files can have no intrinsic size, so they are left to the error event). */
export function failedAlready(img: HTMLImageElement): boolean {
  const src = img.getAttribute("src") ?? "";
  return src !== "" && !/\.svg(?:[?#]|$)/i.test(src) && img.complete === true && img.naturalWidth === 0;
}

export function missingImage(img: HTMLImageElement): Problem {
  const alt = (img.getAttribute("alt") ?? "").trim();
  const label = alt || img.getAttribute("src") || "(image)";
  return {
    kind: "image", severity: "warning", label, el: img, rule: "missing-image",
    message: "Missing image: the file could not be loaded.",
    where: `Image: ${label}`,
  };
}

export function spellingProblems(alerts: readonly Alert[]): Problem[] {
  return alerts.map((a, index) => ({
    kind: "spelling", severity: severityOf(a.severity), label: a.match, alert: a, index, rule: a.check,
    message: a.message, where: `Line ${a.line}, column ${a.col}`,
  }));
}

/** The same problem again after a refresh (the page is rendered anew, the alerts arrive anew). */
export function sameProblem(a: Problem, b: Problem): boolean {
  if (a.kind !== b.kind) return false;
  if (a.alert && b.alert) return a.alert.line === b.alert.line && a.alert.col === b.alert.col && a.alert.check === b.alert.check;
  if (a.el && b.el && a.el === b.el) return true;
  return a.rule === b.rule && a.label === b.label;
}
