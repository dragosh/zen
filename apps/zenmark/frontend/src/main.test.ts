import { readFileSync } from "node:fs";
import { expect, test } from "vitest";

const main = readFileSync(new URL("./main.ts", import.meta.url), "utf8");

// ui/accessibility 13: the options are on <html> before the shell mounts, so a window never shows the defaults first.
// (shell.ts belongs to ui/nav-panel, so main.ts does it, next to followTheme.)
test("accessibility applied before mount", () => {
  const follow = main.indexOf("followAccessibility(");
  expect(follow).toBeGreaterThan(-1);
  expect(follow).toBeLessThan(main.indexOf("createShell("));
  expect(follow).toBeLessThan(main.indexOf("getApi()"));
  expect(main).toContain("followTheme()");
});

// The View menu sends these three events to every window; the window with the focus steps the zoom (Events.On).
test("zoom menu events are listened for", () => {
  for (const e of ["menu:zoom-in", "menu:zoom-out", "menu:zoom-reset"]) expect(main, e).toContain(`"${e}"`);
  expect(main).toContain("Events.On(");
  expect(main).toContain("Window.SetZoom(");
  expect(main).toContain("zoomKey(");
});
