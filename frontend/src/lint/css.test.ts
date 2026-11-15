import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("./lint.css", import.meta.url), "utf8");
const body = css.replace(/\/\*[\s\S]*?\*\//g, "");

test("grayscale tokens only: no colours", () => {
  expect(body).not.toMatch(/#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(|oklch\(|lab\(|color\(/i);
  const named = /\b(red|orange|yellow|green|blue|purple|pink|brown|gold|cyan|magenta|lime|navy|teal|olive|maroon|violet|indigo|crimson|tomato|coral|salmon)\b/i;
  expect(body).not.toMatch(named);
  for (const m of Array.from(body.matchAll(/var\((--[a-z0-9-]+)/g))) {
    expect(["--fg", "--fg-muted", "--bg", "--bg-subtle", "--rule", "--font-ui", "--font-mono", "--radius", "--btn-bg", "--btn-bg-hover", "--focus"]
      .concat(Array.from({ length: 7 }, (_, i) => `--space-${i + 1}`))).toContain(m[1]);
  }
});

test("severities: solid fg, dashed and dotted fg-muted", () => {
  const rule = (sel: string) => body.match(new RegExp(`${sel.replace(/[.-]/g, "\\$&")}[^{]*\\{([^}]*)\\}`))?.[1] ?? "";
  expect(rule(".zen-lint-error")).toMatch(/text-decoration[^;]*solid[^;]*var\(--fg\)|text-decoration-style:\s*solid/);
  expect(rule(".zen-lint-error")).toMatch(/var\(--fg\)/);
  expect(rule(".zen-lint-warning")).toMatch(/dashed/);
  expect(rule(".zen-lint-warning")).toMatch(/var\(--fg-muted\)/);
  expect(rule(".zen-lint-suggestion")).toMatch(/dotted/);
  expect(rule(".zen-lint-suggestion")).toMatch(/var\(--fg-muted\)/);
  expect(rule(".zen-lint-error")).toMatch(/2px/);
});

test("marks drop the browser's mark colours", () => {
  expect(body).toMatch(/mark\.zen-lint[^{]*\{[^}]*background:\s*(none|transparent)/);
});
