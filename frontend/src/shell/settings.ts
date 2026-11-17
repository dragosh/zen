// The Settings page, shown in the main pane (the sidebar stays): a section
// list on the left (Appearance, Configuration files, Plugins) and the section
// on the right. Theme applies to this machine and all windows; the two config
// files open in zen's own editor window; Plugins is shell/pluginhelp.ts.
// All text through textContent.
import type { Api } from "../api";
import { button, el } from "./dom";
import { createPluginsSection } from "./pluginhelp";
import { setTheme, storedTheme, type Theme } from "./theme";

export type SettingsView = {
  el: HTMLElement;
  /** Called each time the page is shown: re-reads the theme and the plugins. */
  show(): void;
  destroy(): void;
};

const THEMES: { id: Theme; label: string }[] = [
  { id: "system", label: "System" },
  { id: "light", label: "Light" },
  { id: "dark", label: "Dark" },
];

const SECTIONS = [
  { id: "appearance", label: "Appearance" },
  { id: "files", label: "Configuration files" },
  { id: "plugins", label: "Plugins" },
] as const;
type SectionId = (typeof SECTIONS)[number]["id"];

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

export function createSettingsView(opts: { api: Api; hasWorkspace(): boolean }): SettingsView {
  const { api } = opts;
  const page = el("section", "zen-settings-page");
  page.setAttribute("aria-label", "Settings");

  // ---- the section list

  const nav = el("nav", "zen-settings-sections");
  nav.setAttribute("aria-label", "Settings sections");
  nav.append(el("h1", "zen-settings-title", "Settings"));
  const tabs = SECTIONS.map(({ id, label }) => {
    const b = button(label, () => choose(id), { className: "zen-settings-section" });
    b.dataset.section = id;
    return b;
  });
  const items = el("div", "zen-settings-sections-list");
  items.append(...tabs);
  nav.addEventListener("keydown", (e) => {
    const step = ({ ArrowDown: 1, ArrowUp: -1 } as Record<string, number>)[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const i = SECTIONS.findIndex((s) => s.id === current);
    const next = (i + step + SECTIONS.length) % SECTIONS.length;
    choose(SECTIONS[next].id);
    tabs[next].focus();
  });
  nav.append(items);

  const body = el("div", "zen-settings-body");
  const panels = new Map<SectionId, HTMLElement>();
  for (const { id, label } of SECTIONS) {
    const p = el("div", "zen-settings-panel");
    p.dataset.section = id;
    p.append(el("h2", "zen-settings-heading", label));
    p.hidden = true;
    panels.set(id, p);
    body.append(p);
  }
  page.append(nav, body);

  let current: SectionId = "appearance";
  function choose(id: SectionId): void {
    current = id;
    for (const t of tabs) {
      const on = t.dataset.section === id;
      if (on) t.setAttribute("aria-current", "true");
      else t.removeAttribute("aria-current");
      t.tabIndex = on ? 0 : -1;
    }
    panels.forEach((p, pid) => { p.hidden = pid !== id; });
  }

  // ---- Appearance: a radio group shown as a segmented control.

  const doc = () => page.ownerDocument;
  const themeLabel = el("p", "zen-settings-label", "Theme");
  themeLabel.id = "zen-settings-theme-label";
  const seg = el("div", "zen-settings-segment");
  seg.setAttribute("role", "radiogroup");
  seg.setAttribute("aria-labelledby", themeLabel.id);
  const radios = THEMES.map(({ id, label }) => {
    const b = button(label, () => pickTheme(id), { className: "zen-settings-option" });
    b.setAttribute("role", "radio");
    b.dataset.theme = id;
    return b;
  });
  seg.append(...radios);
  seg.addEventListener("keydown", (e) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (step === undefined) return;
    e.preventDefault();
    e.stopPropagation(); // not the section list's arrows
    const i = THEMES.findIndex((t) => t.id === storedTheme());
    const next = THEMES[(i + step + THEMES.length) % THEMES.length].id;
    pickTheme(next);
    radios.find((r) => r.dataset.theme === next)!.focus();
  });
  function renderTheme(): void {
    const now = storedTheme();
    for (const r of radios) {
      const on = r.dataset.theme === now;
      r.setAttribute("aria-checked", String(on));
      r.tabIndex = on ? 0 : -1;
    }
  }
  function pickTheme(t: Theme): void {
    setTheme(t, doc());
    renderTheme();
  }
  panels.get("appearance")!.append(themeLabel, seg);

  // ---- Configuration files

  const status = el("p", "zen-settings-status");
  status.setAttribute("role", "status");
  const row = (name: string, hint: string, which: "global" | "workspace") => {
    const r = el("div", "zen-settings-file");
    const text = el("div", "zen-settings-file-text");
    text.append(el("span", "zen-settings-file-name", name), el("span", "zen-settings-file-hint", hint));
    const b = button(`Open ${name}`, () => void openFile(which, name), { className: "zen-settings-file-open", text: "Open" });
    r.append(text, b);
    return { r, b };
  };
  const global = row("config.yml", "All folders", "global");
  const local = row(".zen.yml", "This folder", "workspace");
  async function openFile(which: "global" | "workspace", name: string): Promise<void> {
    status.textContent = "";
    try {
      await api.openConfig(which);
      status.textContent = `${name} opened in the editor.`;
    } catch (err) {
      status.textContent = `Could not open ${name}: ${message(err)}`;
    }
  }
  panels.get("files")!.append(global.r, local.r, status);

  // ---- Plugins

  const plugins = createPluginsSection({ api });
  panels.get("plugins")!.append(plugins.el);

  choose("appearance");
  return {
    el: page,
    show() {
      renderTheme();
      local.b.disabled = !opts.hasWorkspace();
      status.textContent = "";
      void plugins.refresh();
      (tabs.find((t) => t.getAttribute("aria-current") === "true") ?? tabs[0]).focus();
    },
    destroy() {
      plugins.destroy();
      page.remove();
    },
  };
}
