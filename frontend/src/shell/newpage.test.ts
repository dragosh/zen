// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import { openNewPage } from "./newpage";

let api: FakeApi;
beforeEach(() => {
  document.body.replaceChildren();
  api = createFakeApi();
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

test("asks for a layout and a name in the current folder", () => {
  const { q, cards } = open();
  expect(q("[role=dialog]").getAttribute("aria-modal")).toBe("true");
  expect(cards().map((c) => c.querySelector(".zen-newpage-layout-name")!.textContent)).toEqual(["Page", "Slides", "Course"]);
  expect(cards()[0].getAttribute("aria-checked")).toBe("true");
  const name = q<HTMLInputElement>(".zen-newpage-name");
  expect(name.value).toBe("guides/untitled");
  expect(document.activeElement).toBe(name);
  expect(name.selectionStart).toBe("guides/".length);
});

test("arrow keys and clicks pick the layout", () => {
  const { q, cards } = open();
  q(".zen-newpage-layouts").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(cards()[1].getAttribute("aria-checked")).toBe("true");
  expect(document.activeElement).toBe(cards()[1]);
  cards()[2].click();
  expect(cards()[2].getAttribute("aria-checked")).toBe("true");
  expect(cards()[1].getAttribute("aria-checked")).toBe("false");
});

test("Create makes the page with the chosen layout and reports its path", async () => {
  const { q, cards, onCreated, onClose, d } = open();
  cards()[1].click();
  q<HTMLInputElement>(".zen-newpage-name").value = "guides/deck";
  q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(api.callsTo("createPage")).toEqual([["guides/deck", "slides"]]);
  expect(onCreated).toHaveBeenCalledWith("guides/deck.md");
  expect(onClose).toHaveBeenCalled();
  expect(d.el.isConnected).toBe(false);
  expect(await api.source("guides/deck.md")).toContain("layout: slides");
});

test("an error keeps the dialog open and says why", async () => {
  const { q, onCreated } = open("");
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
