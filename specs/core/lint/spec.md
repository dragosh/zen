# core/lint

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.2 |
| Branch | `feature/core-lint` |
| Wave | 5 |
| Depends on | core/workspace, packaging/macos |
| Owns | `internal/core/lint/`, `resources.yml`, `build/darwin/resources/`, `build/darwin/Taskfile.yml` (helper fetch and bundling only), `build/darwin/package_test.go` (helper cases), `.gitignore` (helpers dir) |

## Purpose

zen can run the Vale prose linter, bundled inside Zen.app, on a page or on
unsaved editor text, and return alerts (spelling, style flags) with positions.

## Behaviour

> Since 0.6.0 Vale is the `vale` add-on ([kit/addons](../../kit/addons/spec.md)): an app built without it has no linter and none of this UI.


1. **Bundled binary.** Vale **3.25.1** (MIT), pinned:
   | arch | tarball | tarball sha256 | binary sha256 |
   |---|---|---|---|
   | darwin-arm64 | `https://github.com/vale-cli/vale/releases/download/v3.25.1/vale_3.25.1_macOS_arm64.tar.gz` | `fbcaa68d9b9e229b9c8dc918f209e93572ce943039700b7e04a423ff12f684cd` | `ac853f996e8cce7390d4df35f7c9890a0b7a1728e41336bf98a9ca2e24b5fef5` |
   | darwin-amd64 | `https://github.com/vale-cli/vale/releases/download/v3.25.1/vale_3.25.1_macOS_64-bit.tar.gz` | `e27097e84a812d45546de8d6a4ffa75538cab39c96f79b153b601240f8cefaf9` | `6f44a266dae578e37bc0c46df54b6f137f4fedc3385219c67b889db846843bc4` |
   Binaries are **not committed**. `task darwin:fetch:helpers` downloads the tarball for the target arch into git-ignored `build/darwin/helpers/<arch>/`, checks the tarball and binary sha256 and fails on mismatch; it is skipped when the verified binary is already there. `task package` depends on it and copies `vale` to `Contents/Helpers/vale` (signed with the app).
2. **resources.yml** schema extended: per-arch `sha256` map and an optional `fetch` block (`url`, `sha256`, `member` inside the tarball). Old single-`sha256` entries keep working.
3. **Finding vale** (`lint.Find()`): `Contents/Helpers/vale` next to the running executable; else `ZEN_VALE`; else `build/darwin/helpers/<arch>/vale` relative to the repo when running a dev build (`bin/zen`); else `vale` on `PATH`. Not found → `ErrUnavailable` (linting off, no error to the user).
4. **Config.** If the workspace root has `.vale.ini`, Vale uses it (`--config`), run with cwd = workspace root. Otherwise zen's embedded default: `MinAlertLevel = suggestion`, `[*.{md,markdown}] BasedOnStyles = Vale` (spelling and Vale's built-in checks), written once to a private temp dir.
5. `Lint(ctx, root, path, content)` runs `vale --output=JSON --ext=.md --no-exit` with the content on stdin, 5 s timeout, content capped at 1 MB (larger → no alerts plus a warning). It parses the JSON into `[]Alert` sorted by line then column.
6. Alert positions are 1-based line and 1-based rune columns `[Col, EndCol]` within that line of the **original content** (frontmatter included), matching what the editor shows.
7. No network, no writes to the workspace, Vale's environment limited to `PATH`, `HOME`, `TMPDIR` and `VALE_*` it needs. Concurrent calls are allowed; at most 2 Vale processes run at once per Linter.

## Interfaces

```go
package lint
var ErrUnavailable error
type Alert struct {
    Line, Col, EndCol int    // 1-based; columns in runes
    Severity string          // "error" | "warning" | "suggestion"
    Check, Message, Match string
}
type Linter struct{ /* unexported */ }
func Find() (path string, err error)
func New(valePath string) *Linter
func (l *Linter) Lint(ctx context.Context, root, path string, content []byte) ([]Alert, error)
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1,2 | resources.yml per-arch sha + fetch parsing and verification (local file:// tarball fixture, no network) | `build/darwin/package_test.go: TestResourcesManifest` (extended), `TestFetchHelper` |
| 1 | bundled `Contents/Helpers/vale --version` = 3.25.1 | `TestBundledVale` (requires bundle) |
| 3 | lookup order | `internal/core/lint/lint_test.go: TestFind` |
| 5,6 | JSON parsing, positions in runes, sorting (fake vale script emitting fixed JSON) | `TestParse`, `TestLintWithFake` |
| 4 | workspace `.vale.ini` vs default | `TestConfigChoice` |
| 5 | timeout, size cap, unavailable | `TestLimits` |
| real | real vale finds "speling" and ignores code blocks (skipped if vale not found) | `TestRealVale` |

## UI

None (ui/lint shows the alerts).

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Tampered helper | tarball + binary sha256 pinned in resources.yml | `TestFetchHelper` |
| Hanging or runaway process | 5 s timeout, 2 concurrent max, 1 MB cap | `TestLimits` |
| Vale reading or writing outside | stdin input, cwd = workspace root, no file arguments | `TestLintWithFake` (asserts args) |

## Notes

- **Out of scope:** downloading Vale style packages (`vale sync`), custom dictionaries UI, Linux/Windows helpers.
- **Integration (lead, wave 5b ui/lint):** app service `Lint(path, content)` and UI.
- **License:** add Vale's MIT notice to the bundle (`Contents/Resources/licenses/vale.txt`).
- **Changelog:** Added: Vale prose linter bundled in Zen.app.

## Prompt

> Implement `specs/core/lint/spec.md` following the rules in `specs/README.md`. The
> lead already verified the two tarballs; you may download exactly those two URLs
> (nothing else) and must verify both checksums. Do not commit binaries. Another agent
> edits `internal/app/` in parallel: do not touch it.
