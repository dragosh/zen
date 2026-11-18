# engine/runtime

| | |
|---|---|
| Status | done |
| Target version | zenmd 0.4.0 |
| Branch | `feature/engine-runtime` |
| Wave | 15 (E2b) |
| Depends on | engine/public-api |
| Owns | `runtime/` (new), the moved files from `frontend/src/{plugins/runtime.ts,static,print/ready.ts,print/page.ts,page/page.css,page/print.css,styles/tokens.css,styles/base.css}` and their tests, `frontend/{vite.config.ts,tsconfig.json,package.json scripts,vitest config}`, import paths in `frontend/src/**`, `engine/session` (default runtime), `main.go` and `internal/app/build.go` (`StaticAssets` wiring), `Taskfile.yml` / `build/Taskfile.yml` (build order), `.github/workflows/ci.yml` (only if the build order needs it), `docs/ENGINE.md` (the runtime section) |

## Purpose

The browser half of the engine gets its own home: **`runtime/`**, imported as
`@zenmd/runtime`. It mounts plugin and layout scripts on a page (the `ctx`
contract), runs built sites (`zen.js`, `zen.css`) and prepares pages for print.
The app and `zenmd build` use one build of it, so a page behaves the same in
the app, in a built site and in print. A Go package embeds the built runtime,
so the engine can build sites without the app's frontend.

## Behaviour

1. **What moves to `runtime/src/`:**

   | From `frontend/src/` | To `runtime/src/` |
   |---|---|
   | `plugins/runtime.ts` (+ tests) | `plugins.ts` (the `ctx` contract, `installRuntime`) |
   | `static/static.ts`, `static.css` (+ tests) | `static/` |
   | `print/ready.ts`, `print/page.ts` (+ tests) | `print/` |
   | `page/page.css`, `page/print.css` | `css/page.css`, `css/print.css` |
   | `styles/tokens.css`, `styles/base.css` | `css/tokens.css`, `css/base.css` |
   | `plugins/builtin-*.test.ts`, `demo-progress.test.ts` | `runtime/test/` (they test built-in scripts against the runtime) |

   Everything else stays in `frontend/` for now: the shell, page view, editor, nav, lint UI and print preview. E2c moves those to `kit/ui`.
2. **The runtime's own API types.** `runtime/src/types.ts` defines what the runtime needs from a host:
   - `RuntimeHost`: storage get/set/keys for plugins and layouts, invoke, stream, cancel, `openExternal` and `on` for the plugin events;
   - the `PluginCtx` and `LayoutCtx` types;
   - `OpenProgress`-free.

   `frontend/src/api` implements `RuntimeHost` (the `Api` type extends it). The runtime imports nothing from `frontend/`; a test enforces it.
3. **The package:** `runtime/package.json` (`name: "@zenmd/runtime"`, `private: true`, no dependencies) and `runtime/tsconfig.json`. The frontend imports it as `@zenmd/runtime` through a Vite alias and a `tsconfig` `paths` entry. There is still one `node_modules` (the frontend's) and no new npm packages.
4. **One build:** `runtime/vite.config.ts` builds `runtime/dist/zen.js` (classic IIFE, as `static.ts` today) and `runtime/dist/zen.css`.
   - `frontend`'s `npm run build` builds the runtime first (`npm --prefix ../runtime run build`, or the frontend's Vite with the runtime config, using the same `node_modules`).
   - The app bundle imports the runtime's sources, so they are tree-shaken into the app.
   - `frontend/dist/static/` is no longer produced.
5. **The Go side:**
   - new package `github.com/zenmark-org/zenmd/runtime` (`runtime/embed.go`, `//go:embed dist`), with `func FS() fs.FS` returning `zen.js` and `zen.css`;
   - `engine/session` uses it as the default `Options.Runtime` when none is given;
   - `main.go` no longer sets `app.StaticAssets` from `frontend/dist`, and `app.StaticAssets` goes away;
   - `zenmd build` works from a plain `go build` once `runtime/dist` exists (built by the Taskfile).
6. **Unchanged behaviour:** built sites, portable and hosted modes, print, `ctx.assetUrl`, storage keys and plugin mounting are byte-for-byte the same output for the demo (a test compares a build of `examples/demo` before and after: same files, same HTML).
7. **Tests:**
   - vitest runs both `frontend` and `runtime` tests (one config, or two projects); the count stays at 592 or more;
   - `go test ./runtime` checks `FS()` has both files;
   - the build boundary test (`engine/boundary_test.go`) also asserts that `runtime` (Go) imports only the standard library.

## Architecture

```
runtime/                       @zenmd/runtime (TypeScript) + Go embed
  src/plugins.ts types.ts static/ print/ css/
  test/                        built-in script tests (slides, course, quiz, codeblock, sandbox, progress)
  vite.config.ts  package.json  tsconfig.json
  dist/zen.js zen.css          built; git-ignored
  embed.go                     package runtime: FS()
frontend/                      the app UI (kit/ui after E2c), imports @zenmd/runtime
```

## Interfaces

```ts
// runtime/src/types.ts
export interface RuntimeHost { pluginStorageGet…; pluginStorageSet…; pluginStorageKeys…;
  layoutStorageGet…; layoutStorageSet…; layoutStorageKeys…; pluginInvoke…; pluginStream…; pluginCancel…;
  openExternal(url: string): Promise<boolean>; on(event: "plugin:stream" | "plugins:changed" | "layouts:changed", …): () => void }
export type { PluginCtx, LayoutCtx, PluginHandler, LayoutHandler } from "./plugins";
// runtime/src/index.ts re-exports installRuntime, whenReady, printPage, prepareForPrint, installPagePrint, start (static)
```

```go
// package runtime (github.com/zenmark-org/zenmd/runtime)
func FS() fs.FS // zen.js, zen.css
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1, 7 | the moved tests pass at their new home | vitest (frontend + runtime) |
| 2 | the runtime imports nothing from frontend | `runtime/test/boundary.test.ts` (scans imports) |
| 3–4 | alias and build: one runtime build, no `frontend/dist/static` | `frontend` tsc and vite build; `runtime/test/build.test.ts` (dist has `zen.js` as an IIFE without `import`) |
| 5 | Go embed and default runtime; `StaticAssets` gone | `runtime/embed_test.go: TestFS`; `engine/session` build tests pass without `Options.Runtime` |
| 6 | the demo builds identically | `engine/session/build_test.go: TestDemoBuildUnchanged` (golden file list and hashes, generated before the move) |

## UI

None.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Runtime behaviour drifts between the app and built sites | One build and one source; the demo build is compared byte for byte | `TestDemoBuildUnchanged` |

## Notes

- **Out of scope:**
  - `kit/ui` and the app's move to `apps/` (E2c);
  - publishing `@zenmd/runtime` to npm.
- **Integration (lead):**
  - the changelog;
  - mark E2b done in ENGINE.md and specs/README.md;
  - the CI job keeps working (`wails3 task test` builds the runtime through the frontend build).
- **Changelog:** `Changed (engine): the browser runtime (plugin mounting, built sites, print) is runtime/ (@zenmd/runtime), embedded in Go as package runtime; the app and zenmd build use one build of it. No visible change.`

## Prompt

> Implement `specs/engine/runtime/spec.md` following the rules in
> `specs/README.md`. Before moving anything, generate the golden build of
> `examples/demo` for `TestDemoBuildUnchanged`.
>
> Then work in this order:
> 1. Move the files with `git mv`, add the alias, and get all tests green.
> 2. Add `RuntimeHost` and the boundary test.
> 3. Add the Go embed and the default runtime, and remove `StaticAssets`.
>
> No new npm packages, no downloads. Do not launch the app, and don't run `task install`.
>
> Done when these are clean:
> - `cd frontend && npx tsc --noEmit && npx vitest run`;
> - `task build`;
> - `go vet ./... && go test ./...`;
> - `go test -tags appstore ./...`;
> - `task package` + `ZEN_REQUIRE_BUNDLE=1 go test ./build/darwin`.
