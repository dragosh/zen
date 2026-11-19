# app/export

| | |
|---|---|
| Status | specified |
| Target version | 0.6.0 |
| Branch | `feature/app-export` |
| Wave | 20 |
| Depends on | ui/static-build, ui/print, app/menu (folder-only items, zenmark-org/zenmd#6), cli/commands |
| Owns | `apps/zenmark/{menu.go,menu_test.go,menustate.go,menustate_test.go,print.go,exportpdf.go,exportpdf_test.go,headless.go,headless_test.go}`, `apps/zenmark/frontend/src/shell/{buildsite.ts,buildsite.test.ts,settings.ts,settings.test.ts}`, `apps/zenmark/frontend/src/api/{index,wails,fake}.ts` + tests, `kit/desktop/{pdf.go,pdfnative_darwin.go,pdfnative_other.go,pdf_test.go}`, `kit/ui/src/print/{preview.ts,preview.test.ts}`, `kit/ui/src/host.ts`, `internal/cli/{cli.go,export.go,cli_test.go,export_test.go}`, `main.go` |

## Purpose

One word for getting your folder out of Zenmark: **Export**. Export › Website…
writes the static site (what Build Website does today); Export › PDF… writes the
whole folder as one PDF file (what Print Site does, without the print panel).
The same two targets are in Settings and on the command line
(`zenmd export site`, `zenmd export pdf`). Tracks zenmark-org/zenmd#5.

## Behaviour

### Menu
1. File has an **Export** submenu with **Website…** (⇧⌘B) and **PDF…** (⇧⌘P), in place of Build Website… and Print Site…. Print Page… (⌘P) stays in File: printing one page is not an export.
2. Both Export items (and the Export submenu entry itself) are greyed out while no folder is open, like the other folder-only items (app/menu behaviour 8).
3. Export › Website… opens today's Build Website dialog, titled **Export Website**, with its button **Export** (was Build). Options, progress and results are unchanged.
4. Export › PDF… opens today's Print Preview window for the folder. Its primary button is **Export PDF…**; **Print…** stays as a secondary button for paper.

### PDF in the app
5. Export PDF… shows the macOS save panel (sheet on the preview window) with the name `<folder name>.pdf` and the last folder used for a PDF export (else Documents). Cancel does nothing.
6. After Save, the preview's document is written to that file through macOS printing as a save job (no print panel): portrait, A4 or Letter from the system's default paper, half-inch margins, fitted to the page width, the same `@media print` rules as Print. The preview shows "Exporting…" and then "Saved <file name>" with **Show in Finder**, or the error.
7. The export waits until the preview is ready (ui/print behaviour 3: plugins mounted, images and fonts loaded, 5 s at most), as Print does.

### Settings
8. Settings › Website becomes Settings › **Export**, with two parts: **Website** (today's options, Export… button and last result) and **PDF** (an Export PDF… button that opens the preview as in 4, and the last export's file with Show in Finder). With no folder open, both buttons are disabled and the section says "Open a folder to export it."

### Command line
9. `zenmd export site [folder] [options]` does exactly what `zenmd build` does today, with the same options (`-o/--out`, `--portable`, `--hosted`, `--drafts`, `--clean`, `--no-print`) and output. `zenmd build` keeps working as an alias (no notice). `zenmd help` lists `export site` and `export pdf`, and `build` as "same as export site".
10. `zenmd export pdf [folder] [-o file.pdf]` writes the whole folder as one PDF. Default output: `<folder>/<folder name>.pdf`; a relative `-o` is relative to the current folder; a missing `.pdf` suffix is added. It prints `Wrote <path> (<n> pages)` on stdout and exits 0.
11. `zenmd export pdf` works without the app's window: it starts zenmd as a background app (no window shown, no Dock icon, no menu bar), lays the print document out in a hidden web view, waits for it to be ready (as 7), saves it as in 6, and quits. A second run while the app is open works too (it does not talk to the running app).
12. Errors exit non-zero with one line on stderr: `zenmd export` with no target or an unknown target (usage, exit 2), a folder that does not exist or has no pages, an output folder that does not exist or cannot be written, no window server (e.g. over SSH without a login session: "export pdf needs a logged-in macOS session"), a non-macOS system ("export pdf needs macOS"), and a page layout that did not finish in 30 s.
13. Blocks that were not ready in time are listed on stderr as warnings (as ui/print logs them); the PDF is still written.

## Architecture

- **Menu** (`apps/zenmark/menu.go`): the File menu gets a submenu item `Export` (the `fillMenu` recursion from ui/find already supports submenus) with `labelExportWebsite = "Website…"` and `labelExportPDF = "PDF…"`; they reuse the existing actions `BuildWebsite` (event `menu:build-website`) and `PrintSite`. Event names stay; only labels change. `menustate.go`: `folderOnly` covers both new labels and the `Export` submenu entry; the old labels go.
- **PDF writer** (`kit/desktop`): `SavePDF` runs an `NSPrintOperation` for the window's `WKWebView` with the same `NSPrintInfo` as `zenPrintWindow` (factor the shared setup into one C helper), `jobDisposition = NSPrintSaveJob`, `NSPrintJobSavingURL` = the file, `showsPrintPanel = NO`, `showsProgressPanel = NO`, run with `runOperation` on the main thread (`application.InvokeSync`). It returns the page count (from the operation's `context` / `printInfo` after the run; if not available, count `/Type /Page` in the written file). `pdfnative_other.go` returns `ErrPDFUnsupported`.
- **In-app** (`apps/zenmark/exportpdf.go`): `Zen.ExportPDF(ctx)` is called by the preview window; it shows the save panel (`kit/desktop` dialogs, default name and last folder from the app's settings store), then `desktop.SavePDF` on the calling window (`PrintFromContext` pattern). `Zen.RevealPath(path)` already exists as reveal for files in the workspace; the PDF can be outside it, so `ExportPDF` returns the path and the preview calls a new `RevealExport()` that reveals only the last exported file.
- **Headless** (`apps/zenmark/headless.go`): `ExportPDF(cli.ExportPDF, stdout, stderr) error` opens the folder in a session, starts a Wails application with `Mac.ActivationPolicy = Accessory`, no menu, one hidden window at `/?window=print&export=1&page=`, and a service method `PrintReady(late []string)` that the preview calls when `whenReady` resolves in export mode. Go then calls `desktop.SavePDF`, writes the summary, and quits the app; a 30 s timer fails the export. `kit/desktop.Run` is not used (it builds the full shell); the headless app is built directly with `application.New`.
- **Frontend**: `kit/ui/src/print/preview.ts` gains an `exportPdf` host method (optional, like `printWindow`) and an export mode (`?export=1`): no toolbar, no buttons, calls `printReady(late)` once ready. `buildsite.ts` and `settings.ts` rename the dialog and section.
- **CLI** (`internal/cli`): `export` with `site` delegates to `runBuild`; with `pdf` parses its options into `ExportPDF{Dir, Out string}` and calls a new `PDFExporter` function passed to `cli.Run` from `main.go` (`app.ExportPDF`).

## Interfaces

```go
// kit/desktop
var ErrPDFUnsupported = errors.New("PDF export needs macOS")
// SavePDF writes w's web view to path as a PDF through macOS printing, without a panel. It returns the page count.
func SavePDF(w application.Window, path, title string) (pages int, err error)

// internal/cli
type ExportPDF struct {
	Dir string // absolute folder
	Out string // absolute output file, "" = <Dir>/<name>.pdf
}
type PDFExporter func(ExportPDF, io.Writer, io.Writer) error
func Run(args []string, cwd string, stdout, stderr io.Writer, launch Launcher, build Builder, initer Initer, pdf PDFExporter) int

// apps/zenmark
func ExportPDF(e cli.ExportPDF, stdout, stderr io.Writer) error            // headless, for the CLI
func (z *Zen) ExportPDF(ctx context.Context, title string) (string, error) // save panel + SavePDF; "" when cancelled
func (z *Zen) RevealExport() error                                          // Finder on the last exported file
func (h *headlessService) PrintReady(late []string)                          // export mode only
```

```ts
// kit/ui/src/host.ts (KitHost, optional members)
exportPdf?(title: string): Promise<string>;   // path, "" when cancelled
revealExport?(): Promise<void>;
printReady?(late: string[]): Promise<void>;   // export mode
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | File › Export › Website… ⇧⌘B and PDF… ⇧⌘P; no Build Website… / Print Site… left; Print Page… ⌘P stays | `apps/zenmark/menu_test.go: TestExportSubmenu` |
| 2 | Export items and the submenu are folder-only | `apps/zenmark/menustate_test.go: TestExportFolderOnly` |
| 3 | Dialog titled Export Website, button Export | `apps/zenmark/frontend/src/shell/buildsite.test.ts: "the dialog is Export Website"` |
| 4 | Preview has Export PDF… (primary) and Print… | `kit/ui/src/print/preview.test.ts: "Export PDF is the primary action"` |
| 5 | Cancelled save panel: nothing written, no message | `preview.test.ts: "cancelled export shows nothing"`; `apps/zenmark/exportpdf_test.go: TestExportPDFCancelled` |
| 6 | Saved: "Saved <name>" and Show in Finder; error shown in the preview | `preview.test.ts: "export reports the file or the error"` |
| 6 | SavePDF writes a PDF with pages (darwin, skipped without a window server) | `kit/desktop/pdf_test.go: TestSavePDF` |
| 7 | Export waits for ready like Print | `preview.test.ts: "export waits until the paper is ready"` |
| 8 | Settings › Export with Website and PDF parts; disabled with no folder | `apps/zenmark/frontend/src/shell/settings.test.ts: "Export section"` |
| 9 | `export site` = `build` (same Build value for the same args); `build` still works; help lists both | `internal/cli/export_test.go: TestExportSiteIsBuild`, `TestHelpListsExport` |
| 10 | `export pdf` output path rules (default, relative -o, .pdf added) | `export_test.go: TestExportPDFOut` |
| 11 | Headless export: hidden accessory app, export mode URL, PrintReady triggers save and quit | `apps/zenmark/headless_test.go: TestHeadlessPlan` (pure: the window spec and options), `TestPrintReadySaves` (fake saver) |
| 11 | Export mode preview: no buttons, calls printReady once with the late blocks | `preview.test.ts: "export mode calls printReady"` |
| 12 | Usage errors exit 2; missing folder, no pages, unwritable output, non-macOS, timeout exit 1 with one line | `export_test.go: TestExportErrors`; `headless_test.go: TestHeadlessTimeout` |
| 13 | Late blocks become stderr warnings | `headless_test.go: TestLateBlocksWarn` |

Edge cases: a folder name with spaces or `/`-like characters in the default PDF name (sanitised: no path separators); `-o` pointing at a folder (error, not a file inside it); an output file that exists (overwritten, like `build` overwrites); an empty folder; `export pdf` given a single `.md` file (error: export takes a folder).

## UI

- Menu: File › Export ▸ Website… ⇧⌘B, PDF… ⇧⌘P (macOS submenu), greyed out without a folder.
- Preview toolbar: Close, Print…, **Export PDF…** (primary). States: idle; preparing (buttons disabled until ready, as Print); exporting ("Exporting…", buttons disabled); saved (status line "Saved Guide.pdf" + Show in Finder link); error (status line in `--lint-error` text, role="alert"). The status line is `aria-live="polite"`.
- Settings › Export: two sub-headings, Website and PDF, same layout as the other sections; tokens from docs/UI.md only.
- Keyboard: the save panel is native; Esc closes the preview as before.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| The PDF path is written outside the workspace | It is chosen by the user in the native save panel (app) or given on the command line by the user; the frontend never passes a path to Go, only a title | `exportpdf_test.go: TestExportPathFromPanelOnly` |
| Reveal opens an arbitrary path | `RevealExport` takes no argument and reveals only the file the last export wrote | `exportpdf_test.go: TestRevealExportOnlyLast` |
| The headless window runs plugin scripts | It loads the same print document with the same CSP and sandboxing as Print Site; no new origin or bridge methods except `PrintReady` | `headless_test.go: TestHeadlessUsesPrintCSP` |
| Default file name from the folder name | Path separators and control characters replaced; never `..` | `export_test.go: TestPDFNameSanitised` |

## Notes

- **Out of scope:** PDF on Linux/Windows; choosing paper size and margins (the save job uses the system defaults, Print… is there for control); a PDF of one page from the CLI; exporting other formats (EPUB, DOCX); a progress bar in the CLI.
- **Integration (done by the lead at merge):**
  - `specs/ui/static-build/spec.md` and `specs/ui/print/spec.md`: a line under Behaviour pointing to app/export for the renamed menu items; `specs/app/menu/spec.md`: the Export submenu.
  - `docs/PLAN.md` Service API: `exportPdf`, `revealExport`.
  - User guide (`examples/demo`): rename Build Website / Print Site to Export in the pages that mention them, then update the golden hash of `TestDemoBuildUnchanged`.
  - `docs/BUILD.md` (or wherever `zenmd build` is documented): `zenmd export site|pdf`.
  - `README.md` command list.
- **App Store / sandbox:** the in-app export writes only where the save panel allows (user-selected file, already granted by the panel). The headless CLI is not part of the App Store build (the CLI is installed separately), so no new entitlement.
- **Changelog:** `Added: Export — File › Export › Website… (⇧⌘B) and PDF… (⇧⌘P), Settings › Export, and zenmd export site|pdf; the PDF is saved directly, without the print panel, and zenmd export pdf works from the command line (#5).`

## Prompt

> Implement `specs/app/export/spec.md` following the rules in `specs/README.md`.
> Context: Zenmark is a Wails v3 macOS app (`apps/zenmark`) on the zenmd engine; the
> shared desktop code is in `kit/desktop` (Go) and `kit/ui` (TS). Today File ›
> Build Website… (`buildsite.ts`, `Zen.BuildWebsite`) and File › Print Site…
> (`Zen.PrintSite` opens the Print Preview window, `kit/ui/src/print/preview.ts`,
> which prints with `desktop.PrintFromContext` → `zenPrintWindow` in
> `kit/desktop/printnative_darwin.go`). The menu supports submenus (`fillMenu`) and
> folder-only items (`menustate.go`). The CLI is `internal/cli/cli.go` (`runBuild`),
> called from `main.go`. Build the PDF writer first (`SavePDF`, sharing the
> NSPrintInfo setup with `zenPrintWindow`), then the in-app export, then the
> headless CLI export, then the renames in the menu, dialog and Settings.
> `TestSavePDF` must skip (not fail) when no window server is available.
> Do not launch the app; you may run `go run . export pdf examples/demo -o <scratchpad>/demo.pdf`
> once at the end only if a window server is available, and report the result.
> Done when: every test in the table passes, `go vet ./... && go test ./...` and
> the frontend checks pass, and the report lists the Integration lines.
