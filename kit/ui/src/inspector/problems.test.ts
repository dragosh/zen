// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { AddonInfo, Page } from "../host";
import { alert } from "../lint/test-util";
import { createInspector, type Inspector } from "./index";

const VALE: AddonInfo = { id: "vale", name: "Vale linting", summary: "Spelling and style checks." };
const page = (): Page => ({
  path: "a.md", title: "Alpha", html: "", frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [], layout: { name: "article", source: "" },
});
const HTML = "<h1>A</h1><p><a data-zen-link='broken'>Gone page</a> <a data-zen-link='internal' href='#'>fine</a> <img src='/_ws/ok.png' alt='ok'> <img id='bad' src='/_ws/missing.png' alt='Missing one'></p>";

let made: Inspector[] = [];
const select = vi.fn();
let root: HTMLElement;

function make(addons: readonly AddonInfo[] = [VALE], html = HTML): Inspector {
  const i = createInspector({ doc: document, host: { addons } as never, onSelectIssue: select });
  made.push(i);
  root = document.createElement("article");
  root.innerHTML = html;
  // a loaded image: happy-dom reports every image as finished with no pixels
  for (const img of Array.from(root.querySelectorAll("img"))) {
    Object.defineProperty(img, "complete", { value: true });
    Object.defineProperty(img, "naturalWidth", { value: 10, configurable: true });
  }
  document.body.append(i.el, root);
  i.setPage("a.md", page(), root);
  i.open();
  i.el.querySelector<HTMLButtonElement>('[data-tab="problems"]')!.click();
  return i;
}
const rows = (i: Inspector) => Array.from(i.el.querySelectorAll<HTMLElement>(".zen-problem"));
const kinds = (i: Inspector) => rows(i).map((r) => r.dataset.kind);
const filter = (i: Inspector) => i.el.querySelector<HTMLSelectElement>("select.zen-problems-filter")!;
const choose = (i: Inspector, v: string) => { filter(i).value = v; filter(i).dispatchEvent(new Event("change")); };
const detail = (i: Inspector, f: string) => i.el.querySelector<HTMLElement>(`.zen-problem-details [data-field="${f}"]`)?.textContent;
const failImage = (img: HTMLImageElement) => img.dispatchEvent(new Event("error"));

beforeEach(() => { localStorage.clear(); sessionStorage.clear(); document.body.replaceChildren(); select.mockReset(); });
afterEach(() => { for (const i of made.splice(0)) i.destroy(); vi.restoreAllMocks(); });

test("checks: a link the renderer marked broken is a problem; working links are not", () => {
  const i = make();
  expect(kinds(i)).toEqual(["link"]);
  expect(rows(i)[0].textContent).toContain("Gone page");
  expect(i.el.querySelector('[data-tab="problems"] .zen-panel-count')!.textContent).toBe("1");
});

test("checks: an image that fails to load is a problem when it fails, and one that already failed is found at once", () => {
  const i = make();
  failImage(root.querySelector("#bad")!);
  expect(kinds(i)).toEqual(["link", "image"]);
  expect(rows(i)[1].textContent).toContain("Missing one");
  failImage(root.querySelector("#bad")!); // the same image twice is one problem
  expect(rows(i)).toHaveLength(2);

  const j = createInspector({ doc: document, host: { addons: [] } as never, onSelectIssue: select });
  made.push(j);
  const r = document.createElement("article");
  r.innerHTML = "<img id='x' src='/_ws/nope.png' alt=''>";
  const img = r.querySelector("img")!;
  Object.defineProperty(img, "complete", { value: true });
  Object.defineProperty(img, "naturalWidth", { value: 0 });
  document.body.append(j.el, r);
  j.setPage("a.md", page(), r);
  j.open();
  j.el.querySelector<HTMLButtonElement>('[data-tab="problems"]')!.click();
  expect(kinds(j)).toEqual(["image"]);
});

test("checks follow the page: a new page replaces them, and the old page's images are not listened to", () => {
  const i = make();
  const old = root.querySelector<HTMLImageElement>("#bad")!;
  const next = document.createElement("article");
  next.innerHTML = "<p>nothing wrong</p>";
  i.setPage("b.md", page(), next);
  failImage(old);
  expect(rows(i)).toHaveLength(0);
  expect(i.el.querySelector('[data-section="problems"]')!.textContent).toContain("No problems on this page.");
});

test("spelling: the alerts of the vale add-on are problems of kind spelling", () => {
  const i = make();
  i.setIssues([alert({ line: 3, match: "teh" })]);
  expect(kinds(i)).toEqual(["link", "spelling"]);
  expect(i.el.querySelector('[data-tab="problems"] .zen-panel-count')!.textContent).toBe("2");
});

test("the filter is All / Checks / Spelling and narrows the list", () => {
  const i = make();
  i.setIssues([alert({ line: 3, match: "teh" })]);
  expect(Array.from(filter(i).options).map((o) => o.textContent)).toEqual(["All", "Checks", "Spelling"]);
  expect(filter(i).getAttribute("aria-label")).toBeTruthy();
  choose(i, "checks");
  expect(kinds(i)).toEqual(["link"]);
  choose(i, "spelling");
  expect(kinds(i)).toEqual(["spelling"]);
  choose(i, "all");
  expect(kinds(i)).toEqual(["link", "spelling"]);
});

test("the filter lives in the header, only while Problems is the tab", () => {
  const i = make();
  const group = filter(i).closest<HTMLElement>(".zen-panel-tools-group")!;
  expect(group.hidden).toBe(false);
  i.el.querySelector<HTMLButtonElement>('[data-tab="info"]')!.click();
  expect(group.hidden).toBe(true);
  expect(i.el.querySelector(".zen-panel-head")!.contains(group)).toBe(true);
});

test("without the vale add-on there is no Spelling entry and no spelling problems", () => {
  const i = make([]);
  expect(Array.from(filter(i).options).map((o) => o.value)).toEqual(["all", "checks"]);
  i.setIssues([alert({ line: 1, match: "x" })]);
  expect(kinds(i)).toEqual(["link"]);
});

test("two columns: the list on the left, the selected problem's details on the right", () => {
  const i = make();
  i.setIssues([alert({ line: 7, match: "very", severity: "warning", message: "Avoid very", check: "Vale.Weasel" })]);
  const body = i.el.querySelector<HTMLElement>('[data-section="problems"]')!;
  expect(body.querySelector(".zen-panel-col-main .zen-problem")).not.toBeNull();
  expect(body.querySelector(".zen-panel-col-side .zen-problem-details")).not.toBeNull();
  expect(body.querySelector(".zen-problem-details")!.textContent).toContain("Select a problem");
  rows(i)[1].click();
  expect(rows(i)[1].getAttribute("aria-selected")).toBe("true");
  expect(rows(i)[0].getAttribute("aria-selected")).toBe("false");
  expect(detail(i, "message")).toBe("Avoid very");
  expect(detail(i, "where")).toContain("Line 7");
  expect(detail(i, "rule")).toBe("Vale.Weasel");
  rows(i)[0].click();
  expect(detail(i, "message")).toContain("Broken link");
  expect(detail(i, "rule")).toBe("broken-link");
  expect(detail(i, "where")).toContain("Gone page");
});

test("Show scrolls the page to a broken link and focuses it", () => {
  const spy = vi.fn();
  (window.HTMLElement.prototype as unknown as { scrollIntoView: unknown }).scrollIntoView = spy;
  const i = make();
  rows(i)[0].click();
  i.el.querySelector<HTMLButtonElement>('[data-action="show"]')!.click();
  const a = root.querySelector("a[data-zen-link=broken]")!;
  expect(spy.mock.contexts[0]).toBe(a);
  expect(document.activeElement).toBe(a);
});

test("Show on a missing image scrolls to and focuses the image", () => {
  const spy = vi.fn();
  (window.HTMLElement.prototype as unknown as { scrollIntoView: unknown }).scrollIntoView = spy;
  const i = make();
  failImage(root.querySelector("#bad")!);
  rows(i)[1].click();
  expect(detail(i, "rule")).toBe("missing-image");
  i.el.querySelector<HTMLButtonElement>('[data-action="show"]')!.click();
  expect(document.activeElement).toBe(root.querySelector("#bad"));
});

test("Show on a spelling problem asks the host to scroll to its mark", () => {
  const i = make();
  const a = alert({ line: 3, match: "teh" });
  i.setIssues([a]);
  rows(i)[1].click();
  expect(select).not.toHaveBeenCalled(); // choosing only shows the details
  i.el.querySelector<HTMLButtonElement>('[data-action="show"]')!.click();
  expect(select).toHaveBeenCalledWith(a, 0);
});

test("empty state: No problems on this page.", () => {
  const i = make([VALE], "<p>fine</p>");
  i.setIssues([]);
  expect(i.el.querySelector('[data-section="problems"] .zen-panel-empty-title')!.textContent).toBe("No problems on this page.");
  expect(i.el.querySelector<HTMLElement>('[data-tab="problems"] .zen-panel-count')!.hidden).toBe(true);
});

test("problem text is text, never markup", () => {
  const i = make([VALE], "<p><a data-zen-link='broken'>&lt;img src=x onerror=alert(1)&gt;</a></p>");
  rows(i)[0].click();
  expect(i.el.querySelector(".zen-problem img, .zen-problem-details img")).toBeNull();
  expect(rows(i)[0].textContent).toContain("<img");
});

test("a selected problem that goes away leaves the details empty", () => {
  const i = make();
  i.setIssues([alert({ line: 3, match: "teh" })]);
  rows(i)[1].click();
  i.setIssues([]);
  expect(i.el.querySelector(".zen-problem-details")!.textContent).toContain("Select a problem");
});
