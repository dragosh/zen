// The plugin kit (runtime/src/css/plugin-kit.css, docs/PLUGIN-UI.md): the shared classes for every interactive block.
// Reads the CSS only, no DOM.
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "vitest";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../..");
const read = (p: string) => readFileSync(join(ROOT, p), "utf8");
const strip = (s: string) => s.replace(/\/\*[\s\S]*?\*\//g, "");
const kit = strip(read("runtime/src/css/plugin-kit.css"));
const selectors = (css: string) => (css.match(/[^{}]+(?=\{)/g) ?? []).flatMap((s) => s.split(",").map((x) => x.trim()));
/** The declarations of every rule whose selector list contains exactly `sel`. */
function rule(css: string, sel: string): string {
  let out = "";
  for (const m of Array.from(css.matchAll(/([^{}@]+)\{([^{}]*)\}/g))) if (m[1].split(",").some((x) => x.trim() === sel)) out += m[2];
  return out;
}

test("the kit has every class", () => {
  const sels = selectors(kit);
  for (const c of [".zen-block", ".zen-block-head", ".zen-block-status", ".zen-block-body", ".zen-block-bar", ".zen-btn", ".zen-btn-primary", ".zen-btn-quiet", ".zen-choice", ".zen-field"]) {
    expect(sels.some((s) => s.startsWith(c)), c).toBe(true);
  }
});

test("the kit follows the sandbox's look, in tokens only", () => {
  expect(kit).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
  const tokens = new Set(Array.from(read("runtime/src/css/tokens.css").matchAll(/(--[a-z0-9-]+):/g), (m) => m[1]));
  for (const m of Array.from(kit.matchAll(/var\((--[a-z0-9-]+)/g))) expect(tokens.has(m[1]), m[1]).toBe(true);
  expect(rule(kit, ".zen-block")).toMatch(/1px solid var\(--rule\)/);
  expect(rule(kit, ".zen-block")).toMatch(/var\(--bg-subtle\)/);
  expect(rule(kit, ".zen-block")).toMatch(/var\(--font-ui\)/);
  expect(rule(kit, ".zen-btn")).toMatch(/padding:\s*3px 12px/);
  expect(rule(kit, ".zen-btn")).toMatch(/var\(--btn-bg\)/);
  expect(rule(kit, ".zen-btn-primary")).toMatch(/var\(--btn-primary-bg\)/);
  expect(rule(kit, ".zen-btn-primary")).toMatch(/var\(--btn-primary-fg\)/);
  expect(kit).not.toMatch(/font-family:\s*var\(--font-body\)/);
});

test("every control has a focus-visible ring", () => {
  const sels = selectors(kit);
  for (const c of [".zen-btn:focus-visible", ".zen-choice input:focus-visible", ".zen-field:focus-visible"]) expect(sels, c).toContain(c);
});

test("the kit is shipped with page.css, in the app and in built sites", () => {
  expect(read("runtime/src/css/page.css")).toMatch(/@import\s+["']\.\/plugin-kit\.css["']/);
  expect(read("runtime/src/static/static.css")).toMatch(/@import\s+["']\.\.\/css\/page\.css["']/);
  expect(read("apps/zenmark/frontend/src/shell/features.ts")).toContain("@zenmd/runtime/css/page.css");
});

test("the style guide exists and is linked from PLUGINS.md", () => {
  expect(read("docs/PLUGIN-UI.md")).toContain("zen-btn");
  expect(read("docs/PLUGINS.md")).toContain("PLUGIN-UI.md");
});

// Plugins and layouts keep only their own styles.
function builtinCss(): string[] {
  const out: string[] = [];
  const walk = (d: string) => {
    for (const e of readdirSync(join(ROOT, d), { withFileTypes: true })) {
      if (e.isDirectory()) walk(`${d}/${e.name}`);
      else if (e.name.endsWith(".css")) out.push(`${d}/${e.name}`);
    }
  };
  // The plugins and the course layout (the other layouts own their pages, and use the serif for the page's own text).
  walk("builtins/plugins");
  walk("builtins/layouts/course");
  return out;
}

test("plugins define no button, choice or field rules of their own, and no serif inside blocks", () => {
  const files = builtinCss();
  expect(files.length).toBeGreaterThan(3);
  for (const f of files) {
    const css = strip(read(f));
    // codeblock's Copy and Wrap are small tool chips inside a code block, not a block's controls.
    if (!f.includes("codeblock")) {
      expect(selectors(css).filter((s) => /\bbutton\b|\binput\b|\bselect\b|\btextarea\b/.test(s) && !/sandbox-editor|\.zr-star input/.test(s)), `${f}: controls`).toEqual([]);
    }
    expect(css, `${f}: serif`).not.toMatch(/var\(--font-body\)/);
    expect(selectors(css).filter((s) => /-option\b|-legend\b|-clear\b|-check\b|-recheck\b/.test(s)), `${f}: choices`).toEqual([]);
  }
});

test("the course step list is a grid: the mark in a fixed column, the title in the next", () => {
  const css = strip(read("builtins/layouts/course/course.css"));
  const link = rule(css, ".course-step-link");
  expect(link).toMatch(/display:\s*grid/);
  expect(link).toMatch(/grid-template-columns:\s*[0-9.]+(em|ch|rem|px)\s+1fr/);
});
