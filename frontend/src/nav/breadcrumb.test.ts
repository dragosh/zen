// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { renderBreadcrumb } from "./breadcrumb";

let el: HTMLElement;
beforeEach(() => { document.body.replaceChildren(); el = document.createElement("div"); document.body.append(el); });
const crumbs = [
  { title: "Docs", path: "README.md" }, { title: "Guides", path: "guides/index.md" },
  { title: "Advanced", path: "guides/adv/index.md" }, { title: "<b>Tuning</b>", path: "guides/adv/tuning.md" },
];
const links = () => Array.from(el.querySelectorAll<HTMLElement>(".zen-crumb-link"));

test("every crumb but the last is a link; the last is current", () => {
  const onOpen = vi.fn();
  renderBreadcrumb(el, crumbs, { onOpen, dirty: false });
  expect(links().map((l) => l.textContent)).toEqual(["Docs", "Guides", "Advanced"]);
  links()[1].click();
  expect(onOpen).toHaveBeenCalledWith("guides/index.md");
  const last = el.querySelector('[aria-current="page"]')!;
  expect(last.tagName).not.toBe("BUTTON");
  expect(last.textContent).toBe("<b>Tuning</b>");
  expect(el.querySelector("b")).toBeNull();
  expect(el.querySelector(".zen-crumb-more")).toBeNull();
});

test("collapses middle crumbs into a menu when too wide", () => {
  Object.defineProperty(el, "clientWidth", { configurable: true, get: () => 100 });
  Object.defineProperty(el, "scrollWidth", { configurable: true, get: () => 400 });
  const onOpen = vi.fn();
  renderBreadcrumb(el, crumbs, { onOpen, dirty: false });
  expect(links().map((l) => l.textContent)).toEqual(["Docs"]);
  const more = el.querySelector<HTMLButtonElement>(".zen-crumb-more")!;
  expect(more.textContent).toBe("…");
  expect(more.getAttribute("aria-expanded")).toBe("false");
  more.click();
  expect(more.getAttribute("aria-expanded")).toBe("true");
  const menu = el.querySelectorAll<HTMLElement>("[role=menu] [role=menuitem]");
  expect(Array.from(menu).map((m) => m.textContent)).toEqual(["Guides", "Advanced"]);
  menu[1].click();
  expect(onOpen).toHaveBeenCalledWith("guides/adv/index.md");
});

test("dirty marker follows the breadcrumb", () => {
  renderBreadcrumb(el, crumbs, { onOpen: vi.fn(), dirty: true });
  const dot = el.querySelector<HTMLElement>('[title="Unsaved changes"]')!;
  expect(dot.textContent).toBe("●");
  expect(el.lastElementChild).toBe(dot);
  renderBreadcrumb(el, crumbs, { onOpen: vi.fn(), dirty: false });
  expect(el.querySelector('[title="Unsaved changes"]')).toBeNull();
});
