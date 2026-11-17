// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { PluginInfo } from "../api";
import { createFakeApi } from "../api/fake";
import { createSettingsView, type SettingsView } from "./settings";
import { followTheme, storedTheme, THEME_KEY } from "./theme";

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

type Setup = { api: ReturnType<typeof createFakeApi>; s: SettingsView };
function setup(hasWorkspace = true, plugins: PluginInfo[] = []): Setup {
  const api = createFakeApi({ plugins });
  const s = createSettingsView({ api, hasWorkspace: () => hasWorkspace });
  document.body.append(s.el);
  s.show();
  return { api, s };
}
const settle = async () => { for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0)); };
const radio = (s: SettingsView, id: string) => s.el.querySelector<HTMLButtonElement>(`[role=radio][data-theme="${id}"]`)!;
const section = (s: SettingsView, id: string) => s.el.querySelector<HTMLButtonElement>(`.zen-settings-section[data-section="${id}"]`)!;
const panel = (s: SettingsView, id: string) => s.el.querySelector<HTMLElement>(`.zen-settings-panel[data-section="${id}"]`)!;
const item = (s: SettingsView, id: string) => s.el.querySelector<HTMLElement>(`.zen-settings-plugin[data-plugin="${id}"]`)!;
const pick = (s: SettingsView, id: string) => item(s, id).querySelector<HTMLButtonElement>(".zen-settings-plugin-select")!;
const trust = (s: SettingsView, id: string) => item(s, id).querySelector<HTMLButtonElement>(".zen-settings-plugin-trust");
const helpPane = (s: SettingsView) => s.el.querySelector<HTMLElement>(".zen-plugin-help")!;
const key = (el: Element, k: string) => el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
async function plugins(plugins: PluginInfo[] = PLUGINS, hasWorkspace = true): Promise<Setup> {
  const r = setup(hasWorkspace, plugins);
  section(r.s, "plugins").click();
  await settle();
  return r;
}

// ---- the page and its sections

test("it is a page with a section list on the left: Appearance, Configuration files, Plugins", () => {
  const { s } = setup();
  const names = Array.from(s.el.querySelectorAll(".zen-settings-section")).map((b) => b.textContent);
  expect(names).toEqual(["Appearance", "Configuration files", "Plugins"]);
  expect(s.el.querySelector("h1")!.textContent).toBe("Settings");
  expect(s.el.getAttribute("aria-label")).toBe("Settings");
  expect(s.el.classList.contains("zen-settings-page")).toBe(true);
  expect(s.el.getAttribute("role")).not.toBe("dialog");
});

test("one section shows at a time; the first is Appearance; arrow keys and clicks switch", () => {
  const { s } = setup();
  const shown = () => Array.from(s.el.querySelectorAll<HTMLElement>(".zen-settings-panel")).filter((p) => !p.hidden).map((p) => p.dataset.section);
  expect(shown()).toEqual(["appearance"]);
  expect(section(s, "appearance").getAttribute("aria-current")).toBe("true");
  section(s, "files").click();
  expect(shown()).toEqual(["files"]);
  expect(section(s, "appearance").hasAttribute("aria-current")).toBe(false);
  const list = s.el.querySelector<HTMLElement>(".zen-settings-sections")!;
  key(list, "ArrowDown");
  expect(shown()).toEqual(["plugins"]);
  expect(document.activeElement).toBe(section(s, "plugins"));
  key(list, "ArrowDown"); // wraps
  expect(shown()).toEqual(["appearance"]);
  key(list, "ArrowUp");
  expect(shown()).toEqual(["plugins"]);
});

// ---- appearance

test("theme: system by default, choosing applies and remembers it", () => {
  const { s } = setup();
  expect(radio(s, "system").getAttribute("aria-checked")).toBe("true");
  radio(s, "dark").click();
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(localStorage.getItem(THEME_KEY)).toBe("dark");
  expect(radio(s, "dark").getAttribute("aria-checked")).toBe("true");
  radio(s, "system").click();
  expect(document.documentElement.dataset.theme).toBeUndefined();
  expect(storedTheme()).toBe("system");
});

test("arrow keys move through the theme options", () => {
  const { s } = setup();
  const seg = s.el.querySelector<HTMLElement>("[role=radiogroup]")!;
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(storedTheme()).toBe("light");
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
  expect(storedTheme()).toBe("dark");
});

test("showing the view again re-reads the stored theme", () => {
  const { s } = setup();
  localStorage.setItem(THEME_KEY, "dark");
  s.show();
  expect(radio(s, "dark").getAttribute("aria-checked")).toBe("true");
});

// ---- configuration files

test("config files open through the API; folder file disabled without a folder; errors shown as text", async () => {
  const { api, s } = setup();
  section(s, "files").click();
  s.el.querySelector<HTMLButtonElement>('[aria-label="Open config.yml"]')!.click();
  await settle();
  expect(api.callsTo("openConfig")).toEqual([["global"]]);
  const status = panel(s, "files").querySelector("[role=status]")!;
  expect(status.textContent).toContain("config.yml opened");
  api.failNext("openConfig", new Error("<b>nope</b>"));
  s.el.querySelector<HTMLButtonElement>('[aria-label="Open .zen.yml"]')!.click();
  await settle();
  expect(status.textContent).toContain("<b>nope</b>");
  expect(s.el.querySelector("b")).toBeNull();

  const none = setup(false);
  expect(none.s.el.querySelector<HTMLButtonElement>('[aria-label="Open .zen.yml"]')!.disabled).toBe(true);
});

test("followTheme applies the stored theme and follows other windows", () => {
  localStorage.setItem(THEME_KEY, "light");
  const off = followTheme();
  expect(document.documentElement.dataset.theme).toBe("light");
  localStorage.setItem(THEME_KEY, "dark");
  window.dispatchEvent(new StorageEvent("storage", { key: THEME_KEY }));
  expect(document.documentElement.dataset.theme).toBe("dark");
  off();
});

// ---- plugins: the list

test("plugins: the list reads the API when the view is shown; each row has name, version, origin, kind badge, what runs and status", async () => {
  const { api, s } = await plugins();
  expect(api.callsTo("plugins")).toHaveLength(1);
  const rows = Array.from(s.el.querySelectorAll<HTMLElement>(".zen-settings-plugin"));
  expect(rows.map((r) => r.dataset.plugin)).toEqual(["highlight", "diagram", "note", "calc", "shout", "old", "broken"]);
  const text = (id: string) => item(s, id).textContent!;
  expect(text("highlight")).toContain("0.1.0");
  expect(text("highlight")).toContain("Built in");
  expect(text("highlight")).toContain("On");
  expect(trust(s, "highlight")).toBeNull();
  expect(text("note")).toContain("This folder");
  expect(text("note")).toContain("./note.sh");
  expect(text("note")).toContain("scripts in the page");
  expect(text("note")).toContain("Not allowed yet");
  expect(text("calc")).toContain("Your plugins");
  expect(text("calc")).toContain("bc -l");
  expect(text("old")).toContain("Blocked");
  const badge = (id: string) => item(s, id).querySelector(".zen-settings-plugin-kind")!.textContent;
  expect([badge("highlight"), badge("note"), badge("shout")]).toEqual(["builtin", "program", "script"]);
  expect(text("shout")).toContain("cannot read files");
  s.show();
  await settle();
  expect(api.callsTo("plugins")).toHaveLength(2);
});

test("plugins: an invalid plugin shows its problem and no Allow/Block; all text is plain text", async () => {
  const { s } = await plugins();
  expect(item(s, "broken").textContent).toContain("Invalid");
  expect(item(s, "broken").textContent).toContain("id must equal the folder name <b>x</b>");
  expect(trust(s, "broken")).toBeNull();
  expect(item(s, "old").textContent).toContain("<img src=x>");
  expect(s.el.querySelector(".zen-settings-plugins b, .zen-settings-plugins img")).toBeNull();
});

test("plugins: Allow and Block call pluginTrust and refresh the list; the selection stays", async () => {
  const { api, s } = await plugins();
  pick(s, "diagram").click();
  const allow = trust(s, "note")!;
  expect(allow.textContent).toBe("Allow");
  expect(allow.getAttribute("aria-label")).toBe("Allow note");
  expect(trust(s, "old")!.textContent).toBe("Allow");
  expect(trust(s, "calc")!.textContent).toBe("Block");

  allow.click();
  await settle();
  expect(api.callsTo("pluginTrust")).toEqual([["note", true]]);
  expect(api.callsTo("plugins")).toHaveLength(2);
  expect(item(s, "note").textContent).toContain("On");
  expect(trust(s, "note")!.textContent).toBe("Block");
  expect(item(s, "diagram").querySelector(".zen-settings-plugin-select")!.getAttribute("aria-current")).toBe("true");

  trust(s, "calc")!.click();
  await settle();
  expect(api.callsTo("pluginTrust")).toEqual([["note", true], ["calc", false]]);
  expect(item(s, "calc").textContent).toContain("Blocked");
  expect(trust(s, "calc")!.textContent).toBe("Allow");
});

test("plugins: empty state, and failures shown as text", async () => {
  const { api, s } = await plugins([]);
  expect(s.el.querySelector(".zen-settings-plugins")!.textContent).toBe("No plugins in this folder.");
  expect(helpPane(s).textContent).toContain("No plugins");
  api.failNext("plugins", new Error("<i>no folder</i>"));
  s.show();
  await settle();
  expect(s.el.querySelector(".zen-settings-plugins")!.textContent).toContain("<i>no folder</i>");
  expect(s.el.querySelector("i")).toBeNull();

  const t = await plugins();
  t.api.failNext("pluginTrust", new Error("cannot write trusted-plugins.yml"));
  trust(t.s, "note")!.click();
  await settle();
  expect(t.s.el.querySelector(".zen-settings-plugins-status")!.textContent).toContain("cannot write trusted-plugins.yml");
  expect(item(t.s, "note").textContent).toContain("Not allowed yet");
});

// ---- plugins: selection and keyboard

test("plugins: the first plugin is selected by default and its help shows next to the list", async () => {
  const { s } = await plugins();
  expect(pick(s, "highlight").getAttribute("aria-current")).toBe("true");
  expect(pick(s, "diagram").hasAttribute("aria-current")).toBe(false);
  expect(helpPane(s).querySelector(".zen-plugin-help-title")!.textContent).toContain("highlight");
  const list = s.el.querySelector(".zen-settings-plugins")!;
  const pane = helpPane(s);
  expect(list.compareDocumentPosition(pane) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});

test("plugins: clicking a plugin shows its help", async () => {
  const { s } = await plugins();
  pick(s, "note").click();
  expect(pick(s, "note").getAttribute("aria-current")).toBe("true");
  expect(pick(s, "highlight").hasAttribute("aria-current")).toBe(false);
  expect(helpPane(s).textContent).toContain("Callout boxes.");
});

test("plugins: ↑/↓ move through the entries, Enter selects, the keys wrap and stop at the ends", async () => {
  const { s } = await plugins();
  const list = s.el.querySelector<HTMLElement>(".zen-settings-plugins")!;
  pick(s, "highlight").focus();
  key(list, "ArrowDown");
  expect(document.activeElement).toBe(pick(s, "diagram"));
  expect(pick(s, "highlight").getAttribute("aria-current")).toBe("true"); // moving does not select
  key(list, "Enter");
  expect(pick(s, "diagram").getAttribute("aria-current")).toBe("true");
  expect(helpPane(s).textContent).toContain("Draws diagrams.");
  key(list, "ArrowUp");
  key(list, "ArrowUp");
  expect(document.activeElement).toBe(s.el.querySelector(".zen-settings-plugin-all .zen-settings-plugin-select")); // the All entry is on top
  key(list, "ArrowUp");
  expect(document.activeElement).toBe(s.el.querySelector(".zen-settings-plugin-all .zen-settings-plugin-select")); // the top stops
  key(list, "End");
  expect(document.activeElement).toBe(pick(s, "broken"));
  key(list, "ArrowDown");
  expect(document.activeElement).toBe(pick(s, "broken"));
  key(list, "Home");
  expect(document.activeElement).toBe(s.el.querySelector(".zen-settings-plugin-all .zen-settings-plugin-select"));
});

// ---- plugins: the help pane

test("help pane: summary, description, and what the plugin adds with option types and defaults", async () => {
  const { s } = await plugins();
  const pane = helpPane(s);
  expect(pane.querySelector(".zen-plugin-help-summary")!.textContent).toBe("Colours <b>code</b> blocks.");
  expect(pane.querySelector("b")).toBeNull();
  expect(pane.textContent).toContain("Code colours");
  const opts = Array.from(pane.querySelectorAll(".zen-plugin-option")).map((r) => r.textContent);
  expect(opts).toHaveLength(2);
  expect(opts[0]).toContain("lineNumbers");
  expect(opts[0]).toContain("bool");
  expect(opts[0]).toContain("false");
  expect(opts[0]).toContain("Show a gutter.");
  expect(opts[1]).toContain("style");
  expect(opts[1]).toContain("string");
  expect(opts[1]).toContain("zen");

  pick(s, "note").click();
  const adds = helpPane(s).querySelector(".zen-plugin-adds")!.textContent!;
  expect(adds).toContain("Blocks");
  expect(adds).toContain("note");
  expect(adds).toContain("Inlines");
  expect(adds).toContain("badge");
  expect(adds).toContain("Layouts");
  expect(adds).toContain("slides");
  pick(s, "diagram").click();
  expect(helpPane(s).querySelector(".zen-plugin-adds")!.textContent).toContain("mermaid");
  expect(helpPane(s).querySelector(".zen-plugin-adds")!.textContent).not.toContain("Inlines");
  expect(helpPane(s).querySelector(".zen-plugin-option")).toBeNull();
});

test("help pane: a plugin without help says so; an invalid one shows its problem", async () => {
  const { s } = await plugins();
  pick(s, "calc").click();
  expect(helpPane(s).textContent).toContain("No help");
  expect(helpPane(s).textContent).toContain("bc -l");
  expect(helpPane(s).querySelector(".zen-plugin-snippet")).toBeNull();
  pick(s, "broken").click();
  expect(helpPane(s).textContent).toContain("id must equal the folder name <b>x</b>");
  expect(helpPane(s).querySelector("b")).toBeNull();
});

test("help pane: each snippet has a title, its markdown as text and a Copy button", async () => {
  const { s } = await plugins();
  const snippets = Array.from(helpPane(s).querySelectorAll<HTMLElement>(".zen-plugin-snippet"));
  expect(snippets.map((x) => x.querySelector(".zen-plugin-snippet-title")!.textContent)).toEqual(["A coloured block", "Line numbers"]);
  expect(snippets[0].querySelector("pre")!.textContent).toBe("```go\nfmt.Println(1)\n```");
  expect(snippets[0].querySelector("button")!.textContent).toBe("Copy");
  expect(snippets[0].querySelector("button")!.getAttribute("aria-label")).toBe("Copy A coloured block");
});

test("Copy uses navigator.clipboard and says Copied", async () => {
  const writeText = vi.fn(async () => undefined);
  vi.stubGlobal("navigator", { clipboard: { writeText } });
  const { s } = await plugins();
  const btn = helpPane(s).querySelector<HTMLButtonElement>(".zen-plugin-copy")!;
  btn.click();
  await settle();
  expect(writeText).toHaveBeenCalledWith("```go\nfmt.Println(1)\n```");
  expect(btn.textContent).toBe("Copied");
});

test("Copy falls back to a textarea and execCommand without a clipboard (or when it refuses)", async () => {
  const exec = vi.fn(() => true);
  (document as unknown as { execCommand: unknown }).execCommand = exec;
  let selected = "";
  const orig = HTMLTextAreaElement.prototype.select;
  HTMLTextAreaElement.prototype.select = function (this: HTMLTextAreaElement) { selected = this.value; };
  try {
    vi.stubGlobal("navigator", {});
    const { s } = await plugins();
    const btn = helpPane(s).querySelector<HTMLButtonElement>(".zen-plugin-copy")!;
    btn.click();
    await settle();
    expect(exec).toHaveBeenCalledWith("copy");
    expect(selected).toBe("```go\nfmt.Println(1)\n```");
    expect(btn.textContent).toBe("Copied");
    expect(document.querySelector("textarea")).toBeNull(); // removed again

    vi.stubGlobal("navigator", { clipboard: { writeText: async () => { throw new Error("denied"); } } });
    selected = "";
    btn.click();
    await settle();
    expect(selected).toBe("```go\nfmt.Println(1)\n```");

    exec.mockReturnValue(false);
    btn.click();
    await settle();
    expect(btn.textContent).toBe("Copy failed");
  } finally {
    HTMLTextAreaElement.prototype.select = orig;
    delete (document as unknown as { execCommand?: unknown }).execCommand;
  }
});

test("built-in snippets get a rendered preview through api.preview; layout pages and external plugins do not", async () => {
  const { api, s } = await plugins();
  const calls = api.callsTo("preview");
  expect(calls).toHaveLength(1); // the second snippet starts with frontmatter: a whole page, not a preview
  expect(calls[0][1]).toBe("```go\nfmt.Println(1)\n```");
  const previews = helpPane(s).querySelectorAll(".zen-plugin-snippet-preview");
  expect(previews).toHaveLength(1);
  expect(previews[0].textContent).toContain("fmt.Println(1)");
  pick(s, "note").click();
  await settle();
  expect(api.callsTo("preview")).toHaveLength(1);
  expect(helpPane(s).querySelector(".zen-plugin-snippet")).not.toBeNull();
  expect(helpPane(s).querySelector(".zen-plugin-snippet-preview")).toBeNull();
});

test("a failing preview leaves the snippet alone; a stale preview does not land in another plugin's pane", async () => {
  const { api, s } = await plugins();
  api.failNext("preview", new Error("boom"));
  pick(s, "diagram").click();
  await settle();
  expect(helpPane(s).querySelector(".zen-plugin-snippet")).not.toBeNull();
  expect(helpPane(s).querySelector(".zen-plugin-snippet-preview")).toBeNull();
  pick(s, "highlight").click();
  pick(s, "note").click();
  await settle();
  expect(helpPane(s).querySelector(".zen-plugin-snippet-preview")).toBeNull();
});

// ---- plugins: All plugins

test("All plugins: an entry at the top shows every plugin's main snippet in one page", async () => {
  const { api, s } = await plugins();
  const all = s.el.querySelector<HTMLButtonElement>(".zen-settings-plugin-all .zen-settings-plugin-select")!;
  expect(all.textContent).toContain("All plugins");
  const first = s.el.querySelector(".zen-settings-plugins")!.firstElementChild!;
  expect(first.contains(all)).toBe(true);
  all.click();
  await settle();
  expect(all.getAttribute("aria-current")).toBe("true");
  const pane = helpPane(s);
  expect(pane.dataset.view).toBe("all");
  const items = Array.from(pane.querySelectorAll<HTMLElement>(".zen-plugin-all-item"));
  expect(items.map((i) => i.dataset.plugin)).toEqual(["highlight", "diagram", "note", "calc", "shout", "old"]); // not the invalid one
  expect(items[0].querySelector("pre")!.textContent).toBe("```go\nfmt.Println(1)\n```"); // the main (first) snippet only
  expect(items[0].querySelectorAll(".zen-plugin-snippet")).toHaveLength(1);
  expect(items[2].querySelector("pre")!.textContent).toBe(":::note\nhi\n:::");
  expect(items[3].querySelector("pre")).toBeNull(); // no snippet: description only
  expect(items[3].textContent).toContain("No help");
  expect(items[0].querySelector("button")!.textContent).toBe("Copy");
  expect(api.callsTo("preview").length).toBeGreaterThanOrEqual(2); // built-ins preview
  expect(items[2].querySelector(".zen-plugin-snippet-preview")).toBeNull(); // external: snippet only
  pick(s, "note").click();
  expect(helpPane(s).dataset.view).toBeUndefined();
});
