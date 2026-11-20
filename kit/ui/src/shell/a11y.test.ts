// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { A11Y_DEFAULTS, applyAccessibility, reducedMotion, followAccessibility, resetAccessibility, setAccessibility, storedAccessibility, ZOOM_STEPS, zoomStep } from "./a11y";

const root = document.documentElement;
const A11Y_KEYS = () => Object.keys(localStorage).filter((k) => k.startsWith("zen.a11y."));

beforeEach(() => {
  localStorage.clear();
  resetAccessibility(document);
});
afterEach(() => vi.unstubAllGlobals());

test("defaults: nothing stored, nothing on <html>", () => {
  expect(storedAccessibility()).toEqual(A11Y_DEFAULTS);
  expect(A11Y_DEFAULTS).toEqual({ zoom: 1, textSize: 17, font: "serif", lineSpacing: "normal", lineWidth: "normal", underlineLinks: false, reduceMotion: "system", contrast: "system" });
  applyAccessibility(A11Y_DEFAULTS, document);
  expect(root.dataset.reduceMotion).toBeUndefined();
  expect(root.dataset.contrast).toBeUndefined();
  expect(root.dataset.linkUnderline).toBeUndefined();
  for (const v of ["--zen-text-size", "--zen-text-font", "--zen-line-height", "--zen-measure"]) expect(root.style.getPropertyValue(v), v).toBe("");
});

test("stores, applies and follows other windows", () => {
  const zooms: number[] = [];
  const stop = followAccessibility(document, (z) => zooms.push(z));
  const got = setAccessibility({ reduceMotion: "on", contrast: "more", zoom: 1.25 }, document);
  expect(got.reduceMotion).toBe("on");
  expect(root.dataset.reduceMotion).toBe("on");
  expect(root.dataset.contrast).toBe("more");
  expect(A11Y_KEYS().sort()).toEqual(["zen.a11y.contrast", "zen.a11y.reduceMotion", "zen.a11y.zoom"]);
  expect(storedAccessibility()).toMatchObject({ reduceMotion: "on", contrast: "more", zoom: 1.25 });

  // another window changes a setting: the storage event brings it here, zoom included
  localStorage.setItem("zen.a11y.contrast", "system");
  localStorage.setItem("zen.a11y.zoom", "1.5");
  window.dispatchEvent(new StorageEvent("storage", { key: "zen.a11y.contrast" }));
  expect(root.dataset.contrast).toBeUndefined();
  expect(zooms.at(-1)).toBe(1.5);
  // an unrelated key changes nothing
  const n = zooms.length;
  window.dispatchEvent(new StorageEvent("storage", { key: "zen.theme" }));
  expect(zooms.length).toBe(n);
  // clearing storage (key null) goes back to the defaults
  localStorage.clear();
  window.dispatchEvent(new StorageEvent("storage", { key: null }));
  expect(root.dataset.reduceMotion).toBeUndefined();
  expect(zooms.at(-1)).toBe(1);
  // after stop() nothing follows
  stop();
  localStorage.setItem("zen.a11y.reduceMotion", "on");
  window.dispatchEvent(new StorageEvent("storage", { key: "zen.a11y.reduceMotion" }));
  expect(root.dataset.reduceMotion).toBeUndefined();
});

test("follow applies the stored values at once, and sets a stored zoom", () => {
  localStorage.setItem("zen.a11y.contrast", "more");
  localStorage.setItem("zen.a11y.zoom", "1.75");
  const zooms: number[] = [];
  followAccessibility(document, (z) => zooms.push(z))();
  expect(root.dataset.contrast).toBe("more"); // before anything is painted
  expect(zooms).toEqual([1.75]);
});

test("reset", () => {
  setAccessibility({ zoom: 2, textSize: 24, font: "sans", lineSpacing: "loose", lineWidth: "wide", underlineLinks: true, reduceMotion: "on", contrast: "more" }, document);
  expect(A11Y_KEYS().length).toBeGreaterThan(0);
  expect(resetAccessibility(document)).toEqual(A11Y_DEFAULTS);
  expect(A11Y_KEYS()).toEqual([]);
  expect(storedAccessibility()).toEqual(A11Y_DEFAULTS);
  expect(root.dataset.reduceMotion).toBeUndefined();
  expect(root.dataset.contrast).toBeUndefined();
  expect(root.dataset.linkUnderline).toBeUndefined();
  expect(root.style.getPropertyValue("--zen-text-size")).toBe("");
  expect(root.style.getPropertyValue("--zen-measure")).toBe("");
});

test("zoomStep", () => {
  expect(ZOOM_STEPS).toEqual([0.75, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2]);
  expect(zoomStep(1, 1)).toBe(1.1);
  expect(zoomStep(1, -1)).toBe(0.9);
  expect(zoomStep(1.1, 1)).toBe(1.25);
  expect(zoomStep(2, 0)).toBe(1); // Actual Size
  expect(zoomStep(0.75, 0)).toBe(1);
  // the ends do nothing
  expect(zoomStep(2, 1)).toBe(2);
  expect(zoomStep(0.75, -1)).toBe(0.75);
  // walking up and down visits every step
  let z = 0.75;
  const up = [z];
  while (zoomStep(z, 1) !== z) { z = zoomStep(z, 1); up.push(z); }
  expect(up).toEqual([...ZOOM_STEPS]);
  // a value off the list steps to the next one in that direction
  expect(zoomStep(1.05, 1)).toBe(1.1);
  expect(zoomStep(1.05, -1)).toBe(1);
});

test("reading options set the variables", () => {
  setAccessibility({ textSize: 22, font: "sans", lineSpacing: "relaxed", lineWidth: "wide", underlineLinks: true }, document);
  expect(root.style.getPropertyValue("--zen-text-size")).toBe("22px");
  expect(root.style.getPropertyValue("--zen-text-font")).toBe("var(--font-ui)");
  expect(root.style.getPropertyValue("--zen-line-height")).toBe("1.9");
  expect(root.style.getPropertyValue("--zen-measure")).toBe("84ch");
  expect(root.dataset.linkUnderline).toBe("on");
  setAccessibility({ textSize: 14, font: "serif", lineSpacing: "loose", lineWidth: "narrow", underlineLinks: false }, document);
  expect(root.style.getPropertyValue("--zen-text-size")).toBe("14px");
  expect(root.style.getPropertyValue("--zen-text-font")).toBe(""); // serif is the stylesheet's own
  expect(root.style.getPropertyValue("--zen-line-height")).toBe("2.2");
  expect(root.style.getPropertyValue("--zen-measure")).toBe("60ch");
  expect(root.dataset.linkUnderline).toBeUndefined();
  // the text size is kept in 14..28 whole pixels
  expect(setAccessibility({ textSize: 99 }, document).textSize).toBe(28);
  expect(setAccessibility({ textSize: 3 }, document).textSize).toBe(14);
  expect(setAccessibility({ textSize: 17.6 }, document).textSize).toBe(18);
});

test("invalid values", () => {
  const bad: Record<string, string> = {
    zoom: "3", textSize: "17px;color:red", font: "url(http://x/y)", lineSpacing: "}body{display:none", lineWidth: "100ch;}", underlineLinks: "yes", reduceMotion: "off", contrast: "less",
  };
  for (const [k, v] of Object.entries(bad)) localStorage.setItem(`zen.a11y.${k}`, v);
  expect(storedAccessibility()).toEqual(A11Y_DEFAULTS);
  followAccessibility(document)();
  const style = root.getAttribute("style") ?? "";
  expect(style).not.toMatch(/[;}]\s*color|url\(|display/);
  expect(root.style.getPropertyValue("--zen-text-size")).toBe("");
  expect(root.dataset.contrast).toBeUndefined();
  // out-of-range numbers and junk fall back too
  for (const [k, v] of [["zoom", "NaN"], ["zoom", "1.3"], ["textSize", "29"], ["textSize", "13"], ["textSize", ""], ["textSize", "1e1"]]) {
    localStorage.clear();
    localStorage.setItem(`zen.a11y.${k}`, v);
    expect(storedAccessibility()[k as "zoom" | "textSize"], `${k}=${v}`).toBe(A11Y_DEFAULTS[k as "zoom" | "textSize"]);
  }
  // strings that no setter would write are not applied either
  applyAccessibility({ ...A11Y_DEFAULTS, font: "x;}" as never, lineWidth: "url(x)" as never, textSize: Number.NaN }, document);
  expect(root.getAttribute("style") ?? "").not.toMatch(/url\(|;\s*}|x;/);
});

test("no storage", () => {
  vi.stubGlobal("localStorage", {
    getItem() { throw new Error("denied"); },
    setItem() { throw new Error("denied"); },
    removeItem() { throw new Error("denied"); },
    key() { throw new Error("denied"); },
    get length(): number { throw new Error("denied"); },
  });
  expect(storedAccessibility()).toEqual(A11Y_DEFAULTS);
  // it works for this window only
  expect(() => setAccessibility({ contrast: "more", textSize: 20 }, document)).not.toThrow();
  expect(root.dataset.contrast).toBe("more");
  expect(storedAccessibility()).toMatchObject({ contrast: "more", textSize: 20 });
  expect(() => followAccessibility(document)()).not.toThrow();
  expect(() => resetAccessibility(document)).not.toThrow();
  expect(storedAccessibility()).toEqual(A11Y_DEFAULTS);
  expect(root.dataset.contrast).toBeUndefined();
});

// ---- reduce motion: every reduced-motion rule has its twin (behaviour 11)

type Rule = { selectors: string[]; decls: string };
const squash = (s: string) => s.replace(/\s+/g, " ").trim();
const noComments = (s: string) => s.replace(/\/\*[\s\S]*?\*\//g, "");

/** The rules of a stylesheet, flat, @media wrappers dropped; `inside` lists the rules inside a media block whose header matches. */
function parse(css: string, header: RegExp | null): Rule[] {
  const out: Rule[] = [];
  const walk = (src: string, active: boolean) => {
    let i = 0;
    while (i < src.length) {
      const open = src.indexOf("{", i);
      if (open < 0) break;
      let depth = 1, j = open + 1;
      while (j < src.length && depth > 0) { if (src[j] === "{") depth++; else if (src[j] === "}") depth--; j++; }
      const head = squash(src.slice(i, open));
      const body = src.slice(open + 1, j - 1);
      if (head.startsWith("@media")) walk(body, header ? header.test(head) : false);
      else if (head.startsWith("@")) { /* @keyframes and friends */ }
      else if (header === null || active) out.push({ selectors: head.split(",").map(squash), decls: squash(body).replace(/;$/, "") });
      i = j;
    }
  };
  walk(noComments(css), false);
  return out;
}
const read = (rel: string) => readFileSync(resolve(import.meta.dirname, "../../../..", rel), "utf8");

// The stylesheets this spec owns (the reduced-motion twins live in each).
const OWNED_CSS = [
  "runtime/src/css/base.css", "runtime/src/css/page.css",
  "kit/ui/src/styles/shell.css", "kit/ui/src/panel/panel.css", "kit/ui/src/print/preview.css", "kit/ui/src/find/find.css",
  "kit/ui/src/nav/nav.css", "kit/ui/src/nav/panel.css", "kit/ui/src/inspector/inspector.css", "kit/ui/src/shell/settings.css",
];

// The runtime's own stylesheets (base.css, page.css) are embedded in every built site, and a site, print and PDF must stay
// byte for byte what they are, so their twins are in the app's shell.css; the kit's own files hold theirs.
const SHELL = "kit/ui/src/styles/shell.css";
const RUNTIME = ["runtime/src/css/base.css", "runtime/src/css/page.css"];

test("every reduced-motion rule also matches data-reduce-motion", () => {
  let blocks = 0;
  for (const file of OWNED_CSS) {
    const css = read(file);
    const all = [...parse(css, null), ...(RUNTIME.includes(file) ? parse(read(SHELL), null) : [])];
    const twin = (sel: string, decls: string) => all.some((r) => r.decls === decls && r.selectors.includes(`:root[data-reduce-motion="on"] ${sel}`));
    for (const r of parse(css, /prefers-reduced-motion:\s*reduce/)) {
      blocks++;
      for (const sel of r.selectors) expect(twin(sel, r.decls), `${file}: ${sel} { ${r.decls} } has no :root[data-reduce-motion="on"] twin`).toBe(true);
    }
  }
  expect(blocks).toBeGreaterThan(8);
  // find.css animates only for no-preference: On switches that animation off too
  const find = parse(read("kit/ui/src/find/find.css"), null);
  expect(find.some((r) => r.selectors.includes(':root[data-reduce-motion="on"] .zen-find') && /animation:\s*none/.test(r.decls))).toBe(true);
  // a kit stylesheet with a reduced-motion block has the attribute in it too
  for (const file of OWNED_CSS.filter((f) => !RUNTIME.includes(f))) {
    if (/prefers-reduced-motion:\s*reduce/.test(read(file))) expect(read(file), file).toContain('[data-reduce-motion="on"]');
  }
});

// ---- the reading variables, print and built sites (behaviour 10)

test("the runtime stylesheets, which built sites and print use, are untouched; the app's shell.css has the reading options", () => {
  for (const f of ["tokens.css", "page.css", "base.css", "print.css"]) {
    const css = read(`runtime/src/css/${f}`);
    expect(css, f).not.toMatch(/--zen-text|--zen-line|--zen-measure|--zen-text-font|data-reduce-motion|data-contrast|data-link-underline/);
  }
  const shell = read(SHELL);
  const rules = parse(shell, null);
  const defaults = rules.find((r) => r.selectors.includes(":root") && r.decls.includes("--zen-text-size"))!;
  expect(defaults.decls).toContain("--zen-text-size: 17px");
  expect(defaults.decls).toContain("--zen-line-height: 1.65");
  expect(defaults.decls).toContain("--zen-text-font: var(--font-body)");
  expect(defaults.decls).toContain("--zen-measure: 68ch");
  // today's values: page.css has 17px/1.65 var(--font-body) and 68ch
  expect(read("runtime/src/css/page.css")).toMatch(/max-width:\s*68ch[\s\S]*font:\s*17px\/1\.65 var\(--font-body\)/);
  // the page reads them on screen only, and not in layout pages, the print document or its preview
  const page = parse(shell, /^@media screen$/).filter((r) => r.decls.includes("var(--zen-text-size)"));
  expect(page.length).toBe(1);
  expect(page[0].selectors[0]).toMatch(/\.zen-page:not\(\[data-layout\]\):not\(\.zen-print-doc\)$/);
  expect(page[0].decls).toContain("font: var(--zen-text-size) / var(--zen-line-height) var(--zen-text-font)");
  expect(page[0].decls).toContain("max-width: var(--zen-measure)");
  // underlined links: the reader's pages only, never the print document
  const under = parse(shell, /^@media screen$/).find((r) => r.selectors.some((s) => s.startsWith(':root[data-link-underline="on"] .zen-page:not(.zen-print-doc) a')));
  expect(under?.decls).toContain("text-decoration: underline");
  // headings follow the text size: em, in the sizes of the page's own stylesheet at 17 px
  const h1 = parse(shell, /^@media screen$/).find((r) => r.selectors[0].endsWith(" h1"));
  expect(h1?.decls).toMatch(/font-size: 1\.9764\d*em/);
  expect(2.1 * 16 / 17).toBeCloseTo(1.97647, 4);
});

test("reducedMotion: the option or the system", () => {
  expect(reducedMotion(document)).toBe(false);
  setAccessibility({ reduceMotion: "on" }, document);
  expect(reducedMotion(document)).toBe(true);
  resetAccessibility(document);
  vi.stubGlobal("matchMedia", (q: string) => ({ matches: q.includes("prefers-reduced-motion") }));
  window.matchMedia = (globalThis as unknown as { matchMedia: typeof window.matchMedia }).matchMedia;
  expect(reducedMotion(document)).toBe(true);
});
