// Settings › Templates (ui/settings-layout): the site templates of File › New Site from Template… in the left
// column (name, with a mark only for templates that are not built in or are invalid), and in the right one the
// description, what it asks when a site is made, and Show Templates Folder. All text goes in through textContent.
import type { Api, SiteTemplate } from "../api";
import { button, el } from "@zenmd/kit";
import type { SettingsItem } from "@zenmd/kit/shell/settings";

export type TemplatesSection = {
  items(): Promise<SettingsItem[]>;
  render(item: SettingsItem, right: HTMLElement): () => void;
  empty(): string;
  destroy(): void;
};

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

export function createTemplatesSection(opts: { api: Api }): TemplatesSection {
  const { api } = opts;
  let templates: SiteTemplate[] = [];
  let failure = "";

  async function items(): Promise<SettingsItem[]> {
    try {
      templates = await api.siteTemplates();
      failure = "";
    } catch (err) {
      templates = [];
      failure = `Could not list templates: ${message(err)}`;
      return [];
    }
    return templates.map((t): SettingsItem => ({
      id: t.id, label: t.name || t.id,
      note: [t.error ? "Invalid" : "", t.builtin ? "" : "Your templates"].filter(Boolean).join(" · ") || undefined,
    }));
  }

  function render(item: SettingsItem, right: HTMLElement): () => void {
    const t = templates.find((x) => x.id === item.id);
    if (!t) return () => {};
    const status = el("p", "zen-settings-status");
    status.setAttribute("role", "status");
    const head = el("div", "zen-plugin-help-head");
    if (t.version) head.append(el("span", "zen-settings-plugin-version", t.version));
    head.append(el("span", "zen-settings-plugin-kind", t.builtin ? "Built in" : "Your templates"));
    right.append(head);
    if (t.description) right.append(el("p", "zen-plugin-help-description", t.description));
    if (t.error) right.append(el("p", "zen-settings-plugin-problem", t.error));
    if (t.update) right.append(el("p", "zen-plugin-help-runs", t.update));
    if (t.ask.length > 0) {
      right.append(el("h4", "zen-plugin-section", "Asks when a site is made"));
      const list = el("ul", "zen-template-asks");
      for (const a of t.ask) list.append(el("li", "", a.default ? `${a.label} (default ${a.default})` : a.label));
      right.append(list);
    }
    const folder = button("Show Templates Folder", () => {
      status.textContent = "";
      api.revealTemplates().catch((err) => { status.textContent = `Could not show the templates folder: ${message(err)}`; });
    });
    const actions = el("div", "zen-settings-actions");
    actions.append(folder);
    right.append(actions, status);
    return () => {};
  }

  return { items, render, empty: () => failure || "No templates.", destroy() {} };
}
