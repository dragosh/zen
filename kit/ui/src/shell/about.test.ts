// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test } from "vitest";
import type { BuildInfo } from "../host";
import { openAbout } from "./about";

type Handle = ReturnType<typeof openAbout>;
let open: Handle | null = null;
beforeEach(() => document.body.replaceChildren());
afterEach(() => { open?.close(); open = null; });

const BUILD: BuildInfo = {
  app: "demo", features: ["reader", "editor", "developer"], addons: [],
  plugins: ["codeblock", "quiz"], layouts: ["article", "docs"], templates: ["learning"], dialects: ["myst"], theme: "default",
};
const o = (host: Parameters<typeof openAbout>[1]["host"] = { addons: [], build: BUILD }) => ({ host, name: "Zenmark", version: "0.7.0", logoUrl: "/logo.svg" });
const sheet = () => document.querySelector<HTMLElement>("[role=dialog]")!;
const key = (k: string) => document.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));

test("About sheet with This build, Esc, works bare", () => {
  open = openAbout(document, o());
  const s = sheet();
  expect(s.getAttribute("aria-modal")).toBe("true");
  expect(s.getAttribute("aria-labelledby")).toBeTruthy();
  expect(s.querySelector("img")!.getAttribute("src")).toBe("/logo.svg");
  expect(s.querySelector("h2")!.textContent).toBe("Zenmark");
  expect(s.textContent).toContain("0.7.0");
  expect(s.querySelector(".zen-about-copyright")!.textContent).toMatch(/©\s*\d{4}/);
  // This build: today's section content
  const group = (id: string) => s.querySelector<HTMLElement>(`[data-group="${id}"]`)!;
  expect(Array.from(s.querySelectorAll(".zen-build-group h3")).map((h) => h.textContent)).toEqual(["Features", "Add-ons", "Plugins", "Layouts", "Templates", "Dialects", "Theme"]);
  expect(Array.from(group("features").querySelectorAll<HTMLElement>("li")).filter((li) => li.classList.contains("zen-build-feature-missing")).map((li) => li.dataset.feature))
    .toEqual(["nav", "find", "inspector", "print", "export", "newsite"]);
  expect(group("plugins").querySelector(".zen-build-list")!.textContent).toBe("codeblock, quiz");
  expect(group("addons").textContent).toContain("This build has no add-ons.");
  // Close and Esc close it
  const close = Array.from(s.querySelectorAll("button")).find((b) => b.textContent === "Close")!;
  expect(document.activeElement).toBe(close);
  close.click();
  expect(document.querySelector("[role=dialog]")).toBeNull();
  open = openAbout(document, o());
  key("Escape");
  expect(document.querySelector("[role=dialog]")).toBeNull();
  // a bare shell: no build, no add-ons
  open = openAbout(document, o({ addons: undefined, build: undefined }));
  expect(sheet().textContent).toContain("Zenmark");
  expect(Array.from(sheet().querySelectorAll(".zen-build-group h3")).map((h) => h.textContent)).toEqual(["Add-ons"]);
});

test("About: add-on names are text, never markup", () => {
  open = openAbout(document, o({ addons: [{ id: "v", name: "<b>Voice</b>", summary: "<img src=x onerror=alert(1)>" }], build: BUILD }));
  expect(sheet().querySelector(".zen-addon-name")!.textContent).toBe("<b>Voice</b>");
  expect(sheet().querySelector("b, img[onerror]")).toBeNull();
});

test("About opened twice brings the same sheet to the front", () => {
  open = openAbout(document, o());
  const first = sheet();
  (document.activeElement as HTMLElement).blur();
  const again = openAbout(document, o());
  expect(again).toBe(open);
  expect(document.querySelectorAll("[role=dialog]")).toHaveLength(1);
  expect(sheet()).toBe(first);
  expect(first.contains(document.activeElement)).toBe(true);
  // closing returns the focus to where it was, and a new open makes a new sheet
  open.close();
  expect(document.querySelector("[role=dialog]")).toBeNull();
  open = openAbout(document, o());
  expect(document.querySelectorAll("[role=dialog]")).toHaveLength(1);
});

test("About: Tab stays inside the sheet; a click on the backdrop closes it", () => {
  const before = document.body.appendChild(document.createElement("button"));
  before.focus();
  open = openAbout(document, o());
  const buttons = Array.from(sheet().querySelectorAll<HTMLElement>("button"));
  buttons[buttons.length - 1].focus();
  const e = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(true);
  expect(sheet().contains(document.activeElement)).toBe(true);
  sheet().parentElement!.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  expect(document.querySelector("[role=dialog]")).toBeNull();
  expect(document.activeElement).toBe(before);
});
