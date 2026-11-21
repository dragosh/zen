// The built-in rating plugin's frontend (builtins/plugins/rating/assets/rating.js), mounted on the
// markup its Go block hook emits, with ratings kept through the fake storage api.
import { afterEach, beforeEach, expect, test } from "vitest";
import { createFakeApi, type FakeApi } from "../../apps/zenmark/frontend/src/api/fake";
import { installRuntime } from "../src/plugins";


let api: FakeApi;
let rt: ReturnType<typeof installRuntime>;
let mod: unknown;
let root: HTMLElement;
let navigated: string[];

beforeEach(async () => {
  document.body.replaceChildren();
  navigated = [];
  api = createFakeApi();
  rt = installRuntime({ importModule: async () => mod, api, navigate: (p) => { navigated.push(p); }, page: () => ({ path: "courses/a.md", title: "A course" }) });
  root = document.body.appendChild(document.createElement("article"));
  // @ts-ignore plain browser JavaScript outside src, no types
  mod = await import("../../builtins/plugins/rating/assets/rating.js");
  await rt.load({ js: ["/_plugins/rating/rating.js?v=test"], css: [] });
});
afterEach(() => rt.destroy());

const settle = async () => { for (let i = 0; i < 8; i++) await new Promise((r) => setTimeout(r, 0)); };

/** What zen renders for a rating block: Go's text structure inside the wrapper (the sanitiser allows no fieldset or radio). */
function render(attrs: Record<string, string> = { id: "course", title: "How was this course?" }, labels: string[] = []): HTMLElement {
  const max = Number(attrs.max || 5);
  const w = document.createElement("div");
  w.setAttribute("data-zen-plugin", "rating");
  w.setAttribute("data-zen-name", "rating");
  w.setAttribute("data-zen-attrs", JSON.stringify(attrs));
  let stars = "";
  for (let i = 1; i <= max; i++) {
    stars += `<span class="zr-star"><span class="zr-glyph" aria-hidden="true">☆</span><span class="zr-sr"></span></span>`;
  }
  w.innerHTML = `<div class="zr-rating"><div class="zr-legend"></div><div class="zr-stars">${stars}</div><div class="zr-status">Not rated yet</div></div>`;
  // Text goes in through textContent, as Go's escaping does, so a hostile title stays text.
  w.querySelector(".zr-legend")!.textContent = attrs.title ?? "Rate this page";
  w.querySelectorAll(".zr-sr").forEach((e, i) => { e.textContent = labels[i] ?? `${i + 1} of ${max}`; });
  root.replaceChildren(w);
  rt.mountAll(root);
  return w;
}
const radios = (w: HTMLElement) => Array.from(w.querySelectorAll<HTMLInputElement>("input[type=radio]"));
const status = (w: HTMLElement) => w.querySelector(".zr-status")!.textContent;

const clearBtn = (w: HTMLElement) => w.querySelector<HTMLButtonElement>("button.zr-clear");
const key = (id = "course") => `rating-${id}`;
const writes = (id = "course") => api.callsTo("pluginStorageSet").filter((c) => c[1] === key(id)).map((c) => JSON.parse(c[2] as string));
const keydown = (el: HTMLElement, k: string) => el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
const seed = async (id: string, v: unknown) => { await api.pluginStorageSet("rating", key(id), JSON.stringify(v)); };

test("builds a fieldset with a legend and labelled radios in one group", async () => {
  const w = render();
  await settle();
  const fs = w.querySelector("fieldset")!;
  expect(fs.querySelector("legend")!.textContent).toBe("How was this course?");
  expect(radios(w)).toHaveLength(5);
  expect(new Set(radios(w).map((r) => r.name))).toEqual(new Set(["rating-course"]));
  expect(radios(w).map((r) => r.value)).toEqual(["1", "2", "3", "4", "5"]);
  for (const r of radios(w)) expect(r.closest("label")).not.toBeNull();
  expect(radios(w)[2].closest("label")!.textContent).toContain("3 of 5");
  expect(w.querySelector(".zr-status")!.getAttribute("aria-live")).toBe("polite");
});

test("starts not rated, with no Clear button", async () => {
  const w = render();
  await settle();
  expect(status(w)).toBe("Not rated yet");
  expect(radios(w).some((r) => r.checked)).toBe(false);
  expect(clearBtn(w)?.hidden ?? true).toBe(true);
});

test("mount reads the saved star", async () => {
  await seed("course", { version: 1, value: 4, max: 5, date: "2026-11-21T10:00:00.000Z" });
  const w = render();
  await settle();
  expect(radios(w).map((r) => r.checked)).toEqual([false, false, false, true, false]);
  expect(status(w)).toMatch(/^You rated 4 of 5 · \d{1,2} [A-Z][a-z]{2} \d{4}$/);
  expect(clearBtn(w)!.hidden).toBe(false);
});

test("picking a star saves value, max, date, page, title and question", async () => {
  const w = render();
  await settle();
  radios(w)[2].click();
  await settle();
  const v = writes().at(-1);
  expect(v).toMatchObject({ version: 1, value: 3, max: 5, page: "courses/a.md", title: "A course", question: "How was this course?" });
  expect(v.date).toMatch(/^\d{4}-\d\d-\d\d/);
  expect(status(w)).toMatch(/^You rated 3 of 5 · /);
  expect(clearBtn(w)!.hidden).toBe(false);
});

test("the stored key matches the storage key rule", async () => {
  const w = render({ id: "a".repeat(57) });
  await settle();
  radios(w)[0].click();
  await settle();
  for (const c of api.callsTo("pluginStorageSet")) expect(String(c[1])).toMatch(/^[A-Za-z0-9._-]{1,64}$/);
});

test("arrow keys move the choice, Home and End go to 1 and max", async () => {
  const w = render();
  await settle();
  radios(w)[1].click();
  await settle();
  for (const [k, want] of [["ArrowRight", 2], ["ArrowDown", 3], ["ArrowLeft", 2], ["ArrowUp", 1], ["End", 4], ["Home", 0]] as const) {
    keydown(radios(w).find((r) => r.checked)!, k);
    await settle();
    expect(radios(w).findIndex((r) => r.checked), k).toBe(want);
  }
  expect(writes().at(-1).value).toBe(1);
});

test("arrow keys stop at the ends", async () => {
  const w = render();
  await settle();
  radios(w)[4].click();
  await settle();
  keydown(radios(w)[4], "ArrowRight");
  await settle();
  expect(radios(w).findIndex((r) => r.checked)).toBe(4);
});

test("Clear writes null and returns to not rated", async () => {
  await seed("course", { version: 1, value: 2, max: 5, date: "2026-11-21T10:00:00.000Z" });
  const w = render();
  await settle();
  clearBtn(w)!.click();
  await settle();
  expect(api.callsTo("pluginStorageSet").filter((c) => c[1] === key()).at(-1)![2]).toBe("null");
  expect(status(w)).toBe("Not rated yet");
  expect(radios(w).some((r) => r.checked)).toBe(false);
  expect(clearBtn(w)!.hidden).toBe(true);
});

test("a bad stored value counts as not rated", async () => {
  for (const bad of [{ version: 2, value: 3, max: 5 }, { version: 1, value: 9, max: 5 }, { version: 1, value: 0, max: 5 }, { version: 1, value: 2.5, max: 5 }, { version: 1, value: "3", max: 5 }, "junk", [1], { version: 1, value: 3, max: 7 }]) {
    await seed("course", bad);
    const w = render();
    await settle();
    expect(radios(w).some((r) => r.checked), JSON.stringify(bad)).toBe(false);
    expect(status(w)).toBe("Not rated yet");
  }
});

test("max of 3 gives three stars", async () => {
  const w = render({ id: "short", title: "Short", max: "3" });
  await settle();
  expect(radios(w)).toHaveLength(3);
  radios(w)[2].click();
  await settle();
  expect(status(w)).toMatch(/^You rated 3 of 3 · /);
});

test("labels show beside the stars for the picked value", async () => {
  const labels = ["Poor", "Fair", "Good", "Very good", "Excellent"];
  const w = render({ id: "course", title: "How?", labels: labels.join("|") }, labels);
  await settle();
  const hint = () => w.querySelector(".zr-hint")?.textContent ?? "";
  expect(hint()).toBe("");
  radios(w)[3].click();
  await settle();
  expect(hint()).toBe("Very good");
  radios(w)[3].closest("label")!.dispatchEvent(new MouseEvent("mouseover", { bubbles: true }));
  radios(w)[0].closest("label")!.dispatchEvent(new MouseEvent("mouseover", { bubbles: true }));
  expect(hint()).toBe("Poor");
  w.querySelector(".zr-stars")!.dispatchEvent(new MouseEvent("mouseleave"));
  expect(hint()).toBe("Very good");
});

test("a hostile title and labels stay text", async () => {
  const w = render({ id: "course", title: "<img src=x onerror=alert(1)>", labels: "<b>x</b>" });
  await settle();
  radios(w)[0].click();
  await settle();
  expect(w.querySelector("img")).toBeNull();
  expect(writes().at(-1).question).toBe("<img src=x onerror=alert(1)>");
});

test("clean-up restores the markup", async () => {
  const w = render();
  await settle();
  radios(w)[1].click();
  await settle();
  expect(clearBtn(w)).not.toBeNull();
  rt.unmountAll();
  expect(w.querySelector("button")).toBeNull();
  expect(w.querySelector("fieldset")).toBeNull();
  expect(w.querySelector(".zr-legend")!.textContent).toBe("How was this course?");
  expect(w.querySelector(".zr-status")!.textContent).toBe("Not rated yet");
});

// ---- :::rating-results ----

function renderResults(): HTMLElement {
  const w = document.createElement("div");
  w.setAttribute("data-zen-plugin", "rating");
  w.setAttribute("data-zen-name", "rating-results");
  w.setAttribute("data-zen-attrs", "{}");
  w.innerHTML = '<div class="zr-results"></div>';
  root.replaceChildren(w);
  rt.mountAll(root);
  return w;
}

test("results say so when nothing is rated", async () => {
  const w = renderResults();
  await settle();
  expect(w.textContent).toContain("No ratings yet.");
});

test("results list each rating with page link, question, stars and date", async () => {
  await seed("one", { version: 1, value: 4, max: 5, date: "2026-11-21T10:00:00.000Z", page: "courses/a.md", title: "A course", question: "How was it?" });
  await seed("two", { version: 1, value: 2, max: 3, date: "2026-10-02T10:00:00.000Z", page: "guide/b.md", title: "<b>Guide</b>", question: "Useful?" });
  await seed("gone", null);
  await seed("bad", { version: 3, value: 1 });
  const w = renderResults();
  await settle();
  const items = Array.from(w.querySelectorAll("li"));
  expect(items).toHaveLength(2);
  const first = items.find((li) => li.textContent!.includes("A course"))!;
  expect(first.textContent).toContain("How was it?");
  expect(first.textContent).toContain("★★★★☆");
  expect(first.textContent).toMatch(/\d{1,2} [A-Z][a-z]{2} 2026/);
  const a = first.querySelector("a")!;
  expect(a.textContent).toBe("A course");
  a.click();
  expect(navigated).toEqual(["courses/a.md"]);
  expect(w.querySelector("b")).toBeNull();
  expect(w.textContent).toContain("<b>Guide</b>");
  expect(w.textContent).toContain("★★☆");
});
