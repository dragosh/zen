# ui/static-build

| | |
|---|---|
| Status | specified |
| Target version | 0.2.0 |
| Branch | `feature/ui-static-build` |
| Wave | 12 |
| Depends on | core/static-build, app/menu, ui/print (`print.html`, optional) |
| Owns | `internal/app/buildui.go` + `buildui_test.go` (new), `frontend/src/shell/buildsite.ts` + `buildsite.test.ts` (new), the Build Website item in Settings (`frontend/src/shell/settings.ts`, a "Website" section), `examples/demo/build/index.md` (an "In the app" part) |

## Purpose

Build the open folder as a static website from inside zen, with the same
result as `zen build`: **File › Build Website…** (⇧⌘B), or the Website
section in Settings. It shows the options, progress and the result, and
offers to open the site or show it in Finder.

## Behaviour

### Opening it
1. **File › Build Website…** (⇧⌘B) opens the **Build Website** dialog in the main window. With no folder open, the menu item is disabled.
2. **Settings › Website** shows the same options and a **Build…** button that opens the dialog, plus the last build's result (time, page count, output folder) if there is one.

### The dialog
3. **Options**, pre-filled from the folder's `build:` config, then from what was used last time for this folder (kept per folder in zen's settings data, not in `.zen.yml`):
   - **Output folder:** the path shown relative to the folder when it's inside, `_site` by default, with **Choose…** (the native folder panel, New Folder allowed);
   - **Opens from:** *Disk and web servers* (portable, the default) or *A web server only* (`--hosted`);
   - **Include drafts**;
   - **Start clean** (empties the output first; the same marker rule as `--clean`);
   - **Also write print.html** (on), when ui/print is merged.
4. **Save as folder defaults** (a link) writes these options into the folder's `.zen.yml` `build:` section. It shows the change first ("Adds to .zen.yml: …") and writes only on confirm, keeping the file's other content and comments. When `.zen.yml` is open in an editor window with unsaved changes, it refuses and says so.
5. **Build** runs the build. The dialog shows **Building… 12 / 34 pages** with a progress bar, then the result. **Cancel** while building stops after the current page and leaves what was written, with a note. The output folder stays usable for the next build: the marker is written first.
6. **Result:**
   - "Built 34 pages and 19 files into _site", with the time it took;
   - warnings, if any, listed under it. Each starts with its page, and clicking one opens that page in the main window and closes the dialog;
   - actions **Open in Browser** (opens `index.html` with the default browser), **Show in Finder**, **Build Again** and **Done**.
7. **Errors** (output folder refused, a wrong `build:` type in `.zen.yml`, missing runtime) show as one message with the cause and leave the options open to change. They are the same texts as the CLI's.
8. Building uses the same code and the same read-only trust as `zen build`: plugins and layouts not allowed for the folder are left out with the same warning. It works from the files on disk; unsaved editor text is not included, and the dialog says so when an editor has unsaved changes ("2 pages have unsaved changes; the website uses the saved files").
9. Only one build runs at a time; ⇧⌘B while one runs brings the dialog to the front.

### Menu (with core/init and ui/print merged)
10. File: New Site from Template… ⇧⌘N · ― · New Page… ⌘N · Open Folder… ⌘O · ― · **Build Website…** ⇧⌘B · Print Page… ⌘P · Print Site… ⇧⌘P.

## Architecture

- `internal/app/buildui.go`:
  - the services `BuildOptions() BuildOptions`, which reads `build:` plus the remembered options for the open folder;
  - `BuildWebsite(o BuildOptions) (BuildResult, error)`, which runs `buildSite` with a progress callback and emits `build:progress {done, total}`;
  - `CancelBuild()`;
  - `SaveBuildDefaults(o BuildOptions) (preview string, err error)`, in two steps: preview, then confirm with the same options;
  - `PickBuildFolder()`, `OpenBuiltSite(dir)` (NSWorkspace open of `index.html`; a local file, so no external-link prompt) and `RevealBuiltSite(dir)`.
- `buildSite` gains an optional progress/cancel hook (`site.Options.Progress func(done, total int) bool`, false = stop). That is a small change in `internal/core/site` and `internal/app/build.go`, listed under Integration because core/static-build owns those files.
- The remembered options are stored in `<config dir>/data/<folderKey>/build.json`, next to plugin data. They are written atomically.
- `frontend/src/shell/buildsite.ts` holds the dialog (the pattern of `newpage.ts`), and `settings.ts` gets the Website section.

## Interfaces

```go
// internal/app
type BuildOptions struct {
	Out      string // absolute; the dialog shows it relative when inside the folder
	Portable bool
	Drafts   bool
	Clean    bool
	Print    bool
}
type BuildResult struct {
	Out      string
	Pages    int
	Files    int
	Warnings []BuildWarning
	Millis   int64
	Cancelled bool
}
type BuildWarning struct{ Path, Message string } // Path "" for site-wide
func (z *Zen) BuildOptions() (BuildOptions, error)
func (z *Zen) BuildWebsite(o BuildOptions) (BuildResult, error)
func (z *Zen) CancelBuild()
func (z *Zen) SaveBuildDefaults(o BuildOptions, confirm bool) (string, error)
func (z *Zen) PickBuildFolder() (string, error)
func (z *Zen) OpenBuiltSite(out string) error
func (z *Zen) RevealBuiltSite(out string) error
const EventBuildProgress = "build:progress" // {done, total}
const EventBuildWebsite = "menu:build-website"

// internal/core/site (Integration)
type Options struct { /* … */ Progress func(done, total int) bool }
```

```ts
// frontend/src/shell/buildsite.ts
export function openBuildSite(opts: { api: Api; doc: Document; onOpenPage(path: string): void }): BuildSiteDialog;
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1, 9 | menu event opens the dialog; disabled without a folder; one build at a time | `frontend/src/shell/buildsite.test.ts: "menu"`; `internal/app/buildui_test.go: TestOneBuildAtATime` |
| 2 | Settings › Website shows options and the last result | `frontend/src/shell/settings.test.ts: "website section"` |
| 3 | options from config, then remembered per folder; remembered options saved atomically | `buildui_test.go: TestBuildOptions` |
| 4 | Save as folder defaults: preview, confirm, keeps comments, refuses with a dirty editor | `buildui_test.go: TestSaveBuildDefaults` |
| 5 | progress events, cancel stops after the current page | `buildui_test.go: TestBuildProgressAndCancel` |
| 6 | the result lists warnings by page; clicking opens the page; Open in Browser / Show in Finder call the services | `buildsite.test.ts: "result"` |
| 7 | errors with the CLI's texts, options stay editable | `buildsite.test.ts: "errors"`; `buildui_test.go: TestBuildErrors` |
| 8 | same output as `zen build` for the same options; trust read-only; unsaved-editor note | `buildui_test.go: TestSameAsCLI`, `TestUnsavedNote` |
| 10 | menu entries (Integration) | `internal/app/menu_test.go` (lead) |

Edge cases to cover:
- an output folder outside the folder (allowed, shown as an absolute path);
- an output folder that is the open folder itself (refused, with the CLI's message);
- a folder switch while building, which cancels the build;
- `.zen.yml` missing, so Save creates it with `version: 1` and `build:`;
- `.zen.yml` with a `build:` section already, so only the changed keys are updated.

## UI

- **The dialog**, a modal sheet like New Page, 520px wide:
  1. the title "Build Website", with the folder name muted;
  2. the options as a form. The output folder is a text field with Choose…; Opens from is a radio group with a one-line hint each; then the checkboxes;
  3. the footer: Save as folder defaults (a link, left), Cancel, **Build** (primary).
- **While building:** the form is disabled, a progress bar runs with "Building… n / total pages", and Cancel stays enabled.
- **The result:** a summary line, the warnings list (scrolls past 8 rows; each row is a button with the page title and the message), then Open in Browser (primary), Show in Finder, Build Again and Done.
- **Keyboard and accessibility:**
  - Enter builds and Esc cancels (or closes after the result);
  - focus starts on Build;
  - progress is a `progressbar` with `aria-valuenow`;
  - the result is announced (`role="status"`) and warnings are a list.
- Tokens from docs/UI.md; black and white.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Building writes or deletes in the wrong place | The same `CheckOut` and marker rules as the CLI; a folder chosen by panel still goes through them | `TestBuildErrors` |
| Save as folder defaults damages `.zen.yml` | Preview first; a YAML node edit keeps comments and other keys; refused when an editor holds unsaved changes; atomic write | `TestSaveBuildDefaults` |
| Open in Browser opens something other than the built site | It opens only `<out>/index.html` from the last build of this folder, checked to exist and to have the marker | `buildui_test.go: TestOpenBuiltSite` |
| Plugins not allowed reach the website | Read-only trust, the same as the CLI | `TestSameAsCLI` |

## Notes

- **Out of scope:**
  - publishing the site (GitHub Pages, upload);
  - a live preview server;
  - building on save;
  - building another folder than the open one (use the CLI).
- **Integration (done by the lead at merge):**
  - `internal/core/site`: `Options.Progress` called after each page (false stops; the report notes "cancelled"); `internal/app/build.go`: pass it through `buildSite`.
  - `internal/app/menu.go`: Build Website… ⇧⌘B (`menuActions.BuildWebsite`, disabled without a folder) in the File group with the Print items; `menu_test.go`.
  - The api: add `buildOptions`, `buildWebsite`, `cancelBuild`, `saveBuildDefaults`, `pickBuildFolder`, `openBuiltSite`, `revealBuiltSite`, `build:progress` and `menu:build-website` in `index.ts`, `wails.ts` and `fake.ts`.
  - `frontend/src/shell/shell.ts`: listen for `menu:build-website` and open the dialog.
  - Docs: docs/BUILD.md "In the app"; the changelog.
- **App Store / sandbox:** the default output inside the open folder is covered by its user-selected access, and other folders come from the panel.
- **Changelog:** `Added: File › Build Website… (⇧⌘B) and Settings › Website — build the open folder as a static website from the app, with progress, warnings by page, and Open in Browser.`

## Prompt

> Implement `specs/ui/static-build/spec.md` following the rules in
> `specs/README.md`. Context:
> - `internal/app/build.go` has `buildSite(env, cli.Build, stdout, stderr)`, the CLI's build.
> - `internal/core/site` is the builder.
> - New Page (`frontend/src/shell/newpage.ts`) is the model for the dialog.
> - Settings sections are in `frontend/src/shell/settings.ts`.
> - Plugin data lives under `<config dir>/data/<folderKey>/` (`internal/app/extplugins.go`).
>
> The progress hook in `internal/core/site` and `build.go` is an Integration change. Write it in your branch only if you must for your tests; if you do, keep it minimal and report it, because ui/print also edits `internal/core/site`. Prefer wrapping: count pages through a `site.Input` decorator whose `Render` calls report progress.
>
> Do not edit `menu.go`, the api files or `shell.ts`: give exact diffs. Do not launch the app.
>
> Done when the table's tests pass and `go vet ./internal/... && go test ./internal/...` and `cd frontend && npx tsc --noEmit && npx vitest run` are clean. Report the Integration diffs and what is unverified (the panel, Open in Browser).
