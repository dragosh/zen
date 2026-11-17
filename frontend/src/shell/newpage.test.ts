// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi, type LayoutSeed } from "../api/fake";
import { openNewPage } from "./newpage";

const seed = (id: string, over: Partial<LayoutSeed> = {}): LayoutSeed => ({ id, version: "1.0.0", description: `${id} layout`, source: "builtin", status: "on", ...over });
const LAYOUTS: LayoutSeed[] = [
  seed("memo", { source: "folder", description: "A memo", help: { summary: "", snippets: [{ title: "A memo", markdown: "---\nlayout: memo\nto: Ann\n---\n# Memo\n\nDear Ann," }] } }),
  seed("slides", { description: "A presentation", help: { summary: "", snippets: [{ title: "A deck", markdown: "---\nlayout: slides\n---\n# Deck\n\n---\n\n# Two" }, { title: "Other", markdown: "x" }] } }),
  seed("article", { description: "The reading view" }),
  seed("mine", { source: "user", description: "Mine" }),
  seed("course", { description: "Steps" }),
  seed("blocked-one", { source: "folder", status: "blocked" }),
  seed("broken-one", { source: "folder", status: "invalid" }),
];

let api: FakeApi;
beforeEach(() => {
  document.body.replaceChildren();
  api = createFakeApi({ layouts: LAYOUTS });
});

const settle = async () => { for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0)); };

function open(folder = "guides/") {
  const onCreated = vi.fn();
  const onClose = vi.fn();
  const d = openNewPage({ api, doc: document, folder, onCreated, onClose });
  const q = <T extends Element = HTMLElement>(s: string) => d.el.querySelector<T>(s)!;
  const cards = () => Array.from(d.el.querySelectorAll<HTMLButtonElement>(".zen-newpage-layout"));
  return { d, q, cards, onCreated, onClose };
}

test("asks for a layout and a name in the current folder; layouts come from api.layouts()", async () => {
  const { q, cards } = open();
  expect(q("[role=dialog]").getAttribute("aria-modal")).toBe("true");
  expect(cards().map((c) => c.querySelector(".zen-newpage-layout-name")!.textContent)).toEqual(["Page"]); // the rest arrive from api.layouts()
  await settle();
  expect(cards().map((c) => c.querySelector(".zen-newpage-layout-name")!.textContent)).toEqual(["Page", "course", "slides", "memo", "mine"]);
  expect(cards()[0].getAttribute("aria-checked")).toBe("true");
  const name = q<HTMLInputElement>(".zen-newpage-name");
  expect(name.value).toBe("guides/untitled");
  expect(document.activeElement).toBe(name);
  expect(name.selectionStart).toBe("guides/".length);
});

test("built-in layouts first, then folder, then user; blocked and invalid are hidden; each card shows its description", async () => {
  const { cards } = open();
  await settle();
  expect(api.callsTo("layouts")).toHaveLength(1);
  expect(cards().map((c) => c.dataset.layout)).toEqual(["article", "course", "slides", "memo", "mine"]);
  const hint = (id: string) => cards().find((c) => c.dataset.layout === id)!.querySelector(".zen-newpage-layout-hint")!.textContent;
  expect(hint("article")).toBe("Plain markdown");
  expect(hint("slides")).toBe("A presentation");
  expect(hint("memo")).toBe("A memo");
  expect(cards()[0].getAttribute("aria-checked")).toBe("true");
});

test("the dialog works when layouts() fails: only the plain page is offered", async () => {
  api.failNext("layouts", new Error("down"));
  const { cards, q } = open();
  await settle();
  expect(cards().map((c) => c.dataset.layout)).toEqual(["article"]);
  q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(api.callsTo("createPage")).toEqual([["guides/untitled", ""]]);
});

test("a choice made before the list arrives is kept", async () => {
  const { cards } = open();
  cards()[0].click();
  await settle();
  expect(cards()[0].getAttribute("aria-checked")).toBe("true");
});

test("arrow keys and clicks pick the layout", async () => {
  const { q, cards } = open();
  await settle();
  q(".zen-newpage-layouts").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(cards()[1].getAttribute("aria-checked")).toBe("true");
  expect(document.activeElement).toBe(cards()[1]);
  cards()[2].click();
  expect(cards()[2].getAttribute("aria-checked")).toBe("true");
  expect(cards()[1].getAttribute("aria-checked")).toBe("false");
});

test("Create makes the page with the chosen layout and reports its path", async () => {
  const { q, cards, onCreated, onClose, d } = open();
  await settle();
  cards().find((c) => c.dataset.layout === "slides")!.click();
  q<HTMLInputElement>(".zen-newpage-name").value = "guides/deck";
  q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(api.callsTo("createPage")).toEqual([["guides/deck", ""]]);
  expect(onCreated).toHaveBeenCalledWith("guides/deck.md");
  expect(onClose).toHaveBeenCalled();
  expect(d.el.isConnected).toBe(false);
  // pre-filled with the layout's first snippet
  expect(await api.source("guides/deck.md")).toBe("---\nlayout: slides\n---\n# Deck\n\n---\n\n# Two");
});

test("a folder layout pre-fills the page with its first snippet", async () => {
  const { q, cards } = open();
  await settle();
  cards().find((c) => c.dataset.layout === "memo")!.click();
  q<HTMLInputElement>(".zen-newpage-name").value = "guides/note";
  q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(await api.source("guides/note.md")).toBe("---\nlayout: memo\nto: Ann\n---\n# Memo\n\nDear Ann,");
  expect(api.callsTo("save")).toEqual([["guides/note.md", "---\nlayout: memo\nto: Ann\n---\n# Memo\n\nDear Ann,"]]);
});

test("a layout without a snippet falls back to layout: <id> and a title", async () => {
  const { q, cards } = open();
  await settle();
  cards().find((c) => c.dataset.layout === "mine")!.click();
  q<HTMLInputElement>(".zen-newpage-name").value = "x";
  q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(await api.source("x.md")).toBe("---\nlayout: mine\n---\n# Title");
});

test("the plain page is created without a second write", async () => {
  const { q } = open();
  await settle();
  q<HTMLInputElement>(".zen-newpage-name").value = "plain";
  q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(api.callsTo("createPage")).toEqual([["plain", ""]]);
  expect(api.callsTo("save")).toEqual([]);
});

test("if pre-filling fails the page still opens", async () => {
  vi.spyOn(console, "warn").mockImplementation(() => {});
  const { q, cards, onCreated } = open();
  await settle();
  cards().find((c) => c.dataset.layout === "slides")!.click();
  q<HTMLInputElement>(".zen-newpage-name").value = "d";
  api.failNext("save", new Error("disk"));
  q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(onCreated).toHaveBeenCalledWith("d.md");
});

test("an error keeps the dialog open and says why", async () => {
  const { q, onCreated } = open("");
  await settle();
  q<HTMLInputElement>(".zen-newpage-name").value = "README";
  q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(onCreated).not.toHaveBeenCalled();
  expect(q(".zen-newpage-error").textContent).toBe("a page with that name already exists");
  expect(q<HTMLButtonElement>("button[type=submit]").disabled).toBe(false);
});

test("Esc, Cancel and a click outside close it", () => {
  let o = open();
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(o.onClose).toHaveBeenCalledTimes(1);
  expect(o.d.el.isConnected).toBe(false);
  o = open();
  Array.from(o.d.el.querySelectorAll("button")).find((b) => b.textContent === "Cancel")!.click();
  expect(o.onClose).toHaveBeenCalledTimes(1);
  o = open();
  o.d.el.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  expect(o.onClose).toHaveBeenCalledTimes(1);
});
