// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createTabbedPanel, panelColumns, panelEmpty, panelProps, type TabbedPanelOptions } from "./index";

const TABS = [{ id: "a", label: "Alpha" }, { id: "b", label: "Beta" }, { id: "c", label: "Gamma" }];
const make = (extra: Partial<TabbedPanelOptions> = {}) => {
  const p = createTabbedPanel({ doc: document, label: "Thing", idPrefix: "t", tabs: TABS, ...extra });
  document.body.append(p.el);
  return p;
};
const tabs = (p: ReturnType<typeof make>) => Array.from(p.el.querySelectorAll<HTMLButtonElement>('[role="tab"]'));

beforeEach(() => { localStorage.clear(); document.body.replaceChildren(); });
afterEach(() => vi.restoreAllMocks());

test("tabs and panels are linked with roles, the first is selected and the others hidden", () => {
  const p = make();
  expect(p.el.querySelector('[role="tablist"]')?.getAttribute("aria-label")).toBe("Thing");
  expect(tabs(p).map((t) => t.textContent)).toEqual(["Alpha", "Beta", "Gamma"]);
  expect(p.selected).toBe("a");
  for (const t of tabs(p)) {
    const panel = document.getElementById(t.getAttribute("aria-controls")!)!;
    expect(panel.getAttribute("role")).toBe("tabpanel");
    expect(panel.getAttribute("aria-labelledby")).toBe(t.id);
    expect(panel.hidden).toBe(t.dataset.tab !== "a");
  }
  expect(tabs(p).map((t) => t.tabIndex)).toEqual([0, -1, -1]);
});

test("a click selects; ←/→ wrap, Home and End jump; the choice is remembered under tabKey", () => {
  const p = make({ tabKey: "k.tab" });
  tabs(p)[1].click();
  expect(p.selected).toBe("b");
  expect(localStorage.getItem("k.tab")).toBe("b");
  const key = (k: string) => p.el.querySelector('[role="tablist"]')!.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));
  key("ArrowRight"); expect(p.selected).toBe("c");
  key("ArrowRight"); expect(p.selected).toBe("a");
  key("ArrowLeft"); expect(p.selected).toBe("c");
  key("Home"); expect(p.selected).toBe("a");
  key("End"); expect(p.selected).toBe("c");
  expect(document.activeElement).toBe(tabs(p)[2]);
  p.destroy();
  expect(make({ tabKey: "k.tab" }).selected).toBe("c");
  localStorage.setItem("k.tab", "nope");
  expect(make({ tabKey: "k.tab" }).selected).toBe("a");
});

test("count chips: hidden at 0 and null, a tone, and the tab's accessible name has the number", () => {
  const p = make();
  const chip = tabs(p)[0].querySelector<HTMLElement>(".zen-panel-count")!;
  expect(chip.hidden).toBe(true);
  p.setCount("a", 3, "error");
  expect(chip.hidden).toBe(false);
  expect(chip.textContent).toBe("3");
  expect(chip.dataset.tone).toBe("error");
  expect(tabs(p)[0].getAttribute("aria-label")).toBe("Alpha, 3");
  p.setCount("a", 0);
  expect(chip.hidden).toBe(true);
  expect(tabs(p)[0].getAttribute("aria-label")).toBe("Alpha");
  p.setCount("zzz", 1); // unknown tab: ignored
});

test("the close button appears only with onClose and calls it", () => {
  expect(make().el.querySelector(".zen-panel-close")).toBeNull();
  const onClose = vi.fn();
  const b = make({ onClose }).el.querySelector<HTMLButtonElement>(".zen-panel-close")!;
  expect(b.getAttribute("aria-label")).toBe("Close Thing");
  b.click();
  expect(onClose).toHaveBeenCalledOnce();
});

test("resizable: starts at the remembered height (or the initial one), arrows resize, never below min", () => {
  const p = make({ resize: { key: "k.h", initial: 200, min: 120 } });
  expect(p.height).toBe(200);
  expect(p.el.style.height).toBe("200px");
  const sep = p.el.querySelector<HTMLElement>('[role="separator"]')!;
  sep.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp" }));
  expect(p.height).toBe(216);
  expect(localStorage.getItem("k.h")).toBe("216");
  sep.dispatchEvent(new KeyboardEvent("keydown", { key: "Home" }));
  expect(p.height).toBe(120);
  p.setHeight(10);
  expect(p.height).toBe(120);
  p.destroy();
  localStorage.setItem("k.h", "300");
  expect(make({ resize: { key: "k.h", initial: 200, min: 120 } }).height).toBe(300);
  expect(make().el.querySelector('[role="separator"]')).toBeNull();
});

test("reload picks up a height another panel stored under the same key", () => {
  const p = make({ resize: { key: "k.h", initial: 200, min: 120 } });
  localStorage.setItem("k.h", "260");
  p.reload();
  expect(p.height).toBe(260);
  expect(p.el.style.height).toBe("260px");
});

test("broken storage: the panel still works and starts on the first tab", () => {
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("no"); });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("no"); });
  const p = make({ tabKey: "k.tab", resize: { key: "k.h", initial: 200, min: 120 } });
  expect(p.selected).toBe("a");
  tabs(p)[1].click();
  expect(p.selected).toBe("b");
  expect(p.height).toBe(200);
});

test("panelProps: one label/value pair per item, data-field on the value, text never parsed as HTML", () => {
  const dl = panelProps([["Path", "path", "<b>x</b>.md"], ["Full path", "abs", "/a/b", true]]);
  expect(Array.from(dl.querySelectorAll("dt"), (d) => d.textContent)).toEqual(["Path", "Full path"]);
  const dd = dl.querySelector<HTMLElement>('dd[data-field="path"]')!;
  expect(dd.textContent).toBe("<b>x</b>.md");
  expect(dd.querySelector("b")).toBeNull();
  expect(dl.querySelector('dd[data-field="abs"]')!.classList.contains("zen-panel-mono")).toBe(true);
});

test("panelEmpty: a title and an optional hint", () => {
  expect(panelEmpty("No issues").querySelector(".zen-panel-empty-hint")).toBeNull();
  expect(panelEmpty("No issues", "All good.").textContent).toBe("No issuesAll good.");
});

test("each tab has its own tools in the header, shown only while it is selected", () => {
  const p = make();
  const a = p.toolsFor("a");
  const b = p.toolsFor("b");
  a.append(document.createElement("select"));
  b.append(document.createElement("button"));
  expect(p.tools.contains(a) && p.tools.contains(b)).toBe(true);
  expect([a.hidden, b.hidden]).toEqual([false, true]);
  p.select("b");
  expect([a.hidden, b.hidden]).toEqual([true, false]);
});

test("a tab that is not visible is out of the list, the arrow keys and the selection", () => {
  const p = make({ tabKey: "k" });
  p.setTabVisible("b", false);
  expect(p.visibleTabs).toEqual(["a", "c"]);
  expect(p.isTabVisible("b")).toBe(false);
  expect(tabs(p)[1].hidden).toBe(true);
  p.select("b");
  expect(p.selected).toBe("a"); // refused
  const list = p.el.querySelector<HTMLElement>('[role="tablist"]')!;
  list.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true, cancelable: true }));
  expect(p.selected).toBe("c");
  p.setTabVisible("c", false);
  expect(p.selected).toBe("a"); // the selection moves to a visible tab
  p.setTabVisible("b", true);
  expect(tabs(p)[1].hidden).toBe(false);
});

test("a remembered tab that is not visible falls back to the first visible one", () => {
  localStorage.setItem("k", "b");
  const p = createTabbedPanel({ doc: document, label: "Thing", idPrefix: "t", tabs: TABS, tabKey: "k" });
  p.setTabVisible("b", false);
  expect(p.selected).toBe("a");
});

test("collapse: the chevron hides the bodies and the resizer, keeps the header, and is remembered", () => {
  const p = make({ collapseKey: "c", resize: { key: "h", initial: 200, min: 100 } });
  const chevron = p.el.querySelector<HTMLButtonElement>(".zen-panel-collapse")!;
  expect(p.collapsed).toBe(false);
  expect(chevron.getAttribute("aria-expanded")).toBe("true");
  expect(p.el.style.height).toBe("200px");
  chevron.click();
  expect(p.collapsed).toBe(true);
  expect(p.el.classList.contains("is-collapsed")).toBe(true);
  expect(chevron.getAttribute("aria-expanded")).toBe("false");
  expect(chevron.getAttribute("aria-label")).toBe("Expand Thing");
  expect(Array.from(p.el.querySelectorAll<HTMLElement>('[role="tabpanel"]')).every((b) => b.hidden)).toBe(true);
  expect(p.el.querySelector<HTMLElement>(".zen-panel-resizer")!.hidden).toBe(true);
  expect(p.el.style.height).toBe("");
  expect(p.el.querySelector('[role="tablist"]')).not.toBeNull();
  expect(localStorage.getItem("c")).toBe("1");
  p.destroy();
  const q = make({ collapseKey: "c", resize: { key: "h", initial: 200, min: 100 } });
  expect(q.collapsed).toBe(true);
  q.el.querySelector<HTMLButtonElement>(".zen-panel-collapse")!.click();
  expect(q.collapsed).toBe(false);
  expect(q.el.style.height).toBe("200px");
  expect(localStorage.getItem("c")).toBe("0");
});

test("choosing a tab while collapsed expands the panel", () => {
  const p = make();
  p.setCollapsed(true);
  tabs(p)[1].click();
  expect(p.collapsed).toBe(false);
  expect(p.selected).toBe("b");
});

test("the collapse chevron sits before the close button, after the tools", () => {
  const p = make({ onClose: () => {} });
  const head = p.el.querySelector(".zen-panel-head")!;
  expect(Array.from(head.children).map((c) => c.className.split(" ")[0])).toEqual(["zen-panel-tabs", "zen-panel-tools", "zen-panel-collapse", "zen-panel-close"]);
});

test("panelColumns: a main column on the left and a details column on the right", () => {
  const c = panelColumns();
  expect(c.el.classList.contains("zen-panel-cols")).toBe(true);
  expect(Array.from(c.el.children)).toEqual([c.left, c.right]);
  expect(c.left.classList.contains("zen-panel-col-main")).toBe(true);
  expect(c.right.classList.contains("zen-panel-col-side")).toBe(true);
});

// ---- ui/inspector-polish: resizable columns

const RESIZE_KEY = "zen.panel.cols.info";
const sized = (id?: string) => {
  const c = panelColumns({ id, doc: document });
  document.body.append(c.el);
  c.el.getBoundingClientRect = () => ({ left: 100, width: 1000, right: 1100, top: 0, bottom: 0, height: 0, x: 100, y: 0, toJSON: () => ({}) }) as DOMRect;
  const sep = c.el.querySelector<HTMLElement>('[role="separator"]')!;
  return { c, sep };
};
const press = (el: HTMLElement, key: string) => el.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
const drag = (sep: HTMLElement, x: number) => {
  sep.dispatchEvent(new MouseEvent("mousedown", { bubbles: true, cancelable: true, clientX: 700 }));
  window.dispatchEvent(new MouseEvent("mousemove", { bubbles: true, clientX: x }));
  window.dispatchEvent(new MouseEvent("mouseup", { bubbles: true, clientX: x }));
};

test("resizable columns: a focusable vertical separator sits between the columns", () => {
  const { c, sep } = sized("info");
  expect(Array.from(c.el.children)).toEqual([c.left, sep, c.right]);
  expect(sep.getAttribute("aria-orientation")).toBe("vertical");
  expect(sep.tabIndex).toBe(0);
  expect(sep.getAttribute("aria-valuemin")).toBe("25");
  expect(sep.getAttribute("aria-valuemax")).toBe("75");
  expect(Number(sep.getAttribute("aria-valuenow"))).toBeGreaterThanOrEqual(25);
});

test("resizable columns: ←/→ move 5%, Home and End go to the limits, and each is remembered per tab", () => {
  const { sep } = sized("info");
  const start = Number(sep.getAttribute("aria-valuenow"));
  press(sep, "ArrowRight");
  expect(Number(sep.getAttribute("aria-valuenow"))).toBe(start + 5);
  press(sep, "ArrowLeft"); press(sep, "ArrowLeft");
  expect(Number(sep.getAttribute("aria-valuenow"))).toBe(start - 5);
  press(sep, "Home");
  expect(sep.getAttribute("aria-valuenow")).toBe("25");
  press(sep, "ArrowLeft");
  expect(sep.getAttribute("aria-valuenow")).toBe("25");
  press(sep, "End");
  expect(sep.getAttribute("aria-valuenow")).toBe("75");
  expect(localStorage.getItem(RESIZE_KEY)).toBe("75");
  expect(localStorage.getItem("zen.panel.cols.problems")).toBeNull();
});

test("resizable columns: dragging sets the split from the pointer, within 25-75%, and stores it on release", () => {
  const { c, sep } = sized("info");
  drag(sep, 600); // (600 - 100) / 1000
  expect(sep.getAttribute("aria-valuenow")).toBe("50");
  expect(c.el.style.getPropertyValue("--zen-cols-left")).toBe("50%");
  expect(localStorage.getItem(RESIZE_KEY)).toBe("50");
  drag(sep, 5000);
  expect(sep.getAttribute("aria-valuenow")).toBe("75");
  drag(sep, -400);
  expect(sep.getAttribute("aria-valuenow")).toBe("25");
  // the listeners are gone after the release
  window.dispatchEvent(new MouseEvent("mousemove", { bubbles: true, clientX: 600 }));
  expect(sep.getAttribute("aria-valuenow")).toBe("25");
});

test("resizable columns: a stored split comes back, a value outside 25-75% is clamped, junk is ignored", () => {
  localStorage.setItem(RESIZE_KEY, "40");
  expect(sized("info").sep.getAttribute("aria-valuenow")).toBe("40");
  localStorage.setItem(RESIZE_KEY, "95");
  expect(sized("info").sep.getAttribute("aria-valuenow")).toBe("75");
  localStorage.setItem(RESIZE_KEY, "3");
  expect(sized("info").sep.getAttribute("aria-valuenow")).toBe("25");
  localStorage.setItem(RESIZE_KEY, "wide");
  const n = Number(sized("info").sep.getAttribute("aria-valuenow"));
  expect(n).toBeGreaterThanOrEqual(25);
  expect(n).toBeLessThanOrEqual(75);
});

test("resizable columns: every column scrolls on its own and wraps, and the separator hides when the columns stack", async () => {
  const { readFileSync } = await import("node:fs");
  const css = readFileSync(new URL("./panel.css", import.meta.url), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
  expect(css).toMatch(/\.zen-panel-col\s*\{[^}]*overflow-wrap:\s*anywhere/);
  expect(css).toMatch(/@container \(max-width:\s*700px\)\s*\{[^@]*\.zen-panel-sep\s*\{[^}]*display:\s*none/);
  expect(css).toMatch(/\.zen-panel-sep:(hover|focus-visible)[^{]*\{[^}]*--fg-muted/);
});

test("resizable columns: without an id nothing is stored, and the key follows the tab id", () => {
  const { sep } = sized();
  press(sep, "End");
  expect(sep.getAttribute("aria-valuenow")).toBe("75");
  expect(localStorage.length).toBe(0);
});
