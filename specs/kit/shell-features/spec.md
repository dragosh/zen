# kit/shell-features

| | |
|---|---|
| Status | specified |
| Target version | 0.7.0 |
| Branch | `feature/kit-shell-features` |
| Wave | 25 |
| Depends on | kit/features (the mechanism, `editor`), engine/dialects (merged first: both change kit/assembly and This build), kit/assembly |
| Owns | `kit/assembly/**` (catalog, needs), `apps/zenmark/{service,menu,menustate,app,headless,settings,print,exportpdf,buildui,sitetemplates,logs}.go` (gating) + `readonly_test.go` → `features_test.go`, `internal/cli/export.go` (refusals), `kit/ui/src/host.ts`, `kit/ui/src/shell/{types,keys,addons}.ts`, `apps/zenmark/frontend/src/shell/{features,shell,settings,buildsite,newsite}.ts`, `apps/zenmark/frontend/src/api/{index,wails,fake}.ts`, `apps/zenmark/app-assembly.yml` (see Notes) (+ the tests of all of these) |

## Purpose

Every big piece of the app becomes a **feature** an assembly can leave out, like
`editor` today. With `features: []` an app is a **bare shell**: a window that
opens a folder and has Settings. Zenmark starts from that shell and gets its
pieces back one at a time, each checked in the real app and ticked off on the
roadmap (GitHub Project "Zenmark roadmap"). Every niche app later picks just
the pieces it needs.

## Behaviour

### The catalog
1. Known features, in this order, with what each needs:

   | Id | What it brings | Needs |
   |---|---|---|
   | `reader` | the page view: rendering, links and anchors, live reload, image zoom, plugin and layout assets, Settings › Plugins and Layouts | – |
   | `nav` | the sidebar tree, breadcrumbs, Reveal in Finder | `reader` |
   | `find` | the find bar, ⌘F/⌘G/⇧⌘G, Edit › Find | `reader` |
   | `editor` | editor windows, ⌘E, New Page, sidebar editing, config files (kit/features) | `reader` |
   | `inspector` | the Inspector panel, its button, ⌥⌘I | `reader` |
   | `print` | Print Page (⌘P), Print Site, print preview | `reader` |
   | `export` | File › Export › Website… and PDF…, Settings › Export, `zenmd export` | `reader`, `print` (for PDF) |
   | `newsite` | File › New Site from Template…, the welcome cards | `templates` not empty |
   | `developer` | Settings › Developer, the Developer panel and its log, ⌥⌘J, the Develop menu | – |

2. `kit/assembly` checks the needs and fails with a sentence naming both sides ("feature find needs feature reader"; "feature newsite needs at least one template"). Add-ons can declare needs too: `vale` needs `reader`, and gets the Inspector's Linting tab only when `inspector` is in the build. Dialects, plugins and layouts are only useful with `reader`. Without `reader`, listing any of them fails with "plugins need feature reader".
3. A missing `features` key is still `all` (kit/features). `features: []` is the bare shell.

### The bare shell (`features: []`)
4. The bare shell always has:
   - the window, the home screen, Open Folder…, recent folders and Close Folder;
   - Settings with General, Appearance and This build;
   - About, the standard App, Edit (Undo through Select All) and Window menus;
   - the theme and the opening progress.
5. Opening a folder works (index and watch) and the main area shows a quiet card: the folder's name, its page count and "This build shows no pages." There is no sidebar.
6. No menu item, shortcut, button or Settings section of a missing feature is present, not even greyed out. Its shortcuts do nothing. Its Wails service methods return `ErrFeatureMissing("find")` ("this app is built without find").
7. Missing `editor` keeps every kit/features guarantee (read-only).

### Each feature
8. With a feature in the build, it behaves exactly as today. `features: all` gives today's Zenmark, and the whole existing test suite runs against that build unchanged.
9. The CLI follows the build. `zenmd export site` needs `export`, and `zenmd export pdf` needs `export` and `print`; without them it fails with the message from behaviour 6 and exit code 2. `zenmd version --verbose` already lists the features.
10. Settings › This build lists features in catalog order and greys the names of the missing ones, so a bare build shows what it could have.

## Architecture

- `kit/assembly`:
  - `Catalog.Features` gains the ids above in that order.
  - `FeatureNeeds` (from kit/features) is filled.
  - New `AddonNeeds` holds what each add-on needs (`vale: [reader]`).
  - `Check` applies the needs, plus the rule that plugins, layouts and dialects need `reader`.
- Go, `apps/zenmark`:
  - `hasFeature(id)`, read from `assemblyInfo.Features`.
  - `requireFeature(id) error` at the top of each service method that belongs to a feature: Print*, Export*, BuildWebsite, CreateSite and SiteTemplates, Logs, PageInfo (inspector), Page and Preview (reader).
  - `menuSpecFor(dev, features)` replaces `menuSpecFor(dev, editor)`.
  - `headless.go` refuses `export pdf` without the features.
- Frontend:
  - `Feature.id` widens to the catalog ids.
  - `features.ts` registers only the features the build has.
  - `shell.ts` mounts the reader, nav, find, the Inspector button and the Developer panel only through `hasFeature`.
  - `keys.ts` drops the shortcuts of missing features.
  - Settings builds its section list from the build.
  - The bare main-area card lives in `shell.ts`.
- The editor's check stays in the session (kit/features). The other features need no session check, because none of them writes into the folder.

## Interfaces

```go
// apps/zenmark
func ErrFeatureMissing(id string) error // "this app is built without <id>"
// kit/assembly: Catalog.AddonNeeds map[string][]string
```

```ts
// kit/ui/src/shell/types.ts
export type FeatureId = "reader" | "nav" | "find" | "editor" | "inspector" | "print" | "export" | "newsite" | "developer";
export type Feature = { id: FeatureId; mount(slots: Slots, ctx: Ctx): () => void };
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1–2 | Catalog order; each need fails with both names; vale needs reader; plugins/layouts/dialects need reader | `kit/assembly/assembly_test.go: TestFeatureNeeds`, `TestAddonNeeds` |
| 3 | Missing key = all; `[]` = bare | `TestFeatures` (extended) |
| 4–5 | Bare shell: home, open, settings sections, the folder card, no sidebar | `apps/zenmark/frontend/src/shell/shell.test.ts: "bare shell"` |
| 6 | Per feature: absent from menu, keys, settings; service refuses | `apps/zenmark/menu_test.go: TestMenuPerFeature` (table over features); `features_test.go: TestServiceRefusesMissing`; `shell.test.ts: "each missing feature leaves no trace"` (table) |
| 8 | `features: all` = today | the existing suites, unchanged |
| 9 | CLI export refuses without export/print, exit 2 | `internal/cli/export_test.go: TestExportNeedsFeature` |
| 10 | This build greys missing features | `settings.test.ts: "This build section"` |

Edge cases:
- `newsite` with no templates is an error.
- `inspector` without `vale` has only the File and Page tabs, as today.
- `developer` alone, in a bare shell, still gives a working Developer panel.
- ⌘P with no `print` does nothing.
- A remembered Inspector or Developer panel that was open is not opened when its feature is missing.

## UI

The bare-shell card uses the existing home styles (`.zen-welcome` tokens): the
folder name in the page-title style, the count and the line muted. Missing
features leave no gaps: menus, toolbar and Settings simply have fewer items.
This build: the feature names as a list, with the missing ones in `--zen-muted`
and marked "not in this build" for screen readers.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A page script calls a binding of a feature that is not in the build | Service methods check the feature on the Go side, not only in the UI | `TestServiceRefusesMissing` |
| The bare shell watches a folder it never shows | Unchanged: watching is read-only, and the editor check covers writes | `TestReadOnlyRefusesWrites` (kit/features) |

## Notes

- **The roadmap.** Zenmark's `app-assembly.yml` becomes the bare shell when this merges:
  ```yaml
  features: []
  addons: []
  plugins: []
  layouts: []
  templates: []
  dialects: []
  ```
  This is what the local edits on main started. The GitHub Project then has one issue per step. Each step adds ids to the file, checks them in the real app (`task package`, then open `bin/Zenmark.app`), fixes what turns up, and updates the guide:
  1. `reader` + `layouts: [article]`
  2. `nav`
  3. `plugins: [codeblock]`, then the other plugins
  4. `layouts: all`
  5. `find`
  6. `editor`
  7. `inspector`
  8. `addons: [vale]`
  9. `print`
  10. `export`
  11. `templates: all` + `newsite`
  12. `developer`
  13. `dialects: all`

  When `features: all` is back, Zenmark is at parity for 0.7.0.
- **Releases.** Do not cut a release from main while the roadmap is in progress, because main builds a partial Zenmark. Hotfixes go on a `release/0.6` branch from the 0.6.0 tag.
- **Out of scope:**
  - removing a missing feature's code from the frontend bundle;
  - features of the engine CLI beyond export;
  - per-feature Go build tags (features are run-time gates in one binary, unlike add-ons).
- **Integration (lead, at merge):** docs/BUILD.md (the feature table and needs); docs/ADDONS.md (add-on needs); docs/ENGINE.md if anything engine-facing changes; README; CHANGELOG `Added: every big piece of the app (reader, nav, find, editor, inspector, print, export, new site, developer) is a feature an app assembly can leave out; features: [] builds a bare shell.`
- **App Store / sandbox:** none.

## Prompt

> Implement `specs/kit/shell-features/spec.md` following the rules in `specs/README.md`.
> It extends kit/features (merged): `features:` in the assembly, `editor`, `hasFeature`
> in `kit/ui/src/host.ts`, `readOnlyBuild()` and `menuSpecFor(dev, editor)` in
> apps/zenmark, `Catalog.FeatureNeeds`.
>
> Order of work:
> 1. The catalog and needs in `kit/assembly`, with tests.
> 2. The Go gating: `ErrFeatureMissing`, `requireFeature`, the menu table, the headless/CLI export.
> 3. The frontend: `FeatureId`, `features.ts`, shell mounting, keys, Settings, the bare-shell card, This build.
>
> Keep `apps/zenmark/app-assembly.yml` at `features: all` in the branch and test the bare shell with a test assembly or fake build. The lead switches Zenmark to the bare shell at merge. Do not launch the app.
>
> Done when the table's tests pass with and without `-tags addon_vale`, the existing suites pass unchanged against `features: all`, and the frontend checks pass in `apps/zenmark/frontend`.
