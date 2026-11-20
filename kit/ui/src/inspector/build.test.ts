// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { BuildInfo, KitHost, Page } from "../host";
import { DEV_KEY, setDeveloperEnabled } from "../shell/developer";
import { createInspector, type InspectorPanel } from "./index";

const page = (): Page => ({ path: "a.md", title: "Alpha", html: "", frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [], layout: { name: "article", source: "" } });
const build = (features: string[], extra: Partial<BuildInfo> = {}): BuildInfo => ({
  app: "t", features, addons: [{ id: "vale", name: "Vale linting", summary: "Spelling." }], plugins: ["mermaid"], layouts: ["article"], templates: ["blog"], dialects: ["myst"], theme: "mono", version: "1.2.3", ...extra,
} as BuildInfo);
const host = (features: string[], extra: Partial<BuildInfo> = {}): KitHost => ({ logs: async () => [], onLog: () => () => {}, build: build(features, extra), addons: build(features).addons }) as unknown as KitHost;

let made: InspectorPanel[] = [];
function make(h: KitHost, withPage = true): InspectorPanel {
  const i = createInspector({ doc: document, host: h, onSelectIssue: () => {}, external: true });
  made.push(i);
  document.body.append(i.el);
  if (withPage) i.setPage("a.md", page(), document.createElement("article"));
  return i;
}
const tabIds = (i: InspectorPanel) => Array.from(i.el.querySelectorAll<HTMLElement>('[role="tab"]')).filter((t) => !t.hidden).map((t) => t.dataset.tab);
const section = (i: InspectorPanel) => i.el.querySelector<HTMLElement>('[data-section="build"]')!;

beforeEach(() => { localStorage.clear(); sessionStorage.clear(); document.body.replaceChildren(); vi.spyOn(console, "log").mockImplementation(() => {}); });
afterEach(() => { for (const i of made.splice(0)) i.destroy(); vi.restoreAllMocks(); });

test("the Build tab comes after Console and follows its rule: the developer feature and the switch", () => {
  expect(DEV_KEY).toBe("zen.developer");
  const i = make(host(["inspector", "developer"]));
  expect(tabIds(i)).toEqual(["info", "problems"]);
  setDeveloperEnabled(true);
  expect(tabIds(i)).toEqual(["info", "problems", "console", "build"]);
  setDeveloperEnabled(false);
  expect(tabIds(i)).toEqual(["info", "problems"]);
});

test("a build without the developer feature has no Build tab at all", () => {
  setDeveloperEnabled(true);
  const i = make(host(["inspector"]));
  expect(Array.from(i.el.querySelectorAll<HTMLElement>('[role="tab"]')).map((t) => t.dataset.tab)).toEqual(["info", "problems"]);
  expect(section(i)).toBeNull();
});

test("a developer-only build has Console and Build, and no page is needed", () => {
  setDeveloperEnabled(true);
  const i = make(host(["developer"]), false);
  expect(tabIds(i)).toEqual(["console", "build"]);
});

test("left column: the version and the features, the missing ones greyed", () => {
  setDeveloperEnabled(true);
  const i = make(host(["inspector", "developer"]));
  const left = section(i).querySelector<HTMLElement>(".zen-panel-col-main")!;
  expect(left.textContent).toContain("Version 1.2.3");
  expect(left.querySelector('[data-feature="inspector"]')!.classList.contains("zen-build-feature-missing")).toBe(false);
  expect(left.querySelector('[data-feature="editor"]')!.classList.contains("zen-build-feature-missing")).toBe(true);
  expect(left.querySelector('[data-group="plugins"]')).toBeNull();
});

test("right column: add-ons, plugins, layouts, templates, dialects and the theme", () => {
  setDeveloperEnabled(true);
  const i = make(host(["inspector", "developer"]));
  const right = section(i).querySelector<HTMLElement>(".zen-panel-col-side")!;
  const groups = Array.from(right.querySelectorAll<HTMLElement>("[data-group]")).map((g) => g.dataset.group);
  expect(groups).toEqual(["addons", "plugins", "layouts", "templates", "dialects", "theme"]);
  expect(right.querySelector('[data-group="plugins"]')!.textContent).toContain("mermaid");
  expect(right.querySelector('[data-group="theme"]')!.textContent).toContain("mono");
  expect(right.querySelector('[data-addon="vale"]')).not.toBeNull();
  expect(right.querySelector('[data-feature]')).toBeNull();
});

test("an unstamped build says dev; names are text, not markup", () => {
  setDeveloperEnabled(true);
  const i = make(host(["inspector", "developer"], { version: undefined, plugins: ["<b>x</b>"] }));
  expect(section(i).textContent).toContain("Version dev");
  expect(section(i).querySelector("b")).toBeNull();
});

test("the Build tab's columns have their own split under zen.panel.cols.build", () => {
  setDeveloperEnabled(true);
  const i = make(host(["inspector", "developer"]));
  const sep = section(i).querySelector<HTMLElement>('[role="separator"]')!;
  sep.dispatchEvent(new KeyboardEvent("keydown", { key: "Home", bubbles: true, cancelable: true }));
  expect(localStorage.getItem("zen.panel.cols.build")).toBe("25");
});
