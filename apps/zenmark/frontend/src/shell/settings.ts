// The Settings page of Zenmark (ui/settings-layout): the kit's frame (kit/ui shell/settings.ts), where every
// section has two columns (what you pick on the left, its preview, help and actions on the right), and zen's
// sections: Appearance, Accessibility (zoom, motion, contrast and the reader's text), Configuration (the two config files), Markdown (the dialects the folder uses), Layouts,
// Plugins, Templates, Export (the website and the PDF, in place) and Developer. What the build contains is in
// Zenmark › About. A section whose feature the build leaves out is not there. All text through textContent.
import { appearanceSection, button, createSettingsFrame, developerSection, el, hasFeature, mountPage, reloadSettings, type DialectInfo, type SettingsItem, type SettingsSection, type SettingsView } from "@zenmd/kit";
import { accessibilitySection } from "@zenmd/kit/shell/settings";
import type { Api } from "../api";
import { createBuildForm, type BuildApi } from "./buildsite";
import { createPdfForm, type PdfApi } from "./pdfform";
import { createLayoutsSection } from "./layouthelp";
import { createPluginsSection } from "./pluginhelp";
import { createTemplatesSection } from "./templateshelp";

export type { SettingsView };

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
  /** The PDF form's services (default: the api). */
  pdf?: Partial<PdfApi>;
  /** The page the main window shows, "" for none: Export › PDF › Pages › This section needs one. */
  currentPage?(): string;
  /** Not used any more: File › Export › Website… opens Settings › Export (the shell calls `open("export", "website")`). */
  openBuild?(): void;
  /** The open folder's path, "" for none. */
  root?(): string;
  /** The folder's settings file in use (Workspace.configFile): the old name is shown as such. */
  configFile?(): string;
  /** Developer › Open Console (⌥⌘J): the shell opens the Inspector on the Console. */
  openConsole?(): void;
  /** A page named by a build warning: the shell shows it. */
  onOpenPage?(path: string): void;
  /** Paths with unsaved changes in editor windows. */
  unsaved?(): string[];
  /** Rows the Appearance section shows under its preview (ui/nav-panel adds the sidebar side). */
  appearanceExtra?: HTMLElement[];
  /** Scales the whole window (Settings › Accessibility › Zoom); the app gives Window.SetZoom through followAccessibility. */
  setZoom?(zoom: number): void;
}): SettingsView {
  const { api } = opts;
  let frame: SettingsView | null = null;
  const doc = () => (frame ? frame.el.ownerDocument : document);
  const root = () => (opts.hasWorkspace() ? (opts.root?.() ?? "/") : "");

  // ---- Configuration: the two config files, each with a read-only preview and its actions

  const folderFile = () => opts.configFile?.() || ".zenmd.yml";
  // A file that does not exist is greyed out when this build cannot create it (no editor): there is nothing to show.
  const configItems = async (): Promise<SettingsItem[]> => {
    const canCreate = hasFeature(api, "editor");
    const exists = async (which: "global" | "workspace") => {
      try { return (await api.source(`zen:config/${which}`)) !== ""; } catch { return false; }
    };
    const open = opts.hasWorkspace();
    const [g, w] = await Promise.all([exists("global"), open ? exists("workspace") : Promise.resolve(false)]);
    const missing = (e: boolean) => (e ? {} : { note: canCreate ? undefined : "Not created yet", disabled: !canCreate });
    return [
      { id: "global", label: "config.yml", note: "All folders", ...missing(g) },
      { id: "workspace", label: folderFile() === ".zenmd.yml" ? ".zenmd.yml" : `${folderFile()} (old name)`, note: "This folder", ...(open ? missing(w) : { disabled: true }) },
    ].map((i) => ({ ...i, note: i.note ?? (i.id === "global" ? "All folders" : "This folder") }));
  };
  function renderConfig(item: SettingsItem, right: HTMLElement): () => void {
    const which = item.id as "global" | "workspace";
    const name = which === "workspace" ? folderFile() : item.label;
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
      // Opening and creating need the editor; showing the file in Finder needs nav (its service).
      const editor = hasFeature(api, "editor");
      actions.replaceChildren(
        ...(!editor ? [] : exists
          ? [button("Open in Editor", () => run(() => api.openConfig(which), `${name} opened in the editor.`, `Could not open ${name}`))]
          : [button("Create", () => run(() => api.openConfig(which), `${name} created and opened in the editor.`, `Could not create ${name}`))]),
        ...(exists && hasFeature(api, "nav") ? [button("Reveal in Finder", () => run(() => api.revealInFinder(`zen:config/${which}`), "", `Could not show ${name}`))] : []),
      );
    }
    void load();
    return () => { gone = true; };
  }
  const config: SettingsSection = { id: "config", label: "Configuration", items: configItems, render: renderConfig };

  // ---- Reader: the markdown syntaxes are the items, like Plugins and Layouts (docs/UI.md, UI consistency),
  // under a small "Markdown" heading. CommonMark and GFM come first and are the default (always on); each dialect of the build is on or off for this folder
  // (saved in its .zenmd.yml). The right column has the selected one's switch, summary, sample and preview.

  let dialects: DialectInfo[] = [];
  let dialectFailure = "";
  /** The selected item, kept while the list is read again (after a switch). */
  let pickedSyntax = "";
  const BASE_SYNTAX = "commonmark";
  const reader: SettingsSection = {
    id: "reader", label: "Reader", heading: "Markdown",
    items: async () => {
      dialectFailure = "";
      try {
        dialects = await (api.dialects?.() ?? Promise.resolve([]));
      } catch (err) {
        dialects = [];
        dialectFailure = `Could not read the markdown syntaxes: ${message(err)}`;
        return []; // the failure stands in the left column (empty)
      }
      return [
        { id: BASE_SYNTAX, label: "CommonMark and GFM", note: "Default" },
        ...dialects.map((d) => ({ id: d.id, label: d.name, note: d.on ? "On" : "Off" })),
      ];
    },
    empty: () => dialectFailure,
    current: () => pickedSyntax || BASE_SYNTAX, // never opens on an empty right column
    onPick: (it) => { pickedSyntax = it.id; },
    render(item, right) {
      if (item.id === BASE_SYNTAX) {
        right.append(
          el("p", "zen-addon-summary", "Every folder reads CommonMark and GitHub Flavored Markdown: tables, task lists, strikethrough, autolinks and footnotes."),
          el("p", "zen-settings-hint", "Always on; the dialects below add other syntaxes for the same plugins."),
        );
        return;
      }
      const d = dialects.find((x) => x.id === item.id);
      if (!d) return;
      let gone = false;
      const open = opts.hasWorkspace();
      const editable = hasFeature(api, "editor");
      const reason = !open ? "Open a folder to choose its markdown syntaxes." : !editable ? "This app is read-only, so the folder's settings cannot be changed." : "";
      const status = el("p", "zen-settings-status");
      status.setAttribute("role", "status");
      const label = el("label", "zen-settings-check");
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
      label.append(box, document.createTextNode(` Use ${d.name} in this folder`));
      right.append(
        label, status,
        el("p", "zen-addon-summary zen-dialect-summary", d.summary),
        el("p", "zen-settings-hint", reason || "Saved in this folder's .zenmd.yml, so the folder renders the same everywhere."),
      );
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
      return () => { gone = true; };
    },
  };

  // ---- Layouts, Plugins, Templates

  const plugins = createPluginsSection({ api });
  const layouts = createLayoutsSection({ api });
  const templates = createTemplatesSection({ api, newsite: hasFeature(api, "newsite") });

  // ---- Export: the website (its form, in place) and the PDF (the preview, and the last file)

  const website = createBuildForm({
    api: opts.build ? (opts.build as BuildApi) : (api as unknown as BuildApi),
    doc: document,
    root,
    onOpenPage: (p) => opts.onOpenPage?.(p),
    unsaved: opts.unsaved,
  });
  const withPdf = hasFeature(api, "print");
  const pdf = createPdfForm({
    api: opts.pdf ? (opts.pdf as PdfApi) : (api as unknown as PdfApi),
    doc: document,
    root,
    currentPage: () => opts.currentPage?.() ?? "",
    unsaved: opts.unsaved,
  });
  const exportSection: SettingsSection = {
    id: "export", label: "Export",
    items: () => [{ id: "website", label: "Website" }, ...(withPdf ? [{ id: "pdf", label: "PDF" }] : [])],
    render(item, right) {
      if (item.id === "pdf") {
        right.append(pdf.el);
        void pdf.refresh();
        return;
      }
      right.append(website.el);
      void website.refresh();
    },
    // The page is shown again: the folder's options may have changed (a build that runs, or a result on show, stays).
    show: () => {
      if (website.el.isConnected) void website.refresh(true);
      if (pdf.el.isConnected) void pdf.refresh(true);
    },
    destroy: () => { website.destroy(); pdf.destroy(); },
  };

  frame = createSettingsFrame([
    // Configuration first. The config files are shown with the reader (it reads them) or the editor (it opens them).
    ...(hasFeature(api, "reader") || hasFeature(api, "editor") ? [config] : []),
    appearanceSection(doc, opts.appearanceExtra),
    // Accessibility is in every build; its Reading item is the reader's.
    accessibilitySection(doc, { reader: hasFeature(api, "reader"), setZoom: opts.setZoom }),
    // The sections of a feature the build leaves out are not there at all (kit/shell-features): Markdown, Templates,
    // Layouts and Plugins belong to the reader (Templates also to newsite, and lists none without it), Export to
    // export, Developer to developer.
    ...(hasFeature(api, "reader") || hasFeature(api, "newsite") ? [
      ...(hasFeature(api, "reader") ? [reader] : []),
      { id: "templates", label: "Templates", ...templates },
      ...(hasFeature(api, "reader") ? [
        { id: "layouts", label: "Layouts", ...layouts },
        { id: "plugins", label: "Plugins", ...plugins },
      ] : []),
    ] : []),
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
