// The built-in "course" layout (builtins/layouts/course/course.js), loaded as the app loads it.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach, expect, test } from "vitest";
import { createFakeApi, type FakeApi } from "../../apps/zenmark/frontend/src/api/fake";
import { installRuntime } from "../src/plugins";

// The built-in layouts live in builtins/layouts/ (they were plugins before core/layouts).
const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../..");
// Whichever of the two places has the script (a glob, so a missing one is no error).
const scripts = import.meta.glob<unknown>("../../builtins/layouts/course/course.js");
const ASSETS = join(ROOT, "builtins/layouts/course");
const PATH = "demo/layouts/course.md";

let api: FakeApi;
let rt: ReturnType<typeof installRuntime>;
/** The plugin script, an ES module with a default export; the runtime gets it through importModule. */
let mod: unknown;
let root: HTMLElement;
let badges: [string, string | null][] = [];

beforeEach(async () => {
  document.body.replaceChildren();
  badges = [];
  api = createFakeApi();
  rt = installRuntime({ importModule: async () => mod,
    api,
    navigate: () => {},
    page: () => ({ path: PATH, title: "Course" }),
    badge: (p, t) => { badges.push([p, t]); },
  });
  root = document.body.appendChild(document.createElement("article"));
  mod = await Object.values(scripts)[0]();
  await rt.load({ js: ["/_layouts/course/course.js?v=test"], css: [] });
});
afterEach(() => rt.destroy());

const check = (items: [string, boolean][]) =>
  `<div data-zen-plugin="course" data-zen-name="check"><ul>${items
    .map(([t, d]) => `<li><input type="checkbox" disabled${d ? " checked" : ""}> ${t}</li>`)
    .join("")}</ul></div>`;
// Step two holds a check block, step three none.
const COURSE = `<h1>Intro</h1><p>Welcome</p><h2>Install</h2><p>one</p>${check([["ran it", false]])}` +
  `<h2>Open</h2><p>two</p>${check([["a", false], ["b", false]])}<h2>Done</h2><p>three</p>`;

async function render(html = COURSE, attrs: Record<string, string> | null = { id: "intro" }): Promise<HTMLElement> {
  const w = document.createElement("div");
  w.setAttribute("data-zen-layout", "course");
  w.setAttribute("data-zen-layout-source", "builtin");
  if (attrs) w.setAttribute("data-zen-attrs", JSON.stringify(attrs));
  w.innerHTML = html; // test fixture only
  root.replaceChildren(w);
  rt.mountAll(root);
  await settle();
  return w;
}
const settle = async () => { for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0)); };
const steps = (w: HTMLElement) => Array.from(w.querySelectorAll<HTMLElement>(".course-step"));
const open = (w: HTMLElement) => steps(w).findIndex((s) => !s.hidden);
const states = (w: HTMLElement) => Array.from(w.querySelectorAll<HTMLElement>(".course-steps li")).map((l) => l.dataset.state);
const count = (w: HTMLElement) => w.querySelector(".course-count")?.textContent;
const btn = (w: HTMLElement, cls: string) => w.querySelector<HTMLButtonElement>(`.${cls}`)!;
const boxes = (w: HTMLElement, step: number) => Array.from(steps(w)[step].querySelectorAll<HTMLInputElement>("input[type=checkbox]"));
const next = async (w: HTMLElement) => { btn(w, "course-next").click(); await settle(); };
const tick = async (b: HTMLInputElement) => { b.click(); await settle(); };
const stored = (key: string) => {
  const calls = api.callsTo("layoutStorageSet").filter((c) => c[1] === key);
  return calls.length ? JSON.parse(calls[calls.length - 1][2] as string) : undefined;
};

test("each h2 is a step; only the current one shows, intro stays", async () => {
  const w = await render();
  expect(steps(w)).toHaveLength(3);
  expect(steps(w).map((s) => s.querySelector("h2")?.textContent)).toEqual(["Install", "Open", "Done"]);
  expect(open(w)).toBe(0);
  expect(steps(w).filter((s) => !s.hidden)).toHaveLength(1);
  expect(w.querySelector(".course-intro")?.textContent).toContain("Welcome");
  expect(Array.from(w.querySelectorAll(".course-steps li")).map((l) => l.textContent)).toEqual(["Install", "Open", "Done"]);
  expect(states(w)).toEqual(["current", "locked", "locked"]);
  expect(count(w)).toBe("0 of 3 steps");
  expect(btn(w, "course-prev").disabled).toBe(true);
  expect(badges.at(-1)).toEqual([PATH, "0/3"]);
});

test("a :::check block gates Next until every box is ticked", async () => {
  const w = await render();
  const [b] = boxes(w, 0);
  expect(b.disabled).toBe(false);
  expect(btn(w, "course-next").disabled).toBe(true);
  await tick(b);
  expect(btn(w, "course-next").disabled).toBe(false);
  await tick(b);
  expect(btn(w, "course-next").disabled).toBe(true);
});

test("Next marks the step done, moves on, and keeps progress and badge", async () => {
  const w = await render();
  await tick(boxes(w, 0)[0]);
  await next(w);
  expect(open(w)).toBe(1);
  expect(states(w)).toEqual(["done", "current", "locked"]);
  expect(count(w)).toBe("1 of 3 steps");
  expect(badges.at(-1)).toEqual([PATH, "1/3"]);
  expect(btn(w, "course-prev").disabled).toBe(false);
  const s = stored("intro");
  expect(s.step).toBe(1);
  expect(s.done).toEqual([0]);
  expect(api.callsTo("layoutStorageSet").every((c) => c[0] === "course")).toBe(true);

  const [a, b] = boxes(w, 1);
  expect(btn(w, "course-next").disabled).toBe(true);
  await tick(a);
  expect(btn(w, "course-next").disabled).toBe(true);
  await tick(b);
  expect(btn(w, "course-next").disabled).toBe(false);
});

test("Previous goes back; finished steps can be reopened from the list", async () => {
  const w = await render();
  await tick(boxes(w, 0)[0]);
  await next(w);
  btn(w, "course-prev").click();
  await settle();
  expect(open(w)).toBe(0);
  const items = Array.from(w.querySelectorAll<HTMLButtonElement>(".course-steps button"));
  expect(items[2].disabled).toBe(true); // locked
  items[1].click();
  await settle();
  expect(open(w)).toBe(1);
});

test("the last step finishes the course", async () => {
  const w = await render();
  await tick(boxes(w, 0)[0]);
  await next(w);
  await tick(boxes(w, 1)[0]);
  await tick(boxes(w, 1)[1]);
  await next(w);
  expect(open(w)).toBe(2);
  expect(btn(w, "course-next").textContent).toBe("Finish");
  await next(w);
  expect(count(w)).toBe("3 of 3 steps");
  expect(states(w)).toEqual(["done", "done", "done"]);
  expect(badges.at(-1)).toEqual([PATH, "3/3"]);
});

test("progress, ticks and the last open step come back after a re-render", async () => {
  let w = await render();
  await tick(boxes(w, 0)[0]);
  await next(w);
  await tick(boxes(w, 1)[1]);
  w = await render();
  expect(open(w)).toBe(1);
  expect(count(w)).toBe("1 of 3 steps");
  expect(boxes(w, 1).map((b) => b.checked)).toEqual([false, true]);
  expect(boxes(w, 0).map((b) => b.checked)).toEqual([true]);
  expect(badges.at(-1)).toEqual([PATH, "1/3"]);
});

test("progress is kept under layoutOptions.id, else the page path", async () => {
  let w = await render(COURSE, { id: "intro" });
  await next(await (async () => { await tick(boxes(w, 0)[0]); return w; })());
  expect(api.callsTo("layoutStorageGet")[0]).toEqual(["course", "intro"]);
  w = await render(COURSE, null);
  expect(open(w)).toBe(0); // a different key
  await tick(boxes(w, 0)[0]);
  const keys = api.callsTo("layoutStorageSet").map((c) => c[1] as string);
  expect(keys.at(-1)).toBe("demo-layouts-course.md");
  for (const k of keys) expect(k).toMatch(/^[A-Za-z0-9._-]{1,64}$/);
});

test("Reset clears progress, ticks and the badge", async () => {
  const w = await render();
  await tick(boxes(w, 0)[0]);
  await next(w);
  btn(w, "course-reset").click();
  await settle();
  expect(open(w)).toBe(0);
  expect(count(w)).toBe("0 of 3 steps");
  expect(boxes(w, 0).map((b) => b.checked)).toEqual([false]);
  expect(states(w)).toEqual(["current", "locked", "locked"]);
  expect(badges.at(-1)).toEqual([PATH, "0/3"]);
  const again = await render();
  expect(open(again)).toBe(0);
  expect(count(again)).toBe("0 of 3 steps");
});

test("clean-up gives back the page as rendered", async () => {
  const w = await render();
  rt.unmountAll();
  expect(w.querySelector(".course-root")).toBeNull();
  expect(w.querySelectorAll("h2")).toHaveLength(3);
  expect(Array.from(w.querySelectorAll("*")).some((e) => (e as HTMLElement).hidden)).toBe(false);
  expect(w.querySelectorAll("input").length).toBe(3);
  expect(Array.from(w.querySelectorAll("input")).every((i) => i.disabled)).toBe(true);
});

test("a page without h2 shows as one step; a lone :::check block does not fail", async () => {
  const w = await render("<h1>Only</h1><p>text</p>");
  expect(w.querySelector(".zen-plugin-error")).toBeNull();
  expect(count(w)).toBe("0 of 0 steps");
  const b = document.createElement("div");
  b.setAttribute("data-zen-plugin", "course");
  b.setAttribute("data-zen-name", "check");
  root.replaceChildren(b);
  rt.mountAll(root);
  await settle();
  expect(b.querySelector(".zen-plugin-error")).toBeNull();
});

test("the stylesheet uses only the app's colours", () => {
  const css = readFileSync(join(ASSETS, "course.css"), "utf8");
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
});

test("controls carry the plugin kit classes", async () => {
  const w = await render();
  expect(btn(w, "course-next").classList.contains("zen-btn")).toBe(true);
  expect(btn(w, "course-next").classList.contains("zen-btn-primary")).toBe(true);
  expect(btn(w, "course-prev").classList.contains("zen-btn")).toBe(true);
  expect(btn(w, "course-prev").classList.contains("zen-btn-primary")).toBe(false);
  expect(btn(w, "course-reset").classList.contains("zen-btn-quiet")).toBe(true);
  for (const c of Array.from(w.querySelectorAll('[data-zen-name="check"]'))) {
    expect(c.classList.contains("zen-block")).toBe(true);
    expect(Array.from(c.querySelectorAll("li")).every((li) => li.classList.contains("zen-choice"))).toBe(true);
  }
});

test("a step in the list has its title in its own element, so a wrapped title lines up", async () => {
  const w = await render();
  const link = w.querySelector(".course-steps .course-step-link")!;
  expect(link.querySelector(".course-step-title")!.textContent).toBe("Install");
  expect(link.textContent).toBe("Install");
});

// ---- Skip step (#13) ----
const skip = async (w: HTMLElement) => { btn(w, "course-skip").click(); await settle(); };

test("Skip step is a quiet button after Next in tab order", async () => {
  const w = await render();
  const s = btn(w, "course-skip");
  expect(s.textContent).toBe("Skip step");
  expect(s.classList.contains("zen-btn")).toBe(true);
  expect(s.classList.contains("zen-btn-quiet")).toBe(true);
  const all = Array.from(w.querySelectorAll("button"));
  expect(all.indexOf(s)).toBeGreaterThan(all.indexOf(btn(w, "course-next")));
});

test("skip unlocks the next step and marks the skipped one", async () => {
  const w = await render();
  expect(btn(w, "course-next").disabled).toBe(true); // gated, yet skippable
  await skip(w);
  expect(open(w)).toBe(1);
  expect(states(w)).toEqual(["skipped", "current", "locked"]);
  const items = Array.from(w.querySelectorAll<HTMLButtonElement>(".course-steps button"));
  expect(items[0].disabled).toBe(false);
  await skip(w);
  expect(open(w)).toBe(2);
  expect(states(w)).toEqual(["skipped", "skipped", "current"]);
});

test("the count and bar ignore skipped steps; the header adds the skipped number", async () => {
  const w = await render();
  expect(count(w)).toBe("0 of 3 steps");
  await skip(w);
  expect(count(w)).toBe("0 of 3 steps · 1 skipped");
  expect(w.querySelector<HTMLElement>(".course-progress-fill")!.style.width).toBe("0%");
  expect(badges.at(-1)).toEqual([PATH, "0/3"]);
  await tick(boxes(w, 1)[0]);
  await tick(boxes(w, 1)[1]);
  await next(w);
  expect(count(w)).toBe("1 of 3 steps · 1 skipped");
  expect(w.querySelector<HTMLElement>(".course-progress-fill")!.style.width).toBe(`${(1 / 3) * 100}%`);
});

test("a finished step has no Skip step button", async () => {
  const w = await render();
  await tick(boxes(w, 0)[0]);
  await next(w);
  btn(w, "course-prev").click();
  await settle();
  expect(open(w)).toBe(0);
  expect(btn(w, "course-skip").hidden).toBe(true);
  btn(w, "course-next").click();
  await settle();
  expect(btn(w, "course-skip").hidden).toBe(false);
});

test("skipped steps are saved and restored", async () => {
  let w = await render();
  await skip(w);
  expect(stored("intro").skipped).toEqual([0]);
  expect(stored("intro").done).toEqual([]);
  w = await render();
  expect(states(w)).toEqual(["skipped", "current", "locked"]);
  expect(open(w)).toBe(1);
  expect(count(w)).toBe("0 of 3 steps · 1 skipped");
});

test("ticking every box of a skipped step makes it done", async () => {
  const w = await render();
  await skip(w);
  btn(w, "course-prev").click();
  await settle();
  expect(open(w)).toBe(0);
  await tick(boxes(w, 0)[0]);
  expect(states(w)[0]).toBe("done");
  expect(count(w)).toBe("1 of 3 steps");
  expect(stored("intro").skipped).toEqual([]);
  expect(stored("intro").done).toEqual([0]);
});

test("Next on a skipped step without a checklist makes it done; Finish works with skips", async () => {
  const w = await render();
  await skip(w);
  await skip(w);
  expect(open(w)).toBe(2);
  expect(btn(w, "course-next").textContent).toBe("Finish");
  await next(w);
  expect(states(w)).toEqual(["skipped", "skipped", "done"]);
  expect(count(w)).toBe("1 of 3 steps · 2 skipped");
  // a step without a checklist, skipped, then Next
  const w2 = await render("<h2>A</h2><p>x</p><h2>B</h2><p>y</p>", { id: "plain" });
  await skip(w2);
  btn(w2, "course-prev").click();
  await settle();
  await next(w2);
  expect(states(w2)).toEqual(["done", "current"]);
});

test("Reset clears skipped steps", async () => {
  const w = await render();
  await skip(w);
  btn(w, "course-reset").click();
  await settle();
  expect(states(w)).toEqual(["current", "locked", "locked"]);
  expect(count(w)).toBe("0 of 3 steps");
  expect(stored("intro").skipped).toEqual([]);
});

test("a stored skipped with bad entries is filtered", async () => {
  await api.layoutStorageSet("course", "intro", JSON.stringify({ step: 0, done: [0], skipped: [0, 1, 1.5, -1, 9, "x", null] }));
  const w = await render();
  expect(states(w)[0]).toBe("done"); // done wins over skipped
  expect(states(w)[1]).toBe("skipped");
  expect(count(w)).toBe("1 of 3 steps · 1 skipped");
  await api.layoutStorageSet("course", "intro", JSON.stringify({ step: 0, skipped: "nope" }));
  expect(states(await render())).toEqual(["current", "locked", "locked"]);
});
