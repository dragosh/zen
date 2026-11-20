// The Settings page of Zenmark (ui/settings-layout): the kit's frame (kit/ui shell/settings.ts), where every
// section has two columns (what you pick on the left, its preview, help and actions on the right), and zen's
// sections: Appearance, Configuration (the two config files), Markdown (the dialects the folder uses), Layouts,
// Plugins, Templates, Export (the website and the PDF, in place) and Developer. What the build contains is in
// Zenmark › About. A section whose feature the build leaves out is not there. All text through textContent.
import { appearanceSection, button, createSettingsFrame, developerSection, el, hasFeature, mountPage, reloadSettings, type DialectInfo, type SettingsItem, type SettingsSection, type SettingsView } from "@zenmd/kit";
import type { Api } from "../api";
import { createBuildForm, type BuildApi } from "./buildsite";
import { createLayoutsSection } from "./layouthelp";
import { createPluginsSection } from "./pluginhelp";
import { createTemplatesSection } from "./templateshelp";

export type { SettingsView };

/** The preview records the PDF it saved here (the windows share one origin); Settings shows it. */
export const LAST_PDF_KEY = "zen.lastPdfExport";
export function lastPdfExport(): { path: string; at: number } | undefined {
  try {
    const v = JSON.parse(localStorage.getItem(LAST_PDF_KEY) ?? "null");
    if (v && typeof v.path === "string" && typeof v.at === "number") return v;
  } catch { /* no storage */ }
  return undefined;
}

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

/** A short sample of the syntax a dialect adds, shown beside its rendered preview. Dialects without one show none. */
const DIALECT_SAMPLES: Record<string, string> = {
  myst: ":::{note}\nA note, written the MyST way.\n:::",
};

export function createSettingsView(opts: {
  api: Api;
  hasWorkspace(): boolean;
  /** The build services (default: the api, once it has them). */
  build?: Partial<BuildApi>;
  /** Not used any more: File › Export › Website… opens Settings › Export (the shell calls `open("export", "website")`). */
  openBuild?(): void;
  /** The open folder's path, "" for none. */
  root?(): string;
  /** Developer › Open Console (⌥⌘J): the shell opens the Inspector on the Console. */
  openConsole?(): void;
  /** A page named by a build warning: the shell shows it. */
  onOpenPage?(path: string): void;
  /** Paths with unsaved changes in editor windows. */
  unsaved?(): string[];
  /** Rows the Appearance section shows under its preview (ui/nav-panel adds the sidebar side). */
  appearanceExtra?: HTMLElement[];
}): SettingsView {
  const { api } = opts;
  let frame: SettingsView | null = null;
  const doc = () => (frame ? frame.el.ownerDocument : document);
  const root = () => (opts.hasWorkspace() ? (opts.root?.() ?? "/") : "");

  // ---- Configuration: the two config files, each with a read-only preview and its actions

  const configItems = (): SettingsItem[] => [
    { id: "global", label: "config.yml", note: "All folders" },
    { id: "workspace", label: ".zen.yml", note: "This folder", disabled: !opts.hasWorkspace() },
  ];
  function renderConfig(item: SettingsItem, right: HTMLElement): () => void {
    const which = item.id as "global" | "workspace";
    const name = item.label;
    let gone = false;
    right.append(el("p", "zen-settings-hint", which === "global" ? "The settings for all folders." : "The settings of the open folder."));
    const pre = el("pre", "zen-settings-file-preview");
    const actions = el("div", "zen-settings-actions");
    const status = el("p", "zen-settings-status");
    status.setAttribute("role", "status");
    right.append(pre, actions, status);
    const run = (what: () => Promise<void>, done: string, fail: string): void => {
      status.textContent = "";
      what().then(() => { if (!gone) { status.textContent = done; void load(); } }, (err) => { if (!gone) status.textContent = `${fail}: ${message(err)}`; });
    };
    async function load(): Promise<void> {
      let text = "";
      try { text = await api.source(`zen:config/${which}`); } catch { text = ""; }
      if (gone) return;
      const exists = text !== "";
      pre.textContent = exists ? text : `${name} does not exist yet.`;
      actions.replaceChildren(
        exists
          ? button("Open in Editor", () => run(() => api.openConfig(which), `${name} opened in the editor.`, `Could not open ${name}`))
          : button("Create", () => run(() => api.openConfig(which), `${name} created and opened in the editor.`, `Could not create ${name}`)),
        ...(exists ? [button("Reveal in Finder", () => run(() => api.revealInFinder(`zen:config/${which}`), "", `Could not show ${name}`))] : []),
      );
    }
    void load();
    return () => { gone = true; };
  }
  const config: SettingsSection = { id: "config", label: "Configuration", items: configItems, render: renderConfig };

  // ---- Markdown: the dialects of the build, each on or off for this folder (saved in the folder's .zen.yml)

  let dialects: DialectInfo[] = [];
  let dialectFailure = "";
  const markdown: SettingsSection = {
    id: "markdown", label: "Markdown",
    async items() {
      dialectFailure = "";
      try {
        dialects = await (api.dialects?.() ?? Promise.resolve([]));
      } catch (err) {
        dialects = [];
        dialectFailure = `Could not read the markdown syntaxes: ${message(err)}`;
      }
      return dialects.map((d): SettingsItem => ({ id: d.id, label: d.name, note: d.on ? "On" : "Off" }));
    },
    empty: () => dialectFailure || "This build reads CommonMark and GFM only.",
    render(item, right) {
      const d = dialects.find((x) => x.id === item.id);
      if (!d) return;
      let gone = false;
      const open = opts.hasWorkspace();
      const editable = hasFeature(api, "editor");
      const reason = !open ? "Open a folder to choose its markdown syntaxes." : !editable ? "This app is read-only, so the folder's settings cannot be changed." : "";
      const status = el("p", "zen-settings-status");
      status.setAttribute("role", "status");
      const row = el("label", "zen-settings-check zen-dialect");
      row.dataset.dialect = d.id;
      const box = el("input");
      box.type = "checkbox";
      box.dataset.dialect = d.id;
      box.checked = d.on;
      box.disabled = reason !== "";
      box.addEventListener("change", () => {
        const want = box.checked;
        status.textContent = "";
        api.setDialect(d.id, want).then(() => { if (!gone) reloadSettings(box); }, (err) => {
          box.checked = !want;
          status.textContent = `Could not save ${d.name}: ${message(err)}`;
        });
      });
      row.append(box, el("span", "", "Use in this folder"));
      right.append(row, el("p", "zen-addon-summary zen-dialect-summary", d.summary));
      if (reason) right.append(el("p", "zen-settings-hint", reason));
      else right.append(el("p", "zen-settings-hint", "Saved in this folder's .zen.yml, so the folder renders the same everywhere."));
      const sample = DIALECT_SAMPLES[d.id];
      if (sample) {
        right.append(el("h4", "zen-plugin-section", "Syntax"), Object.assign(el("pre", "zen-settings-file-preview"), { textContent: sample }), el("h4", "zen-plugin-section", "Rendered"));
        const shown = el("div", "zen-page zen-dialect-preview");
        right.append(shown);
        if (!d.on) right.append(el("p", "zen-settings-hint", "Turn it on to see the syntax rendered in this folder."));
        api.preview("zen-dialect-sample.md", sample).then((page) => {
          if (!gone && shown.isConnected) mountPage(shown, page);
        }).catch(() => { /* the sample stays without a preview */ });
      }
      right.append(status);
      return () => { gone = true; };
    },
  };

  // ---- Layouts, Plugins, Templates

  const plugins = createPluginsSection({ api });
  const layouts = createLayoutsSection({ api });
  const templates = createTemplatesSection({ api });

  // ---- Export: the website (its form, in place) and the PDF (the preview, and the last file)

  const website = createBuildForm({
    api: opts.build ? (opts.build as BuildApi) : (api as unknown as BuildApi),
    doc: document,
    root,
    onOpenPage: (p) => opts.onOpenPage?.(p),
    unsaved: opts.unsaved,
  });
  const withPdf = hasFeature(api, "print");
  function renderPdf(right: HTMLElement): void {
    const open = opts.hasWorkspace();
    const status = el("p", "zen-settings-status");
    status.setAttribute("role", "status");
    const exportPdf = button("Export PDF…", () => void api.printSite().catch((err) => { status.textContent = message(err); }), { className: "zen-settings-export-pdf primary" });
    exportPdf.disabled = !open;
    const last = el("p", "zen-settings-export-last");
    const l = open ? lastPdfExport() : undefined;
    if (l) {
      last.append(`Last PDF: ${l.path.split(/[\\/]/).pop()}, ${new Date(l.at).toLocaleTimeString()}. `);
      last.append(button("Show in Finder", () => void api.revealExport().catch((err) => { status.textContent = message(err); }), { className: "zen-buildsite-link" }));
    } else if (open) {
      last.textContent = "No PDF yet.";
    } else {
      last.textContent = "Open a folder to export it.";
    }
    right.append(el("p", "zen-settings-hint", "The pages of the folder as one document, through the print preview."), last, exportPdf, status);
  }
  const exportSection: SettingsSection = {
    id: "export", label: "Export",
    items: () => [{ id: "website", label: "Website" }, ...(withPdf ? [{ id: "pdf", label: "PDF" }] : [])],
    render(item, right) {
      if (item.id === "pdf") { renderPdf(right); return; }
      right.append(website.el);
      void website.refresh();
    },
    // The page is shown again: the folder's options may have changed (a build that runs, or a result on show, stays).
    show: () => { if (website.el.isConnected) void website.refresh(true); },
    destroy: () => website.destroy(),
  };

  frame = createSettingsFrame([
    appearanceSection(doc, opts.appearanceExtra),
    // Nothing opens the config files in an app without the editor: the section is not there.
    ...(hasFeature(api, "editor") ? [config] : []),
    // The sections of a feature the build leaves out are not there at all (kit/shell-features): Markdown, Layouts
    // and Plugins belong to the reader, Templates to newsite, Export to export, Developer to developer.
    ...(hasFeature(api, "reader") ? [
      markdown,
      { id: "layouts", label: "Layouts", ...layouts },
      { id: "plugins", label: "Plugins", ...plugins },
    ] : []),
    ...(hasFeature(api, "newsite") ? [{ id: "templates", label: "Templates", ...templates }] : []),
    ...(hasFeature(api, "export") ? [exportSection] : []),
    ...(hasFeature(api, "developer") ? [developerSection({ openConsole: opts.openConsole })] : []),
  ]);
  const view = frame;
  return {
    ...view,
    destroy() {
      plugins.destroy();
      layouts.destroy();
      templates.destroy();
      view.destroy();
    },
  };
}
