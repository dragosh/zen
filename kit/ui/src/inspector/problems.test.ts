// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { AddonInfo, Page } from "../host";
import { alert } from "../lint/test-util";
import { INSPECTOR_MARK, type ProblemTarget } from "./events";
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


let marks: (ProblemTarget | null)[] = [];
const onMark = (e: Event) => marks.push((e as CustomEvent<ProblemTarget | null>).detail);
beforeEach(() => { marks = []; document.addEventListener(INSPECTOR_MARK, onMark); });
afterEach(() => document.removeEventListener(INSPECTOR_MARK, onMark));
const key = (i: Inspector, k: string) => {
  const r = i.el.querySelector<HTMLElement>(".zen-problem:focus") ?? rows(i)[0];
  r.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
};

test("click shows in page: one click selects, shows the details and asks the page to mark it; there is no Show button", () => {
  const i = make();
  expect(i.el.querySelector('[data-action="show"]')).toBeNull();
  rows(i)[0].click();
  expect(rows(i)[0].getAttribute("aria-selected")).toBe("true");
  expect(detail(i, "rule")).toBe("broken-link");
  expect(marks).toHaveLength(1);
  expect(marks[0]).toMatchObject({ kind: "link", severity: "warning", text: "Gone page" });
  expect(i.el.querySelector('[data-action="show"]')).toBeNull();
  expect(i.el.querySelector(".zen-problem-details button")).toBeNull();
});

test("click shows in page: a missing image sends its src and alt, a spelling problem its word and index", () => {
  const i = make();
  failImage(root.querySelector("#bad")!);
  i.setIssues([alert({ line: 3, match: "teh", severity: "error" })]);
  marks = [];
  rows(i)[1].click();
  expect(marks.at(-1)).toMatchObject({ kind: "image", src: "/_ws/missing.png", text: "Missing one" });
  rows(i)[2].click();
  expect(marks.at(-1)).toMatchObject({ kind: "spelling", severity: "error", text: "teh", offset: 0, length: 3 });
  expect(select).not.toHaveBeenCalled();
});

test("click shows in page: choosing a page warning clears the mark, there is nothing to show", () => {
  const i = warned(["unknown layout \"x\""], { html: "<a data-zen-link='broken'>Gone</a>" });
  select1(i, 1);
  select1(i, 0);
  expect(marks.at(-1)).toBeNull();
});

test("click shows in page: Enter and Space are the button's own click; selecting again marks again", () => {
  const i = make();
  const r = rows(i)[0];
  expect(r.tagName).toBe("BUTTON");
  r.click();
  r.click();
  expect(marks).toHaveLength(2);
});

test("click shows in page: arrow keys move the selection and each move marks that problem", () => {
  const i = make();
  failImage(root.querySelector("#bad")!);
  rows(i)[0].focus();
  rows(i)[0].click();
  key(i, "ArrowDown");
  expect(rows(i)[1].getAttribute("aria-selected")).toBe("true");
  expect(document.activeElement).toBe(rows(i)[1]);
  expect(marks.at(-1)).toMatchObject({ kind: "image" });
  key(i, "ArrowDown"); // the last one stays
  expect(rows(i)[1].getAttribute("aria-selected")).toBe("true");
  key(i, "ArrowUp");
  expect(rows(i)[0].getAttribute("aria-selected")).toBe("true");
  expect(marks.at(-1)).toMatchObject({ kind: "link" });
  expect(document.activeElement).toBe(rows(i)[0]); // focus stays in the list
});

test("click shows in page: the refresh of the same page keeps the selection and does not mark again", () => {
  const i = make();
  rows(i)[0].click();
  marks = [];
  i.setIssues([]);
  expect(rows(i)[0].getAttribute("aria-selected")).toBe("true");
  expect(marks).toEqual([]);
});

test("click shows in page: the mark is cleared when the selection goes away, the page changes or the Inspector closes", () => {
  const i = make();
  i.setIssues([alert({ line: 3, match: "teh" })]);
  rows(i)[1].click();
  marks = [];
  i.setIssues([]);
  expect(marks.at(-1)).toBeNull();
  rows(i)[0].click();
  marks = [];
  i.setPage("b.md", page(), document.createElement("article"));
  expect(marks.at(-1)).toBeNull();
  const j = make();
  rows(j)[0].click();
  marks = [];
  j.close();
  expect(marks.at(-1)).toBeNull();
});

test("hostile href: a problem's href and text are compared as values, never used as a selector", () => {
  const nasty = `"]'),a[x='\\`;
  const i = make([VALE], `<p><a data-zen-link="broken" href='${nasty.replace(/'/g, "&#39;")}'>t</a></p>`);
  expect(() => rows(i)[0].click()).not.toThrow();
  expect(marks[0]!.href).toBe(nasty);
});


// ---- ui/inspector-polish: the page's render warnings

const DIALECT = "dialect myst is not in this build";
const LAYOUT = 'unknown layout "slides"';
function warned(warnings: string[], opts: { pageWarnings?: boolean; html?: string } = {}): Inspector {
  const i = createInspector({ doc: document, host: { addons: [VALE] } as never, onSelectIssue: select, pageWarnings: opts.pageWarnings ?? true });
  made.push(i);
  const r = document.createElement("article");
  r.innerHTML = opts.html ?? "<p>fine</p>";
  document.body.append(i.el, r);
  i.setPage("a.md", { ...page(), warnings }, r);
  i.open();
  i.el.querySelector<HTMLButtonElement>('[data-tab="problems"]')!.click();
  return i;
}
const select1 = (i: Inspector, n = 0) => rows(i)[n].click();

test("page warnings: each is a warning problem under Checks, counted in the chip and the badge, even when it is the only problem", () => {
  const i = warned([DIALECT, LAYOUT]);
  expect(kinds(i)).toEqual(["warning", "warning"]);
  expect(rows(i)[0].dataset.severity).toBe("warning");
  expect(rows(i)[0].textContent).toContain(DIALECT);
  expect(i.el.querySelector('[data-tab="problems"] .zen-panel-count')!.textContent).toBe("2");
  expect(i.button.querySelector(".zen-inspector-badge")!.textContent).toBe("2");
  expect(i.button.getAttribute("aria-label")).toBe("Inspector, 2 issues");
  choose(i, "checks");
  expect(kinds(i)).toEqual(["warning", "warning"]);
  choose(i, "spelling");
  expect(rows(i)).toHaveLength(0);
});

test("page warnings: they come before and count with the other checks and spelling", () => {
  const i = warned([LAYOUT], { html: "<a data-zen-link='broken'>Gone</a>" });
  i.setIssues([alert({ line: 3, match: "teh" })]);
  expect(i.el.querySelector('[data-tab="problems"] .zen-panel-count')!.textContent).toBe("3");
  expect(kinds(i)).toContain("warning");
  choose(i, "checks");
  expect(kinds(i).sort()).toEqual(["link", "warning"]);
});

test("page warnings: the right column has the message and the Fix help for a dialect and for a layout", () => {
  const i = warned([DIALECT, LAYOUT, "something else"]);
  select1(i, 0);
  expect(detail(i, "message")).toBe(DIALECT);
  expect(detail(i, "fix")).toBe("Settings › Markdown, or remove it from .zenmd.yml");
  select1(i, 1);
  expect(detail(i, "fix")).toBe("Settings › Layouts, or change layout: in the front matter");
  select1(i, 2);
  expect(detail(i, "message")).toBe("something else");
  expect(detail(i, "fix")).toBeUndefined();
  expect(i.el.querySelector('.zen-problem-details [data-action="show"]')).toBeNull(); // there is no element to show
});

test("page warnings: the live reload replaces them, and the selection stays on the same warning", () => {
  const i = warned([DIALECT, LAYOUT]);
  select1(i, 1);
  const r = document.createElement("article");
  i.setPage("a.md", { ...page(), warnings: [LAYOUT] }, r);
  expect(kinds(i)).toEqual(["warning"]);
  expect(detail(i, "message")).toBe(LAYOUT);
  i.setPage("a.md", { ...page(), warnings: [] }, r);
  expect(rows(i)).toHaveLength(0);
  expect(i.el.querySelector<HTMLElement>('[data-tab="problems"] .zen-panel-count')!.hidden).toBe(true);
});

test("page warnings: an inspector that is not told to list them (the editor's) does not", () => {
  const i = warned([DIALECT], { pageWarnings: false });
  expect(rows(i)).toHaveLength(0);
});

test("warning text is text", () => {
  const i = warned(['unknown layout "<img src=x onerror=alert(1)>"']);
  expect(i.el.querySelector(".zen-problem img")).toBeNull();
  select1(i);
  expect(i.el.querySelector(".zen-problem-details img")).toBeNull();
  expect(detail(i, "message")).toContain("<img src=x");
});

test("config notice: the folder's old settings file is a warning with a Fix line, like any page warning", () => {
  const NOTICE = "This folder's settings are in .zen.yml; Zenmark now calls the file .zenmd.yml.";
  const i = warned([NOTICE]);
  expect(kinds(i)).toEqual(["warning"]);
  select1(i, 0);
  expect(detail(i, "message")).toBe(NOTICE);
  expect(detail(i, "fix")).toBe("It is renamed the next time a folder setting is saved, or rename it yourself.");
  choose(i, "checks");
  expect(kinds(i)).toEqual(["warning"]);
  // both files: the message only
  const j = warned([".zen.yml is ignored because .zenmd.yml exists."]);
  select1(j, 0);
  expect(detail(j, "fix")).toBeUndefined();
});
