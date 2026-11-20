import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("./panel.css", import.meta.url), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");

test("panel.css uses tokens only", () => {
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(|oklch\(/i);
  const known = ["--bg", "--bg-subtle", "--fg", "--fg-muted", "--rule", "--font-ui", "--font-mono", "--radius", "--btn-bg", "--btn-bg-hover", "--focus", "--lint-error", "--lint-warning"]
    .concat(Array.from({ length: 7 }, (_, i) => `--space-${i + 1}`));
  for (const m of Array.from(css.matchAll(/var\((--[a-z0-9-]+)/g))) expect(known).toContain(m[1]);
});

test("never printed, and motion respects reduced-motion", () => {
  expect(css).toMatch(/@media print[\s\S]*\.zen-panel[\s\S]*display:\s*none/);
  expect(css).toMatch(/prefers-reduced-motion: reduce/);
});

test("two columns scroll on their own and stack under 700 px", () => {
  expect(css).toMatch(/\.zen-panel-col\s*\{[^}]*overflow:\s*auto/);
  expect(css).toMatch(/@container \(max-width:\s*700px\)/);
  expect(css).toMatch(/\.zen-panel\.is-collapsed/);
});
