// Small DOM helpers. Text always goes in through textContent.
import { icon, type IconName } from "./icons";

export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K, className = "", text = "",
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text) node.textContent = text;
  return node;
}

export function button(label: string, onClick: () => void, opts: { className?: string; text?: string; title?: string } = {}): HTMLButtonElement {
  const b = el("button", opts.className ?? "", opts.text ?? label);
  b.type = "button";
  if (opts.text !== undefined && opts.text !== label) b.setAttribute("aria-label", label);
  if (opts.title) b.title = opts.title;
  b.addEventListener("click", onClick);
  return b;
}

/**
 * An icon button of the footer and the top bar (ui/top-bar-actions): no visible text, the name is the aria-label,
 * the tooltip the title; `.zen-settings-button` gives its size, focus ring and on style (aria-pressed / aria-expanded).
 */
export function iconButton(name: IconName, label: string, title: string, onClick: () => void, extraClass = ""): HTMLButtonElement {
  const b = button(label, onClick, { className: `ghost zen-settings-button${extraClass ? " " + extraClass : ""}`, text: "", title });
  b.append(icon(document, name));
  return b;
}

const SVG = "http://www.w3.org/2000/svg";

/** The zen mark: yin and yang reduced to two shapes (docs/ui/mockup.html). */
export function logo(className: string, strokeWidth = 1.5): SVGSVGElement {
  const svg = document.createElementNS(SVG, "svg");
  svg.setAttribute("class", className);
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("aria-hidden", "true");
  const circle = document.createElementNS(SVG, "circle");
  circle.setAttribute("cx", "12");
  circle.setAttribute("cy", "12");
  circle.setAttribute("r", "11");
  circle.setAttribute("fill", "none");
  circle.setAttribute("stroke", "currentColor");
  circle.setAttribute("stroke-width", String(strokeWidth));
  const path = document.createElementNS(SVG, "path");
  path.setAttribute("d", "M12 1a11 11 0 0 1 0 22a5.5 5.5 0 0 1 0-11a5.5 5.5 0 0 0 0-11z");
  path.setAttribute("fill", "currentColor");
  svg.append(circle, path);
  return svg;
}
