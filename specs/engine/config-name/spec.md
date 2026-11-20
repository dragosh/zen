# engine/config-name

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/engine-config-name` |
| Wave | 29 |
| Depends on | core/config, engine/session, engine/dialects, ui/nav-panel (`logo:`), ui/settings-layout (Configuration) |
| Owns | every file that names `.zen.yml` outside `CHANGELOG.md` and the historical specs: `engine/{config,session,watch,render,plugins,sitetemplates}/**`, `engine/doc.go`, `builtins/{sitetemplates,layouts,plugins/sandbox}/**`, `internal/cli/**`, `apps/zenmark/*.go`, `apps/zenmark/frontend/src/**`, `kit/ui/src/**`, `examples/demo/**`, `docs/*.md`, README (+ the tests of all of these). Historical specs under `specs/` keep the old name; only the spec of this feature and `specs/README.md` are edited. |

## Purpose

A folder's settings file is called `.zenmd.yml`, after the engine and the
command (`zenmd`). Folders that still have a `.zen.yml` keep working. The app
moves them to the new name the first time it saves a folder setting, and
says so before that.

## Behaviour

1. **The name.** The folder settings file is `.zenmd.yml`, at the folder's root. The global `config.yml` and the `.zen/` folder (folder plugins and layouts) keep their names.
2. **Reading.**
   - With `.zenmd.yml`, it is read.
   - With only `.zen.yml`, the legacy file is read, exactly as before.
   - With both, `.zenmd.yml` is read and `.zen.yml` is ignored.
3. **Telling the user.**
   - A folder read through the legacy name has one Problems item, under Checks: "This folder's settings are in .zen.yml; Zenmark now calls the file .zenmd.yml." Its Fix line is "It is renamed the next time a folder setting is saved, or rename it yourself." It also goes to the log once, with source "workspace".
   - With both files, the message is ".zen.yml is ignored because .zenmd.yml exists."
   - `zenmd` (CLI) prints the same line on stderr, once per run.
4. **Writing.** Every write of folder settings goes to `.zenmd.yml`: the dialect switch, Save as folder defaults, `nav.hidden`, and editing in the editor. When only `.zen.yml` exists, the first write renames it to `.zenmd.yml` in place, keeping its bytes and comments, and then edits it. The rename is atomic (same directory), and a failed rename leaves `.zen.yml` untouched and returns the error.
5. **Creating.** Settings › Configuration and the editor create `.zenmd.yml`. Site templates (`zenmd init site`, New Site from Template) write `.zenmd.yml`. The built-in templates and the demo folder ship `.zenmd.yml`.
6. **Everything that knew the old name knows both:**
   - the watcher reloads on either file;
   - the index and the sidebar hide both files;
   - `zenmd export` never copies either into a site;
   - Settings › Configuration shows ".zenmd.yml" as the folder item, or ".zen.yml (old name)" while only the legacy file exists;
   - `ConfigFile("workspace")` returns the file in use.
7. **Text.** Every message, hint, doc and help text says `.zenmd.yml`. The legacy name appears only in the messages of behaviour 3 and in the CHANGELOG.

## Architecture

- **`engine/config`:**
  - `WorkspaceFile = ".zenmd.yml"`;
  - `LegacyWorkspaceFile = ".zen.yml"`;
  - `func WorkspaceConfigPath(root string) (path string, legacy bool, both bool)`, the one place that decides which file is in use;
  - `zenyml_edit.go` writes through `MoveLegacy(root)` (the rename) when `legacy` is true.
- **`engine/session`:**
  - `ConfigFile`, `SetDialect`, `SaveBuildDefaults` and `EnsureConfig` use it;
  - a `Session.ConfigNotice() string` gives the Problems item's text, or "".
- **`kit/ui/src/inspector/problems.ts`:** shows the notice as a `warning` Problems item, through the same path as page warnings. Its Fix text is in the notice.
- **Watch, index and site:** match both names.

## Interfaces

```go
// engine/config
const WorkspaceFile = ".zenmd.yml"
const LegacyWorkspaceFile = ".zen.yml"
func WorkspaceConfigPath(root string) (path string, legacy, both bool)
func MoveLegacy(root string) error
// engine/session
func (s *Session) ConfigNotice() string
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1–2 | new, legacy, both | `engine/config/name_test.go: TestWorkspaceConfigPath` |
| 3 | the Problems item, the log once, CLI stderr | `engine/session/confignotice_test.go`; `kit/ui/src/inspector/problems.test.ts: "config notice"`; `internal/cli: TestLegacyConfigNotice` |
| 4 | the first write renames in place (bytes kept), a failed rename leaves it, both → writes `.zenmd.yml` | `engine/config/name_test.go: TestMoveLegacy`, `engine/session/dialects_test.go` (extended) |
| 5 | Create and the templates write `.zenmd.yml`; the demo and templates ship it | `engine/sitetemplates` tests; `builtins/sitetemplates` tests; Settings › Configuration test |
| 6 | the watcher, index, export and Configuration label handle both | `engine/watch`, `engine/session` index test, `engine/site` build test (neither file copied), `apps/zenmark/frontend/src/shell/settings.test.ts` |
| 7 | no `.zen.yml` in docs or messages but the notices | a repo test (`internal/…` or a Go test) that greps docs, `*.go` messages and the frontend for `.zen.yml`, allowing only the legacy constant, the notices and tests |

Edge cases:
- `.zen.yml` is a symlink (follow the confinement rules: never rename a link that points outside the folder; refuse with a message);
- read-only builds never rename;
- a folder that is read-only on disk: the rename fails and is reported;
- `TestDemoBuildUnchanged` and the golden builds are updated for the demo's renamed file, and stay byte-identical otherwise.

## UI

UI consistency (docs/UI.md):
- The notice is a Problems item like any page warning.
- Settings › Configuration keeps its two items. Only the folder item's label and note change.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| The rename follows a symlink out of the folder | Rename only a regular file inside the root, checked with the workspace's confinement | `TestMoveLegacy` (symlink case) |
| A crafted folder has both files to confuse which is applied | One function decides; `.zenmd.yml` always wins and the notice says so | `TestWorkspaceConfigPath` |

## Notes

- **Out of scope:** renaming `.zen/` or `config.yml`, and a bulk migration command.
- **Integration (lead):**
  - CHANGELOG under Changed: `The folder settings file is now .zenmd.yml. A .zen.yml is still read, shown under Problems, and renamed the first time a folder setting is saved.`;
  - `specs/README.md` row.
- **App Store / sandbox:** none.
- **Versioning:** a config file change. Reads stay compatible, so `version: 1` is unchanged.

## Prompt

> Implement `specs/engine/config-name/spec.md` following `specs/README.md` and CLAUDE.md.
>
> Start with `git grep -n '\.zen\.yml'` to see every use.
>
> Work in `../zen-worktrees/engine-config-name`. Tests first. Do not launch the app, push or touch main. Update golden files only for the demo's renamed file, and say which ones changed.
>
> Done when the table's tests pass, `go vet ./... && go test ./...` pass (run `npx vite build` first), and `npx tsc --noEmit && npx vitest run` pass in `apps/zenmark/frontend`.
