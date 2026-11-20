import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("./inspector.css", import.meta.url), "utf8");
const body = css.replace(/\/\*[\s\S]*?\*\//g, "");

test("tokens only: no colour literals, only known variables", () => {
  expect(body).not.toMatch(/#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(|oklch\(|lab\(|color\(/i);
  const known = ["--bg", "--bg-subtle", "--fg", "--fg-muted", "--rule", "--font-ui", "--font-mono", "--radius", "--btn-bg", "--btn-bg-hover", "--focus", "--lint-error", "--lint-warning", "--lint-suggestion"]
    .concat(Array.from({ length: 7 }, (_, i) => `--space-${i + 1}`));
  for (const m of Array.from(body.matchAll(/var\((--[a-z0-9-]+)/g))) expect(known).toContain(m[1]);
});

test("a bottom panel (ui/panel) under the page, never printed", () => {
  expect(body).toMatch(/\.zen-inspector\s*\{\s*order:\s*1;\s*\}/);
  expect(body).not.toMatch(/\.zen-dev\b/);
  expect(body).not.toMatch(/position:\s*absolute;[^}]*right:\s*0;[^}]*width/);
  expect(body).toMatch(/@media print[\s\S]*\.zen-inspector[\s\S]*display:\s*none/);
});

test("is hidden whenever the main pane is (Settings)", () => {
  expect(body).toMatch(/\.zen-main\[hidden\]/);
});

test("the editor window is a flex column, so the panel docks at its bottom", () => {
  const editor = readFileSync(new URL("../editor/editor.css", import.meta.url), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
  expect(editor).toMatch(/\.zen-editor-window\s*\{[^}]*display:\s*flex;[^}]*flex-direction:\s*column/);
});

test("consistent padding", () => {
  const panel = readFileSync(new URL("../panel/panel.css", import.meta.url), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
  const both = `${panel}\n${body}`;
  // one padding rule for .zen-panel-col, in the panel's css, and nothing that overrides it per tab or column
  const rules = Array.from(both.matchAll(/([^{}]*\.zen-panel-col\b[^{}]*)\{([^}]*)\}/g)).filter((m) => /(^|[\s,])padding[a-z-]*:/.test(m[2]) || /(^|;)\s*padding/.test(m[2]));
  expect(rules).toHaveLength(1);
  expect(rules[0][1].trim()).toBe(".zen-panel-col");
  expect(rules[0][2]).toMatch(/padding:\s*var\(--space-2\)\s+var\(--space-3\)/);
  expect(both).not.toMatch(/\.zen-panel-col-(main|side)[^{}]*\{[^}]*padding/);
  expect(body).not.toMatch(/\[data-section=[^{}]*\{[^}]*padding/);
  // a tab without columns (a body of its own) has the same padding
  expect(panel).toMatch(/\.zen-panel-body\s*\{[^}]*padding:\s*var\(--space-2\)\s+var\(--space-3\)/);
  // the list rows reach the column's edge by cancelling that same padding, not another number
  expect(body).toMatch(/\.zen-problem-list\s*\{[^}]*margin:\s*calc\(-1 \* var\(--space-2\)\)\s+calc\(-1 \* var\(--space-3\)\)/);
  expect(body).toMatch(/\.zen-problem\s*\{[^}]*padding:\s*[^;]*var\(--space-3\)/);
});
