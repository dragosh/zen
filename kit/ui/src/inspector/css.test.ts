import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("./inspector.css", import.meta.url), "utf8");
const body = css.replace(/\/\*[\s\S]*?\*\//g, "");

test("tokens only: no colour literals, only known variables", () => {
  expect(body).not.toMatch(/#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(|oklch\(|lab\(|color\(/i);
  const known = ["--bg", "--bg-subtle", "--fg", "--fg-muted", "--rule", "--font-ui", "--font-mono", "--radius", "--btn-bg", "--btn-bg-hover", "--focus", "--zen-inspector-width", "--zen-dev-height", "--lint-error", "--lint-warning", "--lint-suggestion"]
    .concat(Array.from({ length: 7 }, (_, i) => `--space-${i + 1}`));
  for (const m of Array.from(body.matchAll(/var\((--[a-z0-9-]+)/g))) expect(known).toContain(m[1]);
});

test("280px wide, takes room from the page above 900px, overlays below, never printed", () => {
  expect(body).toMatch(/--zen-inspector-width:\s*280px/);
  expect(body).toMatch(/@media \(min-width: 901px\)[\s\S]*margin-right:\s*var\(--zen-inspector-width\)/);
  expect(body).toMatch(/@media print[\s\S]*\.zen-inspector[\s\S]*display:\s*none/);
});

test("is hidden whenever the main pane is (Settings)", () => {
  expect(body).toMatch(/\.zen-main\[hidden\]/);
});

test("the editor window is the panel's positioning context", () => {
  const editor = readFileSync(new URL("../editor/editor.css", import.meta.url), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
  expect(editor).toMatch(/\.zen-editor-window\s*\{[^}]*position:\s*relative/);
});
