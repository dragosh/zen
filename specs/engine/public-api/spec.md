# engine/public-api

| | |
|---|---|
| Status | specified |
| Target version | zenmd 0.4.0 (app 0.4.0) |
| Branch | `feature/engine-public-api` |
| Wave | 14 (E2a); runs **alone**: it touches the import paths of every Go file |
| Depends on | engine/session, engine/cli-rename |
| Owns | `go.mod`, every `*.go` import path, `internal/core/**` → `engine/**`, `internal/{plugins,layouts,sitetemplates}/**` → `builtins/**`, `engine/doc.go` and the package docs, `engine/example_test.go`, `.github/workflows/ci.yml` (the apidiff job), the Taskfiles' `-ldflags` path, `frontend/src/api/wails.ts` (binding import path), `docs/ENGINE.md` (the API section) |

## Purpose

Make zenmd a Go engine other programs can import:
- the module is `github.com/dragosh/zenmd`, after the repository rename;
- the engine packages are public under `engine/`, and the default plugins, layouts and site templates under `builtins/`;
- every exported name is documented;
- CI fails when a change breaks the API without saying so.

No behaviour changes.

## Behaviour

1. **The module path** is `github.com/dragosh/zenmd`. Every import, the `-ldflags -X …/internal/cli.Version` path in the Taskfiles, the Wails binding output path (`frontend/bindings/github.com/dragosh/zenmd/...`) and its import in `frontend/src/api/wails.ts` follow.
2. **Package moves** (names unchanged, so the move is mechanical):

   | From | To | Public |
   |---|---|---|
   | `internal/core/{config,extplugins,index,layouts,lint,plugins,render,scriptplugins,site,sitetemplates,watch,workspace}` | `engine/{same}` | yes |
   | `internal/core/atomicfile` | `internal/atomicfile` | no (a helper) |
   | `engine/session` | stays | yes |
   | `internal/plugins/{check,codeblock,diagram,quiz,sandbox}` | `builtins/plugins/{same}` | yes |
   | `internal/layouts` (+ each layout) | `builtins/layouts` | yes |
   | `internal/sitetemplates` | `builtins/sitetemplates` | yes |
   | `internal/cli` | stays (`cmd/zenmd` comes with E2c) | no |
   | `internal/app` | stays (`apps/zenmd-app` in E2c) | no |

3. **`builtins` offers the default set:**
   - `builtins.Plugins() *plugins.Registry` registers all built-in plugins;
   - `builtins.Layouts(version string) *layouts.Registry`;
   - `builtins.SiteTemplates() fs.FS`.

   `session.DefaultPlugins` and the app's `registerPlugins` use them, so there is one list.
4. **Documentation:**
   - every package under `engine/` and `builtins/` has a package doc (what it is, the main types, a short example where useful);
   - every exported identifier has a doc comment starting with its name;
   - `go vet` and a doc test (`engine/doc_test.go: TestExportedAreDocumented`, which walks the AST) enforce this.
5. **The embed example:** `engine/example_test.go`, `Example_embed`, is about 20 lines. It opens `examples/demo` with `session.New` and the built-ins, prints the sidebar titles, renders `README.md` and builds the site into a temporary folder. It runs in `go test` and shows on pkg.go.dev.
6. **API compatibility:** a CI job runs `golang.org/x/exp/cmd/apidiff` (pinned version) for `engine/...` and `builtins/...` against the latest `v*` tag.
   - An incompatible change fails the job, unless the commit message or PR body contains `api-break:` with a reason.
   - Before the first tag, the job reports only.
   - Locally, `task api:check` runs the same command when `apidiff` is installed and says how to install it when it is not.
7. **Import boundaries, enforced by `engine/boundary_test.go`:**
   - nothing under `engine/` or `builtins/` imports `internal/app`, `internal/cli` or `github.com/wailsapp/...`;
   - `builtins/` may import `engine/`, but `engine/` imports `builtins/` only in tests and examples.
8. **Nothing visible changes:** the app, the CLI, built sites, plugin and layout behaviour, trust and storage are untouched. The 39 service methods and their payloads are the same (the E1 guard tests still pass).

## Architecture

- **Done in three commits:**
  1. the module rename only (`go.mod`, a mechanical rewrite of imports, the ldflags path, the binding path);
  2. the package moves (`git mv`, imports rewritten);
  3. docs, `builtins` constructors, examples, the boundary and doc tests, and the CI job.
- Rewrite imports with `gofmt -r` or `go run golang.org/x/tools/cmd/gomvpkg` only if it is already in the module cache. Otherwise use a careful `sed` on import lines, checked by `go build ./... && go vet ./...`.
- `examples/demo` paths used by tests stay where they are (relative from the moved test files).

## Interfaces

```go
// builtins
func Plugins() *plugins.Registry                 // every built-in plugin, registered
func Layouts(version string) *layouts.Registry   // article, docs, post, landing, wide, handout, slides, course
func SiteTemplates() fs.FS                       // techdocs, learning, planner

// engine (doc only): package engine documents the layout of engine/* and links to session.
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | module path and bindings | `go build ./...`; `build/version_test.go` (ldflags path); `frontend` tsc and vitest |
| 2 | moves; atomicfile internal | `engine/boundary_test.go: TestPackageLayout` (a list of expected public and internal packages) |
| 3 | one list of built-ins | `builtins/builtins_test.go: TestDefaultSets` (ids of plugins, layouts and templates) |
| 4 | everything documented | `engine/doc_test.go: TestExportedAreDocumented` |
| 5 | the embed example runs | `engine/example_test.go: Example_embed` |
| 6 | apidiff job | `.github/workflows/ci.yml` (job `api`), plus `task api:check` (skips with instructions without the tool) |
| 7 | import boundaries | `engine/boundary_test.go: TestBoundaries` (`go list -deps`) |
| 8 | no visible change | all existing tests; the E1 guards `TestServiceMethodsUnchanged`, `TestPayloadShapes` |

## UI

None.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Exporting the sanitiser or trust code tempts embedders to skip them | `session` stays the front door; docs on `render` and `extplugins` say which checks a direct caller must keep; the trust store is never written without a prompter | `TestNilPrompterNeverAllows` (moved), doc review |
| A broken API slips into a release | apidiff in CI, with breaks declared | the CI `api` job |

## Notes

- **Owner first:** rename the repository on GitHub to `zenmd`. GitHub redirects the old URL; update the remote with `git remote set-url origin git@github.com:dragosh/zenmd.git`.
- **Out of scope:**
  - renaming packages (singular names, merging `index` into `workspace`): this is a move, not a redesign;
  - `runtime/` (E2b);
  - `kit/` (E2c).
- **Integration (lead):**
  - the changelog line;
  - mark E2a done in `docs/ENGINE.md` and `specs/README.md`;
  - a `v0.4.0` tag after release, so apidiff has a baseline.
- **Changelog:** `Changed (engine): the module is github.com/dragosh/zenmd; the engine packages are public under engine/ and the default plugins, layouts and site templates under builtins/. No visible change.`

## Prompt

> Implement `specs/engine/public-api/spec.md` following the rules in
> `specs/README.md`. Context:
> - The module is `github.com/dragosh/zen` today.
> - 91 Go files import `internal/core`.
> - `engine/session` already exists.
> - The bindings are generated by `wails3 generate bindings -clean=true -ts -i` (in `$(go env GOPATH)/bin`).
>
> Work in three commits: the module rename, then the moves, then docs, constructors, examples and tests. All tests stay green after each commit.
>
> Do not download tools (apidiff runs in CI). Do not launch the app, and don't run `task install`.
>
> Done when `go vet ./... && go test ./...` (after a frontend build), `go test -race ./engine/...`, `go test -tags appstore ./...`, `cd frontend && npx tsc --noEmit && npx vitest run` and `task package` + `ZEN_REQUIRE_BUNDLE=1 go test ./build/darwin` are clean. Report the final package list.
