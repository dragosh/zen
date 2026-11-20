// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import type { LayoutInfo, PluginInfo, SiteTemplate } from "../api";
import { createFakeApi, MYST_DIALECT, ZENMARK_BUILD } from "../api/fake";
import { rememberBuild, type BuildApi } from "./buildsite";
import { createSettingsView, type SettingsView } from "./settings";
import { storedTheme, THEME_KEY, type DialectInfo } from "@zenmd/kit";

beforeEach(() => {
  localStorage.clear();
  document.body.replaceChildren();
  delete document.documentElement.dataset.theme;
});
afterEach(() => vi.unstubAllGlobals());

const none = { help: { summary: "", snippets: [] }, blocks: [], inlines: [], layouts: [], options: [] };
const PLUGINS: PluginInfo[] = [
  {
    id: "highlight", version: "0.1.0", description: "Code colours", source: "builtin", status: "on", runs: [], problem: "", kind: "builtin",
    help: {
      summary: "Colours <b>code</b> blocks.",
      snippets: [
        { title: "A coloured block", markdown: "```go\nfmt.Println(1)\n```" },
        { title: "Line numbers", markdown: "---\nplugins:\n  highlight: { lineNumbers: true }\n---\n# Page" },
      ],
    },
    blocks: [], inlines: [], layouts: [],
    options: [{ name: "lineNumbers", type: "bool", default: false, description: "Show a gutter." }, { name: "style", type: "string", default: "zen", description: "" }],
  },
  {
    id: "diagram", version: "0.1.0", description: "Mermaid diagrams", source: "builtin", status: "on", runs: [], problem: "", kind: "builtin",
    help: { summary: "Draws diagrams.", snippets: [{ title: "Flowchart", markdown: "```mermaid\ngraph LR\n  A --> B\n```" }] },
    blocks: ["mermaid", "diagram"], inlines: [], layouts: [], options: [],
  },
  {
    id: "note", version: "0.2.0", description: "Callouts", source: "folder", status: "undecided", runs: ["./note.sh", "scripts in the page"], problem: "", kind: "program",
    help: { summary: "Callout boxes.", snippets: [{ title: "A note", markdown: ":::note\nhi\n:::" }] },
    blocks: ["note"], inlines: ["badge"], layouts: ["slides"], options: [],
  },
  { id: "calc", version: "0.1.0", description: "", source: "user", status: "on", runs: ["bc -l"], problem: "", kind: "program", ...none },
  { id: "shout", version: "0.1.0", description: "", source: "folder", status: "on", runs: ["renders blocks inside zen; cannot read files, run programs or use the network"], problem: "", kind: "script", ...none },
  { id: "old", version: "1.0.0", description: "", source: "folder", status: "blocked", runs: ["<img src=x>"], problem: "", kind: "program", ...none },
  { id: "broken", version: "", description: "", source: "folder", status: "invalid", runs: [], problem: "plugin.yml: id must equal the folder name <b>x</b>", kind: "program", ...none },
];


const TEMPLATES: SiteTemplate[] = [
  { id: "techdocs", name: "Technical documentation", description: "Tutorials and <b>reference</b>", version: "1.0.0", ask: [{ key: "title", label: "Site title", default: "{{folder}}" }], builtin: true, update: "", error: "" },
  { id: "mine", name: "My site", description: "Mine", version: "0.2.0", ask: [], builtin: false, update: "", error: "" },
  { id: "bad", name: "Bad one", description: "", version: "", ask: [], builtin: false, update: "", error: "template.yml: no files <i>x</i>" },
];

type Opts = {
  hasWorkspace?: boolean; plugins?: PluginInfo[]; layouts?: LayoutInfo[]; templates?: SiteTemplate[];
  features?: string[]; sources?: Record<string, string>; dialects?: DialectInfo[];
  view?: Partial<Parameters<typeof createSettingsView>[0]>;
};
function setup(o: Opts = {}) {
  const api = createFakeApi({
    plugins: o.plugins ?? [], layouts: o.layouts ?? [], templates: o.templates, sources: o.sources,
    ...(o.dialects ? { dialects: o.dialects } : {}),
    ...(o.features ? { build: { features: o.features } } : {}),
  });
  const has = o.hasWorkspace ?? true;
  const s = createSettingsView({ api, hasWorkspace: () => has, root: () => (has ? "/Users/me/site" : ""), ...o.view });
  document.body.append(s.el);
  s.show();
  return { api, s };
}
const settle = async () => { for (let i = 0; i < 8; i++) await new Promise((r) => setTimeout(r, 0)); };
const key = (el: Element, k: string) => el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
const tab = (s: SettingsView, id: string) => s.el.querySelector<HTMLButtonElement>(`.zen-settings-section[data-section="${id}"]`)!;
const panel = (s: SettingsView, id: string) => s.el.querySelector<HTMLElement>(`.zen-settings-panel[data-section="${id}"]`)!;
const rows = (s: SettingsView, id: string) => Array.from(panel(s, id).querySelectorAll<HTMLElement>("[role=option]"));
const row = (s: SettingsView, id: string, item: string) => panel(s, id).querySelector<HTMLElement>(`[role=option][data-item="${item}"]`)!;
const detail = (s: SettingsView, id: string) => panel(s, id).querySelector<HTMLElement>(".zen-settings-detail")!;
const btn = (root: ParentNode, text: string) => Array.from(root.querySelectorAll<HTMLButtonElement>("button")).find((b) => b.textContent === text)!;
const names = (s: SettingsView) => Array.from(s.el.querySelectorAll(".zen-settings-section")).map((b) => b.textContent);
async function go(s: SettingsView, id: string, item?: string): Promise<void> {
  tab(s, id).click();
  await settle();
  if (item) { row(s, id, item).click(); await settle(); }
}

// ---- 1: the section list per build

test("section list per build", () => {
  const list = (features?: string[]) => {
    const { s } = setup({ features });
    const n = names(s);
    s.destroy();
    return n;
  };
  expect(list()).toEqual(["Appearance", "Accessibility", "Configuration", "Markdown", "Layouts", "Plugins", "Templates", "Export", "Developer"]); // the full build; no This build
  // the starting shell
  expect(list(["reader", "nav", "inspector", "developer"])).toEqual(["Appearance", "Accessibility", "Markdown", "Layouts", "Plugins", "Developer"]);
  // the bare shell
  expect(list([])).toEqual(["Appearance", "Accessibility"]);
  // a feature brings its section
  expect(list(["reader", "editor"])).toEqual(["Appearance", "Accessibility", "Configuration", "Markdown", "Layouts", "Plugins"]);
  expect(list(["newsite"])).toEqual(["Appearance", "Accessibility", "Templates"]);
  expect(list(["export"])).toEqual(["Appearance", "Accessibility", "Export"]);
  expect(list(["developer"])).toEqual(["Appearance", "Accessibility", "Developer"]);
  expect(list(["editor"])).toEqual(["Appearance", "Accessibility", "Configuration"]);
  expect(list(["reader"])).toEqual(["Appearance", "Accessibility", "Markdown", "Layouts", "Plugins"]);
});

test("every section is two columns, and the arrows of the section list move through the sections", async () => {
  const { s } = setup();
  await settle();
  for (const id of ["appearance", "accessibility", "config", "markdown", "layouts", "plugins", "templates", "export", "developer"]) {
    expect(panel(s, id).querySelector(".zen-settings-cols"), id).not.toBeNull();
  }
  expect(s.el.querySelector("h1")!.textContent).toBe("Settings");
  const list = s.el.querySelector<HTMLElement>(".zen-settings-sections")!;
  key(list, "ArrowDown");
  expect(tab(s, "config").getAttribute("aria-current")).toBe("true");
});

// ---- appearance

test("appearance: the theme is the list, and the old theme tests still hold", async () => {
  const { s } = setup();
  await settle();
  expect(rows(s, "appearance").map((r) => r.dataset.item)).toEqual(["system", "light", "dark"]);
  row(s, "appearance", "dark").click();
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(storedTheme()).toBe("dark");
  localStorage.removeItem(THEME_KEY);
  s.show();
  await settle();
  expect(row(s, "appearance", "system").getAttribute("aria-selected")).toBe("true");
});

// ---- 4: configuration

const CONFIG = "theme: dark\nname: <b>x</b>\n";
test("configuration section", async () => {
  const { api, s } = setup({ sources: { "zen:config/global": CONFIG, "zen:config/workspace": "plugins: {}\n" } });
  await go(s, "config");
  expect(rows(s, "config").map((r) => r.querySelector(".zen-settings-item-label")!.textContent)).toEqual(["config.yml", ".zen.yml"]);
  expect(rows(s, "config")[0].textContent).toContain("All folders");
  expect(rows(s, "config")[1].textContent).toContain("This folder");
  expect(rows(s, "config")[1].getAttribute("aria-disabled")).toBeNull();
  // the file's preview and its actions
  expect(api.callsTo("source")).toContainEqual(["zen:config/global"]);
  const d = detail(s, "config");
  expect(d.querySelector("h3")!.textContent).toBe("config.yml");
  expect(d.querySelector("pre")!.textContent).toBe(CONFIG);
  expect(btn(d, "Create")).toBeUndefined(); // it exists
  btn(d, "Open in Editor").click();
  await settle();
  expect(api.callsTo("openConfig")).toEqual([["global"]]);
  expect(d.querySelector("[role=status]")!.textContent).toContain("config.yml opened");
  btn(d, "Reveal in Finder").click();
  await settle();
  expect(api.callsTo("revealInFinder")).toEqual([["zen:config/global"]]);
  // the local file
  row(s, "config", "workspace").click();
  await settle();
  expect(detail(s, "config").querySelector("pre")!.textContent).toBe("plugins: {}\n");
  // a failing action is shown as text
  api.failNext("openConfig", new Error("<b>nope</b>"));
  btn(detail(s, "config"), "Open in Editor").click();
  await settle();
  expect(detail(s, "config").querySelector("[role=status]")!.textContent).toContain("<b>nope</b>");
  expect(detail(s, "config").querySelector("b")).toBeNull();
});

test("configuration section: Local is disabled with no folder; a missing file offers Create", async () => {
  const none = setup({ hasWorkspace: false, sources: { "zen:config/global": "" } });
  await go(none.s, "config");
  expect(row(none.s, "config", "workspace").getAttribute("aria-disabled")).toBe("true");
  row(none.s, "config", "workspace").click();
  await settle();
  expect(detail(none.s, "config").querySelector("h3")!.textContent).toBe("config.yml"); // not selectable
  // the global file is empty: Create (it writes the commented template and opens it)
  const d = detail(none.s, "config");
  expect(btn(d, "Open in Editor")).toBeUndefined();
  btn(d, "Create").click();
  await settle();
  expect(none.api.callsTo("openConfig")).toEqual([["global"]]);
  // no source at all (the fake throws) counts as missing too
  const miss = setup({ sources: {} });
  await go(miss.s, "config", "workspace");
  expect(btn(detail(miss.s, "config"), "Create")).toBeDefined();
});

test("configuration section: switching folders refreshes Local", async () => {
  let open = false;
  const api = createFakeApi({ sources: { "zen:config/workspace": "a: 1\n" } });
  const s = createSettingsView({ api, hasWorkspace: () => open });
  document.body.append(s.el);
  s.show();
  await go(s, "config");
  expect(row(s, "config", "workspace").getAttribute("aria-disabled")).toBe("true");
  open = true;
  s.show();
  await settle();
  expect(row(s, "config", "workspace").getAttribute("aria-disabled")).toBeNull();
});

test("configuration preview is text", async () => {
  const { s } = setup({ sources: { "zen:config/global": "<img src=x onerror=alert(1)><script>boom()</script>" } });
  await go(s, "config");
  const pre = detail(s, "config").querySelector("pre")!;
  expect(pre.textContent).toContain("<img src=x onerror=alert(1)>");
  expect(pre.children).toHaveLength(0);
  expect(detail(s, "config").querySelector("img, script")).toBeNull();
});

// ---- 5: markdown

test("markdown section", async () => {
  const { api, s } = setup({ dialects: [{ ...MYST_DIALECT }, { id: "x", name: "<b>X</b>", summary: "<i>s</i>", on: true }] });
  await go(s, "markdown");
  expect(rows(s, "markdown").map((r) => [r.dataset.item, r.querySelector(".zen-settings-item-note")!.textContent])).toEqual([["myst", "Off"], ["x", "On"]]);
  const box = () => detail(s, "markdown").querySelector<HTMLInputElement>('input[type="checkbox"]')!;
  expect(detail(s, "markdown").querySelector("h3")!.textContent).toBe("MyST");
  expect(box().checked).toBe(false);
  expect(box().disabled).toBe(false);
  expect(detail(s, "markdown").textContent).toContain(MYST_DIALECT.summary);
  // the syntax sample, and its rendered preview through api.preview
  const sample = detail(s, "markdown").querySelector("pre")!;
  expect(sample.textContent).toContain("{note}");
  expect(api.callsTo("preview")).toHaveLength(1);
  expect(api.callsTo("preview")[0][1]).toBe(sample.textContent);
  expect(detail(s, "markdown").querySelector(".zen-page")).not.toBeNull();
  // turning it on writes the folder and the list shows it
  box().click();
  await settle();
  expect(api.callsTo("setDialect")).toEqual([["myst", true]]);
  expect(row(s, "markdown", "myst").querySelector(".zen-settings-item-note")!.textContent).toBe("On");
  expect(box().checked).toBe(true);
  // a refused change puts the box back and says why
  api.failNext("setDialect", new Error("could not save .zen.yml"));
  box().click();
  await settle();
  expect(box().checked).toBe(true);
  expect(detail(s, "markdown").querySelector("[role=status]")!.textContent).toContain("could not save .zen.yml");
  // names and summaries are text
  row(s, "markdown", "x").click();
  await settle();
  expect(detail(s, "markdown").querySelector("h3")!.textContent).toBe("<b>X</b>");
  expect(panel(s, "markdown").querySelector("b, i")).toBeNull();
  expect(detail(s, "markdown").querySelector("pre")).toBeNull(); // no sample for a dialect that does not give one
});

test("markdown section: no folder or a read-only app disables the switch; no dialects says so", async () => {
  const none = setup({ hasWorkspace: false });
  await go(none.s, "markdown");
  expect(detail(none.s, "markdown").querySelector<HTMLInputElement>("input")!.disabled).toBe(true);
  expect(detail(none.s, "markdown").textContent).toContain("Open a folder");
  const ro = setup({ features: ZENMARK_BUILD.features.filter((f) => f !== "editor") });
  await go(ro.s, "markdown");
  expect(detail(ro.s, "markdown").querySelector<HTMLInputElement>("input")!.disabled).toBe(true);
  expect(detail(ro.s, "markdown").textContent).toContain("read-only");
  const empty = setup({ dialects: [] });
  await go(empty.s, "markdown");
  expect(rows(empty.s, "markdown")).toHaveLength(0);
  expect(detail(empty.s, "markdown").textContent).toBe("This build reads CommonMark and GFM only.");
});

test("markdown section: switching folders refreshes the dialect states", async () => {
  const { api, s } = setup();
  await go(s, "markdown");
  expect(api.callsTo("dialects").length).toBeGreaterThan(0);
  const before = api.callsTo("dialects").length;
  await api.setDialect("myst", true);
  s.show();
  await settle();
  expect(api.callsTo("dialects").length).toBeGreaterThan(before);
  expect(row(s, "markdown", "myst").querySelector(".zen-settings-item-note")!.textContent).toBe("On");
});

// ---- 6: plugin, layout and template lists

const help = (s: SettingsView, id = "plugins") => detail(s, id).querySelector<HTMLElement>(".zen-plugin-help")!;
const trust = (s: SettingsView, id = "plugins") => detail(s, id).querySelector<HTMLButtonElement>(".zen-settings-plugin-trust");
const pluginView = async (plugins: PluginInfo[] = PLUGINS) => {
  const r = setup({ plugins });
  await go(r.s, "plugins");
  return r;
};

test("plugin, layout and template lists", async () => {
  const { api, s } = await pluginView();
  expect(api.callsTo("plugins")).toHaveLength(1);
  expect(rows(s, "plugins").map((r) => r.dataset.item)).toEqual(["*", "highlight", "diagram", "note", "calc", "shout", "old", "broken"]);
  const note = (id: string) => row(s, "plugins", id).querySelector(".zen-settings-item-note")?.textContent ?? null;
  // "on" and "Built in" are gone; only exceptions show
  expect(note("highlight")).toBeNull();
  expect(note("diagram")).toBeNull();
  expect(row(s, "plugins", "highlight").textContent).not.toMatch(/\bOn\b|Built in/);
  expect(note("calc")).toBe("Your plugins");
  expect(note("shout")).toBe("This folder");
  expect(note("note")).toBe("Not allowed yet · This folder");
  expect(note("old")).toBe("Blocked · This folder");
  expect(note("broken")).toBe("Invalid · This folder");
  // layouts
  const l = setup({ layouts: LAYOUTS });
  await go(l.s, "layouts");
  const lnote = (id: string) => row(l.s, "layouts", id).querySelector(".zen-settings-item-note")?.textContent ?? null;
  expect(rows(l.s, "layouts").map((r) => r.dataset.item)).toEqual(["article", "post", "slides", "memo", "mine", "old", "broken"]);
  expect([lnote("article"), lnote("post"), lnote("slides")]).toEqual([null, null, null]);
  expect(lnote("memo")).toBe("Not allowed yet · This folder");
  expect(lnote("mine")).toBe("Your layouts");
  expect(lnote("old")).toBe("Blocked · This folder");
  expect(lnote("broken")).toBe("Invalid · This folder");
  // templates
  const t = setup({ templates: TEMPLATES });
  await go(t.s, "templates");
  expect(rows(t.s, "templates").map((r) => r.dataset.item)).toEqual(["techdocs", "mine", "bad"]);
  expect(rows(t.s, "templates").map((r) => r.querySelector(".zen-settings-item-note")?.textContent ?? null)).toEqual([null, "Your templates", "Invalid · Your templates"]);
  expect(rows(t.s, "templates")[0].querySelector(".zen-settings-item-label")!.textContent).toBe("Technical documentation");
});

test("plugins: the list reads the API each time the page is shown; text is plain; empty and failing states", async () => {
  const { api, s } = await pluginView();
  s.show();
  await settle();
  expect(api.callsTo("plugins")).toHaveLength(2); // the section chosen, and the page shown again
  expect(panel(s, "plugins").querySelector("b, img")).toBeNull();
  row(s, "plugins", "broken").click();
  expect(help(s).textContent).toContain("id must equal the folder name <b>x</b>");
  expect(help(s).querySelector("b")).toBeNull();
  expect(trust(s)).toBeNull(); // invalid: no Allow/Block
  const e = await pluginView([]);
  expect(detail(e.s, "plugins").textContent).toContain("No plugins in this folder.");
  e.api.failNext("plugins", new Error("<i>no folder</i>"));
  e.s.show();
  await settle();
  expect(detail(e.s, "plugins").textContent).toContain("Could not list plugins: <i>no folder</i>");
  expect(detail(e.s, "plugins").querySelector("i")).toBeNull();
});

test("plugins: Allow and Block call pluginTrust and refresh the list; the selection stays", async () => {
  const { api, s } = await pluginView();
  row(s, "plugins", "highlight").click();
  expect(trust(s)).toBeNull(); // built in
  row(s, "plugins", "note").click();
  expect(trust(s)!.textContent).toBe("Allow");
  expect(trust(s)!.getAttribute("aria-label")).toBe("Allow note");
  trust(s)!.click();
  await settle();
  expect(api.callsTo("pluginTrust")).toEqual([["note", true]]);
  expect(api.callsTo("plugins").length).toBeGreaterThanOrEqual(2);
  expect(row(s, "plugins", "note").querySelector(".zen-settings-item-note")!.textContent).toBe("This folder");
  expect(row(s, "plugins", "note").getAttribute("aria-selected")).toBe("true");
  expect(trust(s)!.textContent).toBe("Block");
  expect(document.activeElement).toBe(trust(s));
  row(s, "plugins", "calc").click();
  trust(s)!.click();
  await settle();
  expect(api.callsTo("pluginTrust")).toEqual([["note", true], ["calc", false]]);
  expect(row(s, "plugins", "calc").querySelector(".zen-settings-item-note")!.textContent).toBe("Blocked · Your plugins");
  // a failure is shown as text
  api.failNext("pluginTrust", new Error("cannot write <b>trusted</b>"));
  row(s, "plugins", "note").click();
  trust(s)!.click();
  await settle();
  expect(detail(s, "plugins").querySelector("[role=status]")!.textContent).toContain("cannot write <b>trusted</b>");
  expect(detail(s, "plugins").querySelector("b")).toBeNull();
});

test("plugins: the first one is selected and its help shows on the right; keys move through the list", async () => {
  const { s } = await pluginView();
  // "All plugins" comes first and is the list's first row; the first plugin is selected until the reader picks
  expect(row(s, "plugins", "highlight").getAttribute("aria-selected")).toBe("true");
  expect(detail(s, "plugins").querySelector("h3")!.textContent).toBe("highlight");
  const list = panel(s, "plugins").querySelector<HTMLElement>("[role=listbox]")!;
  row(s, "plugins", "highlight").focus();
  key(list, "ArrowDown");
  expect(document.activeElement).toBe(row(s, "plugins", "diagram"));
  expect(help(s).textContent).toContain("Draws diagrams.");
  key(list, "Home");
  expect(document.activeElement).toBe(row(s, "plugins", "*"));
});

test("plugin help: summary, what it adds with option types and defaults, problems, runs, no help", async () => {
  const { s } = await pluginView();
  const pane = help(s);
  expect(pane.querySelector(".zen-plugin-help-summary")!.textContent).toBe("Colours <b>code</b> blocks.");
  expect(pane.querySelector("b")).toBeNull();
  expect(pane.textContent).toContain("Code colours");
  const opts = Array.from(pane.querySelectorAll(".zen-plugin-option")).map((r) => r.textContent);
  expect(opts[0]).toContain("lineNumbers");
  expect(opts[0]).toContain("bool");
  expect(opts[0]).toContain("false");
  expect(opts[0]).toContain("Show a gutter.");
  row(s, "plugins", "note").click();
  const adds = help(s).querySelector(".zen-plugin-adds")!.textContent!;
  expect(adds).toContain("Blocks");
  expect(adds).toContain("badge");
  expect(adds).not.toContain("Layouts");
  row(s, "plugins", "calc").click();
  expect(help(s).textContent).toContain("No help");
  expect(help(s).textContent).toContain("bc -l");
  row(s, "plugins", "shout").click();
  expect(help(s).textContent).toContain("cannot read files");
});

test("plugin help: snippets are text with Copy; built-in ones get a preview, external and whole-page ones do not", async () => {
  const writeText = vi.fn(async () => undefined);
  vi.stubGlobal("navigator", { clipboard: { writeText } });
  const { api, s } = await pluginView();
  const snippets = Array.from(help(s).querySelectorAll<HTMLElement>(".zen-plugin-snippet"));
  expect(snippets.map((x) => x.querySelector(".zen-plugin-snippet-title")!.textContent)).toEqual(["A coloured block", "Line numbers"]);
  expect(snippets[0].querySelector("pre")!.textContent).toBe("```go\nfmt.Println(1)\n```");
  expect(snippets[0].querySelector("button")!.getAttribute("aria-label")).toBe("Copy A coloured block");
  const copy = snippets[0].querySelector<HTMLButtonElement>("button")!;
  copy.click();
  await settle();
  expect(writeText).toHaveBeenCalledWith("```go\nfmt.Println(1)\n```");
  expect(copy.textContent).toBe("Copied");
  expect(api.callsTo("preview")).toHaveLength(1); // the second snippet is a whole page
  expect(help(s).querySelectorAll(".zen-plugin-snippet-preview")).toHaveLength(1);
  row(s, "plugins", "note").click();
  await settle();
  expect(api.callsTo("preview")).toHaveLength(1);
  expect(help(s).querySelector(".zen-plugin-snippet-preview")).toBeNull();
});

test("plugin help: a failing preview leaves the snippet alone; a stale one does not land in another pane", async () => {
  const { api, s } = await pluginView();
  api.failNext("preview", new Error("boom"));
  row(s, "plugins", "diagram").click();
  await settle();
  expect(help(s).querySelector(".zen-plugin-snippet")).not.toBeNull();
  expect(help(s).querySelector(".zen-plugin-snippet-preview")).toBeNull();
  row(s, "plugins", "highlight").click();
  row(s, "plugins", "note").click();
  await settle();
  expect(help(s).querySelector(".zen-plugin-snippet-preview")).toBeNull();
});

test("All plugins: the main snippet of every plugin on one page", async () => {
  const { s } = await pluginView();
  row(s, "plugins", "*").click();
  await settle();
  expect(detail(s, "plugins").querySelector("h3")!.textContent).toBe("All plugins");
  const pane = help(s);
  expect(pane.dataset.view).toBe("all");
  const items = Array.from(pane.querySelectorAll<HTMLElement>(".zen-plugin-all-item"));
  expect(items.map((i) => i.dataset.plugin)).toEqual(["highlight", "diagram", "note", "calc", "shout", "old"]);
  expect(items[0].querySelectorAll(".zen-plugin-snippet")).toHaveLength(1);
  expect(items[3].textContent).toContain("No help");
  row(s, "plugins", "note").click();
  expect(help(s).dataset.view).toBeUndefined();
});

const LAYOUTS: LayoutInfo[] = [
  {
    id: "article", version: "1.0.0", description: "The reading view", source: "builtin", status: "on", kind: "layout", hasTemplate: false, hasScript: false, problem: "",
    fields: [], options: [],
    help: { summary: "The default <b>reading</b> column.", snippets: [{ title: "Plain page", markdown: "---\nlayout: article\n---\n# Title" }] },
  },
  {
    id: "post", version: "1.0.0", description: "A blog post", source: "builtin", status: "on", kind: "layout", hasTemplate: true, hasScript: false, problem: "",
    fields: [{ name: "date", type: "date", description: "Publication date" }, { name: "author", type: "string", description: "" }],
    options: [{ name: "toc", type: "bool", default: true, description: "Show a rail." }],
    help: { summary: "Cover, title, byline.", snippets: [{ title: "A post", markdown: "---\nlayout: post\ndate: 2026-11-17\n---\n# Title" }, { title: "Without a cover", markdown: "---\nlayout: { name: post, cover: false }\n---\n# T" }] },
  },
  {
    id: "slides", version: "1.0.0", description: "A presentation", source: "builtin", status: "on", kind: "layout", hasTemplate: false, hasScript: true, problem: "",
    fields: [], options: [], help: { summary: "", snippets: [{ title: "A deck", markdown: "---\nlayout: slides\n---\n# A\n\n---\n\n# B" }] },
  },
  {
    id: "memo", version: "0.1.0", description: "A memo", source: "folder", status: "undecided", kind: "layout", hasTemplate: true, hasScript: true, problem: "",
    fields: [{ name: "to", type: "string", description: "Recipient" }], options: [],
    help: { summary: "A memo.", snippets: [{ title: "A memo", markdown: "---\nlayout: memo\nto: Ann\n---\n# Memo" }] },
  },
  { id: "mine", version: "0.1.0", description: "", source: "user", status: "on", kind: "layout", hasTemplate: false, hasScript: false, problem: "", fields: [], options: [], help: { summary: "", snippets: [] } },
  { id: "old", version: "0.1.0", description: "", source: "folder", status: "blocked", kind: "layout", hasTemplate: false, hasScript: false, problem: "", fields: [], options: [], help: { summary: "", snippets: [] } },
  { id: "broken", version: "", description: "", source: "folder", status: "invalid", kind: "layout", hasTemplate: false, hasScript: false, problem: "layout.yml: template <b>x</b> not found", fields: [], options: [], help: { summary: "", snippets: [] } },
];

const lHelp = (s: SettingsView) => detail(s, "layouts").querySelector<HTMLElement>(".zen-layout-help")!;
const layoutView = async (layouts: LayoutInfo[] = LAYOUTS) => {
  const r = setup({ layouts });
  await go(r.s, "layouts");
  return r;
};

test("layouts: the first one is selected; help has description, fields, options and hints; text is plain", async () => {
  const { api, s } = await layoutView();
  expect(api.callsTo("layouts")).toHaveLength(1);
  expect(row(s, "layouts", "article").getAttribute("aria-selected")).toBe("true");
  expect(detail(s, "layouts").querySelector("h3")!.textContent).toBe("article");
  row(s, "layouts", "post").click();
  const h = lHelp(s);
  expect(h.textContent).toContain("A blog post");
  expect(h.textContent).toContain("Cover, title, byline.");
  expect(Array.from(h.querySelectorAll(".zen-layout-field-name")).map((f) => f.textContent)).toEqual(["date", "author"]);
  const opts = Array.from(h.querySelectorAll(".zen-layout-options .zen-plugin-option"));
  expect(opts[0].textContent).toContain("default true");
  expect(h.textContent).toContain("page template");
  row(s, "layouts", "article").click();
  expect(lHelp(s).textContent).toContain("The default <b>reading</b> column.");
  expect(lHelp(s).querySelector("b")).toBeNull();
  row(s, "layouts", "memo").click();
  expect(lHelp(s).textContent).toContain("runs a page script");
  row(s, "layouts", "broken").click();
  expect(lHelp(s).textContent).toContain("template <b>x</b> not found");
});

test("layouts: Allow/Block only for external, valid layouts; they call layoutTrust and refresh", async () => {
  const { api, s } = await layoutView();
  const t = () => trust(s, "layouts");
  row(s, "layouts", "post").click();
  expect(t()).toBeNull();
  row(s, "layouts", "broken").click();
  expect(t()).toBeNull();
  row(s, "layouts", "memo").click();
  expect(t()!.textContent).toBe("Allow");
  t()!.click();
  await settle();
  expect(api.callsTo("layoutTrust")).toEqual([["memo", true]]);
  expect(api.callsTo("pluginTrust")).toEqual([]);
  expect(row(s, "layouts", "memo").querySelector(".zen-settings-item-note")!.textContent).toBe("This folder");
  expect(t()!.textContent).toBe("Block");
  expect(row(s, "layouts", "memo").getAttribute("aria-selected")).toBe("true");
  api.failNext("layoutTrust", new Error("<i>denied</i>"));
  t()!.click();
  await settle();
  expect(detail(s, "layouts").querySelector("[role=status]")!.textContent).toContain("Could not block memo");
  expect(detail(s, "layouts").querySelector("i")).toBeNull();
});

test("layouts: snippets with Copy; built-in layouts preview through api.preview, external ones do not; empty and failing lists", async () => {
  const writeText = vi.fn(async () => {});
  vi.stubGlobal("navigator", { clipboard: { writeText } });
  const { api, s } = await layoutView();
  const initial = api.callsTo("preview").length;
  row(s, "layouts", "post").click();
  await settle();
  const snippets = Array.from(lHelp(s).querySelectorAll<HTMLElement>(".zen-plugin-snippet"));
  expect(snippets.map((x) => x.querySelector("h4")!.textContent)).toEqual(["A post", "Without a cover"]);
  snippets[0].querySelector<HTMLButtonElement>(".zen-plugin-copy")!.click();
  await settle();
  expect(writeText).toHaveBeenCalledWith("---\nlayout: post\ndate: 2026-11-17\n---\n# Title");
  expect(api.callsTo("preview").length - initial).toBe(2);
  row(s, "layouts", "memo").click();
  await settle();
  expect(lHelp(s).querySelector(".zen-plugin-snippet-preview")).toBeNull();
  const e = await layoutView([]);
  expect(detail(e.s, "layouts").textContent).toContain("No layouts in this folder.");
  e.api.failNext("layouts", new Error("down"));
  e.s.show();
  await settle();
  expect(detail(e.s, "layouts").textContent).toContain("Could not list layouts: down");
});

test("templates: description, what it asks, the invalid reason as text; Show Templates Folder", async () => {
  const { api, s } = setup({ templates: TEMPLATES });
  await go(s, "templates");
  const d = () => detail(s, "templates");
  expect(d().querySelector("h3")!.textContent).toBe("Technical documentation");
  expect(d().textContent).toContain("Tutorials and <b>reference</b>");
  expect(d().textContent).toContain("Site title");
  expect(d().querySelector("b")).toBeNull();
  btn(d(), "Show Templates Folder").click();
  await settle();
  expect(api.callsTo("revealTemplates")).toHaveLength(1);
  row(s, "templates", "bad").click();
  expect(d().textContent).toContain("template.yml: no files <i>x</i>");
  expect(d().querySelector("i")).toBeNull();
  api.failNext("revealTemplates", new Error("no folder"));
  btn(d(), "Show Templates Folder").click();
  await settle();
  expect(d().querySelector("[role=status]")!.textContent).toContain("no folder");
});

// ---- 7: export in place

const WEBSITE_OPTIONS = { out: "/Users/me/site/_site", portable: false, drafts: true, clean: false, print: true };
function exportSetup(o: { hasWorkspace?: boolean; view?: Opts["view"]; features?: string[] } = {}) {
  const base = createFakeApi(o.features ? { build: { features: o.features } } : {});
  const build = {
    buildOptions: vi.fn(() => Promise.resolve({ ...WEBSITE_OPTIONS })),
    buildWebsite: vi.fn(() => new Promise<never>(() => {})),
    cancelBuild: vi.fn(() => Promise.resolve()),
    saveBuildDefaults: vi.fn(() => Promise.resolve("")),
    pickBuildFolder: vi.fn(() => Promise.resolve("")),
    openBuiltSite: vi.fn(() => Promise.resolve()),
    revealBuiltSite: vi.fn(() => Promise.resolve()),
    on: () => () => {},
    ...(o.view?.build ?? {}),
  } as unknown as BuildApi;
  const has = o.hasWorkspace ?? true;
  const s = createSettingsView({ api: base, hasWorkspace: () => has, root: () => (has ? "/Users/me/site" : ""), ...o.view, build });
  document.body.append(s.el);
  s.show();
  return { api: base, build, s };
}

test("export in place", async () => {
  const { s, build } = exportSetup();
  await go(s, "export");
  expect(rows(s, "export").map((r) => r.querySelector(".zen-settings-item-label")!.textContent)).toEqual(["Website", "PDF"]);
  // Website is selected, and its form is on the right: no modal
  expect(detail(s, "export").querySelector("h3")!.textContent).toBe("Website");
  const d = detail(s, "export");
  expect(document.querySelector(".zen-buildsite-backdrop, [aria-modal=true]")).toBeNull();
  expect(d.querySelector("[role=dialog]")).toBeNull();
  expect(build.buildOptions).toHaveBeenCalled();
  expect(d.querySelector<HTMLInputElement>(".zen-buildsite-out")!.value).toBe("_site");
  expect(Array.from(d.querySelectorAll<HTMLInputElement>("input[type=radio]")).map((r) => [r.value, r.checked])).toEqual([["portable", false], ["hosted", true]]);
  expect(Array.from(d.querySelectorAll<HTMLInputElement>("input[type=checkbox]")).map((b) => [b.name, b.checked])).toEqual([["drafts", true], ["clean", false], ["print", true]]);
  expect(btn(d, "Export").type).toBe("submit");
  // Build starts, progress shows with Cancel
  btn(d, "Export").click();
  await settle();
  expect(build.buildWebsite).toHaveBeenCalledWith({ out: "/Users/me/site/_site", portable: false, drafts: true, clean: false, print: true });
  expect(d.querySelector("[role=progressbar]")).not.toBeNull();
  btn(d, "Cancel").click();
  expect(build.cancelBuild).toHaveBeenCalledTimes(1);
});

test("export in place: PDF, the last PDF, no folder", async () => {
  const { api, s } = exportSetup();
  try { localStorage.setItem("zen.lastPdfExport", JSON.stringify({ path: "/Users/me/Guide.pdf", at: Date.now() })); } catch { /* no storage */ }
  await go(s, "export", "pdf");
  const d = detail(s, "export");
  expect(d.querySelector("h3")!.textContent).toBe("PDF");
  expect(d.textContent).toContain("Guide.pdf");
  btn(d, "Show in Finder").click();
  await settle();
  expect(api.callsTo("revealExport")).toHaveLength(1);
  btn(d, "Export PDF…").click();
  await settle();
  expect(api.callsTo("printSite")).toHaveLength(1);
  try { localStorage.removeItem("zen.lastPdfExport"); } catch { /* no storage */ }

  const none = exportSetup({ hasWorkspace: false });
  await go(none.s, "export", "pdf");
  expect(btn(detail(none.s, "export"), "Export PDF…").disabled).toBe(true);
  row(none.s, "export", "website").click();
  await settle();
  expect(detail(none.s, "export").textContent).toContain("Open a folder to export it.");
  expect(btn(detail(none.s, "export"), "Export").disabled).toBe(true);
  expect(none.build.buildOptions).not.toHaveBeenCalled();
});

test("export in place: the PDF item only with print; a build that runs keeps running when the reader leaves, and the result shows on return", async () => {
  const only = exportSetup({ features: ["export"] });
  await go(only.s, "export");
  expect(rows(only.s, "export").map((r) => r.dataset.item)).toEqual(["website"]);
  let finish: (r: unknown) => void = () => {};
  const { s, build } = exportSetup({ view: { build: { buildWebsite: vi.fn(() => new Promise((res) => { finish = res; })) } as unknown as Partial<BuildApi> } });
  await go(s, "export");
  btn(detail(s, "export"), "Export").click();
  await settle();
  row(s, "export", "pdf").click();
  await settle();
  tab(s, "appearance").click();
  tab(s, "export").click();
  await settle();
  row(s, "export", "website").click();
  await settle();
  expect(detail(s, "export").querySelector("[role=progressbar]")).not.toBeNull(); // still building, not restarted
  finish({ out: "/Users/me/site/_site", pages: 34, files: 19, warnings: [], millis: 800, cancelled: false });
  await settle();
  row(s, "export", "pdf").click();
  await settle();
  row(s, "export", "website").click();
  await settle();
  expect(detail(s, "export").querySelector(".zen-buildsite-summary")!.textContent).toContain("Built 34 pages and 19 files into _site");
  void build;
});

test("export in place: open() lands on Website (File › Export › Website…, ⇧⌘B)", async () => {
  const { s } = exportSetup();
  s.open("export", "website");
  await settle();
  expect(tab(s, "export").getAttribute("aria-current")).toBe("true");
  expect(row(s, "export", "website").getAttribute("aria-selected")).toBe("true");
  void rememberBuild;
});

// ---- 8: developer

test("developer section is wired: the switch, and Open Console goes to the shell", async () => {
  const openConsole = vi.fn();
  const { s } = setup({ view: { openConsole } });
  await go(s, "developer");
  const d = detail(s, "developer");
  expect(rows(s, "developer").map((r) => r.textContent)).toEqual(["Console"]);
  expect(d.textContent).toContain("Show Console in the Inspector");
  d.querySelector<HTMLInputElement>("input[type=checkbox]")!.click();
  const open = Array.from(d.querySelectorAll("button")).find((b) => b.textContent!.includes("Open Console"))!;
  open.click();
  expect(openConsole).toHaveBeenCalledTimes(1);
});

// ---- ui/accessibility: Settings › Accessibility, in the two-column frame

const spin = (s: SettingsView, label: string) => detail(s, "accessibility").querySelector<HTMLElement>(`[role=spinbutton][aria-label="${label}"]`)!;
const group = (s: SettingsView, label: string) => detail(s, "accessibility").querySelector<HTMLElement>(`[role=radiogroup][aria-label="${label}"]`)!;
const radio = (g: HTMLElement, text: string) => Array.from(g.querySelectorAll<HTMLElement>("[role=radio]")).find((r) => r.textContent === text)!;
const a11yRoot = () => document.documentElement;
const resetBtn = (s: SettingsView) => btn(detail(s, "accessibility"), "Reset to defaults");

describe("Accessibility section", () => {
  beforeEach(() => {
    for (const k of Object.keys(localStorage)) if (k.startsWith("zen.a11y.")) localStorage.removeItem(k);
    delete a11yRoot().dataset.reduceMotion;
    delete a11yRoot().dataset.contrast;
    delete a11yRoot().dataset.linkUnderline;
    a11yRoot().removeAttribute("style");
  });

  test("it is in the bare shell, and Reading only with reader", async () => {
    const bare = setup({ features: [] });
    await go(bare.s, "accessibility");
    expect(names(bare.s)).toEqual(["Appearance", "Accessibility"]);
    expect(rows(bare.s, "accessibility").map((r) => r.dataset.item)).toEqual(["zoom", "motion", "contrast"]);
    expect(rows(bare.s, "accessibility").map((r) => r.querySelector(".zen-settings-item-label")!.textContent)).toEqual(["Zoom", "Motion", "Contrast"]);
    bare.s.destroy();
    document.body.replaceChildren();

    const full = setup({ features: ["reader"] });
    await go(full.s, "accessibility");
    expect(rows(full.s, "accessibility").map((r) => r.dataset.item)).toEqual(["zoom", "motion", "contrast", "reading"]);
    // two columns: the list, and the right column with the item's controls
    expect(panel(full.s, "accessibility").querySelector(".zen-settings-cols")).not.toBeNull();
    expect(detail(full.s, "accessibility").querySelector("h3")!.textContent).toBe("Zoom");
  });

  test("zoom: a stepper that shows the value, and sets the whole window", async () => {
    const zooms: number[] = [];
    const { s } = setup({ features: [], view: { setZoom: (z: number) => zooms.push(z) } });
    await go(s, "accessibility", "zoom");
    const z = spin(s, "Zoom");
    expect(z.getAttribute("aria-valuetext")).toBe("100 percent");
    expect(z.textContent).toContain("100%");
    expect(resetBtn(s).disabled).toBe(true); // at the defaults
    btn(detail(s, "accessibility"), "+").click();
    expect(zooms).toEqual([1.1]);
    expect(localStorage.getItem("zen.a11y.zoom")).toBe("1.1");
    expect(spin(s, "Zoom").getAttribute("aria-valuetext")).toBe("110 percent");
    expect(spin(s, "Zoom").textContent).toContain("110%");
    expect(resetBtn(s).disabled).toBe(false);
    // the keys of a spinbutton: arrows, Home and End
    key(spin(s, "Zoom"), "ArrowUp");
    expect(zooms.at(-1)).toBe(1.25);
    key(spin(s, "Zoom"), "ArrowDown");
    expect(zooms.at(-1)).toBe(1.1);
    key(spin(s, "Zoom"), "End");
    expect(zooms.at(-1)).toBe(2);
    expect(btn(detail(s, "accessibility"), "+").disabled).toBe(true); // the end
    key(spin(s, "Zoom"), "Home");
    expect(zooms.at(-1)).toBe(0.75);
    expect(btn(detail(s, "accessibility"), "−").disabled).toBe(true);
    // Reset puts it back, in the window too
    resetBtn(s).click();
    expect(zooms.at(-1)).toBe(1);
    expect(localStorage.getItem("zen.a11y.zoom")).toBeNull();
    expect(spin(s, "Zoom").getAttribute("aria-valuetext")).toBe("100 percent");
    expect(resetBtn(s).disabled).toBe(true);
  });

  test("motion and contrast apply at once; each radiogroup takes arrow keys", async () => {
    const { s } = setup({ features: [] });
    await go(s, "accessibility", "motion");
    const m = group(s, "Reduce motion");
    expect(Array.from(m.querySelectorAll("[role=radio]")).map((r) => r.textContent)).toEqual(["System", "On"]);
    expect(radio(m, "System").getAttribute("aria-checked")).toBe("true");
    radio(m, "On").click();
    expect(a11yRoot().dataset.reduceMotion).toBe("on");
    expect(localStorage.getItem("zen.a11y.reduceMotion")).toBe("on");
    expect(radio(group(s, "Reduce motion"), "On").getAttribute("aria-checked")).toBe("true");
    key(radio(group(s, "Reduce motion"), "On"), "ArrowLeft");
    expect(a11yRoot().dataset.reduceMotion).toBeUndefined();
    await go(s, "accessibility", "contrast");
    radio(group(s, "Increase contrast"), "On").click();
    expect(a11yRoot().dataset.contrast).toBe("more");
    expect(resetBtn(s).disabled).toBe(false);
    resetBtn(s).click();
    expect(a11yRoot().dataset.contrast).toBeUndefined();
  });

  test("reading: text size, font, spacing, width, underline and a live sample", async () => {
    const { s } = setup({ features: ["reader"] });
    await go(s, "accessibility", "reading");
    const size = spin(s, "Text size");
    expect(size.getAttribute("aria-valuetext")).toBe("17 pixels");
    key(size, "ArrowUp");
    expect(a11yRoot().style.getPropertyValue("--zen-text-size")).toBe("18px");
    expect(spin(s, "Text size").getAttribute("aria-valuetext")).toBe("18 pixels");
    key(spin(s, "Text size"), "End");
    expect(a11yRoot().style.getPropertyValue("--zen-text-size")).toBe("28px");
    key(spin(s, "Text size"), "Home");
    expect(a11yRoot().style.getPropertyValue("--zen-text-size")).toBe("14px");
    radio(group(s, "Font"), "Sans").click();
    expect(a11yRoot().style.getPropertyValue("--zen-text-font")).toBe("var(--font-ui)");
    radio(group(s, "Line spacing"), "Loose").click();
    expect(a11yRoot().style.getPropertyValue("--zen-line-height")).toBe("2.2");
    radio(group(s, "Line width"), "Narrow").click();
    expect(a11yRoot().style.getPropertyValue("--zen-measure")).toBe("60ch");
    const under = detail(s, "accessibility").querySelector<HTMLInputElement>("input[type=checkbox]")!;
    expect(under.closest("label")!.textContent).toBe("Underline links");
    under.checked = true;
    under.dispatchEvent(new Event("change", { bubbles: true }));
    expect(a11yRoot().dataset.linkUnderline).toBe("on");
    // the sample paragraph is a preview, hidden from the screen reader
    const sample = detail(s, "accessibility").querySelector<HTMLElement>(".zen-a11y-sample")!;
    expect(sample.getAttribute("aria-hidden")).toBe("true");
    expect(sample.textContent).not.toBe("");
    resetBtn(s).click();
    expect(a11yRoot().style.getPropertyValue("--zen-text-size")).toBe("");
    expect(a11yRoot().dataset.linkUnderline).toBeUndefined();
    expect(spin(s, "Text size").getAttribute("aria-valuetext")).toBe("17 pixels");
  });

  test("without storage it says so", async () => {
    const { s } = setup({ features: [] });
    await go(s, "accessibility", "motion");
    expect(detail(s, "accessibility").textContent).not.toContain("Not saved");
    vi.stubGlobal("localStorage", { getItem() { throw new Error("no"); }, setItem() { throw new Error("no"); }, removeItem() { throw new Error("no"); } });
    await go(s, "accessibility", "contrast");
    expect(detail(s, "accessibility").textContent).toContain("Not saved: applies to this window only.");
  });
});

test("appearanceExtra: the extra rows show in Appearance", async () => {
  const extra = Object.assign(document.createElement("div"), { className: "my-extra-row", textContent: "Left / Right" });
  const { s } = setup({ view: { appearanceExtra: [extra] } });
  await settle();
  expect(detail(s, "appearance").querySelector(".my-extra-row")).toBe(extra);
  expect(detail(s, "appearance").querySelector(".zen-theme-preview")).not.toBeNull();
  // the preview is before the extra row
  expect(detail(s, "appearance").querySelector(".zen-theme-preview")!.compareDocumentPosition(extra) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});
