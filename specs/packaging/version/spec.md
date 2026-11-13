# packaging/version

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/packaging-version` |
| Wave | 1 |
| Depends on | none |
| Owns | `VERSION`, `CHANGELOG.md`, `Taskfile.yml`, `build/Taskfile.yml`, `build/darwin/Taskfile.yml`, `build/linux/Taskfile.yml`, `build/version_test.go`, `build/config.yml` |

## Purpose

Every build knows its SemVer version from one file, as described in docs/VERSIONING.md.

## Behaviour

1. `VERSION` holds one valid SemVer 2.0.0 string without `v` (`0.1.0-alpha.1`).
2. All `go build` invocations in the Taskfiles (dev and production, darwin and linux) add `-X github.com/dragosh/zen/internal/cli.Version=<VERSION>` to their ldflags. `zen --version` of a task-built binary prints `zen <VERSION>`.
3. `task version` prints the version. `task version:check` fails if `VERSION` is not valid SemVer or `CHANGELOG.md` has no `## [Unreleased]` section.
4. `task version:bump PART=major|minor|patch|pre` rewrites `VERSION` (pre: `alpha.1` → `alpha.2`).
5. `build/config.yml` / Info.plist values use `MAJOR.MINOR.PATCH` for the short version.
6. `task install` and `task uninstall` keep working.

## Architecture

Taskfile + a Go test (`build/version_test.go`, package `build_test`) that reads
`../VERSION` and `../CHANGELOG.md`; no runtime code. `internal/cli.Version`
already exists.

## Interfaces

Tasks: `version`, `version:check`, `version:bump`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | VERSION is SemVer | `build/version_test.go: TestVersionFileIsSemver` |
| 3 | changelog has Unreleased | `build/version_test.go: TestChangelogHasUnreleased` |
| 2 | stamped binary | `build/version_test.go: TestBuildStampsVersion` (runs `go build` with the ldflags from a helper script shared with the Taskfile; skipped with `-short`) |

## UI

None.

## Security

None: build metadata only, no input from users at runtime.

## Notes

- **Out of scope:** tagging, release automation, signing.
- **Integration:** none.
- **Changelog:** Added: SemVer versioning from a single `VERSION` file.

## Prompt

> Implement `specs/packaging/version/spec.md` following the rules in `specs/README.md`
> and docs/VERSIONING.md. `VERSION` and `CHANGELOG.md` exist already. Do not change
> anything in the Taskfiles unrelated to version stamping. `wails3` and `task` are in
> `$(go env GOPATH)/bin`. Done when the tests pass and `task build && ./bin/zen --version`
> prints `zen 0.1.0-alpha.1`. Revert incidental changes to `build/darwin/icons.icns`.
