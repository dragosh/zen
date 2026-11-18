// The Settings page of Zenmark: the kit's frame (kit/ui shell/settings.ts, with Appearance) and
// zen's sections: Configuration files (the two config files open in zen's own editor window),
// Plugins (shell/pluginhelp.ts), Layouts and Website. All text through textContent.
import { appearanceSection, button, createSettingsFrame, el, type SettingsSection, type SettingsView } from "@zenmd/kit";
import type { Api } from "../api";
import { createWebsiteSection, type BuildApi } from "./buildsite";
import { createLayoutsSection } from "./layouthelp";
import { createPluginsSection } from "./pluginhelp";

export type { SettingsView };

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

export function createSettingsView(opts: {
  api: Api;
  hasWorkspace(): boolean;
  /** The build services (default: the api, once it has them) and what Build… does (ui/static-build). */
  build?: Partial<BuildApi>;
  openBuild?(): void;
  /** The open folder's path, "" for none (for the last build of the folder). */
  root?(): string;
}): SettingsView {
  const { api } = opts;
  let frame: SettingsView | null = null;
  const doc = () => (frame ? frame.el.ownerDocument : document);

  // ---- Configuration files

  const filesEl = el("div");
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
  filesEl.append(global.r, local.r, status);
  const files: SettingsSection = {
    id: "files", label: "Configuration files", el: filesEl,
    show() {
      local.b.disabled = !opts.hasWorkspace();
      status.textContent = "";
    },
  };

  // ---- Plugins, Layouts, Website

  const plugins = createPluginsSection({ api });
  const layouts = createLayoutsSection({ api });
  const website = createWebsiteSection({
    api: opts.build ?? (api as unknown as Partial<BuildApi>),
    root: () => (opts.hasWorkspace() ? (opts.root?.() ?? "/") : ""),
    openBuild: opts.openBuild,
  });

  frame = createSettingsFrame([
    appearanceSection(doc),
    files,
    { id: "plugins", label: "Plugins", el: plugins.el, show: () => void plugins.refresh(), destroy: () => plugins.destroy() },
    { id: "layouts", label: "Layouts", el: layouts.el, show: () => void layouts.refresh(), destroy: () => layouts.destroy() },
    { id: "website", label: "Website", el: website.el, show: () => void website.refresh() },
  ]);
  return frame;
}
