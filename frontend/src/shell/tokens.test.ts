import { expect, test } from "vitest";
import css from "../styles/tokens.css?raw";

// docs/UI.md "Tokens"
const light: Record<string, string> = {
  "--bg": "#ffffff", "--bg-subtle": "#f5f5f4", "--fg": "#111111", "--fg-muted": "#6b6b6b",
  "--rule": "#e2e2e0", "--btn-bg": "#ececeb", "--btn-bg-hover": "#dededc",
  "--btn-primary-bg": "#111111", "--btn-primary-fg": "#ffffff", "--focus": "#111111",
};
const dark: Record<string, string> = {
  "--bg": "#0b0b0b", "--bg-subtle": "#161616", "--fg": "#ededed", "--fg-muted": "#9a9a9a",
  "--rule": "#2a2a2a", "--btn-bg": "#232323", "--btn-bg-hover": "#303030",
  "--btn-primary-bg": "#ededed", "--btn-primary-fg": "#0b0b0b", "--focus": "#ededed",
};
const shared = ["--font-body", "--font-ui", "--font-mono", "--radius",
  "--space-1", "--space-2", "--space-3", "--space-4", "--space-5", "--space-6", "--space-7"];

/** Declarations of every rule whose selector matches, keyed by selector. */
function rules(source: string): { selector: string; media: string; decls: Record<string, string> }[] {
  const out: { selector: string; media: string; decls: Record<string, string> }[] = [];
  const clean = source.replace(/\/\*[\s\S]*?\*\//g, "");
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(clean))) {
    let selector = m[1].trim();
    let media = "";
    const at = selector.match(/^(@media[^{]*)\{\s*([\s\S]*)$/);
    if (at) { media = at[1].trim(); selector = at[2].trim(); }
    const decls: Record<string, string> = {};
    for (const d of m[2].split(";")) {
      const i = d.indexOf(":");
      if (i > 0) decls[d.slice(0, i).trim()] = d.slice(i + 1).trim();
    }
    out.push({ selector, media, decls });
  }
  return out;
}

const all = rules(css);
const find = (pred: (r: (typeof all)[number]) => boolean) => {
  const r = all.find(pred);
  expect(r, "rule not found").toBeDefined();
  return r!.decls;
};

test("light theme defines every UI.md token", () => {
  const d = find((r) => r.selector === ":root" && r.media === "");
  for (const [k, v] of Object.entries(light)) expect(d[k], k).toBe(v);
  for (const k of shared) expect(d[k], k).toBeTruthy();
  expect(d["--font-body"]).toContain('"New York"');
  expect(d["--font-ui"]).toContain("-apple-system");
  expect(d["--font-mono"]).toContain('"SF Mono"');
  expect(d["--radius"]).toBe("6px");
});

test("dark theme follows prefers-color-scheme unless data-theme is light", () => {
  const d = find((r) => /prefers-color-scheme:\s*dark/.test(r.media));
  const rule = all.find((r) => /prefers-color-scheme:\s*dark/.test(r.media))!;
  expect(rule.selector).toContain(':not([data-theme="light"])');
  for (const [k, v] of Object.entries(dark)) expect(d[k], k).toBe(v);
});

test("data-theme=dark forces the dark theme", () => {
  const d = find((r) => r.media === "" && r.selector === ':root[data-theme="dark"]');
  for (const [k, v] of Object.entries(dark)) expect(d[k], k).toBe(v);
});

test("data-theme=light forces the light theme", () => {
  // light is the :root default; the dark media rule must not apply to it
  const forced = all.filter((r) => r.selector.includes('[data-theme="light"]') && !r.selector.includes(":not("));
  for (const r of forced) for (const [k, v] of Object.entries(light)) if (k in r.decls) expect(r.decls[k]).toBe(v);
});
