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

test("a layout page drops the reading column (ui/plugins)", () => {
  const rule = css.match(/\.zen-page\[data-layout\]\s*\{([^}]*)\}/);
  expect(rule?.[1]).toMatch(/max-width:\s*none/);
  expect(rule?.[1]).toMatch(/padding:\s*0/);
});

test("print: no sidebar, header or panels, the article uses the whole page (core/plugin-extras)", () => {
  const print = css.match(/@media\s+print\s*\{([\s\S]*?)\n\}/);
  expect(print, "an @media print block").not.toBeNull();
  const body = print![1];
  for (const hidden of [".zen-sidebar", ".zen-resizer", ".zen-top", ".zen-actions", ".zen-progress", ".zen-page-warnings", ".zen-page-error"]) {
    expect(body, hidden).toContain(hidden);
  }
  expect(body).toMatch(/display:\s*none/);
  expect(body).toMatch(/\.zen-main[^{]*\{[^}]*overflow:\s*visible/);
  expect(body).toMatch(/\.zen-page[^{]*\{[^}]*max-width:\s*none/);
  expect(body).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
});
