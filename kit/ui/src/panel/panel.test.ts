// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createTabbedPanel, panelEmpty, panelProps, type TabbedPanelOptions } from "./index";

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
