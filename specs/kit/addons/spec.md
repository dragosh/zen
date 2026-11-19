# kit/addons

| | |
|---|---|
| Status | done |
| Target version | 0.6.0 |
| Branch | `feature/kit-addons` |
| Wave | 21 |
| Depends on | core/lint, ui/lint, ui/inspector, ui/panel, packaging/macos |
| Owns | `kit/addons/**` (new), `addons/vale/**` (new), `apps/zenmark/addons*.go` (new), `engine/session/lint.go` (+ test), `build/darwin/resources/**`, `resources.yml`, `build/darwin/Taskfile.yml`, `build/Taskfile.yml`, `Taskfile.yml`, `.github/workflows/release.yml`, `kit/ui/src/host.ts`, `kit/ui/src/inspector/index.ts`, `kit/ui/src/editor/index.ts`, `kit/ui/src/page/index.ts` (+ their tests), `kit/ui/src/shell/addons.ts` (new, the Settings section), `apps/zenmark/frontend/src/api/{index,wails,fake}.ts`, `apps/zenmark/frontend/src/shell/settings.ts` (+ tests), `internal/cli/cli.go` (version line) |

## Purpose

An **add-on** is an optional capability that an app includes only when it is
built with it: a helper program, the Go code that drives it, and the UI it adds.
The default build has none. Vale linting becomes the first add-on (`vale`); a
later one could be voice dictation for another app. Apps built on the kit
(Zenmark today, HashDoc and the others later) pick their add-ons at build time.

## Behaviour

### Building
1. `ADDONS` chooses the add-ons of a build, a comma-separated list of ids: `task build ADDONS=vale`, `task package ADDONS=vale`, `task release ADDONS=vale`, `task run|dev ADDONS=vale`. Default: empty, no add-ons.
2. Each add-on is compiled in with the Go build tag `addon_<id>`; without it, none of its Go code is linked. An unknown id fails the build early with the list of known add-ons.
3. A helper in `resources.yml` may name its add-on (`addon: vale`). `task package` bundles a helper into `Contents/Helpers` only when it has no `addon:` or its add-on is in `ADDONS`; its licence goes with it. Helpers of add-ons not in the build are not fetched.
4. The release workflow builds with the repository variable `ADDONS` (empty when unset), and the release notes list the add-ons of the build.

### Running
5. The app knows its add-ons at run time (`addons.List()` in Go; `startup().addons` in the frontend: `{ id, name, summary }[]`, in a fixed order).
6. **Without `vale`**: no lint calls are made (`Lint` answers "unavailable" at once, without looking for a vale binary, even on PATH or `$ZEN_VALE`); the reader shows no lint marks; the editor has no lint marks and no Problems console; the Inspector has no Linting tab (File and Page only) and its button shows no count badge; Settings shows nothing about Vale.
7. **With `vale`**: everything works as today (ui/lint, ui/inspector), with Vale found as today (bundled helper, then `$ZEN_VALE`, the dev helper, PATH).
8. Settings has an **Add-ons** section: each included add-on with its name and summary, or "This build has no add-ons." when there are none. It is read-only (add-ons are chosen when the app is built).
9. `zenmd version` (and `--version`) prints the add-ons after the version: `zenmd 0.6.0 (add-ons: vale)`, or just `zenmd 0.6.0` without any. The About panel's version string is the same.
10. A folder's `lint:` setting in `.zen.yml` is kept and only has an effect when `vale` is included.

### The add-on contract (for later add-ons)
11. An add-on is a Go package `addons/<id>` whose files carry `//go:build addon_<id>`. Its `init` calls `addons.Register(addons.Addon{ID, Name, Summary})` and wires itself in through the kit's hooks. The app imports it in `apps/<app>/addons_<id>.go`, under the same tag.
12. Hooks in this spec: the text linter (`session.SetLinter`). The frontend sees only the add-on ids and gates its own UI on them (`hasAddon(host, "vale")`). Later add-ons add their hooks in their own specs: a panel tab (ui/panel), menu items, a Wails service, a Settings section, entitlements (a microphone for voice).

## Architecture

- `kit/addons` (Go, no dependencies): a registry. `Register` panics on a duplicate or invalid id (`^[a-z][a-z0-9-]{0,31}$`); `List` returns them sorted by id; `Has(id)`.
- `addons/vale` (Go, `addon_vale`): registers `{ID: "vale", Name: "Vale linting", Summary: "Spelling and style checks for your pages, by the Vale prose linter."}` and calls `session.SetLinter(func() session.TextLinter { p, err := lint.Find(); if err != nil { return nil }; return lint.New(p) })`.
- `engine/session/lint.go`: the linter is no longer found on first use. `SetLinter(find func() TextLinter)` installs a finder (called once, lazily, as today); without one, `Lint` is unavailable. `TextLinter` becomes the exported name of today's `textLinter`. `engine/lint` itself is unchanged (it stays a library).
- `apps/zenmark/addons_vale.go`: `//go:build addon_vale` + `import _ "github.com/zenmark-org/zenmd/addons/vale"`. `Startup` gains `Addons []AddonInfo`.
- Build: `build/Taskfile.yml` turns `ADDONS` into tags (`addon_vale,…`) and appends them to `EXTRA_TAGS`; a small check (`go run ./kit/addons/cmd/check -known vale`) validates the list. `build/darwin/resources` gains `-addons vale,...` and skips entries whose `addon` is not listed (and `fetch:helpers` takes the same list).
- Frontend: `KitHost.addons?: readonly AddonInfo[]` (from startup); `hasAddon(host, id)` in `kit/ui/src/host.ts`. The Inspector builds its tab list from the add-ons (`issues` only with `vale`). The page and editor skip lint wiring without `vale`. `kit/ui/src/shell/addons.ts` renders the Settings section.

## Interfaces

```go
// kit/addons
type Addon struct{ ID, Name, Summary string }
func Register(a Addon)
func List() []Addon
func Has(id string) bool

// engine/session
type TextLinter interface { Lint(ctx context.Context, root, path string, content []byte) ([]lint.Alert, error) }
func SetLinter(find func() TextLinter) (restore func())

// apps/zenmark
type AddonInfo struct{ ID, Name, Summary string }
// Startup.Addons []AddonInfo
```

```ts
// kit/ui/src/host.ts
export type AddonInfo = { id: string; name: string; summary: string };
// KitHost.addons?: readonly AddonInfo[]
export function hasAddon(host: Pick<KitHost, "addons">, id: string): boolean;
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1–2 | ADDONS → tags; unknown id fails with the known list | `kit/addons/cmd/check/check_test.go: TestTags`, `TestUnknown` |
| 3 | Helpers bundled only for included add-ons; licence too; not fetched otherwise | `build/darwin/resources/resources_test.go: TestAddonHelpers`; `build/darwin/package_test.go: TestPackageWithoutAddons` |
| 4 | Release workflow passes ADDONS | `build/release_workflow_test.go: TestReleaseUsesAddons` (reads the YAML) |
| 5 | Registry: order, duplicates, invalid ids | `kit/addons/addons_test.go` |
| 6 | No linter set: Lint unavailable without looking for vale (a vale on PATH is ignored) | `engine/session/lint_test.go: TestLintNeedsAddon` |
| 6 | Inspector without vale: File and Page tabs only, no badge | `kit/ui/src/inspector/inspector.test.ts: "without the vale add-on there is no Linting tab"` |
| 6 | Reader and editor without vale: no lint calls, no marks, no Problems console | `kit/ui/src/page/lint.test.ts`, `kit/ui/src/editor/editor.test.ts: "no lint without the vale add-on"` |
| 7 | With vale: existing ui/lint and ui/inspector tests pass with `addons: [vale]` | the existing suites (host fixture includes vale) |
| 7 | The vale add-on registers and installs the linter | `addons/vale/vale_test.go` (`-tags addon_vale`) |
| 8 | Settings › Add-ons lists them, or says none | `apps/zenmark/frontend/src/shell/settings.test.ts: "Add-ons section"` |
| 9 | Version line with and without add-ons | `internal/cli/cli_test.go: TestVersionAddons` |
| 11 | The app links vale only with the tag | `apps/zenmark/addons_test.go: TestNoAddonsByDefault` (default build: `addons.List()` empty) and the same file under `addon_vale` |

CI and `task test` run the Go tests twice: without tags and with `-tags addon_vale`.

## UI

- Settings › Add-ons: a list of cards (name in 13 px semibold, summary muted), or the empty line. Tokens only.
- Inspector without vale: tabs File, Page; File first.
- Nothing else changes visibly; with vale everything looks as today.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A build without vale still runs a `vale` found on PATH or `$ZEN_VALE` | Without the add-on no finder is installed, so nothing is looked up or run | `TestLintNeedsAddon` |
| An add-on id reaches a build tag or a path | Ids are checked against `^[a-z][a-z0-9-]{0,31}$` and the known list before use | `TestUnknown`, `addons_test.go` |
| Add-on names shown in Settings | Text only (`textContent`) | `settings.test.ts` |

## Notes

- **Out of scope:** installing or removing add-ons after the build; a frontend bundle per add-on (the lint UI code stays in the bundle and is gated at run time; the size that matters is the 30 MB Vale binary, which is left out); the voice add-on itself and the hooks it needs (panel tab, service, menu, microphone entitlement) — each gets its own spec.
- **Integration (lead, at merge):**
  - `docs/ENGINE.md`: `session.SetLinter`; `docs/PLAN.md`: `Startup.addons`.
  - `docs/BUILD.md` and `docs/DISTRIBUTION.md`: `ADDONS=`, and the `ADDONS` repository variable for releases.
  - User guide: the lint pages say Vale is an add-on included in some builds; golden hash.
  - `specs/core/lint`, `specs/ui/lint`, `specs/ui/inspector`: a line pointing here.
  - README: build with add-ons.
- **Decide before the next release:** the official Zenmark DMG is built without add-ons by default; set the repository variable `ADDONS=vale` to keep linting in it.
- **App Store / sandbox:** a helper is only bundled (and signed) when its add-on is in the build; later add-ons may add entitlements through their own specs.
- **Changelog:** `Changed: Vale linting is now an add-on, included only in builds made with ADDONS=vale; Settings › Add-ons lists what a build includes.`

## Prompt

> Implement `specs/kit/addons/spec.md` following the rules in `specs/README.md`.
> Context: Vale is bundled today through `resources.yml` → `build/darwin/resources`
> (`task package`), found by `engine/lint.Find`, used lazily by `engine/session/lint.go`
> (`currentLinter`), exposed as `Zen.Lint`, and shown by `kit/ui` (lint marks in page
> and editor, the Problems console, the Inspector's Linting tab). Build tags already
> exist for the App Store channel (`CHANNEL_TAGS`, `EXTRA_TAGS` in `build/darwin/Taskfile.yml`).
> Start with `kit/addons` and `engine/session.SetLinter`, then `addons/vale` and the app
> import, then the Taskfile/resources changes, then the frontend gating and Settings ›
> Add-ons, then `zenmd version`. Keep every existing lint/inspector test passing with a
> host that includes the vale add-on. Do not launch the app; `task package ADDONS=vale`
> and `task package` (without) may be run in the worktree to check the bundle has
> `Contents/Helpers/vale` only in the first. Done when the table's tests pass with and
> without `-tags addon_vale`, and the frontend checks pass.
