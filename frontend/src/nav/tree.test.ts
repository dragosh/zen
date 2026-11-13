// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { renderTree } from "./tree";
import { deepTree, item, items, visible } from "./fixtures";

let el: HTMLElement;
beforeEach(() => { document.body.replaceChildren(); el = document.createElement("div"); document.body.append(el); });
const opts = (o: Partial<Parameters<typeof renderTree>[2]> = {}) => ({
  current: "", onOpen: vi.fn(), onToggle: vi.fn(), expanded: new Set<string>(["guides"]), ...o,
});
const row = (path: string) => item(el, path).querySelector<HTMLElement>(".zen-nav-row")!;

test("renders a tree with disclosure triangles on nodes with children", () => {
  renderTree(el, deepTree(), opts());
  expect(el.querySelector("[role=tree]")).not.toBeNull();
  expect(items(el).map((i) => i.dataset.path)).toEqual(["README.md", "guides", "guides/install.md", "guides/adv", "guides/adv/tuning.md", "api.md", "api/auth.md"]);
  expect(item(el, "guides").querySelector(".zen-nav-twisty")).not.toBeNull();
  expect(item(el, "README.md").querySelector(".zen-nav-twisty")).toBeNull();
  expect(item(el, "guides").getAttribute("aria-expanded")).toBe("true");
  expect(item(el, "guides/adv").getAttribute("aria-expanded")).toBe("false");
  expect(visible(el)).toEqual(["README.md", "guides", "guides/install.md", "guides/adv", "api.md"]);
});

test("clicking a page navigates, clicking a group or triangle toggles", () => {
  const o = opts();
  renderTree(el, deepTree(), o);
  row("guides/install.md").click();
  expect(o.onOpen).toHaveBeenCalledWith("guides/install.md");
  row("guides").click();
  expect(o.onToggle).toHaveBeenCalledWith("guides", false);
  expect(o.onOpen).toHaveBeenCalledTimes(1);
  item(el, "api.md").querySelector<HTMLElement>(".zen-nav-twisty")!.click();
  expect(o.onToggle).toHaveBeenCalledWith("api.md", true);
  expect(o.onOpen).toHaveBeenCalledTimes(1);
});

test("marks the current page and expands its ancestors", () => {
  const o = opts({ current: "guides/adv/tuning.md", expanded: new Set() });
  renderTree(el, deepTree(), o);
  expect(item(el, "guides/adv/tuning.md").getAttribute("aria-current")).toBe("page");
  expect(el.querySelectorAll("[aria-current]").length).toBe(1);
  expect(item(el, "guides").getAttribute("aria-expanded")).toBe("true");
  expect(item(el, "guides/adv").getAttribute("aria-expanded")).toBe("true");
  expect(o.expanded.has("guides/adv")).toBe(true);
});

test("long titles ellipsize with the full title in title", () => {
  renderTree(el, deepTree(), opts());
  expect(row("guides/install.md").title).toBe("Install");
});

test("escapes titles", () => {
  const evil = '<img src=x onerror="alert(1)">';
  renderTree(el, [{ title: evil, path: '"><b>x</b>.md', kind: "page", children: [] }], opts());
  expect(el.querySelector("img, b")).toBeNull();
  expect(row('"><b>x</b>.md').textContent).toContain(evil);
});

test("empty tree shows a message", () => {
  renderTree(el, [], opts());
  expect(el.textContent).toContain("No markdown files in this folder");
  expect(items(el).length).toBe(0);
});
