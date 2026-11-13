// @vitest-environment happy-dom
import { expect, test } from "vitest";
import { renderEmptyState } from "./shell";

test("empty state names the app and replaces previous content", () => {
  const root = document.createElement("div");
  root.innerHTML = "<p>stale</p>";

  renderEmptyState(root);

  expect(root.querySelector("h1")?.textContent).toBe("zen");
  expect(root.textContent).not.toContain("stale");
});
