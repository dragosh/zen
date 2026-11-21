// The app's line icons, one set (ui/top-bar-actions): a 24x24 viewBox drawn at 16px, stroke currentColor,
// width 1.5, no fill, round caps and joins. Built with DOM APIs, never innerHTML.

export type IconName = "cog" | "export" | "search" | "pencil" | "panel" | "printer";

const SVG = "http://www.w3.org/2000/svg";

type Shape = { tag: "path"; d: string } | { tag: "circle"; cx: number; cy: number; r: number } | { tag: "rect"; x: number; y: number; w: number; h: number; r: number };

const SHAPES: Record<IconName, Shape[]> = {
  // a gear outline with a round hub
  cog: [
    { tag: "path", d: "M10.3 2.6h3.4l.5 2.6 1.6.9 2.5-.9 1.7 2.9-2 1.7v1.9l2 1.7-1.7 2.9-2.5-.9-1.6.9-.5 2.6h-3.4l-.5-2.6-1.6-.9-2.5.9-1.7-2.9 2-1.7v-1.9l-2-1.7 1.7-2.9 2.5.9 1.6-.9z" },
    { tag: "circle", cx: 12, cy: 12, r: 3 },
  ],
  // a tray with an arrow leaving it
  export: [
    { tag: "path", d: "M5 13v6h14v-6" },
    { tag: "path", d: "M12 15V3M7.5 7.5 12 3l4.5 4.5" },
  ],
  // a magnifying glass: a lens and its handle
  search: [
    { tag: "circle", cx: 10.5, cy: 10.5, r: 6.5 },
    { tag: "path", d: "M15.4 15.4 21 21" },
  ],
  // a pencil on the diagonal, with the line where the tip starts
  pencil: [
    { tag: "path", d: "M4 20l.9-3.9L16 5a2.1 2.1 0 0 1 3 3L7.9 19.1z" },
    { tag: "path", d: "M14.5 6.5l3 3" },
  ],
  // a window with a bottom panel (a rule across it) showing a >_ prompt
  panel: [
    { tag: "rect", x: 3, y: 4, w: 18, h: 16, r: 2 },
    { tag: "path", d: "M3 13.5h18" },
    { tag: "path", d: "M6.5 15.3l2 1.6-2 1.6M11 18.6h3.5" },
  ],
  // a printer: the paper going in at the top, the body, and the printed sheet coming out
  printer: [
    { tag: "path", d: "M7 8V3.5h10V8" },
    { tag: "rect", x: 3, y: 8, w: 18, h: 9, r: 2 },
    { tag: "path", d: "M7 14h10v6.5H7z" },
  ],
};

export function icon(doc: Document, name: IconName): SVGSVGElement {
  const svg = doc.createElementNS(SVG, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("width", "16");
  svg.setAttribute("height", "16");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("class", "zen-icon");
  for (const s of SHAPES[name]) {
    const n = doc.createElementNS(SVG, s.tag);
    if (s.tag === "path") n.setAttribute("d", s.d);
    else if (s.tag === "circle") {
      n.setAttribute("cx", String(s.cx));
      n.setAttribute("cy", String(s.cy));
      n.setAttribute("r", String(s.r));
    } else {
      n.setAttribute("x", String(s.x));
      n.setAttribute("y", String(s.y));
      n.setAttribute("width", String(s.w));
      n.setAttribute("height", String(s.h));
      n.setAttribute("rx", String(s.r));
    }
    n.setAttribute("fill", "none");
    n.setAttribute("stroke", "currentColor");
    n.setAttribute("stroke-width", "1.5");
    n.setAttribute("stroke-linecap", "round");
    n.setAttribute("stroke-linejoin", "round");
    svg.append(n);
  }
  return svg;
}
