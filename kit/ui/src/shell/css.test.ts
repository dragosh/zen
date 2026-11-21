import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const shellCss = readFileSync(new URL("../styles/shell.css", import.meta.url), "utf8");

// UI consistency (docs/UI.md): empty states in the panel are the panel's own (.zen-panel-empty), never restyled per tab.
test("the Console has no empty-state padding of its own", () => {
  expect(shellCss).not.toMatch(/\.zen-dev-empty|\.zen-console-hint/);
});

// The top bar's icons stay at the right edge whatever else is hidden (Settings hides the breadcrumb, which used to
// push them there): the group pushes itself right.
test("the top bar's action group is right-aligned on its own", () => {
  const rule = shellCss.match(/\.zen-top-actions\s*\{[^}]*\}/)?.[0] ?? "";
  expect(rule).toMatch(/margin-left:\s*auto/);
});
