import { expect, test } from "vitest";
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("../../../../runtime/src/css/tokens.css", import.meta.url), "utf8");

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

type Rule = { selector: string; media: string; decls: Record<string, string> };

/** Flat list of rules with their enclosing @media (one level of nesting). */
function rules(source: string): Rule[] {
  const out: Rule[] = [];
  const clean = source.replace(/\/\*[\s\S]*?\*\//g, "");
  let media = "";
  let buf = "";
  for (const ch of clean) {
    if (ch === "{") {
      const head = buf.trim();
      buf = "";
      if (head.startsWith("@media")) media = head;
      else out.push({ selector: head, media, decls: {} });
    } else if (ch === "}") {
      if (buf.trim() === "" && out.length && Object.keys(out[out.length - 1].decls).length && !buf.includes(":")) {
        media = ""; // closing an @media block
      } else {
        const decls = out[out.length - 1].decls;
        for (const d of buf.split(";")) {
          const i = d.indexOf(":");
          if (i > 0) decls[d.slice(0, i).trim()] = d.slice(i + 1).trim();
        }
      }
      buf = "";
    } else buf += ch;
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

// ---- Increase contrast (ui/accessibility, behaviour 12)

type Flat = { selector: string; media: string; decls: Record<string, string> };
function flat(source: string): Flat[] {
  const out: Flat[] = [];
  const walk = (src: string, media: string) => {
    let i = 0;
    while (i < src.length) {
      const open = src.indexOf("{", i);
      if (open < 0) break;
      let depth = 1, j = open + 1;
      while (j < src.length && depth > 0) { if (src[j] === "{") depth++; else if (src[j] === "}") depth--; j++; }
      const head = src.slice(i, open).replace(/\s+/g, " ").trim();
      const body = src.slice(open + 1, j - 1);
      if (head.startsWith("@media")) walk(body, head);
      else {
        const decls: Record<string, string> = {};
        for (const d of body.split(";")) { const k = d.indexOf(":"); if (k > 0) decls[d.slice(0, k).trim()] = d.slice(k + 1).trim(); }
        out.push({ selector: head, media, decls });
      }
      i = j;
    }
  };
  walk(source.replace(/\/\*[\s\S]*?\*\//g, ""), "");
  return out;
}
const lum = (hex: string) => {
  const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
};
const ratio = (a: string, b: string) => { const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x); return (hi + 0.05) / (lo + 0.05); };

test("increase contrast", () => {
  // The overrides are in the app's shell.css, not in the runtime's tokens.css (that is embedded in every built site, which stays as it is).
  const shellCss = readFileSync(new URL("../styles/shell.css", import.meta.url), "utf8");
  const rs = flat(shellCss).filter((r) => /prefers-contrast:\s*more/.test(r.media) || r.selector.includes('[data-contrast="more"]'));
  expect(rs.length).toBeGreaterThan(0);
  // by the system, and by the option: both
  expect(rs.some((r) => /prefers-contrast:\s*more/.test(r.media))).toBe(true);
  expect(rs.some((r) => r.selector.includes('[data-contrast="more"]'))).toBe(true);
  // nothing else changes colour: only the muted text and the rules
  for (const r of rs) for (const k of Object.keys(r.decls)) expect(["--fg-muted", "--rule", "outline-width"], `${r.selector}: ${k}`).toContain(k);
  // focus rings are 3 px, by the system and by the option
  const rings = rs.filter((r) => r.decls["outline-width"] !== undefined);
  expect(rings.some((r) => /prefers-contrast:\s*more/.test(r.media))).toBe(true);
  expect(rings.some((r) => r.selector.includes('[data-contrast="more"]'))).toBe(true);
  for (const r of rings) expect(r.decls["outline-width"]).toMatch(/^3px/);
  // muted text becomes the text colour
  const muted = rs.filter((r) => "--fg-muted" in r.decls);
  expect(muted.length).toBeGreaterThan(1);
  for (const r of muted) expect(r.decls["--fg-muted"]).toBe("var(--fg)");
  // the rule is at least 3:1 against its background, in both themes
  const isDark = (r: Flat) => r.selector.includes('[data-theme="dark"]') || /prefers-color-scheme:\s*dark/.test(r.media);
  const withRule = rs.filter((r) => "--rule" in r.decls);
  const lightRules = withRule.filter((r) => !isDark(r));
  const darkRules = withRule.filter(isDark);
  expect(lightRules.length).toBeGreaterThan(0);
  expect(darkRules.length).toBeGreaterThan(0);
  for (const r of lightRules) expect(ratio(r.decls["--rule"], light["--bg"]), r.selector).toBeGreaterThanOrEqual(3);
  for (const r of darkRules) expect(ratio(r.decls["--rule"], dark["--bg"]), r.selector).toBeGreaterThanOrEqual(3);
  // they beat the theme rules they override: no theme selector is more specific
  for (const r of rs.filter((x) => "--rule" in x.decls || "--fg-muted" in x.decls)) expect(r.selector).toMatch(/:root/);
});
