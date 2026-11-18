# zen — versioning

zen follows [Semantic Versioning 2.0.0](https://semver.org).

- The version lives in one place: the `VERSION` file at the repo root
  (`0.1.0-alpha.1`, no `v` prefix, one line).
- Builds inject it: `-ldflags "-X github.com/zenmark-org/zenmd/internal/cli.Version=$(cat VERSION)"`.
  `zenmd --version` prints `zenmd 0.1.0-alpha.1`. An unstamped build prints `zenmd dev`.
- The macOS bundle uses `CFBundleShortVersionString = MAJOR.MINOR.PATCH` and
  `CFBundleVersion =` the commit count (monotonic).
- Git tags are `v<VERSION>` on `main`.

## What bumps what

| Change | Before 1.0 | From 1.0 |
|---|---|---|
| Breaking change to CLI, config file, plugin API, or file handling | MINOR | MAJOR |
| New feature, backwards compatible | MINOR | MINOR |
| Bug fix | PATCH | PATCH |

Pre-releases move `alpha.N` → `beta.N` → `rc.N`. `alpha`: features still
landing. `beta`: feature complete for the milestone. `rc`: only fixes.

## Things versioned separately

| Thing | Field | Rule |
|---|---|---|
| Plugin API | `apiVersion` (integer) in manifests, `plugins.APIVersion` | bump on any breaking change to hooks or types; the registry refuses other versions |
| Config files | `version: 1` | bump on breaking schema change; older versions are migrated on read |
| Each plugin | `Manifest.Version` (SemVer) | independent of the app |

## Per feature

Each spec names its **target version**. A merged feature adds a line under
`## [Unreleased]` in `CHANGELOG.md` ([Keep a Changelog](https://keepachangelog.com)
format: Added / Changed / Fixed / Security). Releasing = move Unreleased under
the new version heading, update `VERSION`, tag.
