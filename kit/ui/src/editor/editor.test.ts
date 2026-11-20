// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import { createStore } from "../shell/store";
import type { Ctx, Slots, Store } from "../shell/types";
import { editorFeature } from "./index";

const tick = () => new Promise((r) => setTimeout(r, 0));
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

type Setup = { api: FakeApi; store: Store; slots: Slots; ta: HTMLTextAreaElement; unmount: () => void };

async function setup(path = "README.md"): Promise<Setup> {
  const api = createFakeApi();
  const store = createStore({ path, mode: "edit" });
  const slots: Slots = {
    nav: document.createElement("nav"), breadcrumb: document.createElement("div"),
    actions: document.createElement("div"), main: document.createElement("main"),
  };
  document.body.append(slots.actions, slots.main);
  const ctx: Ctx = { api, store, brand: "Zenmark", navigate: (p) => store.set({ path: p, anchor: "" }) };
  const unmount = editorFeature.mount(slots, ctx);
  await tick();
  const ta = slots.main.querySelector("textarea")!;
  return { api, store, slots, ta, unmount };
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
const dialog = () => document.querySelector<HTMLElement>("[role=dialog]");
function answer(label: string) {
  const b = Array.from(dialog()!.querySelectorAll("button")).find((x) => x.textContent === label)!;
  b.click();
}
const buttonByText = (root: ParentNode, text: string) =>
  Array.from(root.querySelectorAll("button")).find((b) => b.textContent === text)!;

let s: Setup;
beforeEach(() => {
  localStorage.clear();
});
afterEach(() => {
  s?.unmount();
  document.body.replaceChildren();
});

describe("editor: load and keys", () => {
  it("loads the source into a textarea", async () => {
    s = await setup();
    expect(s.api.callsTo("source")).toEqual([["README.md"]]);
    expect(s.ta.value).toMatch(/^# Sample docs/);
    expect(s.store.get().dirty).toBe(false);
    expect(s.slots.main.querySelector("[role=toolbar]")).not.toBeNull();
  });

  it("Tab inserts two spaces and Shift-Tab outdents", async () => {
    s = await setup();
    type(s.ta, "ab");
    s.ta.setSelectionRange(1, 1);
    expect(key(s.ta, "Tab").defaultPrevented).toBe(true);
    expect(s.ta.value).toBe("a  b");
    s.ta.setSelectionRange(0, 0);
    type(s.ta, "  x");
    s.ta.setSelectionRange(3, 3);
    key(s.ta, "Tab", { shiftKey: true });
    expect(s.ta.value).toBe("x");
  });

  it("Enter continues a list and ends an empty item", async () => {
    s = await setup();
    type(s.ta, "- a");
    s.ta.setSelectionRange(3, 3);
    expect(key(s.ta, "Enter").defaultPrevented).toBe(true);
    expect(s.ta.value).toBe("- a\n- ");
    key(s.ta, "Enter");
    expect(s.ta.value).toBe("- a\n");
    type(s.ta, "plain");
    s.ta.setSelectionRange(5, 5);
    expect(key(s.ta, "Enter").defaultPrevented).toBe(false);
  });

  it("⌘B applies bold to the selection", async () => {
    s = await setup();
    type(s.ta, "a b c");
    s.ta.setSelectionRange(2, 3);
    key(s.ta, "b", { metaKey: true });
    expect(s.ta.value).toBe("a **b** c");
    expect(s.store.get().dirty).toBe(true);
  });

  it("toolbar buttons edit the textarea", async () => {
    s = await setup();
    type(s.ta, "x");
    s.ta.setSelectionRange(0, 1);
    s.slots.main.querySelector<HTMLButtonElement>("[data-command=h1]")!.click();
    expect(s.ta.value).toBe("# x");
  });
});

describe("editor: split is optional, main window follows", () => {
  it("starts source-only: Inspector, Split and Done, no preview, no divider", async () => {
    s = await setup();
    expect(Array.from(s.slots.actions.querySelectorAll("button")).map((b) => b.getAttribute("aria-label") ?? b.textContent)).toEqual(["Inspector", "Split", "Done"]);
    expect(s.slots.main.querySelector("[role=separator]:not(.zen-lint-console-resize), .zen-editor-preview, .zen-page")).toBeNull();
    expect(s.api.callsTo("preview")).toEqual([]);
  });

  it("shares unsaved text with the main window after typing stops", async () => {
    s = await setup();
    type(s.ta, "# Fresh");
    expect(s.api.broadcasts).toEqual([]);
    await wait(200);
    expect(s.api.broadcasts.at(-1)).toEqual({ event: "editor:draft", payload: { path: "README.md", content: "# Fresh" } });
    expect(s.api.callsTo("preview")).toEqual([]);
  });

  it("drops the draft when the text is saved", async () => {
    s = await setup();
    type(s.ta, "# Fresh");
    await wait(200);
    key(s.ta, "s", { metaKey: true });
    await tick();
    expect(s.api.broadcasts.filter((b) => b.event === "editor:draft").at(-1))
      .toEqual({ event: "editor:draft", payload: { path: "README.md", content: null } });
  });

  it("drops the draft when typing returns to the saved text", async () => {
    s = await setup();
    const original = s.ta.value;
    type(s.ta, original + "x");
    await wait(200);
    type(s.ta, original);
    await wait(200);
    expect(s.api.broadcasts.at(-1)).toEqual({ event: "editor:draft", payload: { path: "README.md", content: null } });
  });

  it("drops the draft when the editor goes away", async () => {
    s = await setup();
    type(s.ta, "# Fresh");
    await wait(200);
    s.unmount();
    expect(s.api.broadcasts.at(-1)).toEqual({ event: "editor:draft", payload: { path: "README.md", content: null } });
  });
});

describe("editor: dirty and save", () => {
  it("tracks dirty against the loaded text", async () => {
    s = await setup();
    const original = s.ta.value;
    type(s.ta, original + "x");
    expect(s.store.get().dirty).toBe(true);
    type(s.ta, original);
    expect(s.store.get().dirty).toBe(false);
  });

  it("⌘S saves and clears dirty", async () => {
    s = await setup();
    type(s.ta, "# New");
    const ev = key(s.ta, "s", { metaKey: true });
    expect(ev.defaultPrevented).toBe(true);
    await tick();
    expect(s.api.callsTo("save")).toEqual([["README.md", "# New"]]);
    expect(s.store.get().dirty).toBe(false);
    // The main window re-renders on editor:saved even without file watching.
    expect(s.api.broadcasts).toEqual([{ event: "editor:saved", payload: { path: "README.md" } }]);
  });

  it("a failed save shows the message and keeps dirty", async () => {
    s = await setup();
    type(s.ta, "# New");
    s.api.failNext("save", new Error("disk full"));
    key(s.ta, "s", { metaKey: true });
    await tick();
    expect(s.store.get().dirty).toBe(true);
    expect(s.slots.main.textContent).toContain("disk full");
  });
});

describe("editor: Done closes the editor window", () => {
  let closes: number;
  const onClose = () => void closes++;
  beforeEach(() => {
    closes = 0;
    window.addEventListener("zen:close-window", onClose);
  });
  afterEach(() => window.removeEventListener("zen:close-window", onClose));

  it("Done when clean closes the window", async () => {
    s = await setup();
    buttonByText(s.slots.actions, "Done").click();
    await tick();
    expect(closes).toBe(1);
    expect(dialog()).toBeNull();
  });

  it("Esc with nothing focused acts as Done", async () => {
    s = await setup();
    (document.activeElement as HTMLElement | null)?.blur();
    key(document, "Escape");
    await tick();
    expect(closes).toBe(1);
  });

  it("Done when dirty asks; Cancel stays", async () => {
    s = await setup();
    type(s.ta, "changed");
    buttonByText(s.slots.actions, "Done").click();
    expect(dialog()).not.toBeNull();
    expect(dialog()!.getAttribute("aria-modal")).toBe("true");
    expect(dialog()!.getAttribute("aria-labelledby")).toBeTruthy();
    answer("Cancel");
    await tick();
    expect(dialog()).toBeNull();
    expect(closes).toBe(0);
    expect(s.store.get().dirty).toBe(true);
  });

  it("Done → Save saves then closes", async () => {
    s = await setup();
    type(s.ta, "changed");
    buttonByText(s.slots.actions, "Done").click();
    answer("Save");
    await tick();
    await tick();
    expect(s.api.callsTo("save")).toEqual([["README.md", "changed"]]);
    expect(s.api.broadcasts).toHaveLength(1);
    expect(closes).toBe(1);
  });

  it("Done → Save that fails stays open", async () => {
    s = await setup();
    type(s.ta, "changed");
    s.api.failNext("save", new Error("disk full"));
    buttonByText(s.slots.actions, "Done").click();
    answer("Save");
    await tick();
    await tick();
    expect(closes).toBe(0);
    expect(s.slots.main.textContent).toContain("disk full");
  });

  it("Done → Discard closes without saving", async () => {
    s = await setup();
    type(s.ta, "changed");
    buttonByText(s.slots.actions, "Done").click();
    answer("Discard");
    await tick();
    expect(s.api.callsTo("save")).toHaveLength(0);
    expect(closes).toBe(1);
    expect(s.store.get().dirty).toBe(false);
  });

  it("the page is fixed: store navigation does not load another page", async () => {
    s = await setup();
    type(s.ta, "changed");
    s.store.set({ path: "guides/usage.md" });
    await tick();
    expect(dialog()).toBeNull();
    expect(s.api.callsTo("source")).toEqual([["README.md"]]);
    expect(s.ta.value).toBe("changed");
  });
});

describe("editor: closing the window", () => {
  const beforeClose = () => {
    const calls: string[] = [];
    const ev = new CustomEvent("zen:before-close", {
      cancelable: true,
      detail: { close: () => calls.push("close"), cancel: () => calls.push("cancel") },
    });
    window.dispatchEvent(ev);
    return { prevented: ev.defaultPrevented, calls };
  };

  it("beforeunload is prevented while dirty", async () => {
    s = await setup();
    const clean = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(clean);
    expect(clean.defaultPrevented).toBe(false);
    type(s.ta, "changed");
    const ev = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
  });

  it("the close handshake passes when clean", async () => {
    s = await setup();
    expect(beforeClose().prevented).toBe(false);
  });

  it("the close handshake asks while dirty; Cancel reports cancel", async () => {
    s = await setup();
    type(s.ta, "changed");
    const r = beforeClose();
    expect(r.prevented).toBe(true);
    answer("Cancel");
    await tick();
    expect(r.calls).toEqual(["cancel"]);
    expect(s.store.get().dirty).toBe(true);
  });

  it("the close handshake asks while dirty; Save saves then closes", async () => {
    s = await setup();
    type(s.ta, "changed");
    const r = beforeClose();
    answer("Save");
    await tick();
    await tick();
    expect(s.api.callsTo("save")).toEqual([["README.md", "changed"]]);
    expect(r.calls).toEqual(["close"]);
  });

  it("the close handshake while Done is asking shares that dialog", async () => {
    s = await setup();
    type(s.ta, "changed");
    buttonByText(s.slots.actions, "Done").click();
    const r = beforeClose();
    expect(r.prevented).toBe(true);
    expect(document.querySelectorAll("[role=dialog]")).toHaveLength(1);
    answer("Discard");
    await tick();
    expect(r.calls).toEqual(["close"]);
  });
});

describe("editor: external change", () => {
  it("reloads silently when clean, keeping the caret", async () => {
    s = await setup();
    await s.api.save("README.md", "# Changed on disk\n");
    s.ta.setSelectionRange(3, 3);
    s.api.emit("page:changed", { path: "README.md" });
    await tick();
    expect(s.ta.value).toBe("# Changed on disk\n");
    expect(s.ta.selectionStart).toBe(3);
    expect(s.store.get().dirty).toBe(false);
  });

  it("ignores other paths", async () => {
    s = await setup();
    const n = s.api.callsTo("source").length;
    s.api.emit("page:changed", { path: "guides/usage.md" });
    await tick();
    expect(s.api.callsTo("source")).toHaveLength(n);
  });

  it("when dirty offers Reload from disk / Keep mine", async () => {
    s = await setup();
    type(s.ta, "mine");
    await s.api.save("README.md", "theirs");
    s.api.emit("page:changed", { path: "README.md" });
    await tick();
    expect(s.ta.value).toBe("mine");
    buttonByText(s.slots.main, "Keep mine").click();
    expect(s.ta.value).toBe("mine");
    expect(s.store.get().dirty).toBe(true);
    s.api.emit("page:changed", { path: "README.md" });
    await tick();
    buttonByText(s.slots.main, "Reload from disk").click();
    await tick();
    expect(s.ta.value).toBe("theirs");
    expect(s.store.get().dirty).toBe(false);
  });

  it("unmount removes listeners", async () => {
    s = await setup();
    s.unmount();
    type(s.ta, "x");
    const ev = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(false);
    s = { ...s, unmount: () => {} };
  });
});

describe("editor: lint", () => {
  const TEH = { line: 1, col: 1, endCol: 3, severity: "error" as const, check: "Vale.Spelling", message: "m", match: "teh", description: "", link: "" };
  async function lintSetup(alerts = [TEH]) {
    const api = createFakeApi({ sources: { "a.md": "teh cat" }, lint: { available: true, alerts } });
    const store = createStore({ path: "a.md", mode: "edit" });
    const slots: Slots = {
      nav: document.createElement("nav"), breadcrumb: document.createElement("div"),
      actions: document.createElement("div"), main: document.createElement("main"),
    };
    document.body.append(slots.actions, slots.main);
    const ctx: Ctx = { api, store, brand: "Zenmark", navigate: () => {} };
    const unmount = editorFeature.mount(slots, ctx);
    for (let i = 0; i < 5; i++) await tick();
    const ta = slots.main.querySelector("textarea")!;
    s = { api, store, slots, ta, unmount };
    return s;
  }
  const mirror = () => s.slots.main.querySelector<HTMLElement>(".zen-lint-mirror")!;
  const problems = () => s.slots.main.querySelector<HTMLElement>(".zen-lint-console")!;
  const summary = () => problems().querySelector(".zen-lint-console-summary")!.textContent;
  const rows = () => Array.from(problems().querySelectorAll<HTMLElement>(".zen-lint-console-row"));
  afterEach(() => vi.useRealTimers());

  it("lints right after load and draws marks behind the textarea", async () => {
    await lintSetup();
    expect(s.api.callsTo("lint")).toEqual([["a.md", "teh cat"]]);
    expect(mirror().querySelector("mark.zen-lint-error")!.textContent).toBe("teh");
    expect(problems().hidden).toBe(false);
    expect(summary()).toBe("1 error");
    // The problems console docks under the source.
    expect(problems().previousElementSibling?.classList.contains("zen-editor-panes")).toBe(true);
    // The textarea keeps its place in the panes, wrapped with the mirror.
    expect(s.ta.parentElement!.classList.contains("zen-lint-wrap")).toBe(true);
    expect(s.ta.parentElement!.parentElement!.classList.contains("zen-editor-panes")).toBe(true);
  });

  it("lints the unsaved text 600 ms after typing stops", async () => {
    await lintSetup();
    vi.useFakeTimers();
    type(s.ta, "teh cats");
    await vi.advanceTimersByTimeAsync(300);
    type(s.ta, "teh catss");
    await vi.advanceTimersByTimeAsync(599);
    expect(s.api.callsTo("lint")).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(s.api.callsTo("lint")).toEqual([["a.md", "teh cat"], ["a.md", "teh catss"]]);
  });

  it("the latest response wins", async () => {
    await lintSetup();
    const resolvers: ((r: { available: boolean; alerts: typeof TEH[] }) => void)[] = [];
    s.api.lint = () => new Promise((r) => resolvers.push(r));
    vi.useFakeTimers();
    type(s.ta, "teh cat dog");
    await vi.advanceTimersByTimeAsync(600);
    type(s.ta, "teh cat dog emu");
    await vi.advanceTimersByTimeAsync(600);
    expect(resolvers).toHaveLength(2);
    resolvers[1]({ available: true, alerts: [TEH, { ...TEH, col: 13, endCol: 15, match: "emu" }] });
    await vi.advanceTimersByTimeAsync(0);
    resolvers[0]({ available: true, alerts: [] });
    await vi.advanceTimersByTimeAsync(0);
    expect(summary()).toBe("2 errors");
    expect(mirror().querySelectorAll("mark")).toHaveLength(2);
  });

  it("lints again right after save", async () => {
    await lintSetup();
    type(s.ta, "teh dog");
    key(document, "s", { metaKey: true });
    for (let i = 0; i < 5; i++) await tick();
    expect(s.api.callsTo("lint").at(-1)).toEqual(["a.md", "teh dog"]);
  });

  it("clicking a problem selects its range and explains it", async () => {
    await lintSetup([{ ...TEH, col: 5, endCol: 7, match: "cat", description: "Checks spelling." }]);
    rows()[0].click();
    expect(document.activeElement).toBe(s.ta);
    expect([s.ta.selectionStart, s.ta.selectionEnd]).toEqual([4, 7]);
    expect(problems().querySelector(".zen-lint-console-detail")!.textContent).toContain("Checks spelling.");
  });

  it("'Learn more' opens the rule's link through openExternal", async () => {
    await lintSetup([{ ...TEH, link: "https://vale.sh/docs/spelling" }]);
    rows()[0].click();
    problems().querySelector<HTMLButtonElement>(".zen-lint-console-link")!.click();
    await tick();
    expect(s.api.callsTo("openExternal")).toEqual([["https://vale.sh/docs/spelling"]]);
  });

  it("shows nothing when Vale is unavailable", async () => {
    const api = createFakeApi({ sources: { "a.md": "teh cat" } });
    const store = createStore({ path: "a.md", mode: "edit" });
    const slots: Slots = {
      nav: document.createElement("nav"), breadcrumb: document.createElement("div"),
      actions: document.createElement("div"), main: document.createElement("main"),
    };
    document.body.append(slots.actions, slots.main);
    const unmount = editorFeature.mount(slots, { api, store, brand: "Zenmark", navigate: () => {} });
    for (let i = 0; i < 5; i++) await tick();
    s = { api, store, slots, ta: slots.main.querySelector("textarea")!, unmount };
    expect(s.slots.main.querySelector<HTMLElement>(".zen-lint-console")!.hidden).toBe(true);
    expect(mirror().querySelector("mark")).toBeNull();
  });
});

describe("editor: add-ons", () => {
  it("no lint without the vale add-on: no calls, no marks, no Problems console, no Linting tab", async () => {
    const api = createFakeApi({ sources: { "a.md": "teh cat" }, lint: { available: true, alerts: [{ line: 1, col: 1, endCol: 3, severity: "error", check: "Vale.Spelling", message: "m", match: "teh", description: "", link: "" }] }, addons: [] });
    const store = createStore({ path: "a.md", mode: "edit" });
    const slots: Slots = {
      nav: document.createElement("nav"), breadcrumb: document.createElement("div"),
      actions: document.createElement("div"), main: document.createElement("main"),
    };
    document.body.append(slots.actions, slots.main);
    const unmount = editorFeature.mount(slots, { api, store, brand: "Zenmark", navigate: () => {} });
    for (let i = 0; i < 5; i++) await tick();
    const ta = slots.main.querySelector("textarea")!;
    s = { api, store, slots, ta, unmount };
    expect(api.callsTo("lint")).toEqual([]);
    expect(slots.main.querySelector(".zen-lint-console")).toBeNull();
    expect(slots.main.querySelector(".zen-lint-mirror mark")).toBeNull();
    type(ta, "teh cats");
    await wait(700);
    key(ta, "s", { metaKey: true });
    await wait(700);
    expect(api.callsTo("lint")).toEqual([]);
    // the syntax highlighting layer stays
    expect(slots.main.querySelector(".zen-lint-mirror")).not.toBeNull();
    // the Inspector has Page Info and Problems (no Spelling filter); the Console is hidden while its switch is off
    const inspectorTabs = Array.from(document.querySelectorAll<HTMLElement>(".zen-inspector [role=tab]")).filter((t) => !t.hidden).map((t) => t.dataset.tab);
    expect(inspectorTabs).toEqual(["info", "problems"]);
    expect(Array.from(document.querySelectorAll<HTMLSelectElement>(".zen-inspector .zen-problems-filter option")).map((o) => o.value)).toEqual(["all", "checks"]);
  });
});

describe("editor: settings files", () => {
  it("edits YAML: no toolbar, no Vale, no draft, YAML colours; saves through the API", async () => {
    const api = createFakeApi({
      sources: { "zen:config/global": "version: 1\nplugins:\n  lint: false\n" },
      lint: { available: true, alerts: [{ line: 1, col: 1, endCol: 7, severity: "error", check: "X", message: "m", match: "version", description: "", link: "" }] },
    });
    const store = createStore({ path: "zen:config/global", mode: "edit" });
    const slots: Slots = {
      nav: document.createElement("nav"), breadcrumb: document.createElement("div"),
      actions: document.createElement("div"), main: document.createElement("main"),
    };
    document.body.append(slots.actions, slots.main);
    const unmount = editorFeature.mount(slots, { api, store, brand: "Zenmark", navigate: () => {} });
    for (let i = 0; i < 5; i++) await tick();
    const ta = slots.main.querySelector("textarea")!;
    s = { api, store, slots, ta, unmount };

    expect(ta.value).toBe("version: 1\nplugins:\n  lint: false\n");
    expect(slots.main.querySelector<HTMLElement>("[role=toolbar]")!.hidden).toBe(true);
    expect(api.callsTo("lint")).toEqual([]);
    const mirror = slots.main.querySelector(".zen-lint-mirror")!;
    expect(Array.from(mirror.querySelectorAll(".tok-f")).map((n) => n.textContent)).toEqual(["version", "plugins", "lint"]);
    expect(mirror.querySelector(".md-heading, mark")).toBeNull();

    type(ta, "version: 1\n");
    await wait(200);
    expect(api.broadcasts.filter((b) => b.event === "editor:draft")).toEqual([]);
    key(ta, "s", { metaKey: true });
    await tick();
    expect(api.callsTo("save")).toEqual([["zen:config/global", "version: 1\n"]]);
  });
});
