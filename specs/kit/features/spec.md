# kit/features

| | |
|---|---|
| Status | specified |
| Target version | 0.6.0 |
| Branch | `feature/kit-features` |
| Wave | 23 |
| Depends on | kit/assembly, ui/editor-window, ui/sidebar-menu, ui/new-page, app/menu, app/export |
| Owns | `kit/assembly/**`, `apps/zenmark/app-assembly.yml` (the `features` key only), `apps/zenmark/assembly_gen.go` (generated), `apps/zenmark/{app,editor,settings,menu,menustate}.go`, `engine/session/readonly.go` (new) + the write methods it guards (`session.go` Save, `newpage.go`, `nav.go`, `buildui.go` SaveBuildDefaults, EnsureConfig), `internal/cli/cli.go` (version line), `kit/ui/src/host.ts`, `kit/ui/src/page/{index,links}.ts`, `kit/ui/src/nav/{index,entries}.ts`, `kit/ui/src/shell/addons.ts` (This build), `apps/zenmark/frontend/src/shell/{features,shell,settings,buildsite}.ts`, `apps/zenmark/frontend/src/api/{index,wails,fake}.ts` (+ the tests of all of these) |

## Purpose

An app can leave out a **feature**, one of the big pieces of the app itself, in
`app-assembly.yml`. The first one is the **editor**: an app built without it is a
read-only reader. It shows a folder, follows changes made on disk and exports it,
but it never changes a page or the folder's settings. Recipe books, handbooks and
course readers become apps of their own, from the same code.

```yaml
# apps/hashdish/app-assembly.yml
app: hashdish
features: []          # no editor: read-only
addons: []
plugins: [codeblock]
layouts: [article, wide]
templates: []
theme: default
```

Features are not add-ons. An add-on brings a helper program or a permission
(docs/ADDONS.md). A feature is part of the app that is there by default and
can be turned off.

## Behaviour

### The file
1. `app-assembly.yml` gains the key `features`: a list of feature ids, or `all`. **A missing `features` key means `all`**, unlike the other keys (where missing means none), so an existing file never makes an app read-only by accident. `features: []` means none. Unknown ids fail with the known ids listed, like the other keys. The known features in this spec: `editor`.
2. `zenmd version --verbose` prints a `features:` line. Settings › This build lists the features, as the first section, before Add-ons.

### Without `editor` (a read-only app)
3. **Nothing writes into the folder.** The session refuses the writes with `session.ErrReadOnly` ("this app is read-only"), whoever calls them (a button, a script in the page, the CLI binary): `Save`, `CreatePage`, `ReorderNav`, `SetNavHidden`, `SaveBuildDefaults`, and `EnsureConfig` when it would create a file. `OpenEditor` and `OpenConfig` refuse too, and no editor window is ever opened.
4. **Reading is unchanged.** `Page`, `PageInfo`, `Source`, `Preview`, search, the Inspector, Find, print and export all work. Live reload still shows changes made by other apps. Plugin storage (in app data, not the folder) still works.
5. **The UI has no edit actions.** That means:
   - no Edit button and no ⌘E in the reader;
   - layout links marked `data-zen-action="edit"` (the docs layout's "Edit this page") are hidden;
   - the sidebar context menu has no Open in Editor, Hide, Move Up or Move Down, and dragging does not reorder;
   - no File › New Page… (⌘N does nothing).

   Settings has no "Save as folder defaults" (Export) and no buttons that open the config files. Nothing that is left over is just greyed out: it is simply not there.
6. A launch that asks for edit mode (`Startup.mode = "edit"`, e.g. `zenmd open --edit`) opens the page in the reader.
7. Export › Website still writes its output, because exporting is the user's own choice of a target folder. When that folder is inside the open folder (the default `_site`), the output is written, but the build defaults are not saved.

### With `editor` (Zenmark)
8. Everything works as it does today. Zenmark's assembly gets `features: all`.

### Later features
9. The mechanism is general: the next optional pieces (candidates: `newsite`, `export`, `inspector`, `developer`) each get one id in the catalog and their gating in their own spec. The assembly check can then reject combinations that make no sense ("feature newsite needs feature editor").

## Architecture

- `kit/assembly`:
  - `Assembly.Features []string` and `Catalog.Features []string` (`editor`).
  - `Load` treats a missing `features` key as `all`, using the same `all` expansion and strict ids as the other lists.
  - `Catalog.FeatureNeeds map[string][]string` is there for later; it is empty now, and `Check` reads it.
- The generator writes `Features` into `assemblyInfo` (`session.BuildInfo.Features`).
- `engine/session/readonly.go`:
  - `Options.ReadOnly bool` and `var ErrReadOnly = errors.New("this app is read-only")`;
  - a `s.writable() error` guard at the top of each write method listed in behaviour 3.
  - The guard lives in the session, not only in the app, so every caller is covered.
- `apps/zenmark/app.go` and `headless.go`:
  - `ReadOnly: !slices.Contains(assemblyInfo.Features, "editor")`;
  - `OpenEditor` and `OpenConfig` return `ErrReadOnly` first;
  - `openStartupEditor` falls back to the reader;
  - `menu.go` leaves out New Page… when the editor is missing. The item is omitted, not disabled; `menustate.go` must cope with the absent item.
- Frontend:
  - `hasFeature(host, id)` in `kit/ui/src/host.ts`. It is true when `host.build` is absent (the fake, older hosts), so the default is "everything".
  - `features.ts` registers `editorFeature` only with the editor.
  - The page, links, nav, shell (New Page, ⌘E, ⌘N), settings and buildsite read `hasFeature(api, "editor")`.
  - The editor's code stays in the bundle and is gated at run time, as with add-ons.
- `kit/ui/src/shell/addons.ts` (This build) adds the Features section.

## Interfaces

```go
// kit/assembly
type Assembly struct {
	App                                         string
	Features, Addons, Plugins, Layouts, Templates []string
	Theme                                       string
}
// Catalog gains: Features []string; FeatureNeeds map[string][]string

// engine/session
var ErrReadOnly = errors.New("this app is read-only")
// Options.ReadOnly bool
// BuildInfo.Features []string `json:"features"`
```

```ts
// kit/ui/src/host.ts
export type BuildInfo = { app: string; features: string[]; addons: AddonInfo[]; plugins: string[]; layouts: string[]; templates: string[]; theme: string };
/** Whether the app has the feature: true when the host does not say (no build info). */
export function hasFeature(host: Pick<KitHost, "build">, id: string): boolean;
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Missing `features` = all; `[]` = none; unknown id fails with the known list | `kit/assembly/assembly_test.go: TestFeatures` |
| 1 | Generated file carries the features; drift fails | `kit/assembly/cmd/gen/gen_test.go: TestFeaturesGenerated`, `apps/zenmark/assembly_test.go: TestAssemblyGenerated` |
| 2 | Version line | `internal/cli/cli_test.go: TestVersionVerbose` (features line) |
| 2 | Settings › This build lists features | `apps/zenmark/frontend/src/shell/settings.test.ts: "This build section"` |
| 3 | Every write method returns ErrReadOnly and leaves the folder byte-for-byte unchanged | `engine/session/readonly_test.go: TestReadOnlyRefusesWrites` (table over the methods; hashes the folder before and after) |
| 3 | OpenEditor / OpenConfig refuse, no window | `apps/zenmark/readonly_test.go: TestReadOnlyNoEditor` |
| 4 | Reads, preview, export still work read-only | `engine/session/readonly_test.go: TestReadOnlyReads` |
| 5 | Reader: no Edit button, ⌘E ignored, edit links hidden | `kit/ui/src/page/page.test.ts: "read-only: no edit actions"` |
| 5 | Sidebar: no editing items, drag does not reorder | `kit/ui/src/nav/nav.test.ts: "read-only sidebar"` |
| 5 | No New Page in the menu; ⌘N inert | `apps/zenmark/menu_test.go: TestMenuWithoutEditor`; `shell.test.ts: "read-only: no new page"` |
| 5 | Settings without folder-defaults and config buttons | `settings.test.ts: "read-only settings"`, `buildsite.test.ts: "read-only: no save defaults"` |
| 6 | Edit-mode launch opens the reader | `apps/zenmark/readonly_test.go: TestReadOnlyStartupEdit` |
| 7 | Export › Website writes output, saves no defaults | `engine/session/readonly_test.go: TestReadOnlyExport` |
| 8 | With editor: today's suites pass unchanged (`hasFeature` true without build info) | the existing suites; `kit/ui/src/host.test.ts: hasFeature` |

Edge cases:
- `features: all` mixed with ids is an error;
- a duplicate id is an error;
- `ErrReadOnly` reaches the frontend as a plain error message, not a crash;
- an editor window opened from an older state file at startup is refused;
- the fake API's `build: null` still means "all features".

## UI

Read-only means removed, not greyed out: the toolbar, the sidebar menu, the File
menu and Settings simply have fewer items. Settings › This build gets a
**Features** section (a comma list, muted, like Plugins). The section says "Read-only
(no editor)" when `editor` is missing. Tokens only.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A page script or plugin calls `Save` (or another write) through the bindings in a read-only app | The session refuses it, so the check does not depend on the UI | `TestReadOnlyRefusesWrites` |
| A write path is added later and the guard is forgotten | The test's table lists the session's exported methods that write; a new exported method whose name starts with `Save`, `Create`, `Set`, `Reorder`, `Write`, `Delete` or `Rename` makes the test fail until it is listed as guarded or exempt | `readonly_test.go: TestEveryWriterGuarded` (reflection over `*Session`) |
| Ids from the YAML reach generated code | The existing id check covers `features` | `gen_test.go: TestNoInjection` |

## Notes

- **Out of scope:**
  - the other features (`newsite`, `export`, `inspector`, `developer`), each its own spec;
  - removing the editor's code from the bundle (tree-shaking per feature);
  - a read-only *mode* switched at run time in Zenmark;
  - App Store entitlements (a read-only app could ask only for read access to the folder, a later packaging spec).
- **Integration (lead, at merge):**
  - `docs/BUILD.md` "What an app is built from": the `features` key.
  - `docs/ADDONS.md`: a line under *Add-on, plugin or layout?* saying features are the fourth kind (part of the app, on by default).
  - `docs/ENGINE.md`: `session.Options.ReadOnly`, `ErrReadOnly`.
  - `specs/kit/assembly/spec.md`: a pointer here.
  - README; the user guide only if it describes the assembly.
- **The working tree:** `apps/zenmark/app-assembly.yml` has local edits (empty plugins/layouts/templates). The agent adds only `features: all` to the committed version, and the lead re-applies the local edits after the merge.
- **App Store / sandbox:** none now; see out of scope.
- **Changelog:** `Added: app-assembly.yml can leave out features; an app built without the editor is read-only and never changes the folder.`

## Prompt

> Implement `specs/kit/features/spec.md` following the rules in `specs/README.md`.
>
> It builds on kit/assembly (merged):
> - `kit/assembly` `Load`/`Check`/`Catalog`, with the generator in `kit/assembly/cmd/gen`, which writes `apps/zenmark/assembly_gen.go`;
> - `session.BuildInfo` / `Startup.Build`;
> - `KitHost.build` and `hasAddon` in `kit/ui/src/host.ts`;
> - Settings › This build in `kit/ui/src/shell/addons.ts`.
>
> The writes to guard are in `engine/session`: `Save` (session.go), `CreatePage` (newpage.go), `ReorderNav`/`SetNavHidden` (nav.go), `SaveBuildDefaults` (buildui.go) and `EnsureConfig`. The app wrappers are in `apps/zenmark/{service,nav,editor,settings}.go`.
>
> The frontend's edit entry points are:
> - the Edit button and ⌘E in `kit/ui/src/page/index.ts`;
> - `data-zen-action="edit"` in `page/links.ts`;
> - the sidebar menu and drag in `kit/ui/src/nav/{index,entries}.ts`;
> - New Page in `apps/zenmark/frontend/src/shell/shell.ts`;
> - `editorFeature` in `shell/features.ts`;
> - the config buttons in `shell/settings.ts`;
> - Save as folder defaults in `shell/buildsite.ts`.
>
> Order of work:
> 1. The `features` key and the catalog, with tests.
> 2. `session.Options.ReadOnly` + `ErrReadOnly` + the guards, with `TestReadOnlyRefusesWrites` and `TestEveryWriterGuarded` first.
> 3. The generator and the app wiring (menu, OpenEditor, startup).
> 4. `hasFeature` and the frontend gating.
> 5. This build and the version line.
>
> Test read-only with a test assembly or `Options.ReadOnly`. Don't change Zenmark's own features: its file gets `features: all`.
>
> Do not launch the app.
>
> Done when the table's tests pass with and without `-tags addon_vale`, and the frontend checks pass (`npx tsc --noEmit && npx vitest run` in kit/ui and apps/zenmark/frontend).
