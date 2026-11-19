// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import type { Page } from "../host";
import { pageHooks } from "../page";
import { createStore } from "../shell/store";
import type { Ctx, Slots, Store } from "../shell/types";
import { editorFeature } from "./index";

const tick = () => new Promise((r) => setTimeout(r, 0));
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
const SPLIT_KEY = "zen.editor.split";
const POS_KEY = "zen.editor.split.pos";

type Setup = { api: FakeApi; store: Store; slots: Slots; ta: HTMLTextAreaElement; unmount: () => void; navs: string[] };

async function setup(path = "README.md", api = createFakeApi()): Promise<Setup> {
  const store = createStore({ path, mode: "edit" });
  const slots: Slots = {
    nav: document.createElement("nav"), breadcrumb: document.createElement("div"),
    actions: document.createElement("div"), main: document.createElement("main"),
  };
  document.body.append(slots.actions, slots.main);
  const navs: string[] = [];
  const ctx: Ctx = { api, store, brand: "Zenmark", navigate: (p) => { navs.push(p); } };
  const unmount = editorFeature.mount(slots, ctx);
  await tick();
  return { api, store, slots, ta: slots.main.querySelector("textarea")!, unmount, navs };
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
const splitBtn = (s: Setup) => Array.from(s.slots.actions.querySelectorAll("button")).find((b) => b.textContent === "Split") as HTMLButtonElement | undefined;
const pane = (s: Setup) => s.slots.main.querySelector<HTMLElement>(".zen-editor-preview");
const divider = (s: Setup) => s.slots.main.querySelector<HTMLElement>(".zen-editor-divider");
const setWidth = (w: number) => { (window as unknown as { innerWidth: number }).innerWidth = w; window.dispatchEvent(new Event("resize")); };

let s: Setup;
beforeEach(() => {
  localStorage.clear();
  setWidth(1200);
});
afterEach(() => {
  s?.unmount();
  document.body.replaceChildren();
});

describe("editor split: toggle and persistence", () => {
  it("is off by default: a Split button, no preview, no api.preview", async () => {
    s = await setup();
    expect(splitBtn(s)!.getAttribute("aria-pressed")).toBe("false");
    expect(Array.from(s.slots.actions.querySelectorAll("button")).map((b) => b.getAttribute("aria-label") ?? b.textContent)).toEqual(["Inspector", "Split", "Done"]);
    expect(pane(s)).toBeNull();
    expect(divider(s)).toBeNull();
    expect(s.api.callsTo("preview")).toEqual([]);
  });

  it("the button shows the rendered unsaved text next to the source and remembers the choice", async () => {
    s = await setup();
    type(s.ta, "# Fresh");
    splitBtn(s)!.click();
    await wait(200);
    expect(splitBtn(s)!.getAttribute("aria-pressed")).toBe("true");
    expect(s.api.callsTo("preview").at(-1)).toEqual(["README.md", "# Fresh"]);
    expect(pane(s)!.querySelector("article.zen-page h1")!.textContent).toBe("Fresh");
    expect(s.slots.main.querySelector(".zen-editor-panes")!.contains(pane(s))).toBe(true);
    expect(localStorage.getItem(SPLIT_KEY)).toBe("1");
    splitBtn(s)!.click();
    expect(pane(s)).toBeNull();
    expect(divider(s)).toBeNull();
    expect(localStorage.getItem(SPLIT_KEY)).toBe("0");
  });

  it("opens split when it was left on", async () => {
    localStorage.setItem(SPLIT_KEY, "1");
    s = await setup();
    await wait(200);
    expect(pane(s)).not.toBeNull();
    expect(pane(s)!.querySelector("h1")!.textContent).toBe("Sample docs");
  });

  it("⌘⇧P toggles; plain ⌘P does not", async () => {
    s = await setup();
    expect(key(document, "p", { metaKey: true }).defaultPrevented).toBe(false);
    expect(pane(s)).toBeNull();
    expect(key(document, "p", { metaKey: true, shiftKey: true }).defaultPrevented).toBe(true);
    expect(pane(s)).not.toBeNull();
    key(document, "P", { ctrlKey: true, shiftKey: true });
    expect(pane(s)).toBeNull();
  });

  it("works from the source textarea too", async () => {
    s = await setup();
    key(s.ta, "p", { metaKey: true, shiftKey: true });
    expect(pane(s)).not.toBeNull();
  });

  it("stacks the panes below 900px", async () => {
    setWidth(700);
    s = await setup();
    splitBtn(s)!.click();
    const panes = s.slots.main.querySelector(".zen-editor-panes")!;
    expect(panes.classList.contains("is-stacked")).toBe(true);
    expect(divider(s)!.getAttribute("aria-orientation")).toBe("horizontal");
    setWidth(1200);
    expect(panes.classList.contains("is-stacked")).toBe(false);
    expect(divider(s)!.getAttribute("aria-orientation")).toBe("vertical");
  });
});

describe("editor split: divider", () => {
  it("starts at 50 and moves 5 per arrow key, clamped to 20 to 80", async () => {
    s = await setup();
    splitBtn(s)!.click();
    const d = divider(s)!;
    expect(d.getAttribute("role")).toBe("separator");
    expect(d.tabIndex).toBe(0);
    expect(d.getAttribute("aria-valuenow")).toBe("50");
    expect(d.getAttribute("aria-valuemin")).toBe("20");
    expect(d.getAttribute("aria-valuemax")).toBe("80");
    expect(key(d, "ArrowRight").defaultPrevented).toBe(true);
    expect(d.getAttribute("aria-valuenow")).toBe("55");
    key(d, "ArrowLeft");
    key(d, "ArrowLeft");
    expect(d.getAttribute("aria-valuenow")).toBe("45");
    for (let i = 0; i < 20; i++) key(d, "ArrowRight");
    expect(d.getAttribute("aria-valuenow")).toBe("80");
    for (let i = 0; i < 30; i++) key(d, "ArrowLeft");
    expect(d.getAttribute("aria-valuenow")).toBe("20");
    expect(localStorage.getItem(POS_KEY)).toBe("20");
    expect(s.slots.main.querySelector<HTMLElement>(".zen-editor-panes")!.style.getPropertyValue("--zen-split")).toBe("20%");
  });

  it("remembers the position and ignores a stored value out of range", async () => {
    localStorage.setItem(SPLIT_KEY, "1");
    localStorage.setItem(POS_KEY, "65");
    s = await setup();
    expect(divider(s)!.getAttribute("aria-valuenow")).toBe("65");
    s.unmount();
    document.body.replaceChildren();
    localStorage.setItem(POS_KEY, "5");
    s = await setup();
    expect(divider(s)!.getAttribute("aria-valuenow")).toBe("20");
  });

  it("drags with the mouse, clamped, and saves on release", async () => {
    s = await setup();
    splitBtn(s)!.click();
    const panes = s.slots.main.querySelector<HTMLElement>(".zen-editor-panes")!;
    panes.getBoundingClientRect = () => ({ left: 100, top: 0, width: 1000, height: 600, right: 1100, bottom: 600, x: 100, y: 0, toJSON() {} });
    const d = divider(s)!;
    d.dispatchEvent(new MouseEvent("mousedown", { button: 0, bubbles: true, clientX: 600 }));
    window.dispatchEvent(new MouseEvent("mousemove", { clientX: 400 }));
    expect(d.getAttribute("aria-valuenow")).toBe("30");
    window.dispatchEvent(new MouseEvent("mousemove", { clientX: 1090 }));
    expect(d.getAttribute("aria-valuenow")).toBe("80");
    window.dispatchEvent(new MouseEvent("mousemove", { clientX: 0 }));
    window.dispatchEvent(new MouseEvent("mouseup"));
    expect(d.getAttribute("aria-valuenow")).toBe("20");
    expect(localStorage.getItem(POS_KEY)).toBe("20");
    window.dispatchEvent(new MouseEvent("mousemove", { clientX: 600 }));
    expect(d.getAttribute("aria-valuenow")).toBe("20");
  });

  it("drags vertically when stacked", async () => {
    setWidth(700);
    s = await setup();
    splitBtn(s)!.click();
    const panes = s.slots.main.querySelector<HTMLElement>(".zen-editor-panes")!;
    panes.getBoundingClientRect = () => ({ left: 0, top: 50, width: 700, height: 400, right: 700, bottom: 450, x: 0, y: 50, toJSON() {} });
    const d = divider(s)!;
    d.dispatchEvent(new MouseEvent("mousedown", { button: 0, bubbles: true, clientY: 250 }));
    window.dispatchEvent(new MouseEvent("mousemove", { clientY: 130 }));
    window.dispatchEvent(new MouseEvent("mouseup"));
    expect(d.getAttribute("aria-valuenow")).toBe("20");
  });
});

describe("editor split: preview updates", () => {
  it("debounces typing by 150 ms into one request", async () => {
    s = await setup();
    splitBtn(s)!.click();
    await wait(200);
    const before = s.api.callsTo("preview").length;
    type(s.ta, "# A");
    await wait(60);
    type(s.ta, "# AB");
    await wait(60);
    type(s.ta, "# ABC");
    await wait(100);
    expect(s.api.callsTo("preview")).toHaveLength(before);
    await wait(100);
    expect(s.api.callsTo("preview")).toHaveLength(before + 1);
    expect(s.api.callsTo("preview").at(-1)).toEqual(["README.md", "# ABC"]);
    expect(pane(s)!.querySelector("h1")!.textContent).toBe("ABC");
  });

  it("the latest response wins, even when an older one arrives last", async () => {
    s = await setup();
    const waiting: { text: string; resolve(): void }[] = [];
    const real = s.api.preview.bind(s.api);
    s.api.preview = (path, text) => new Promise<Page>((resolve) => {
      void real(path, text).then((p) => waiting.push({ text, resolve: () => resolve(p) }));
    });
    splitBtn(s)!.click();
    await wait(200);
    waiting.splice(0).forEach((w) => w.resolve()); // the first render
    type(s.ta, "# One");
    await wait(200);
    type(s.ta, "# Two");
    await wait(200);
    expect(waiting.map((w) => w.text)).toEqual(["# One", "# Two"]);
    waiting[1].resolve();
    await tick();
    waiting[0].resolve();
    await tick();
    expect(pane(s)!.querySelector("h1")!.textContent).toBe("Two");
  });

  it("keeps the last good preview when a request fails", async () => {
    s = await setup();
    splitBtn(s)!.click();
    await wait(200);
    s.api.preview = () => Promise.reject(new Error("boom"));
    type(s.ta, "# Broken");
    await wait(200);
    expect(pane(s)!.querySelector("h1")!.textContent).toBe("Sample docs");
  });

  it("does not render while split is off", async () => {
    s = await setup();
    type(s.ta, "# x");
    await wait(250);
    expect(s.api.callsTo("preview")).toEqual([]);
  });

  it("follows the source scroll proportionally", async () => {
    s = await setup();
    splitBtn(s)!.click();
    await wait(200);
    const p = pane(s)!;
    for (const [n, v] of [["scrollHeight", 1000], ["clientHeight", 200]] as const) Object.defineProperty(p, n, { value: v, configurable: true });
    for (const [n, v] of [["scrollHeight", 2200], ["clientHeight", 200]] as const) Object.defineProperty(s.ta, n, { value: v, configurable: true });
    s.ta.scrollTop = 1000;
    s.ta.dispatchEvent(new Event("scroll"));
    expect(p.scrollTop).toBe(400);
  });
});

describe("editor split: inert links and plugins", () => {
  const withLinks = (api: FakeApi) => {
    api.decorate = (p) => ({ ...p, html: `${p.html}<p><a href="b.md" data-zen-link="internal" data-path="b.md">in</a> <a href="https://example.com" data-zen-link="external">out</a></p>` });
  };

  it("clicking links in the preview neither navigates nor opens anything", async () => {
    const api = createFakeApi();
    withLinks(api);
    s = await setup("README.md", api);
    splitBtn(s)!.click();
    await wait(200);
    for (const a of Array.from(pane(s)!.querySelectorAll("a"))) {
      const ev = new MouseEvent("click", { bubbles: true, cancelable: true });
      a.dispatchEvent(ev);
      expect(ev.defaultPrevented).toBe(true);
      const mid = new MouseEvent("auxclick", { bubbles: true, cancelable: true, button: 1 });
      a.dispatchEvent(mid);
      expect(mid.defaultPrevented).toBe(true);
    }
    expect(s.navs).toEqual([]);
    expect(api.callsTo("openExternal")).toEqual([]);
  });

  it("mounts plugin scripts on the preview blocks like the reader", async () => {
    const api = createFakeApi();
    const url = "/_plugins/zz/zz.js?v=aaaaaaaaaaaa";
    api.decorate = (p) => ({ ...p, html: `${p.html}<div data-zen-plugin="zz" data-zen-name="box">raw</div>`, assets: { css: [], js: [url] } });
    const saved = pageHooks.importModule;
    const mounted: string[] = [];
    pageHooks.importModule = async () => ({ default: { mount(el: HTMLElement) { el.textContent = "live"; mounted.push("m"); return () => mounted.push("c"); } } });
    try {
      s = await setup("README.md", api);
      splitBtn(s)!.click();
      await wait(200);
      expect(pane(s)!.querySelector('[data-zen-plugin="zz"]')!.textContent).toBe("live");
      type(s.ta, "# Again");
      await wait(250);
      expect(mounted).toEqual(["m", "c", "m"]);
      splitBtn(s)!.click();
      expect(mounted).toEqual(["m", "c", "m", "c"]);
    } finally {
      pageHooks.importModule = saved;
    }
  });
});

describe("editor split: settings files", () => {
  const config = () => createFakeApi({ sources: { "zen:config/global": "version: 1\n" } });

  it("hides Split for zen:config/* and ignores the shortcut, even when remembered", async () => {
    localStorage.setItem(SPLIT_KEY, "1");
    s = await setup("zen:config/global", config());
    expect(splitBtn(s)).toBeUndefined();
    expect(key(document, "p", { metaKey: true, shiftKey: true }).defaultPrevented).toBe(false);
    await wait(200);
    expect(pane(s)).toBeNull();
    expect(s.api.callsTo("preview")).toEqual([]);
  });
});

describe("editor split: everything else is unchanged", () => {
  it("drafts, lint, save, line numbers and the full-width console behave the same", async () => {
    s = await setup();
    splitBtn(s)!.click();
    type(s.ta, "# Fresh\n");
    await wait(200);
    expect(s.api.broadcasts.at(-1)).toEqual({ event: "editor:draft", payload: { path: "README.md", content: "# Fresh\n" } });
    expect(s.store.get().dirty).toBe(true);
    await wait(500);
    expect(s.api.callsTo("lint").at(-1)).toEqual(["README.md", "# Fresh\n"]);
    expect(s.slots.main.querySelectorAll(".zen-ed-line").length).toBeGreaterThan(0);
    const root = s.slots.main.querySelector(".zen-editor")!;
    const panes = root.querySelector(".zen-editor-panes")!;
    const consoleEl = root.querySelector(".zen-lint-console")!;
    expect(consoleEl.parentElement).toBe(root);
    expect(panes.contains(consoleEl)).toBe(false);
    key(s.ta, "s", { metaKey: true });
    await tick();
    expect(s.api.callsTo("save")).toEqual([["README.md", "# Fresh\n"]]);
    expect(s.store.get().dirty).toBe(false);
    expect(s.api.broadcasts.filter((b) => b.event === "editor:draft").at(-1)).toEqual({ event: "editor:draft", payload: { path: "README.md", content: null } });
  });

  it("toggling split changes no draft, save or dirty state", async () => {
    s = await setup();
    type(s.ta, "# Fresh");
    splitBtn(s)!.click();
    splitBtn(s)!.click();
    expect(s.store.get().dirty).toBe(true);
    expect(s.ta.value).toBe("# Fresh");
    expect(s.api.callsTo("save")).toEqual([]);
  });

  it("the unsaved dialog still guards Done with split on", async () => {
    s = await setup();
    splitBtn(s)!.click();
    type(s.ta, "x");
    Array.from(s.slots.actions.querySelectorAll("button")).find((b) => b.textContent === "Done")!.click();
    await tick();
    expect(document.querySelector("[role=dialog]")).not.toBeNull();
  });
});
