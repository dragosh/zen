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

const list = (i: InspectorPanel) => section(i).querySelector<HTMLElement>(".zen-panel-col-main [role=listbox]")!;
const items = (i: InspectorPanel) => Array.from(list(i).querySelectorAll<HTMLElement>('[role="option"]'));
const item = (i: InspectorPanel, id: string) => list(i).querySelector<HTMLElement>(`[data-item="${id}"]`)!;
const right = (i: InspectorPanel) => section(i).querySelector<HTMLElement>(".zen-panel-col-side")!;
const dev = (extra: Partial<BuildInfo> = {}, features = ["inspector", "developer"]) => { setDeveloperEnabled(true); return make(host(features, extra)); };

test("left column: a list of the eight items with a note each, in order; Version is selected first", () => {
  const i = dev();
  expect(list(i).getAttribute("role")).toBe("listbox");
  expect(items(i).map((o) => o.dataset.item)).toEqual(["version", "features", "addons", "plugins", "layouts", "templates", "dialects", "theme"]);
  expect(items(i).map((o) => o.querySelector(".zen-build-item-name")!.textContent)).toEqual(["Version", "Features", "Add-ons", "Plugins", "Layouts", "Templates", "Dialects", "Theme"]);
  expect(items(i).map((o) => o.querySelector(".zen-build-item-note")!.textContent)).toEqual(["1.2.3", "2 of 9", "1", "1", "1", "1", "1", "1"]);
  expect(items(i).map((o) => o.getAttribute("aria-selected"))).toEqual(["true", "false", "false", "false", "false", "false", "false", "false"]);
  expect(section(i).querySelector(".zen-panel-col-main [data-feature]")).toBeNull(); // the details are on the right
});

test("right column: Version has the app version, the engine version only when it differs, and the channel", () => {
  const i = dev({ engineVersion: "1.2.3", channel: "beta" } as never);
  expect(right(i).textContent).toContain("1.2.3");
  expect(right(i).textContent).toContain("beta");
  expect(right(i).querySelector('[data-field="engine"]')).toBeNull();
  const j = dev({ engineVersion: "1.1.0", channel: "stable" } as never);
  expect(right(j).querySelector('[data-field="engine"]')!.textContent).toBe("1.1.0");
  expect(right(j).querySelector('[data-field="channel"]')!.textContent).toBe("stable");
});

test("clicking an item shows its details on the right and marks it selected, as in Problems", () => {
  const i = dev();
  item(i, "features").click();
  expect(item(i, "features").getAttribute("aria-selected")).toBe("true");
  expect(item(i, "version").getAttribute("aria-selected")).toBe("false");
  const f = right(i);
  expect(f.querySelectorAll("[data-feature]")).toHaveLength(9);
  expect(f.querySelector('[data-feature="inspector"]')!.classList.contains("zen-build-feature-missing")).toBe(false);
  const editor = f.querySelector('[data-feature="editor"]')!;
  expect(editor.classList.contains("zen-build-feature-missing")).toBe(true);
  expect(editor.textContent).toContain("not in this build");
  const order = Array.from(f.querySelectorAll<HTMLElement>("[data-feature]")).map((e) => e.dataset.feature);
  expect(order.slice(0, 3)).toEqual(["reader", "nav", "find"]);
});

test("the other items list their ids and descriptions", () => {
  const i = dev();
  item(i, "addons").click();
  expect(right(i).querySelector('[data-addon="vale"]')!.textContent).toContain("Vale linting");
  expect(right(i).querySelector('[data-addon="vale"]')!.textContent).toContain("Spelling.");
  for (const [id, text] of [["plugins", "mermaid"], ["layouts", "article"], ["templates", "blog"], ["dialects", "myst"], ["theme", "mono"]]) {
    item(i, id).click();
    expect(right(i).textContent).toContain(text);
  }
});

test("an empty item says None in this build., and its note is 0", () => {
  const i = dev({ plugins: [], dialects: [] });
  expect(item(i, "plugins").querySelector(".zen-build-item-note")!.textContent).toBe("0");
  item(i, "plugins").click();
  expect(right(i).textContent).toContain("None in this build.");
});

test("arrow keys move the selection and Enter or Space pick, like Problems and the Console", () => {
  const i = dev();
  item(i, "version").focus();
  item(i, "version").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, cancelable: true }));
  expect(item(i, "features").getAttribute("aria-selected")).toBe("true");
  expect(document.activeElement).toBe(item(i, "features"));
  item(i, "features").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp", bubbles: true, cancelable: true }));
  expect(item(i, "version").getAttribute("aria-selected")).toBe("true");
  item(i, "plugins").click(); // Enter and Space are a button's own click
});

test("the selection survives the panel being shown again", () => {
  const i = dev();
  item(i, "layouts").click();
  i.close();
  i.open();
  expect(item(i, "layouts").getAttribute("aria-selected")).toBe("true");
});

test("an unstamped build says dev; names are text, not markup", () => {
  const i = dev({ version: undefined, plugins: ["<b>x</b>"], layouts: ["<i>y</i>"] });
  expect(item(i, "version").querySelector(".zen-build-item-note")!.textContent).toBe("dev");
  item(i, "plugins").click();
  expect(right(i).textContent).toContain("<b>x</b>");
  expect(section(i).querySelector("b, i")).toBeNull();
});

test("the Build tab's columns have their own split under zen.panel.cols.build", () => {
  setDeveloperEnabled(true);
  const i = make(host(["inspector", "developer"]));
  const sep = section(i).querySelector<HTMLElement>('[role="separator"]')!;
  sep.dispatchEvent(new KeyboardEvent("keydown", { key: "Home", bubbles: true, cancelable: true }));
  expect(localStorage.getItem("zen.panel.cols.build")).toBe("25");
});
