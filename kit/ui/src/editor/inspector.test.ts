// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import type { Alert } from "../host";
import { alert } from "../lint/test-util";
import { createStore } from "../shell/store";
import type { Ctx, Slots, Store } from "../shell/types";
import { editorFeature } from "./index";

const tick = () => new Promise((r) => setTimeout(r, 0));
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
const OPEN_KEY = "zen.inspector.open";

type Setup = { api: FakeApi; store: Store; slots: Slots; ta: HTMLTextAreaElement; unmount: () => void };

async function setup(path = "README.md", api = createFakeApi()): Promise<Setup> {
  const store = createStore({ path, mode: "edit" });
  const slots: Slots = {
    nav: document.createElement("nav"), breadcrumb: document.createElement("div"),
    actions: document.createElement("div"), main: document.createElement("main"),
  };
  document.body.append(slots.actions, slots.main);
  const ctx: Ctx = { api, store, brand: "Zenmark", navigate: () => undefined };
  const unmount = editorFeature.mount(slots, ctx);
  await tick();
  return { api, store, slots, ta: slots.main.querySelector("textarea")!, unmount };
}
function type(ta: HTMLTextAreaElement, value: string) {
  ta.value = value;
  ta.dispatchEvent(new Event("input", { bubbles: true }));
}
function key(target: EventTarget, k: string, mods: KeyboardEventInit = {}) {
  const ev = new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...mods });
  target.dispatchEvent(ev);
  return ev;
}
const toggle = (s: Setup) => s.slots.actions.querySelector<HTMLButtonElement>(".zen-inspector-toggle");
const panel = (s: Setup) => document.querySelector<HTMLElement>(".zen-inspector");
const field = (name: string) => document.querySelector<HTMLElement>(`.zen-inspector [data-field="${name}"]`)?.textContent;
const issuesBody = () => document.querySelector<HTMLElement>('.zen-inspector [data-section="issues"]')!;
const label = (b: Element) => b.textContent;
const ALERT: Alert = alert({ match: "dabc", line: 1, col: 3, endCol: 6, message: "Weak word" });

let s: Setup;
beforeEach(() => {
  localStorage.clear();
  (window as unknown as { innerWidth: number }).innerWidth = 1200;
});
afterEach(() => {
  s?.unmount();
  document.body.replaceChildren();
});

describe("editor inspector: button, panel, shortcut", () => {
  it("E1: the button is in the actions before Split and Done; the panel sits before the main pane", async () => {
    s = await setup();
    const names = Array.from(s.slots.actions.querySelectorAll("button")).map((b) => b.getAttribute("aria-label") ?? label(b));
    expect(names).toEqual(["Inspector", "Split", "Done"]);
    expect(toggle(s)!.hidden).toBe(false);
    expect(panel(s)!.hidden).toBe(true);
    expect(panel(s)!.nextElementSibling).toBe(s.slots.main);
  });

  it("E1: the toolbar toggle is the icon with a badge and a name that says the count; the panel is tabbed", async () => {
    s = await setup();
    expect(toggle(s)!.querySelector("svg")).not.toBeNull();
    expect(toggle(s)!.getAttribute("aria-label")).toBe("Inspector");
    expect(Array.from(panel(s)!.querySelectorAll('[role="tab"]')).map((t) => t.textContent)).toEqual(["Linting", "File", "Page"]);
  });

  it("E1: the button toggles the panel and remembers it under the reader's key", async () => {
    s = await setup();
    toggle(s)!.click();
    expect(panel(s)!.hidden).toBe(false);
    expect(localStorage.getItem(OPEN_KEY)).toBe("1");
    toggle(s)!.click();
    expect(panel(s)!.hidden).toBe(true);
    expect(localStorage.getItem(OPEN_KEY)).toBe("0");
  });

  it("E1: opens when it was left open", async () => {
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup();
    expect(panel(s)!.hidden).toBe(false);
  });

  it("E1: ⌥⌘I toggles, from the document and from the source", async () => {
    s = await setup();
    expect(key(document, "i", { metaKey: true, altKey: true, code: "KeyI" }).defaultPrevented).toBe(true);
    expect(panel(s)!.hidden).toBe(false);
    key(s.ta, "ˆ", { metaKey: true, altKey: true, code: "KeyI" });
    expect(panel(s)!.hidden).toBe(true);
  });

  it("E1: a settings file has no Inspector, no button, no shortcut", async () => {
    s = await setup("zen:config/global");
    expect(toggle(s)).toBeNull();
    expect(panel(s)).toBeNull();
    key(document, "i", { metaKey: true, altKey: true, code: "KeyI" });
    expect(panel(s)).toBeNull();
    expect(s.api.callsTo("preview")).toHaveLength(0);
  });
});

describe("editor inspector: issues", () => {
  it("E2: shows the alerts of the editor's lint next to the problems console, and selecting one selects it in the source", async () => {
    const api = createFakeApi();
    api.lintResult = { available: true, alerts: [ALERT] };
    s = await setup("README.md", api);
    toggle(s)!.click();
    await wait(50);
    expect(toggle(s)!.querySelector(".zen-inspector-badge")!.textContent).toBe("1");
    const item = issuesBody().querySelector<HTMLElement>(".zen-lint-item")!;
    expect(item.textContent).toContain("Weak word");
    expect(s.slots.main.querySelector(".zen-lint-console")).not.toBeNull(); // the console stays
    item.click();
    expect(s.ta.selectionStart).toBe(2);
    expect(s.ta.selectionEnd).toBe(6);
  });

  it("E2: shows Checking while the first lint is pending, then the result; Linting off when unavailable", async () => {
    const api = createFakeApi();
    let release!: () => void;
    const real = api.lint.bind(api);
    api.lint = (p, t) => new Promise((resolve) => { release = () => resolve(real(p, t)); });
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup("README.md", api);
    expect(issuesBody().textContent).toBe("Checking…");
    api.lintResult = { available: false, alerts: [] };
    release();
    await tick();
    expect(issuesBody().textContent).toBe("Linting off");
  });

  it("E2: typing updates the list after the lint delay", async () => {
    const api = createFakeApi();
    api.lintResult = { available: true, alerts: [] };
    s = await setup("README.md", api);
    toggle(s)!.click();
    await wait(30);
    expect(issuesBody().textContent).toBe("No issues");
    api.lintResult = { available: true, alerts: [ALERT, { ...ALERT, line: 2 }] };
    type(s.ta, "ab cdef\nsecond");
    await wait(750);
    expect(toggle(s)!.querySelector(".zen-inspector-badge")!.textContent).toBe("2");
  });
});

describe("editor inspector: file", () => {
  it("E3: shows the file and fetches it again after a save", async () => {
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup();
    await wait(30);
    expect(field("path")).toBe("README.md");
    expect(field("size")).not.toBe("…");
    const before = s.api.callsTo("pageInfo").length;
    expect(before).toBeGreaterThan(0);
    type(s.ta, "# Changed");
    key(s.ta, "s", { metaKey: true });
    await wait(30);
    expect(s.api.callsTo("pageInfo").length).toBeGreaterThan(before);
  });

  it("E3: Reveal in Finder reveals the page", async () => {
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup();
    await wait(30);
    document.querySelector<HTMLButtonElement>('.zen-inspector [data-action="reveal"]')!.click();
    expect(s.api.callsTo("revealInFinder")).toEqual([["README.md"]]);
  });
});

describe("editor inspector: page statistics", () => {
  it("E4: counts the unsaved text, 500 ms after typing stops, in one request", async () => {
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup();
    await wait(30);
    const before = s.api.callsTo("preview").length;
    type(s.ta, "# One");
    await wait(300);
    type(s.ta, "# One two three four");
    await wait(300);
    expect(s.api.callsTo("preview")).toHaveLength(before);
    await wait(300);
    expect(s.api.callsTo("preview")).toHaveLength(before + 1);
    expect(s.api.callsTo("preview").at(-1)).toEqual(["README.md", "# One two three four"]);
    expect(field("headings")).toBe("1");
    expect(field("words")).toBe("4");
  });

  it("E4: renders nothing while the Inspector is closed, and once when it opens", async () => {
    s = await setup();
    type(s.ta, "# Closed");
    await wait(700);
    expect(s.api.callsTo("preview")).toHaveLength(0);
    toggle(s)!.click();
    await wait(30);
    expect(s.api.callsTo("preview")).toEqual([["README.md", "# Closed"]]);
    expect(field("words")).toBe("1");
  });

  it("E4: leaves nothing of the render in the document", async () => {
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup();
    type(s.ta, "# T\n\n[x](https://x.org)");
    await wait(700);
    expect(document.querySelectorAll("body > a, .zen-editor a").length).toBe(0);
    expect(document.querySelector(".zen-editor-preview")).toBeNull();
    expect(field("links")).toBe("0 internal · 1 external");
  });

  it("E4: a stale answer is dropped", async () => {
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup();
    await wait(30);
    const waiting: { text: string; resolve(): void }[] = [];
    const real = s.api.preview.bind(s.api);
    s.api.preview = (path, text) => new Promise((resolve) => {
      void real(path, text).then((p) => waiting.push({ text, resolve: () => resolve(p) }));
    });
    type(s.ta, "# a b");
    await wait(650);
    type(s.ta, "# a b c d e");
    await wait(650);
    expect(waiting.map((w) => w.text)).toEqual(["# a b", "# a b c d e"]);
    waiting[1].resolve();
    await tick();
    waiting[0].resolve();
    await tick();
    expect(field("words")).toBe("5");
  });

  it("E4: with Split on, the statistics come from the preview and no extra request is made", async () => {
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup();
    Array.from(s.slots.actions.querySelectorAll("button")).find((b) => b.textContent === "Split")!.click();
    await wait(250);
    const before = s.api.callsTo("preview").length;
    type(s.ta, "# Split view of words");
    await wait(700);
    expect(s.api.callsTo("preview")).toHaveLength(before + 1); // the preview's own
    expect(field("words")).toBe("4");
    expect(field("headings")).toBe("1");
  });
});

describe("editor inspector: unmount", () => {
  it("E6: removes the button and the panel and stops rendering", async () => {
    localStorage.setItem(OPEN_KEY, "1");
    s = await setup();
    await wait(30);
    type(s.ta, "# Late");
    s.unmount();
    const calls = s.api.callsTo("preview").length;
    await wait(700);
    expect(toggle(s)).toBeNull();
    expect(panel(s)).toBeNull();
    expect(s.api.callsTo("preview")).toHaveLength(calls);
    s = await setup(); // afterEach unmounts again
  });
});
