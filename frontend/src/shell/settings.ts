// The settings panel, opened from the cog at the bottom of the sidebar:
// theme (this machine, all windows), the two config files, opened in zen's
// own editor window, and the plugins with Allow / Block for external ones.
// All text through textContent.
import type { Api, PluginInfo } from "../api";
import { button, el } from "./dom";
import { setTheme, storedTheme, type Theme } from "./theme";

export type SettingsPanel = {
  el: HTMLElement;
  open(): void;
  close(): void;
  toggle(): void;
  isOpen(): boolean;
  destroy(): void;
};

const THEMES: { id: Theme; label: string }[] = [
  { id: "system", label: "System" },
  { id: "light", label: "Light" },
  { id: "dark", label: "Dark" },
];

const SOURCE_LABEL: Record<PluginInfo["source"], string> = { builtin: "Built in", folder: "This folder", user: "Your plugins" };
const STATUS_LABEL: Record<PluginInfo["status"], string> = {
  on: "On", off: "Off", blocked: "Blocked", undecided: "Not allowed yet", invalid: "Invalid",
};
const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

export function createSettings(opts: { api: Api; anchor: HTMLButtonElement; hasWorkspace(): boolean }): SettingsPanel {
  const { api, anchor } = opts;
  const doc = anchor.ownerDocument;
  const panel = el("section", "zen-settings");
  panel.id = "zen-settings";
  panel.setAttribute("role", "dialog");
  panel.setAttribute("aria-label", "Settings");
  panel.hidden = true;
  anchor.setAttribute("aria-controls", panel.id);
  anchor.setAttribute("aria-expanded", "false");

  const title = el("h2", "zen-settings-title", "Settings");

  // Theme: a radio group shown as a segmented control.
  const themeLabel = el("p", "zen-settings-label", "Appearance");
  themeLabel.id = "zen-settings-theme-label";
  const seg = el("div", "zen-settings-segment");
  seg.setAttribute("role", "radiogroup");
  seg.setAttribute("aria-labelledby", themeLabel.id);
  const radios = THEMES.map(({ id, label }) => {
    const b = button(label, () => choose(id), { className: "zen-settings-option" });
    b.setAttribute("role", "radio");
    b.dataset.theme = id;
    return b;
  });
  seg.append(...radios);
  seg.addEventListener("keydown", (e) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const i = THEMES.findIndex((t) => t.id === storedTheme());
    const next = THEMES[(i + step + THEMES.length) % THEMES.length].id;
    choose(next);
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
  function choose(t: Theme): void {
    setTheme(t, doc);
    renderTheme();
  }

  // Config files.
  const filesLabel = el("p", "zen-settings-label", "Configuration files");
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
      close();
    } catch (err) {
      status.textContent = `Could not open ${name}: ${message(err)}`;
    }
  }

  // Plugins: read from the API each time the panel opens and after Allow / Block.
  const pluginsLabel = el("p", "zen-settings-label", "Plugins");
  pluginsLabel.id = "zen-settings-plugins-label";
  const pluginList = el("div", "zen-settings-plugins");
  pluginList.setAttribute("role", "list");
  pluginList.setAttribute("aria-labelledby", pluginsLabel.id);
  const pluginStatus = el("p", "zen-settings-status zen-settings-plugins-status");
  pluginStatus.setAttribute("role", "status");
  let pluginSeq = 0;

  function pluginRow(p: PluginInfo): HTMLElement {
    const r = el("div", "zen-settings-plugin");
    r.setAttribute("role", "listitem");
    r.dataset.plugin = p.id;
    r.dataset.status = p.status;
    const text = el("div", "zen-settings-plugin-text");
    const head = el("div", "zen-settings-plugin-head");
    head.append(el("span", "zen-settings-plugin-name", p.id));
    if (p.version) head.append(el("span", "zen-settings-plugin-version", p.version));
    head.append(el("span", "zen-settings-plugin-state", STATUS_LABEL[p.status]));
    text.append(head, el("span", "zen-settings-plugin-hint", SOURCE_LABEL[p.source]));
    if (p.runs.length > 0) text.append(el("span", "zen-settings-plugin-runs", `Runs: ${p.runs.join(", ")}`));
    if (p.problem) text.append(el("span", "zen-settings-plugin-problem", p.problem));
    r.append(text);
    if (p.source !== "builtin" && p.status !== "invalid") {
      const allow = p.status === "blocked" || p.status === "undecided";
      const verb = allow ? "Allow" : "Block";
      r.append(button(`${verb} ${p.id}`, () => void trust(p.id, allow), { className: "zen-settings-plugin-trust", text: verb }));
    }
    return r;
  }
  async function refreshPlugins(): Promise<void> {
    const mine = ++pluginSeq;
    try {
      const list = await api.plugins();
      if (mine !== pluginSeq) return;
      if (list.length === 0) pluginList.replaceChildren(el("p", "zen-settings-plugins-empty", "No plugins in this folder."));
      else pluginList.replaceChildren(...list.map(pluginRow));
    } catch (err) {
      if (mine !== pluginSeq) return;
      pluginList.replaceChildren(el("p", "zen-settings-plugins-empty", `Could not list plugins: ${message(err)}`));
    }
  }
  async function trust(id: string, allow: boolean): Promise<void> {
    pluginStatus.textContent = "";
    for (const b of Array.from(pluginList.querySelectorAll("button"))) b.disabled = true;
    try {
      await api.pluginTrust(id, allow);
    } catch (err) {
      pluginStatus.textContent = `Could not ${allow ? "allow" : "block"} ${id}: ${message(err)}`;
    }
    await refreshPlugins();
  }

  panel.append(title, themeLabel, seg, filesLabel, global.r, local.r, status, pluginsLabel, pluginList, pluginStatus);

  const onDocDown = (e: Event) => {
    const t = e.target as Node | null;
    if (t && !panel.contains(t) && !anchor.contains(t)) close();
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape" && !panel.hidden) {
      e.preventDefault();
      e.stopPropagation();
      close();
      anchor.focus();
    }
  };

  function open(): void {
    if (!panel.hidden) return;
    renderTheme();
    local.b.disabled = !opts.hasWorkspace();
    status.textContent = "";
    pluginStatus.textContent = "";
    void refreshPlugins();
    panel.hidden = false;
    anchor.setAttribute("aria-expanded", "true");
    doc.addEventListener("mousedown", onDocDown, true);
    doc.addEventListener("keydown", onKey, true);
    (radios.find((r) => r.getAttribute("aria-checked") === "true") ?? radios[0]).focus();
  }
  function close(): void {
    if (panel.hidden) return;
    panel.hidden = true;
    anchor.setAttribute("aria-expanded", "false");
    doc.removeEventListener("mousedown", onDocDown, true);
    doc.removeEventListener("keydown", onKey, true);
  }

  return {
    el: panel,
    open,
    close,
    toggle: () => (panel.hidden ? open() : close()),
    isOpen: () => !panel.hidden,
    destroy() {
      close();
      panel.remove();
    },
  };
}
