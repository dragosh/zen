import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("../../../../runtime/src/css/page.css", import.meta.url), "utf8");

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

const printCss = readFileSync(new URL("../../../../runtime/src/css/print.css", import.meta.url), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
/** The declarations of the rules whose selector list contains `sel`, inside @media print of print.css. */
function printRule(sel: string): string {
  const media = printCss.slice(printCss.indexOf("@media print"));
  let out = "";
  for (const m of Array.from(media.matchAll(/([^{}@]+)\{([^{}]*)\}/g))) {
    if (m[1].split(",").some((x) => x.trim() === sel)) out += m[2];
  }
  return out;
}

test("print rules (ui/print)", () => {
  expect(css).toMatch(/^\s*(\/\*[\s\S]*?\*\/\s*)?@import\s+["']\.\/print\.css["']\s*;/); // page.css imports it first
  // Always light, whatever the app theme.
  const light = printRule(":root:root:root");
  expect(light).toMatch(/--bg:\s*#fff/i);
  expect(light).toMatch(/--fg:\s*#000/i);
  expect(light).toMatch(/color-scheme:\s*light/);
  expect(printRule("html, body".split(",")[0].trim()) + printRule("body")).toMatch(/background:\s*#fff/i);
  // No chrome and no controls.
  const hidden = printCss.slice(printCss.indexOf("@media print"));
  for (const s of [".zen-sidebar", ".zen-top", ".zen-actions", ".zen-page-warnings", ".zen-site-side", ".zen-site-crumbs", ".zen-print-bar", ".zen-lint-wrap", ".zen-editor-divider", ".zen-editor-source", ".zen-editor-bar", ".zen-page button", ".course-nav", "[data-zen-action]"]) {
    expect(hidden, s).toContain(s);
  }
  expect(printRule(".zen-page button")).toMatch(/display:\s*none/);
  // Blocks stay together where they fit; headings stay with what follows.
  expect(printRule("h1")).toMatch(/break-after:\s*avoid/);
  for (const s of ["pre", "figure", "tr", ".zen-diagram"]) expect(printRule(s), s).toMatch(/break-inside:\s*avoid/);
  expect(printRule("thead")).toMatch(/display:\s*table-header-group/);
  // Long code lines wrap.
  expect(printRule("pre")).toMatch(/white-space:\s*pre-wrap/);
  expect(printRule("pre")).toMatch(/overflow-wrap:\s*(anywhere|break-word)/);
  // External links print their URL, unless the text is the URL; internal links are plain text.
  const after = printRule('.zen-page a[data-zen-link="external"]:not([data-zen-print="url"])::after');
  expect(after).toMatch(/content:\s*" \(" attr\(href\) "\)"/);
  expect(after).toMatch(/font-size:\s*[\d.]+pt/);
  expect(printRule('.zen-page a[data-zen-link="internal"]')).toMatch(/text-decoration:\s*none/);
  // The combined document: sheets, cover and contents.
  expect(printRule(".zen-print-page")).toMatch(/break-before:\s*page/);
  expect(printRule(".zen-print-cover")).toMatch(/break-after:\s*page/);
  expect(printRule(".zen-print-toc")).toMatch(/break-after:\s*page/);
});

test("print.css has no @page: the layouts and the print document set their own", () => {
  expect(printCss).not.toMatch(/@page/);
});

test("the print document is not a reading column (screen)", () => {
  const c = readFileSync(new URL("../../../../runtime/src/css/print.css", import.meta.url), "utf8");
  expect(c).toMatch(/\.zen-page\.zen-print-doc\s*\{[^}]*max-width:\s*none/);
});

test("the image zoom view is on top of everything and never printed (ui/image-zoom)", () => {
  expect(css).toMatch(/\.zen-zoom\s*\{[^}]*position:\s*fixed[^}]*inset:\s*0/);
  expect(css).toMatch(/\.zen-page img\s*\{\s*cursor:\s*zoom-in/);
  expect(printCss).toMatch(/\.zen-zoom\s*\{\s*display:\s*none\s*!important/);
});
