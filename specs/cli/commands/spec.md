# cli/commands

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/cli-commands` |
| Wave | 2 |
| Depends on | packaging/version; supersedes cli/entrypoint |
| Owns | `internal/cli/`, `main.go` |

## Purpose

`zen` in a terminal opens the current folder in the app; `zen edit` opens the editor.

## Behaviour

1. `zen` → launch with `{Dir: cwd, Page: "", Mode: "read"}`.
2. `zen <dir>` → that directory. `zen <file.md>` → `Dir` = the file's directory, or the nearest ancestor containing `.zen.yml` or `.git` (not above the user's home), `Page` = the file relative to `Dir`.
3. `zen edit [path]` → as 1/2 with `Mode: "edit"`; without a path, `Page` is left empty (the app opens the home page in the editor, or the welcome hint if there is none). A path to a markdown file that does not exist yet is allowed for `edit` only (the editor creates it on save) if its parent directory exists.
4. `-h`, `--help`, `zen help` print usage (listing `edit`) and exit 0. `-v`, `--version`, `zen version` print `zen <Version>` and exit 0.
5. Errors go to stderr with exit code 2 for usage errors (unknown option, more than one path, missing path for read, path that is neither a directory nor a markdown file) and 1 for launch failures.
6. `--` ends option parsing; macOS `-psn_*` arguments are ignored.
7. The `Launcher` signature changes to `func(Launch) error`; `main.go` passes it to `app.Run`.

## Architecture

`internal/cli` stays free of Wails imports and of other core packages
(standard library only). `main.go` maps `cli.Launch` onto `app.Options`.

## Interfaces

```go
package cli

var Version = "dev"

type Launch struct {
    Dir  string // absolute directory to open
    Page string // workspace-relative page, "" for default
    Mode string // "read" | "edit"
}
type Launcher func(Launch) error
func Run(args []string, cwd string, stdout, stderr io.Writer, launch Launcher) int
```

`app.Options` gains `Dir, Page, Mode string` (replacing `Files`); the lead
adapts `internal/app` at merge until app/services lands.

## Tests

| # | Behaviour | Test (`internal/cli/cli_test.go`) |
|---|---|---|
| 1 | no arguments → cwd | `TestNoArgsOpensCwd` |
| 2 | directory, file, workspace-root detection | `TestPathArguments` |
| 3 | edit with / without path, new file | `TestEdit` |
| 4 | help and version forms | `TestHelpAndVersion` |
| 5 | usage errors and exit codes | `TestUsageErrors` |
| 6 | `--`, `-psn_` | `TestSeparators` |

## UI

Terminal output only; usage text fits 80 columns.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Root detection climbing to `/` or `$HOME` and exposing too much | stop at the file's directory when no marker is found below home; never use home or `/` itself as root via detection | `TestPathArguments` (no-marker and home cases) |

## Notes

- **Out of scope:** reusing a running instance (single-instance is a beta spec), headless subcommands (`zen render`), shell completions.
- **Integration (lead):** in `internal/app`, replace `Options.Files` use with `Dir/Page/Mode`; update `specs/cli/entrypoint/spec.md` status to "superseded".
- **Changelog:** Changed: `zen` opens the current folder; Added: `zen edit [path]`.

## Prompt

> Implement `specs/cli/commands/spec.md` following the rules in `specs/README.md`.
> Rewrite `internal/cli` and its tests (existing tests for the old behaviour are replaced)
> and adapt `main.go`. You may make the minimal change to `internal/app/app.go` `Options`
> needed to compile (add the three fields, keep `Files` working) and must report it.
> Done when the tests in the table pass and `go build ./...` works.
