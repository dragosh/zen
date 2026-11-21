// The demo folder's frontend-only plugin (examples/showcase/.zen/plugins/progress),
// loaded as the app loads it: an ES module with a default export, imported by the runtime.
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach, expect, test } from "vitest";
import { createFakeApi, type FakeApi } from "../../apps/zenmark/frontend/src/api/fake";
import { installRuntime } from "../src/plugins";

const DIR = join(dirname(fileURLToPath(import.meta.url)), "../../examples/showcase/.zen/plugins/progress");
const file = (name: string) => join(DIR, name);

let api: FakeApi;
let rt: ReturnType<typeof installRuntime>;
/** The plugin script, an ES module with a default export; the runtime gets it through importModule. */
let mod: unknown;
let root: HTMLElement;

beforeEach(async () => {
  document.body.replaceChildren();
  api = createFakeApi();
  rt = installRuntime({ importModule: async () => mod, api, navigate: () => {}, page: () => ({ path: "plugins/writing-plugins.md", title: "Writing plugins" }) });
  root = document.body.appendChild(document.createElement("article"));
  // @ts-ignore plain browser JavaScript outside src, no types
  mod = await import("../../examples/showcase/.zen/plugins/progress/progress.js");
  await rt.load({ js: ["/_plugins/progress/progress.js?v=test"], css: [] });
});
afterEach(() => rt.destroy());

/** What zen renders for a :::progress container holding a task list. */
function render(attrs: string | null, items: [string, boolean][]): HTMLElement {
  const w = document.createElement("div");
  w.setAttribute("data-zen-plugin", "progress");
  w.setAttribute("data-zen-name", "progress");
  if (attrs !== null) w.setAttribute("data-zen-attrs", attrs);
  const ul = w.appendChild(document.createElement("ul"));
  for (const [text, done] of items) {
    const li = ul.appendChild(document.createElement("li"));
    const box = li.appendChild(document.createElement("input"));
    box.type = "checkbox";
    box.disabled = true;
    box.checked = done;
    li.append(` ${text}`);
  }
  root.replaceChildren(w);
  rt.mountAll(root);
  return w;
}
const ITEMS: [string, boolean][] = [["Read the how-to", false], ["Allow the plugins", true], ["Write your own", false]];
const boxes = (w: HTMLElement) => Array.from(w.querySelectorAll<HTMLInputElement>("input[type=checkbox]"));
const count = (w: HTMLElement) => w.querySelector(".progress-count")?.textContent;
const settle = async () => { for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0)); };
const tick = (box: HTMLInputElement) => { box.click(); };

test("the plugin folder is a frontend-only plugin", () => {
  const yml = readFileSync(file("plugin.yml"), "utf8");
  expect(yml).toMatch(/^id: progress$/m);
  expect(yml).toMatch(/^apiVersion: 1$/m);
  expect(yml).toMatch(/^blocks: \[progress\]$/m);
  expect(yml).toMatch(/js: \[progress\.js\]/);
  expect(yml).toMatch(/css: \[progress\.css\]/);
  expect(yml).not.toMatch(/^backend:/m);
  expect(existsSync(file("progress.css"))).toBe(true);
  const css = readFileSync(file("progress.css"), "utf8");
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i); // the app's tokens only
});

test("turns the task list into a checklist and counts what is done", async () => {
  const w = render("{\"id\":\"tour\"}", ITEMS);
  await settle();
  expect(boxes(w).map((b) => b.disabled)).toEqual([false, false, false]);
  expect(boxes(w).map((b) => b.checked)).toEqual([false, true, false]);
  expect(count(w)).toBe("1 of 3 done");
  expect(api.callsTo("pluginStorageGet")).toEqual([["progress", "tour"]]);
});

test("ticking updates the count and is remembered across renders", async () => {
  let w = render("{\"id\":\"tour\"}", ITEMS);
  await settle();
  tick(boxes(w)[0]);
  await settle();
  expect(count(w)).toBe("2 of 3 done");
  expect(api.callsTo("pluginStorageSet")).toHaveLength(1);
  expect(api.callsTo("pluginStorageSet")[0].slice(0, 2)).toEqual(["progress", "tour"]);
  tick(boxes(w)[1]);
  await settle();
  expect(count(w)).toBe("1 of 3 done");

  // a re-render brings back the markdown's own ticks; the stored state wins
  w = render("{\"id\":\"tour\"}", ITEMS);
  await settle();
  expect(boxes(w).map((b) => b.checked)).toEqual([true, false, false]);
  expect(count(w)).toBe("1 of 3 done");

  // items are remembered by their text, so reordering keeps the ticks
  w = render("{\"id\":\"tour\"}", [ITEMS[2], ITEMS[0], ["A new step", false]]);
  await settle();
  expect(boxes(w).map((b) => b.checked)).toEqual([false, true, false]);
});

test("each id has its own state; without an id the page's blocks share one", async () => {
  let w = render("{\"id\":\"one\"}", ITEMS);
  await settle();
  tick(boxes(w)[2]);
  await settle();
  w = render("{\"id\":\"two\"}", ITEMS);
  await settle();
  expect(boxes(w).map((b) => b.checked)).toEqual([false, true, false]);

  w = render(null, ITEMS);
  await settle();
  tick(boxes(w)[0]);
  await settle();
  w = render("{\"id\":\"my list / 2\"}", ITEMS);
  await settle();
  tick(boxes(w)[0]);
  await settle();
  const keys = api.callsTo("pluginStorageSet").map((c) => c[1] as string);
  expect(keys).toEqual(["one", "default", "my-list---2"]);
  for (const k of keys) expect(k).toMatch(/^[A-Za-z0-9._-]{1,64}$/);
});

test("clean-up puts the block back as it was rendered", async () => {
  const w = render("{\"id\":\"tour\"}", ITEMS);
  await settle();
  rt.unmountAll();
  expect(w.querySelector(".progress-count")).toBeNull();
  expect(boxes(w).every((b) => b.disabled)).toBe(true);
  tick(boxes(w)[0]);
  await settle();
  expect(api.callsTo("pluginStorageSet")).toEqual([]);
});

test("a block without a task list says so instead of failing", async () => {
  const w = document.createElement("div");
  w.setAttribute("data-zen-plugin", "progress");
  w.append(document.createElement("p"));
  root.replaceChildren(w);
  rt.mountAll(root);
  await settle();
  expect(w.querySelector(".zen-plugin-error")).toBeNull();
  expect(count(w)).toBe("No tasks in this list.");
});
