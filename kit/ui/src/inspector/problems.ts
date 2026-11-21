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
import type { ProblemTarget } from "./events";

export type ProblemKind = "link" | "image" | "spelling" | "warning";
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
  /** Warnings: what to do about it, when it is known. */
  fix?: string;
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

/** What to do about a render warning the renderer is known to give; "" for any other. */
export function warningFix(message: string): string {
  if (/^dialect \S+ is not in this build/.test(message)) return "Settings › Reader, or remove it from .zenmd.yml";
  if (/^This folder's settings are in /.test(message)) return "It is renamed the next time a folder setting is saved, or rename it yourself.";
  if (/^unknown layout\b/.test(message)) return "Settings › Layouts, or change layout: in the front matter";
  return "";
}

/** The render warnings of a page (`Page.warnings`) as problems: a dialect not in the build, an unknown layout. Plain text, as they come. */
export function warningProblems(warnings: readonly string[]): Problem[] {
  return warnings.map((message) => {
    const fix = warningFix(message);
    return { kind: "warning", severity: "warning", label: message, message, where: "Page", rule: "page-warning", ...(fix ? { fix } : {}) };
  });
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

/**
 * What the page needs to find a problem again (INSPECTOR_MARK): values only, compared in JS by the page.
 * `offset` tells equal links or images apart (the nth with the same href or src); for spelling it is the alert's index.
 */
export function problemTarget(p: Problem, root: ParentNode | null): ProblemTarget {
  const base = { kind: p.kind, severity: p.severity };
  if (p.kind === "spelling" && p.alert) return { ...base, text: p.alert.match, offset: p.index ?? 0, length: Array.from(p.alert.match).length };
  const el = p.el;
  if (!el || !root) return base;
  if (p.kind === "link") {
    const href = el.getAttribute("href") ?? "";
    const label = text(el);
    const same = Array.from(root.querySelectorAll<HTMLElement>("a")).filter((a) => a.getAttribute("data-zen-link") === "broken" && (a.getAttribute("href") ?? "") === href && text(a) === label);
    return { ...base, href, text: label, offset: Math.max(0, same.indexOf(el)) };
  }
  if (p.kind === "image") {
    const src = el.getAttribute("src") ?? "";
    const same = Array.from(root.querySelectorAll<HTMLElement>("img")).filter((i) => (i.getAttribute("src") ?? "") === src);
    return { ...base, src, text: (el.getAttribute("alt") ?? "").trim(), offset: Math.max(0, same.indexOf(el)) };
  }
  return base;
}
