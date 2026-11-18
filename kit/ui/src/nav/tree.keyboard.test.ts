// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { renderTree } from "./tree";
import { deepTree, item, items } from "./fixtures";

let el: HTMLElement;
let o: Parameters<typeof renderTree>[2] & { onOpen: ReturnType<typeof vi.fn> };
beforeEach(() => {
  document.body.replaceChildren();
  el = document.createElement("div");
  document.body.append(el);
  o = { current: "README.md", onOpen: vi.fn(), expanded: new Set(["guides"]), onToggle: vi.fn() } as never;
  // keep expansion in the set and re-render, as index.ts does
  o.onToggle = vi.fn((p: string, open: boolean) => { if (open) o.expanded.add(p); else o.expanded.delete(p); renderTree(el, deepTree(), o); });
  renderTree(el, deepTree(), o);
});
const press = (key: string) =>
  (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
const focused = () => (document.activeElement as HTMLElement).closest<HTMLElement>("[role=treeitem]")?.dataset.path;

test("one tab stop, on the current page", () => {
  const stops = el.querySelectorAll('[tabindex="0"]');
  expect(stops.length).toBe(1);
  expect(stops[0].closest<HTMLElement>("[role=treeitem]")!.dataset.path).toBe("README.md");
});

test("up/down move through visible nodes, Home/End jump", () => {
  item(el, "README.md").querySelector<HTMLElement>("[tabindex]")!.focus();
  press("ArrowDown"); expect(focused()).toBe("guides");
  press("ArrowDown"); expect(focused()).toBe("guides/install.md");
  press("ArrowDown"); expect(focused()).toBe("guides/adv");
  press("ArrowDown"); expect(focused()).toBe("api.md");
  press("ArrowDown"); expect(focused()).toBe("api.md");
  press("ArrowUp"); expect(focused()).toBe("guides/adv");
  press("Home"); expect(focused()).toBe("README.md");
  press("End"); expect(focused()).toBe("api.md");
  expect(el.querySelectorAll('[tabindex="0"]').length).toBe(1);
});

test("right expands then moves to first child, left collapses then moves to parent", () => {
  item(el, "guides").querySelector<HTMLElement>("[tabindex]")!.focus();
  press("ArrowDown"); press("ArrowDown"); // guides/adv
  press("ArrowRight");
  expect(o.onToggle).toHaveBeenCalledWith("guides/adv", true);
  expect(focused()).toBe("guides/adv");
  press("ArrowRight"); expect(focused()).toBe("guides/adv/tuning.md");
  press("ArrowLeft"); expect(focused()).toBe("guides/adv");
  press("ArrowLeft");
  expect(o.onToggle).toHaveBeenCalledWith("guides/adv", false);
  expect(focused()).toBe("guides/adv");
  press("ArrowLeft"); expect(focused()).toBe("guides");
});

test("Enter and Space open pages and toggle groups", () => {
  item(el, "README.md").querySelector<HTMLElement>("[tabindex]")!.focus();
  press("End");
  press("Enter");
  expect(o.onOpen).toHaveBeenCalledWith("api.md");
  press("Home"); press("ArrowDown");
  press(" ");
  expect(o.onToggle).toHaveBeenCalledWith("guides", false);
  expect(items(el).length).toBe(7);
});
