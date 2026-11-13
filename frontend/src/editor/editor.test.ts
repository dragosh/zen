// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
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
  const ctx: Ctx = { api, store, navigate: (p) => store.set({ path: p, anchor: "" }) };
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

describe("editor: split", () => {
  it("is on by default and ⌘⇧P toggles and remembers it", async () => {
    s = await setup();
    const split = buttonByText(s.slots.actions, "Split");
    const root = s.slots.main.querySelector<HTMLElement>(".zen-editor")!;
    expect(split.getAttribute("aria-pressed")).toBe("true");
    expect(root.classList.contains("split")).toBe(true);
    key(document, "p", { metaKey: true, shiftKey: true });
    expect(split.getAttribute("aria-pressed")).toBe("false");
    expect(root.classList.contains("split")).toBe(false);
    s.unmount();
    document.body.replaceChildren();
    s = await setup();
    expect(s.slots.main.querySelector(".zen-editor")!.classList.contains("split")).toBe(false);
    buttonByText(s.slots.actions, "Split").click();
    expect(s.slots.main.querySelector(".zen-editor")!.classList.contains("split")).toBe(true);
  });

  it("remembers the divider position clamped to 20–80%", async () => {
    s = await setup();
    const divider = s.slots.main.querySelector<HTMLElement>("[role=separator]")!;
    expect(divider.getAttribute("aria-valuenow")).toBe("50");
    for (let i = 0; i < 20; i++) key(divider, "ArrowRight");
    expect(divider.getAttribute("aria-valuenow")).toBe("80");
    s.unmount();
    document.body.replaceChildren();
    s = await setup();
    expect(s.slots.main.querySelector("[role=separator]")!.getAttribute("aria-valuenow")).toBe("80");
  });

  it("previews the text", async () => {
    s = await setup();
    type(s.ta, "# Fresh");
    await wait(200);
    expect(s.api.callsTo("preview").at(-1)).toEqual(["README.md", "# Fresh"]);
    expect(s.slots.main.querySelector(".zen-page h1")!.textContent).toBe("Fresh");
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

describe("editor: leaving with unsaved changes", () => {
  it("Done when clean returns to read mode", async () => {
    s = await setup();
    buttonByText(s.slots.actions, "Done").click();
    await tick();
    expect(s.store.get().mode).toBe("read");
    expect(dialog()).toBeNull();
  });

  it("Esc with nothing focused acts as Done", async () => {
    s = await setup();
    (document.activeElement as HTMLElement | null)?.blur();
    key(document, "Escape");
    await tick();
    expect(s.store.get().mode).toBe("read");
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
    expect(s.store.get().mode).toBe("edit");
    expect(s.store.get().dirty).toBe(true);
  });

  it("Done → Save saves then leaves", async () => {
    s = await setup();
    type(s.ta, "changed");
    buttonByText(s.slots.actions, "Done").click();
    answer("Save");
    await tick();
    await tick();
    expect(s.api.callsTo("save")).toEqual([["README.md", "changed"]]);
    expect(s.store.get().mode).toBe("read");
  });

  it("Done → Discard leaves without saving", async () => {
    s = await setup();
    type(s.ta, "changed");
    buttonByText(s.slots.actions, "Done").click();
    answer("Discard");
    await tick();
    expect(s.api.callsTo("save")).toHaveLength(0);
    expect(s.store.get().mode).toBe("read");
    expect(s.store.get().dirty).toBe(false);
  });

  it("navigating while dirty asks; Cancel restores the path", async () => {
    s = await setup();
    type(s.ta, "changed");
    s.store.set({ path: "guides/usage.md" });
    expect(dialog()).not.toBeNull();
    answer("Cancel");
    await tick();
    expect(s.store.get().path).toBe("README.md");
    expect(s.ta.value).toBe("changed");
  });

  it("navigating while dirty → Discard loads the new page", async () => {
    s = await setup();
    type(s.ta, "changed");
    s.store.set({ path: "guides/usage.md" });
    answer("Discard");
    await tick();
    await tick();
    expect(s.ta.value).toMatch(/^# Usage/);
    expect(s.store.get().dirty).toBe(false);
  });

  it("navigating while clean loads the new page", async () => {
    s = await setup();
    s.store.set({ path: "guides/usage.md" });
    await tick();
    expect(dialog()).toBeNull();
    expect(s.ta.value).toMatch(/^# Usage/);
  });

  it("closing the window while dirty is prevented", async () => {
    s = await setup();
    const clean = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(clean);
    expect(clean.defaultPrevented).toBe(false);
    type(s.ta, "changed");
    const ev = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
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
