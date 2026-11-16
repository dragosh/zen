// @vitest-environment happy-dom
import { beforeEach, expect, test } from "vitest";
import type { PluginInfo } from "../api";
import { createFakeApi } from "../api/fake";
import { createSettings } from "./settings";
import { followTheme, storedTheme, THEME_KEY } from "./theme";

beforeEach(() => {
  localStorage.clear();
  document.body.replaceChildren();
  delete document.documentElement.dataset.theme;
});

const PLUGINS: PluginInfo[] = [
  { id: "highlight", version: "0.1.0", description: "Code colours", source: "builtin", status: "on", runs: [], problem: "", kind: "builtin" },
  { id: "note", version: "0.2.0", description: "Callouts", source: "folder", status: "undecided", runs: ["./note.sh", "scripts in the page"], problem: "", kind: "program" },
  { id: "calc", version: "0.1.0", description: "", source: "user", status: "on", runs: ["bc -l"], problem: "", kind: "program" },
  { id: "shout", version: "0.1.0", description: "", source: "folder", status: "on", runs: ["renders blocks inside zen; cannot read files, run programs or use the network"], problem: "", kind: "script" },
  { id: "old", version: "1.0.0", description: "", source: "folder", status: "blocked", runs: ["<img src=x>"], problem: "", kind: "program" },
  { id: "broken", version: "", description: "", source: "folder", status: "invalid", runs: [], problem: "plugin.yml: id must equal the folder name <b>x</b>", kind: "program" },
];

function setup(hasWorkspace = true, plugins: PluginInfo[] = []) {
  const api = createFakeApi({ plugins });
  const cog = document.createElement("button");
  document.body.append(cog);
  const s = createSettings({ api, anchor: cog, hasWorkspace: () => hasWorkspace });
  document.body.append(s.el);
  return { api, cog, s };
}
const radio = (s: { el: HTMLElement }, id: string) => s.el.querySelector<HTMLButtonElement>(`[role=radio][data-theme="${id}"]`)!;

test("opens and closes, reflecting state on the cog; Esc and outside clicks close", () => {
  const { cog, s } = setup();
  expect(s.el.hidden).toBe(true);
  s.toggle();
  expect(s.el.hidden).toBe(false);
  expect(cog.getAttribute("aria-expanded")).toBe("true");
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(s.isOpen()).toBe(false);
  expect(document.activeElement).toBe(cog);
  s.open();
  document.body.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  expect(s.isOpen()).toBe(false);
});

test("theme: system by default, choosing applies and remembers it", () => {
  const { s } = setup();
  s.open();
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
  s.open();
  const seg = s.el.querySelector<HTMLElement>("[role=radiogroup]")!;
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
  expect(storedTheme()).toBe("light");
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
  seg.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
  expect(storedTheme()).toBe("dark");
});

test("config files open through the API; folder file disabled without a folder; errors shown as text", async () => {
  const { api, s } = setup();
  s.open();
  s.el.querySelector<HTMLButtonElement>('[aria-label="Open config.yml"]')!.click();
  await Promise.resolve(); await Promise.resolve();
  expect(api.callsTo("openConfig")).toEqual([["global"]]);
  expect(s.el.querySelector("[role=status]")!.textContent).toContain("config.yml opened");
  api.failNext("openConfig", new Error("<b>nope</b>"));
  s.el.querySelector<HTMLButtonElement>('[aria-label="Open .zen.yml"]')!.click();
  await Promise.resolve(); await Promise.resolve(); await Promise.resolve();
  expect(s.el.querySelector("[role=status]")!.textContent).toContain("<b>nope</b>");
  expect(s.el.querySelector("b")).toBeNull();

  const none = setup(false);
  none.s.open();
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

const settle = async () => { for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0)); };
const item = (s: { el: HTMLElement }, id: string) => s.el.querySelector<HTMLElement>(`.zen-settings-plugin[data-plugin="${id}"]`)!;

test("plugins: lists name, version, origin, what runs and status, after the config files", async () => {
  const { api, s } = setup(true, PLUGINS);
  s.open();
  await settle();
  expect(api.callsTo("plugins")).toHaveLength(1);
  const rows = Array.from(s.el.querySelectorAll<HTMLElement>(".zen-settings-plugin"));
  expect(rows.map((r) => r.dataset.plugin)).toEqual(["highlight", "note", "calc", "shout", "old", "broken"]);
  const files = s.el.querySelector(".zen-settings-file")!;
  expect(files.compareDocumentPosition(rows[0]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

  const text = (id: string) => item(s, id).textContent!;
  expect(text("highlight")).toContain("highlight");
  expect(text("highlight")).toContain("0.1.0");
  expect(text("highlight")).toContain("Built in");
  expect(text("highlight")).toContain("On");
  expect(item(s, "highlight").querySelector("button")).toBeNull();

  expect(text("note")).toContain("This folder");
  expect(text("note")).toContain("./note.sh");
  expect(text("note")).toContain("scripts in the page");
  expect(text("note")).toContain("Not allowed yet");
  expect(text("calc")).toContain("Your plugins");
  expect(text("calc")).toContain("bc -l");
  expect(text("old")).toContain("Blocked");
  // What each plugin is made of.
  expect(text("highlight")).toContain("built in");
  expect(text("note")).toContain("program");
  expect(text("shout")).toContain("script");
  expect(text("shout")).toContain("cannot read files");
});

test("plugins: Allow and Block call pluginTrust and refresh the list", async () => {
  const { api, s } = setup(true, PLUGINS);
  s.open();
  await settle();
  const allow = item(s, "note").querySelector<HTMLButtonElement>("button")!;
  expect(allow.textContent).toBe("Allow");
  expect(allow.getAttribute("aria-label")).toBe("Allow note");
  expect(item(s, "old").querySelector("button")!.textContent).toBe("Allow");
  expect(item(s, "calc").querySelector("button")!.textContent).toBe("Block");

  allow.click();
  await settle();
  expect(api.callsTo("pluginTrust")).toEqual([["note", true]]);
  expect(api.callsTo("plugins")).toHaveLength(2);
  expect(item(s, "note").textContent).toContain("On");
  expect(s.isOpen()).toBe(true);

  item(s, "calc").querySelector<HTMLButtonElement>("button")!.click();
  await settle();
  expect(api.callsTo("pluginTrust")).toEqual([["note", true], ["calc", false]]);
  expect(item(s, "calc").textContent).toContain("Blocked");
  expect(item(s, "calc").querySelector("button")!.textContent).toBe("Allow");
});

test("plugins: an invalid plugin shows its problem and no button; all text is plain text", async () => {
  const { s } = setup(true, PLUGINS);
  s.open();
  await settle();
  expect(item(s, "broken").textContent).toContain("Invalid");
  expect(item(s, "broken").textContent).toContain("id must equal the folder name <b>x</b>");
  expect(item(s, "broken").querySelector("button")).toBeNull();
  expect(item(s, "old").textContent).toContain("<img src=x>");
  expect(s.el.querySelector(".zen-settings-plugins b, .zen-settings-plugins img")).toBeNull();
});

test("plugins: empty state, and failures shown as text", async () => {
  const { api, s } = setup();
  s.open();
  await settle();
  expect(s.el.querySelector(".zen-settings-plugins")!.textContent).toBe("No plugins in this folder.");
  s.close();

  api.failNext("plugins", new Error("<i>no folder</i>"));
  s.open();
  await settle();
  expect(s.el.querySelector(".zen-settings-plugins")!.textContent).toContain("<i>no folder</i>");
  expect(s.el.querySelector("i")).toBeNull();

  const t = setup(true, PLUGINS);
  t.s.open();
  await settle();
  t.api.failNext("pluginTrust", new Error("cannot write trusted-plugins.yml"));
  item(t.s, "note").querySelector<HTMLButtonElement>("button")!.click();
  await settle();
  expect(t.s.el.querySelector(".zen-settings-plugins-status")!.textContent).toContain("cannot write trusted-plugins.yml");
  expect(item(t.s, "note").textContent).toContain("Not allowed yet");
});
