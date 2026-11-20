import { readFileSync } from "node:fs";
import { expect, test } from "vitest";

const main = readFileSync(new URL("./main.ts", import.meta.url), "utf8");

// main.ts applies the options and listens for the zoom keys and the View menu's events through the api.
// (That the attributes are on <html> before the shell renders is tested in shell/shell.test.ts.)
test("zoom: the menu events come through the api, and the zoom goes to the window", () => {
  for (const e of ["menu:zoom-in", "menu:zoom-out", "menu:zoom-reset"]) expect(main, e).toContain(`"${e}"`);
  expect(main).toContain("api.on(event");
  expect(main).not.toContain("Events.On(");
  expect(main).toContain("Window.SetZoom(");
  expect(main).toContain("zoomKey(");
  expect(main.indexOf("followAccessibility(")).toBeLessThan(main.indexOf("getApi()"));
});
