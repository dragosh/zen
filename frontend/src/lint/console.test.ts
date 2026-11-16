// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { renderConsole } from "./console";
import { alert } from "./test-util";

beforeEach(() => {
  localStorage.clear();
  document.body.replaceChildren();
});

function setup() {
  const onSelect = vi.fn();
  const onLink = vi.fn();
  const c = renderConsole({ onSelect, onLink });
  document.body.append(c.el);
  return { c, onSelect, onLink };
}
const rows = (c: { el: HTMLElement }) => Array.from(c.el.querySelectorAll<HTMLElement>(".zen-lint-console-row"));

test("hidden when unavailable, 'No problems' when clean", () => {
  const { c } = setup();
  c.update(null);
  expect(c.el.hidden).toBe(true);
  c.update([]);
  expect(c.el.hidden).toBe(false);
  expect(c.el.querySelector(".zen-lint-console-summary")?.textContent).toBe("No problems");
});

test("lists position, message and rule with a severity summary", () => {
  const { c } = setup();
  c.update([
    alert({ match: "teh", line: 2, col: 5, message: "Did you really mean 'teh'?" }),
    alert({ match: "very", severity: "warning", check: "Vale.Avoid", line: 3, col: 1 }),
  ]);
  expect(c.el.querySelector(".zen-lint-console-summary")?.textContent).toBe("1 error, 1 warning");
  const r = rows(c);
  expect(r).toHaveLength(2);
  expect(r[0].querySelector(".zen-lint-console-pos")?.textContent).toBe("2:5");
  expect(r[0].querySelector(".zen-lint-console-msg")?.textContent).toBe("Did you really mean 'teh'?");
  expect(r[1].querySelector(".zen-lint-console-rule")?.textContent).toBe("Vale.Avoid");
  expect(r[1].classList.contains("zen-lint-console-row-warning")).toBe(true);
});

test("clicking a row jumps and explains why, with a https link", () => {
  const { c, onSelect, onLink } = setup();
  const a = alert({ match: "teh", description: "Checks spelling against a dictionary.", link: "https://vale.sh/docs/spelling" });
  c.update([a]);
  rows(c)[0].click();
  expect(onSelect).toHaveBeenCalledWith(a, 0);
  expect(rows(c)[0].getAttribute("aria-selected")).toBe("true");
  const detail = c.el.querySelector(".zen-lint-console-detail")!;
  expect(detail.textContent).toContain("Checks spelling against a dictionary.");
  expect(detail.textContent).toContain("Line 1, column 1");
  detail.querySelector<HTMLButtonElement>(".zen-lint-console-link")!.click();
  expect(onLink).toHaveBeenCalledWith("https://vale.sh/docs/spelling");
});

test("no link button for non-https links", () => {
  const { c } = setup();
  c.update([alert({ match: "x", link: "javascript:alert(1)" })]);
  rows(c)[0].click();
  expect(c.el.querySelector(".zen-lint-console-link")).toBeNull();
});

test("Vale text stays text", () => {
  const { c } = setup();
  c.update([alert({ match: "<b>", message: "<script>alert(1)</script>", description: "<img src=x onerror=1>" })]);
  rows(c)[0].click();
  expect(c.el.querySelector("script, img, b")).toBeNull();
  expect(c.el.textContent).toContain("<script>alert(1)</script>");
});

test("collapses, remembers it, and keeps the selection across updates", () => {
  const { c } = setup();
  const a = alert({ match: "teh", line: 1 });
  c.update([a, alert({ match: "cta", line: 2 })]);
  rows(c)[1].click();
  c.update([alert({ match: "new", line: 1 }), alert({ match: "cta", line: 2 })]);
  expect(rows(c)[1].getAttribute("aria-selected")).toBe("true");
  c.el.querySelector<HTMLButtonElement>(".zen-lint-console-toggle")!.click();
  expect(c.el.querySelector<HTMLElement>(".zen-lint-console-body")!.hidden).toBe(true);
  expect(localStorage.getItem("zen.lint.console.open")).toBe("0");
});

test("arrow keys move the selection, Enter jumps", () => {
  const { c, onSelect } = setup();
  c.update([alert({ match: "a" }), alert({ match: "b", line: 2 })]);
  const list = c.el.querySelector<HTMLElement>(".zen-lint-console-list")!;
  list.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
  list.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
  expect(rows(c)[1].getAttribute("aria-selected")).toBe("true");
  list.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  expect(onSelect).toHaveBeenCalledTimes(1);
  expect(onSelect.mock.calls[0][1]).toBe(1);
});

const tabs = (c: { el: HTMLElement }) => Array.from(c.el.querySelectorAll<HTMLButtonElement>(".zen-lint-console-tab"));
const shownRows = (c: { el: HTMLElement }) => rows(c).filter((r) => !r.hidden);
const MIXED = () => [
  alert({ match: "teh", line: 1 }),
  alert({ match: "very", severity: "warning", check: "Vale.Avoid", line: 2 }),
  alert({ match: "utilize", severity: "suggestion", check: "Vale.Use", line: 3 }),
  alert({ match: "recieve", line: 4 }),
];

test("severity tabs show counts and filter the list", () => {
  const { c } = setup();
  c.update(MIXED());
  const t = tabs(c);
  expect(t.map((b) => b.textContent)).toEqual(["All4", "Errors2", "Warnings1", "Suggestions1"]);
  expect(t[0].getAttribute("aria-selected")).toBe("true");
  expect(shownRows(c)).toHaveLength(4);
  t[1].click();
  expect(t[1].getAttribute("aria-selected")).toBe("true");
  expect(shownRows(c).map((r) => r.querySelector(".zen-lint-console-pos")!.textContent)).toEqual(["1:1", "4:1"]);
  expect(c.el.querySelector(".zen-lint-console-body")!.getAttribute("aria-labelledby")).toBe(t[1].id);
  // remembered
  expect(localStorage.getItem("zen.lint.console.tab")).toBe("error");
});

test("arrow keys move between tabs; the list skips hidden rows", () => {
  const { c } = setup();
  c.update(MIXED());
  const t = tabs(c);
  t[0].dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(t[1].getAttribute("aria-selected")).toBe("true");
  expect(document.activeElement).toBe(t[1]);
  const list = c.el.querySelector<HTMLElement>(".zen-lint-console-list")!;
  list.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
  list.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
  expect(rows(c)[3].getAttribute("aria-selected")).toBe("true");
});

test("a tab with nothing in it says so", () => {
  const { c } = setup();
  c.update([alert({ match: "teh" })]);
  tabs(c)[2].click();
  expect(shownRows(c)).toHaveLength(0);
  expect(c.el.querySelector(".zen-lint-console-hint")!.textContent).toBe("No warnings.");
});

test("the top edge resizes the panel by drag and by keys, and remembers it", () => {
  const { c } = setup();
  c.update(MIXED());
  const grip = c.el.querySelector<HTMLElement>(".zen-lint-console-resize")!;
  const h = () => c.el.style.getPropertyValue("--zen-lint-console-h");
  expect(h()).toBe("180px");
  grip.dispatchEvent(new PointerEvent("pointerdown", { button: 0, clientY: 500, bubbles: true }));
  window.dispatchEvent(new PointerEvent("pointermove", { clientY: 400 }));
  window.dispatchEvent(new PointerEvent("pointerup", {}));
  expect(h()).toBe("280px");
  expect(localStorage.getItem("zen.lint.console.height")).toBe("280");
  grip.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
  expect(h()).toBe("256px");
  grip.dispatchEvent(new KeyboardEvent("keydown", { key: "End", bubbles: true }));
  expect(grip.getAttribute("aria-valuenow")).toBe("640");
  grip.dispatchEvent(new KeyboardEvent("keydown", { key: "Home", bubbles: true }));
  expect(h()).toBe("72px");
  grip.dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
  expect(h()).toBe("180px");
});

test("a stored height is restored and clamped", () => {
  localStorage.setItem("zen.lint.console.height", "9999");
  const { c } = setup();
  expect(c.el.style.getPropertyValue("--zen-lint-console-h")).toBe("640px");
});
