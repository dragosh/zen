import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("./lint.css", import.meta.url), "utf8");
const body = css.replace(/\/\*[\s\S]*?\*\//g, "");

/** The three lint signal tokens are the only place colours may appear. */
const outsideTokens = body.replace(/--lint-(error|warning|suggestion):\s*#[0-9a-f]{3,8};/gi, "");

test("colours only as the three lint signal tokens, defined for light and dark", () => {
  expect(outsideTokens).not.toMatch(/#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(|oklch\(|lab\(|color\(/i);
  for (const t of ["error", "warning", "suggestion"]) {
    expect(body.match(new RegExp(`--lint-${t}:`, "g"))?.length).toBe(3); // light, dark (system), dark (forced)
  }
  const named = /\b(red|orange|yellow|green|blue|purple|pink|brown|gold|cyan|magenta|lime|navy|teal|olive|maroon|violet|indigo|crimson|tomato|coral|salmon)\b/i;
  expect(body).not.toMatch(named);
  for (const m of Array.from(body.matchAll(/var\((--[a-z0-9-]+)/g))) {
    expect(["--lint-error", "--lint-warning", "--lint-suggestion", "--fg", "--fg-muted", "--bg", "--bg-subtle", "--rule", "--font-ui", "--font-mono", "--radius", "--btn-bg", "--btn-bg-hover", "--focus", "--zen-lint-console-h"]
      .concat(Array.from({ length: 7 }, (_, i) => `--space-${i + 1}`))).toContain(m[1]);
  }
});

test("marks are wavy squiggles coloured by severity, unlike link underlines", () => {
  const rule = (sel: string) => body.match(new RegExp(`(^|\\n)${sel.replace(/[.-]/g, "\\$&")}\\s*\\{([^}]*)\\}`))?.[2] ?? "";
  expect(rule("mark.zen-lint")).toMatch(/text-decoration-style:\s*wavy/);
  expect(rule(".zen-lint-error")).toMatch(/var\(--lint-error\)/);
  expect(rule(".zen-lint-warning")).toMatch(/var\(--lint-warning\)/);
  expect(rule(".zen-lint-suggestion")).toMatch(/var\(--lint-suggestion\)/);
});

test("marks drop the browser's mark colours", () => {
  expect(body).toMatch(/mark\.zen-lint[^{]*\{[^}]*background:\s*(none|transparent)/);
});
