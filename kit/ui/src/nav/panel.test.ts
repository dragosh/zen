import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

test("panel.css: tokens only, the right side is mirrored, motion respects the setting", () => {
  const css = readFileSync(new URL("./panel.css", import.meta.url), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(/i);
  const known = ["--bg", "--bg-subtle", "--fg", "--fg-muted", "--rule", "--font-ui", "--radius", "--btn-bg", "--btn-bg-hover", "--focus"].concat(Array.from({ length: 7 }, (_, i) => `--space-${i + 1}`));
  for (const m of Array.from(css.matchAll(/var\((--[a-z0-9-]+)/g))) expect(known).toContain(m[1]);
  expect(css).toMatch(/data-nav-side="right"/);
  expect(css).toMatch(/\.zen-sidebar-head\s*\{[^}]*height:\s*44px/);
  expect(css).toMatch(/\.zen-sidebar-logo\s*\{[^}]*height:\s*24px/);
  expect(css).toMatch(/text-overflow:\s*ellipsis/);
});
