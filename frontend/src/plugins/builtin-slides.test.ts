// @vitest-environment happy-dom
// The built-in "slides" layout (builtins/layouts/slides/slides.js), loaded as the app loads it.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi } from "../api/fake";
import { installRuntime } from "./runtime";

// The built-in layouts live in builtins/layouts/ (they were plugins before core/layouts).
const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../../..");
// Whichever of the two places has the script (a glob, so a missing one is no error).
const scripts = import.meta.glob<unknown>("../../../builtins/layouts/slides/slides.js");
const ASSETS = join(ROOT, "builtins/layouts/slides");

let rt: ReturnType<typeof installRuntime>;
/** The plugin script, an ES module with a default export; the runtime gets it through importModule. */
let mod: unknown;
let root: HTMLElement;
let anchor = "";
let listeners: ((a: string) => void)[] = [];
const navigated: [string, string | undefined][] = [];

beforeEach(async () => {
  document.body.replaceChildren();
  anchor = "";
  listeners = [];
  navigated.length = 0;
  rt = installRuntime({ importModule: async () => mod,
    api: createFakeApi(),
    navigate: (p, a) => { navigated.push([p, a]); },
    page: () => ({ path: "demo/deck.md", title: "Deck" }),
    anchor: () => anchor,
    onAnchor: (fn) => { listeners.push(fn); return () => { listeners = listeners.filter((l) => l !== fn); }; },
  });
  root = document.body.appendChild(document.createElement("article"));
  mod = await Object.values(scripts)[0]();
  await rt.load({ js: ["/_layouts/slides/slides.js?v=test"], css: [] });
});
afterEach(() => rt.destroy());

/** What zen renders for a page with `layout: slides`: the page HTML inside the layout wrapper. */
function render(html: string, attrs?: Record<string, string>): HTMLElement {
  const w = document.createElement("div");
  w.setAttribute("data-zen-layout", "slides");
  w.setAttribute("data-zen-layout-source", "builtin");
  if (attrs) w.setAttribute("data-zen-attrs", JSON.stringify(attrs));
  w.innerHTML = html; // test fixture only
  root.replaceChildren(w);
  rt.mountAll(root);
  return w;
}
const DECK = "<h1>Title</h1><p>Intro</p><hr><h2>Two</h2><p>Body</p><p>Note: say hello</p><hr><h2>Three</h2><ul><li>x</li></ul>";
const slides = (w: HTMLElement) => Array.from(w.querySelectorAll<HTMLElement>(".slides-slide"));
const current = (w: HTMLElement) => slides(w).findIndex((s) => !s.hidden);
const count = (w: HTMLElement) => w.querySelector(".slides-count")?.textContent;
const key = (k: string) => document.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));

test("splits on hr, one slide visible", () => {
  const w = render(DECK);
  expect(slides(w)).toHaveLength(3);
  expect(slides(w).map((s) => s.querySelector("h1,h2")?.textContent)).toEqual(["Title", "Two", "Three"]);
  expect(slides(w).filter((s) => !s.hidden)).toHaveLength(1);
  expect(current(w)).toBe(0);
  expect(w.querySelector("hr")).toBeNull();
});

test("without hr each h1/h2 starts a slide", () => {
  const w = render("<h1>A</h1><p>a</p><h2>B</h2><p>b</p><h2>C</h2>");
  expect(slides(w)).toHaveLength(3);
  expect(slides(w)[1].textContent).toContain("b");
});

test("speaker notes are kept out of the slide", () => {
  const w = render(DECK);
  const s = slides(w)[1];
  const visible = Array.from(s.querySelectorAll("p")).filter((p) => !p.closest(".slides-notes"));
  expect(visible.map((p) => p.textContent)).toEqual(["Body"]);
  expect(s.querySelector(".slides-notes")?.textContent).toContain("say hello");
});

test("progress and n / total", () => {
  const w = render(DECK);
  expect(count(w)).toBe("1 / 3");
  const bar = w.querySelector(".slides-progress");
  expect(bar?.getAttribute("role")).toBe("progressbar");
  key("ArrowRight");
  expect(count(w)).toBe("2 / 3");
  expect(bar?.getAttribute("aria-valuenow")).toBe("2");
  expect(bar?.getAttribute("aria-valuemax")).toBe("3");
});

test("keys move through the deck and stop at the ends", () => {
  const w = render(DECK);
  key("ArrowLeft");
  expect(current(w)).toBe(0);
  key("ArrowRight");
  expect(current(w)).toBe(1);
  key(" ");
  expect(current(w)).toBe(2);
  key("PageDown");
  expect(current(w)).toBe(2);
  key("PageUp");
  expect(current(w)).toBe(1);
  key("End");
  expect(current(w)).toBe(2);
  key("Home");
  expect(current(w)).toBe(0);
});

test("keys typed into a field are left alone", () => {
  const w = render(DECK);
  const input = document.body.appendChild(document.createElement("input"));
  input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(current(w)).toBe(0);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", metaKey: true }));
  expect(current(w)).toBe(0);
});

test("the current slide lives in the anchor", () => {
  render(DECK);
  key("ArrowRight");
  expect(navigated.at(-1)).toEqual(["demo/deck.md", "slide-2"]);
});

test("the anchor is restored after a re-render and followed when it changes", () => {
  anchor = "slide-3";
  const w = render(DECK);
  expect(current(w)).toBe(2);
  expect(count(w)).toBe("3 / 3");
  for (const l of listeners) l("slide-2");
  expect(current(w)).toBe(1);
  for (const l of listeners) l("not-a-slide");
  expect(current(w)).toBe(1);
  anchor = "slide-99";
  expect(current(render(DECK))).toBe(2); // out of range is clamped
});

test("O opens the overview grid; a click picks a slide", () => {
  const w = render(DECK);
  const r = w.querySelector<HTMLElement>(".slides-root")!;
  key("o");
  expect(r.dataset.overview).toBe("true");
  const thumbs = w.querySelectorAll<HTMLElement>(".slides-thumb");
  expect(thumbs).toHaveLength(3);
  thumbs[2].click();
  expect(r.dataset.overview).toBeUndefined();
  expect(current(w)).toBe(2);
  key("o");
  key("Escape");
  expect(r.dataset.overview).toBeUndefined();
});

test("F toggles full page (and asks for full screen); Esc leaves", () => {
  const w = render(DECK);
  const r = w.querySelector<HTMLElement>(".slides-root")!;
  const btn = w.querySelector<HTMLButtonElement>(".slides-full")!;
  r.requestFullscreen = vi.fn(() => Promise.resolve());
  key("f");
  expect(r.dataset.fullpage).toBe("true");
  expect(btn.getAttribute("aria-pressed")).toBe("true");
  expect(r.requestFullscreen).toHaveBeenCalled();
  key("Escape");
  expect(r.dataset.fullpage).toBeUndefined();
  key("F");
  expect(r.dataset.fullpage).toBe("true");
  key("f");
  expect(r.dataset.fullpage).toBeUndefined();
});

test("the Full page button toggles without full screen support", () => {
  const w = render(DECK);
  const r = w.querySelector<HTMLElement>(".slides-root")!;
  const btn = w.querySelector<HTMLButtonElement>(".slides-full")!;
  btn.click();
  expect(r.dataset.fullpage).toBe("true");
  btn.click();
  expect(r.dataset.fullpage).toBeUndefined();
});

test("keys for the sidebar tree, menus and buttons outside the deck are left alone", () => {
  const w = render(DECK);
  const nav = document.body.appendChild(document.createElement("nav"));
  const item = nav.appendChild(document.createElement("li"));
  item.setAttribute("role", "treeitem");
  item.tabIndex = 0;
  for (const k of ["ArrowRight", " ", "End", "f"]) item.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));
  const b = document.body.appendChild(document.createElement("button"));
  b.dispatchEvent(new KeyboardEvent("keydown", { key: " ", bubbles: true }));
  expect(current(w)).toBe(0);
  expect(w.querySelector<HTMLElement>(".slides-root")!.dataset.fullpage).toBeUndefined();
  // a key another handler already used is left alone too
  const e = new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true, cancelable: true });
  e.preventDefault();
  document.dispatchEvent(e);
  expect(current(w)).toBe(0);
});

test("a click on the deck gives it the keys", () => {
  const w = render(DECK);
  const r = w.querySelector<HTMLElement>(".slides-root")!;
  r.querySelector(".slides-viewport")!.dispatchEvent(new Event("pointerdown", { bubbles: true }));
  expect(document.activeElement).toBe(r);
  r.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(current(w)).toBe(1);
});

test("the ratio option sets the slide shape", () => {
  let w = render(DECK);
  expect(w.querySelector<HTMLElement>(".slides-root")!.style.getPropertyValue("--slides-ratio")).toBe("16 / 9");
  w = render(DECK, { ratio: "4:3" });
  expect(w.querySelector<HTMLElement>(".slides-root")!.style.getPropertyValue("--slides-ratio")).toBe("4 / 3");
  w = render(DECK, { ratio: "wide" });
  expect(w.querySelector<HTMLElement>(".slides-root")!.style.getPropertyValue("--slides-ratio")).toBe("16 / 9");
});

test("clean-up gives back the page and stops listening", () => {
  const w = render(DECK);
  rt.unmountAll();
  expect(w.querySelector(".slides-root")).toBeNull();
  expect(w.querySelectorAll("hr")).toHaveLength(2);
  expect(w.querySelectorAll("p").length).toBe(3);
  expect(Array.from(w.querySelectorAll("*")).some((e) => (e as HTMLElement).hidden)).toBe(false);
  const n = navigated.length;
  key("ArrowRight");
  expect(navigated).toHaveLength(n);
  expect(listeners).toHaveLength(0);
});

test("the stylesheet prints one slide per page and uses only the app's colours", () => {
  const css = readFileSync(join(ASSETS, "slides.css"), "utf8");
  expect(css).toMatch(/@media print/);
  expect(css).toMatch(/break-after:\s*page|page-break-after:\s*always/);
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
});
