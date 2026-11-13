// @vitest-environment happy-dom
import { expect, test } from "vitest";
import { renderDocument, renderEmptyState } from "./shell";

test("empty state names the app and replaces previous content", () => {
  const root = document.createElement("div");
  root.innerHTML = "<p>stale</p>";

  renderEmptyState(root);

  expect(root.querySelector("h1")?.textContent).toBe("zen");
  expect(root.textContent).not.toContain("stale");
});

test("document shows the HTML from Go", () => {
  const root = document.createElement("div");
  renderDocument(root, { name: "a.md", html: "<h1>Title</h1>" });

  expect(root.querySelector("h1")?.textContent).toBe("Title");
});

test("document shows read errors as text", () => {
  const root = document.createElement("div");
  renderDocument(root, { name: "a.md", html: "", error: "no such file" });

  expect(root.textContent).toContain("no such file");
});
