# engine/cli-rename

| | |
|---|---|
| Status | specified |
| Target version | 0.3.0 (engine `zenmd` 0.3.0) |
| Branch | `feature/engine-cli-rename` |
| Wave | 13 (E1) |
| Depends on | cli/commands, core/static-build, core/init, packaging/appstore |
| Owns | `internal/cli`, `main.go`, root `Taskfile.yml` (`APP_NAME`, install/uninstall tasks), `build/darwin/Taskfile.yml` (executable name, `install:cli`), `build/darwin/Info.plist` (`CFBundleExecutable`), `build/darwin/package_test.go`, `build/version_test.go`, command examples in `README.md`, `docs/*.md` (except ENGINE.md and PLAN.md), `examples/demo/**/*.md`, `internal/sitetemplates/**` text |

## Purpose

The command line tool and the executable are **`zenmd`**: the name of the
engine (docs/ENGINE.md). `zen` keeps working as an alias for one minor
release and says it is going away.

## Behaviour

1. The executable inside the bundle is `Contents/MacOS/zenmd`; `CFBundleExecutable` is `zenmd`. `bin/zenmd` is what `task build` produces. The bundle file stays `Zen.app` and the displayed name stays zen.md.
2. `zenmd [path]`, `zenmd edit`, `zenmd build`, `zenmd init`, `zenmd help` and `zenmd version` behave exactly as the `zen` commands do today.
3. `zenmd --version` / `zenmd version` print `zenmd 0.3.0` (the version from `VERSION`). An unstamped build prints `zenmd dev`.
4. The usage text starts with `zenmd - the zen.md engine: markdown folders as pages, sites and print`, and every example in it uses `zenmd`.
5. **The `zen` alias:**
   - `task install:cli` (and the root `task install`) links `INSTALL_DIR/zenmd` and `INSTALL_DIR/zen` to `Contents/MacOS/zenmd`.
   - When the program is started as `zen` (`argv[0]` base name, after resolving the link name rather than its target), a command prints one line to stderr, `zen is now zenmd; the zen name stops working in 0.4.0`, then runs normally.
   - Opening the GUI (`zen [path]` with no command) prints the notice too.
   - It never prints when launched from Finder, by Open With, or through the hidden script-host mode.
   - Exit codes and stdout are unchanged, so scripts keep working.
6. **Upgrading from an older install:** `task install:cli` replaces a `zen` link that points at the old `Contents/MacOS/zen`. A link that points somewhere else (another tool called `zen`) is left alone with a warning, and only `zenmd` is installed. `task uninstall:cli` removes both links, but only if they point into Zen.app.
7. **Docs and content:**
   - Every command example in the README, docs, the demo and the site templates becomes `zenmd …`.
   - Running text that names the product stays "zen.md".
   - The `.zen.yml` and `.zen/` file names are unchanged: they are the format, and renaming them is out of scope.
8. Nothing else changes: the config folder (`…/Application Support/zen/`), trust and plugin data, `window.zen`, the Tengo `zen` module, the `zen` code style, and the `ZEN_*` environment variables plugins receive.

## Architecture

- `internal/cli`:
  - `Run` is unchanged apart from the usage text and the version prefix;
  - `ProgramName(argv0 string) string` decides `zenmd` or `zen` (the alias);
  - `Notice(w io.Writer, name string)` prints the alias line.
- `main.go` passes `os.Args[0]` through. The script-host check stays first, so helpers never print.
- The Taskfile renames `APP_NAME`; the bundle steps, codesign and helper paths follow it.
- `install:cli` gets the link logic of behaviour 6, written as a small Go program (`build/darwin/installcli`) so it can be tested, instead of shell.

## Interfaces

```go
// internal/cli
const Name = "zenmd"
const Alias = "zen" // removed in 0.4.0
func ProgramName(argv0 string) string           // "zenmd" or "zen"
func Notice(w io.Writer, invokedAs string)      // prints the alias line for "zen", nothing otherwise
func Run(args []string, cwd string, stdout, stderr io.Writer, launch Launcher, build Builder, initer Initer) int // unchanged signature
```

```go
// build/darwin/installcli
// go run ./build/darwin/installcli -app /Applications/Zen.app -dir /usr/local/bin [-uninstall]
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | bundle executable and plist | `build/darwin/package_test.go: TestBundleLayout`, `TestInfoPlist` (`CFBundleExecutable == "zenmd"`) |
| 2–4 | commands, version, usage text | `internal/cli/cli_test.go: TestUsageNamesZenmd`, `TestVersion` (updated) |
| 5 | the alias notice: once, on stderr, only for `zen`; never for the script host or Finder launches | `cli_test.go: TestAliasNotice`; `main_test.go` (root): `TestScriptHostIsQuiet` |
| 6 | install/uninstall links, replacing old links, leaving foreign ones | `build/darwin/installcli/main_test.go: TestInstallLinks`, `TestForeignZenLeftAlone`, `TestUninstallOnlyOurs` |
| 7 | no `zen build`/`zen init`/`zen edit` examples left in docs, demo or templates | `internal/cli/docs_test.go: TestDocsUseZenmd` (scans the files listed in Owns) |
| 8 | config dir, `.zen.yml`, `window.zen` and the Tengo `zen` module unchanged | the existing tests for config, runtime and scriptplugins still pass |

Edge cases to cover:
- `argv[0]` given as a full path (`/usr/local/bin/zen`) or as a relative path;
- a link chain `zen` → `zenmd` → the bundle;
- `ZEN` in upper case on a case-insensitive disk;
- `go run .`, where `argv[0]` is a temporary binary (counts as `zenmd`).

## UI

None (command line). The app's About and menus are unchanged.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Install replaces another tool's `zen` | Only links that point into a Zen.app bundle are replaced or removed | `TestForeignZenLeftAlone`, `TestUninstallOnlyOurs` |
| The notice breaks scripts that parse output | It goes to stderr only; stdout and exit codes are unchanged | `TestAliasNotice` |

## Notes

- **Out of scope:**
  - renaming `.zen.yml`, `.zen/` or the config folder;
  - the Go module path (wave E2);
  - the `ZEN_*` plugin environment variables.
- **Integration (done by the lead at merge):**
  - `docs/PLAN.md` and `docs/ENGINE.md`: mark E1 cli-rename as done;
  - the changelog line;
  - `specs/README.md` status.
- **App Store / sandbox:** the App Store build has no CLI; the executable rename applies to it too.
- **Changelog:** `Changed: the command is now zenmd (the zen.md engine); zen still works until 0.4.0 and says so. Run task install again to get both links.`

## Prompt

> Implement `specs/engine/cli-rename/spec.md` following the rules in
> `specs/README.md`. Context:
> - The CLI is `internal/cli/cli.go` and `main.go`.
> - The bundle steps are in `build/darwin/Taskfile.yml` (`create:app:bundle`, `install:cli`, `codesign`) and the root `Taskfile.yml` (`APP_NAME`).
> - The bundle tests are `build/darwin/package_test.go`.
>
> Rename the executable and the command; add the alias and its notice; write the `installcli` helper with tests; update every command example in the docs, the demo and the site templates.
>
> Do not run `task install` or `task uninstall` against `/usr/local/bin`: test `installcli` in temporary folders only. Do not launch the app.
>
> Done when the table's tests pass and `go vet ./... && go test ./...` (after `task build`) and `cd frontend && npx tsc --noEmit && npx vitest run` are clean, and `task package` then `ZEN_REQUIRE_BUNDLE=1 go test ./build/darwin` pass. Report what the user must run once to switch over: `task install`.
