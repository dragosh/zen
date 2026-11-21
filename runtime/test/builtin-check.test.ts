// The built-in check plugin's frontend (builtins/plugins/check/assets/check.js), mounted on the
// markup the engine emits for a :::check block, with the runtime's ctx and the fake api's storage.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach, expect, test } from "vitest";
import { createFakeApi, type FakeApi } from "../../apps/zenmark/frontend/src/api/fake";
import { installRuntime } from "../src/plugins";

const HERE = dirname(fileURLToPath(import.meta.url));
const ASSETS = join(HERE, "../../builtins/plugins/check/assets");

let api: FakeApi;
let rt: ReturnType<typeof installRuntime>;
let mod: unknown;
let root: HTMLElement;

beforeEach(async () => {
  document.body.replaceChildren();
  api = createFakeApi();
  rt = installRuntime({ importModule: async () => mod, api, navigate: () => {}, page: () => ({ path: "how-to/check.md", title: "Check" }) });
  root = document.body.appendChild(document.createElement("article"));
  // @ts-ignore plain browser JavaScript outside src, no types
  mod = await import("../../builtins/plugins/check/assets/check.js");
  await rt.load({ js: ["/_plugins/check/check.js?v=test"], css: [] });
});
afterEach(() => rt.destroy());

const settle = async () => { for (let i = 0; i < 8; i++) await new Promise((r) => setTimeout(r, 0)); };

/** What the engine renders for a :::check block: the task list inside the plugin's wrapper. */
function block(items: [string, boolean][]): HTMLElement {
  const w = document.createElement("div");
  w.setAttribute("data-zen-plugin", "check");
  w.setAttribute("data-zen-name", "check");
  const ul = w.appendChild(document.createElement("ul"));
  ul.className = "contains-task-list";
  for (const [text, on] of items) {
    const li = ul.appendChild(document.createElement("li"));
    const box = li.appendChild(document.createElement("input"));
    box.type = "checkbox";
    box.disabled = true;
    box.checked = on;
    li.append(" ");
    li.append(document.createTextNode(text));
  }
  return w;
}
async function render(...blocks: HTMLElement[]): Promise<HTMLElement[]> {
  root.replaceChildren(...blocks);
  rt.mountAll(root);
  await settle();
  return blocks;
}
const boxes = (w: HTMLElement) => Array.from(w.querySelectorAll<HTMLInputElement>("input[type=checkbox]"));
const state = (w: HTMLElement) => boxes(w).map((b) => b.checked);
const resetBtn = (w: HTMLElement) => w.querySelector<HTMLButtonElement>("button");
const sets = () => api.callsTo("pluginStorageSet");
const lastSaved = () => JSON.parse(sets().at(-1)![2] as string);

const ITEMS: [string, boolean][] = [["I opened a folder", false], ["I can see its pages", true], ["I read this", false]];

test("boxes of a check block are enabled; a plain task list stays disabled", async () => {
  const plain = document.createElement("ul");
  plain.innerHTML = '<li><input type="checkbox" disabled> plain</li>';
  root.append(plain);
  const [w] = await render(block(ITEMS));
  root.append(plain);
  expect(boxes(w).every((b) => !b.disabled)).toBe(true);
  expect(plain.querySelector("input")!.disabled).toBe(true);
  expect(w.querySelector(".zen-plugin-error")).toBeNull();
});

test("persists by label", async () => {
  const [w] = await render(block(ITEMS));
  boxes(w)[0].click();
  await settle();
  const call = sets().at(-1)!;
  expect(call[0]).toBe("check");
  expect(call[1]).toMatch(/^[A-Za-z0-9._-]{1,64}$/);
  expect(call[1]).toMatch(/^check-/);
  expect(lastSaved()).toEqual({ version: 1, ticked: ["I opened a folder", "I can see its pages"] });
  const key = call[1];

  // Another mount, with a new item in front: the ticks follow the labels.
  rt.unmountAll();
  const [w2] = await render(block([["A new item", false], ...ITEMS.map(([t]): [string, boolean] => [t, false])]));
  expect(state(w2)).toEqual([false, true, true, false]);
  expect(sets().filter((c) => c[1] !== key)).toEqual([]);
});

test("a click on the label text ticks the box", async () => {
  const [w] = await render(block(ITEMS));
  w.querySelectorAll("li")[0].dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await settle();
  expect(state(w)).toEqual([true, true, false]);
  w.querySelectorAll("li")[0].dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await settle();
  expect(state(w)).toEqual([false, true, false]);
});

test("the first block of a page and the second keep separate entries", async () => {
  const [a, b] = await render(block(ITEMS), block(ITEMS));
  boxes(b)[2].click();
  await settle();
  expect(state(a)).toEqual([false, true, false]);
  const keys = new Set(sets().map((c) => c[1]));
  expect(keys.size).toBe(1);
  rt.unmountAll();
  const [a2, b2] = await render(block(ITEMS), block(ITEMS));
  expect(state(a2)).toEqual([false, true, false]);
  expect(state(b2)).toEqual([false, true, true]);
});

test("bad stored value", async () => {
  const [w0] = await render(block(ITEMS));
  boxes(w0)[0].click();
  await settle();
  const key = sets().at(-1)![1] as string;
  rt.unmountAll();
  for (const bad of ['"x"', "42", "[1]", '{"version":2,"ticked":["I read this"]}', '{"version":1,"ticked":"nope"}', '{"version":1}', "null"]) {
    await api.pluginStorageSet("check", key, bad);
    const [w] = await render(block(ITEMS));
    expect(state(w), bad).toEqual([false, true, false]);
    expect(resetBtn(w), bad).toBeNull();
    rt.unmountAll();
  }
  // Non-string entries in a good value are dropped, not applied.
  await api.pluginStorageSet("check", key, '{"version":1,"ticked":[true,7,null,"I read this"]}');
  const [w] = await render(block(ITEMS));
  expect(state(w)).toEqual([false, false, true]);
});

test("reset", async () => {
  const [w] = await render(block(ITEMS));
  expect(resetBtn(w)).toBeNull();
  boxes(w)[0].click();
  await settle();
  const btn = resetBtn(w)!;
  expect(btn.textContent).toBe("Reset");
  expect(btn.type).toBe("button");
  expect(btn.getAttribute("aria-label")).toBe("Reset checklist");
  // After the list, not inside it.
  expect(w.querySelector("ul")!.nextElementSibling).toBe(btn);
  // Back to the markdown by hand: nothing differs, the button goes.
  boxes(w)[0].click();
  await settle();
  expect(resetBtn(w)).toBeNull();
  boxes(w)[0].click();
  boxes(w)[1].click();
  await settle();
  expect(state(w)).toEqual([true, false, false]);
  resetBtn(w)!.click();
  await settle();
  expect(state(w)).toEqual([false, true, false]);
  expect(resetBtn(w)).toBeNull();
  expect(sets().at(-1)![2]).toBe("null");
  expect(boxes(w).every((b) => !b.disabled)).toBe(true);
});

test("cleanup disables the boxes again and removes the button", async () => {
  const [w] = await render(block(ITEMS));
  boxes(w)[0].click();
  await settle();
  expect(resetBtn(w)).not.toBeNull();
  rt.unmountAll();
  expect(resetBtn(w)).toBeNull();
  expect(boxes(w).every((b) => b.disabled)).toBe(true);
  const n = sets().length;
  boxes(w)[0].dispatchEvent(new Event("change", { bubbles: true }));
  await settle();
  expect(sets().length).toBe(n);
});

test("course owns it", async () => {
  const layout = document.createElement("div");
  layout.setAttribute("data-zen-layout", "course");
  const w = block(ITEMS);
  layout.append(w);
  root.replaceChildren(layout);
  rt.mountAll(root);
  await settle();
  expect(boxes(w).every((b) => b.disabled)).toBe(true);
  expect(resetBtn(w)).toBeNull();
  expect(sets()).toEqual([]);
  expect(api.callsTo("pluginStorageGet")).toEqual([]);
});

test("labels are text", async () => {
  const hostile = '<img src=x onerror="alert(1)"> & <b>x</b>';
  const [w] = await render(block([[hostile, false], ["safe", false]]));
  expect(w.querySelector("img")).toBeNull();
  boxes(w)[0].click();
  await settle();
  expect(lastSaved().ticked).toEqual(["<img src=x onerror=\"alert(1)\"> & <b>x</b>"]);
  rt.unmountAll();
  await api.pluginStorageSet("check", sets().at(-1)![1] as string, JSON.stringify({ version: 1, ticked: ["<script>x</script>"] }));
  const [w2] = await render(block([["<script>x</script>", false], ["safe", false]]));
  expect(state(w2)).toEqual([true, false]);
  expect(w2.querySelector("script")).toBeNull();
});

test("whitespace in a label does not matter", async () => {
  const [w] = await render(block([["two   words\n here", false]]));
  boxes(w)[0].click();
  await settle();
  expect(lastSaved().ticked).toEqual(["two words here"]);
});

test("the stylesheet uses only the app's colours and no animation", () => {
  const css = readFileSync(join(ASSETS, "check.css"), "utf8");
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
  expect(css).not.toMatch(/transition|animation/);
});
