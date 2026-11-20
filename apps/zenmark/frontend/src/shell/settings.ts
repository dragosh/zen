// The Settings page of Zenmark: the kit's frame (kit/ui shell/settings.ts, with Appearance) and
// zen's sections: Configuration files (the two config files open in zen's own editor window),
// Plugins (shell/pluginhelp.ts), Layouts, Markdown (the dialects the folder uses), Export (the website and the PDF) and This build (kit/addons, kit/assembly,
// read-only). All text through textContent.
import { appearanceSection, button, buildSection, createSettingsFrame, developerSection, el, hasFeature, type SettingsSection, type SettingsView } from "@zenmd/kit";
import type { Api } from "../api";
import { createWebsiteSection, type BuildApi } from "./buildsite";
import { createLayoutsSection } from "./layouthelp";
import { createPluginsSection } from "./pluginhelp";

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

  // ---- Markdown (engine/dialects): one checkbox per dialect of the build, saved in the folder's .zen.yml

  const markdownEl = el("div", "zen-addons zen-markdown");
  const markdownStatus = el("p", "zen-settings-status");
  markdownStatus.setAttribute("role", "status");
  const markdownNote = el("p", "zen-build-list", "Saved in this folder's .zen.yml, so the folder renders the same everywhere.");
  let markdownSeq = 0;
  async function showMarkdown(): Promise<void> {
    const mine = ++markdownSeq;
    markdownStatus.textContent = "";
    let list: Awaited<ReturnType<Api["dialects"]>> = [];
    try {
      list = await (api.dialects?.() ?? Promise.resolve([]));
    } catch (err) {
      markdownStatus.textContent = `Could not read the markdown syntaxes: ${message(err)}`;
    }
    if (mine !== markdownSeq) return;
    if (list.length === 0) {
      markdownEl.replaceChildren(el("p", "zen-addons-empty", "This build has no markdown dialects."), markdownStatus);
      return;
    }
    const open = opts.hasWorkspace();
    const editable = hasFeature(api, "editor");
    const reason = !open ? "Open a folder to choose its markdown syntaxes." : !editable ? "This app is read-only, so the folder's settings cannot be changed." : "";
    const rows = list.map((d) => {
      const row = el("label", "zen-addon zen-dialect");
      row.dataset.dialect = d.id;
      const box = doc().createElement("input");
      box.type = "checkbox";
      box.dataset.dialect = d.id;
      box.checked = d.on;
      box.disabled = reason !== "";
      box.addEventListener("change", () => {
        const want = box.checked;
        markdownStatus.textContent = "";
        api.setDialect(d.id, want).catch((err) => {
          box.checked = !want;
          markdownStatus.textContent = `Could not save ${d.name}: ${message(err)}`;
        });
      });
      const head = el("div", "zen-dialect-head");
      head.append(box, el("span", "zen-addon-name zen-dialect-name", d.name));
      row.append(head, el("p", "zen-addon-summary zen-dialect-summary", d.summary));
      return row;
    });
    markdownEl.replaceChildren(...rows, ...(reason ? [el("p", "zen-build-list", reason)] : []), markdownNote, markdownStatus);
  }

  // ---- Plugins, Layouts, Export

  const plugins = createPluginsSection({ api });
  const layouts = createLayoutsSection({ api });
  const website = createWebsiteSection({
    api: opts.build ?? (api as unknown as Partial<BuildApi>),
    root: () => (opts.hasWorkspace() ? (opts.root?.() ?? "/") : ""),
    openBuild: opts.openBuild,
  });

  // Export: the website (its options and last build) and the PDF (the preview, and the last file).
  const exportEl = el("div", "zen-settings-export");
  const webHead = el("h3", "zen-settings-export-heading", "Website");
  const pdfHead = el("h3", "zen-settings-export-heading", "PDF");
  const pdfButton = button("Export PDF…", () => void api.printSite().catch((err) => { pdfStatus.textContent = message(err); }), { className: "zen-settings-export-pdf primary" });
  const pdfLast = el("p", "zen-settings-export-last");
  const pdfStatus = el("p", "zen-settings-status");
  pdfStatus.setAttribute("role", "status");
  const showPdf = (): void => {
    const open = opts.hasWorkspace();
    pdfButton.disabled = !open;
    pdfStatus.textContent = "";
    pdfLast.replaceChildren();
    const last = open ? lastPdfExport() : undefined;
    if (last) {
      pdfLast.append(`Last PDF: ${last.path.split(/[\\/]/).pop()}, ${new Date(last.at).toLocaleTimeString()}. `);
      pdfLast.append(button("Show in Finder", () => void api.revealExport().catch((err) => { pdfStatus.textContent = message(err); }), { className: "zen-buildsite-link" }));
    } else if (open) {
      pdfLast.textContent = "No PDF yet.";
    }
  };
  // The PDF is the print document: its part is there only with print as well as export.
  const withPdf = hasFeature(api, "print");
  exportEl.append(webHead, website.el, ...(withPdf ? [pdfHead, pdfLast, pdfButton, pdfStatus] : []));

  frame = createSettingsFrame([
    appearanceSection(doc),
    // Nothing opens the config files in an app without the editor: the section is not there.
    ...(hasFeature(api, "editor") ? [files] : []),
    // The sections of a feature the build leaves out are not there at all (kit/shell-features): Plugins, Layouts and
    // Markdown belong to the reader, Export to export, Developer to developer. This build is always there.
    ...(hasFeature(api, "reader") ? [
      { id: "plugins", label: "Plugins", el: plugins.el, show: () => void plugins.refresh(), destroy: () => plugins.destroy() },
      { id: "layouts", label: "Layouts", el: layouts.el, show: () => void layouts.refresh(), destroy: () => layouts.destroy() },
      { id: "markdown", label: "Markdown", el: markdownEl, show: () => void showMarkdown(), destroy: () => { markdownSeq++; } },
    ] : []),
    ...(hasFeature(api, "export") ? [{ id: "export", label: "Export", el: exportEl, show: () => { void website.refresh(); if (withPdf) showPdf(); } }] : []),
    buildSection(api),
    ...(hasFeature(api, "developer") ? [developerSection()] : []),
  ]);
  return frame;
}
