import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("./page.css", import.meta.url), "utf8");

const selectors = css.replace(/\/\*[\s\S]*?\*\//g, "").match(/[^{}]+(?=\{)/g)!.flatMap((s) => s.split(",").map((x) => x.trim()));
const has = (needle: string) => selectors.some((s) => s.includes(needle));

test("every element the renderer emits has a rule", () => {
  for (const n of [".zen-page", "h1", "h2", "h3", "h4", "h5", "h6", "p", "ul", "ol", "li", "input[type=\"checkbox\"]",
    "table", "th", "td", "code", "pre", "blockquote", ".footnotes", "sup", "img", "hr", ".zen-plugin-error",
    "a[data-zen-link=\"broken\"]", ".zen-progress", ".zen-page-warnings", ".zen-page-error"]) {
    expect(has(n), n).toBe(true);
  }
  expect(css).toMatch(/img[^{]*\{[^}]*max-width:\s*100%/);
});

test("only tokens, no new colours", () => {
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
});
