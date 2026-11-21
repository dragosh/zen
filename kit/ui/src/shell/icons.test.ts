// @vitest-environment happy-dom
import { expect, test } from "vitest";
import { icon, type IconName } from "./icons";

const NAMES: IconName[] = ["cog", "export", "search", "pencil", "panel", "printer", "sidebar"];

// One icon set (ui/top-bar-actions): the same drawing for every icon of the app.
test.each(NAMES)("%s: 24 viewBox, 16px, stroke only, aria-hidden", (name) => {
  const svg = icon(document, name);
  expect(svg.namespaceURI).toBe("http://www.w3.org/2000/svg");
  expect(svg.getAttribute("viewBox")).toBe("0 0 24 24");
  expect(svg.getAttribute("width")).toBe("16");
  expect(svg.getAttribute("height")).toBe("16");
  expect(svg.getAttribute("aria-hidden")).toBe("true");
  expect(svg.textContent).toBe("");
  const parts = Array.from(svg.children);
  expect(parts.length).toBeGreaterThan(0);
  for (const p of parts) {
    expect(p.getAttribute("fill")).toBe("none");
    expect(p.getAttribute("stroke")).toBe("currentColor");
    expect(p.getAttribute("stroke-width")).toBe("1.5");
    expect(p.getAttribute("stroke-linecap")).toBe("round");
  }
});

test("every icon is its own drawing", () => {
  const drawn = NAMES.map((n) => Array.from(icon(document, n).children).map((c) => c.outerHTML).join(""));
  expect(new Set(drawn).size).toBe(NAMES.length);
});

test("a new svg each call", () => {
  expect(icon(document, "cog")).not.toBe(icon(document, "cog"));
});

// One source of truth: the drawings live in the runtime (the exported site draws the same sidebar icon), the kit re-exports them.
test("the kit's icon is the runtime's", async () => {
  const rt = await import("@zenmd/runtime/icons");
  expect(rt.icon).toBe(icon);
});
