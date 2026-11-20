import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const shellCss = readFileSync(new URL("../styles/shell.css", import.meta.url), "utf8");

// UI consistency (docs/UI.md): empty states in the panel are the panel's own (.zen-panel-empty), never restyled per tab.
test("the Console has no empty-state padding of its own", () => {
  expect(shellCss).not.toMatch(/\.zen-dev-empty|\.zen-console-hint/);
});
