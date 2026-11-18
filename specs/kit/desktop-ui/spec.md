# kit/desktop-ui

| | |
|---|---|
| Status | specified |
| Target version | zenmd 0.4.0 (app 0.4.0) |
| Branch | `feature/kit-desktop-ui` |
| Wave | 16 (E2c); runs **alone**: it moves the app and its frontend |
| Depends on | engine/public-api, engine/runtime |
| Owns | `internal/app/**` → `apps/zenmd-app/**` and `kit/desktop/**`, `frontend/**` → `apps/zenmd-app/frontend/**` and `kit/ui/**`, `main.go`, `Taskfile.yml`, `build/Taskfile.yml`, `build/darwin/Taskfile.yml`, `build/config.yml` (frontend path), `.github/workflows/ci.yml`, `.gitignore`, `runtime/package.json` and `runtime/vite.config.ts` (paths to the frontend's node_modules), docs paths in `docs/ENGINE.md` and `README.md` |

## Purpose

Add the middle layer of docs/ENGINE.md:
- **`kit/`** holds what any desktop app built on zenmd needs: the window shell in Go, and the shared UI in TypeScript;
- **`apps/zenmd-app/`** holds what makes zen.md itself: its main window, menu, `Zen` service, its dialogs and its frontend.

A second app then needs a main, a menu and its own features, and copies nothing. zen.md looks and behaves exactly the same.

## Behaviour

1. **Go: `kit/desktop`** (`github.com/dragosh/zenmd/kit/desktop`) takes the reusable parts of `internal/app`:
   - the window manager and close handshake (`windows.go`);
   - the event sink to Wails;
   - Wails dialogs as a `session.Prompter` and the folder pickers;
   - native print (`printnative_*.go`) and the print-window helper;
   - Finder and `open` (`finder.go`);
   - the App Store channel and bookmarks (`channel*.go`, `bookmarks_*`);
   - `devbuild`;
   - a `Run` helper: creating the Wails application, the main window, maximising, Open With / `ApplicationOpenedWithFile`, and the startup-editor wait.

   It imports `engine/*` and Wails, never `apps/`.
2. **Go: `apps/zenmd-app`** (package `app`, path `…/apps/zenmd-app`) takes the rest of `internal/app`:
   - `Run(Options)`, which uses `desktop.Run`;
   - the menu;
   - the `Zen` service, its aliases and its delegations;
   - New Site, Build Website, Print Page/Site and settings;
   - the CLI adapters (`BuildSite`, `InitSite`).

   It **embeds its own frontend** (`//go:embed all:frontend/dist`), so the root `main.go` no longer embeds anything.
3. **The root `main.go`** stays the `zenmd` binary: the script-host check, the alias notice, and `cli.Run(..., app.Launch, app.BuildSite, app.InitSite)`. It is about 25 lines and imports `apps/zenmd-app`. `internal/app` no longer exists.
4. **TS: `kit/ui`** (`@zenmd/kit`, the same alias setup as `@zenmd/runtime`) takes the reusable UI:
   - the DOM helpers, store and history;
   - the layout frame, theme and keys;
   - the opening loader and clipboard;
   - the sidebar (`nav/`), the page view (`page/`) and the editor (`editor/`);
   - the lint UI (`lint/`), the print preview and the settings frame.

   It imports `@zenmd/runtime` and its own files only. The api types it needs are an interface it declares (`KitHost`), which the app's api implements.
5. **TS: `apps/zenmd-app/frontend`** (`git mv frontend apps/zenmd-app/frontend`) keeps what is zen.md's:
   - `main.ts`, `brand.ts`, `api/` (Wails, fake, index);
   - `shell/shell.ts` (the composition), `features.ts`;
   - New Page, New Site and Build Website dialogs;
   - the plugin and layout help, and the Settings sections.

   `node_modules` stays one install, now at `apps/zenmd-app/frontend/node_modules`. `runtime/` and `kit/ui` use it through their configs, as `runtime/` does today.
6. **Build and CI paths follow:**
   - Wails `build/config.yml` and the Taskfiles point at `apps/zenmd-app/frontend`;
   - bindings are generated for `./apps/zenmd-app`;
   - `go build` still builds `.` (the root main) to `bin/zenmd`;
   - CI's `wails3 task test` and `darwin:package:dmg` keep working;
   - the `api` job's placeholder folders become `apps/zenmd-app/frontend/dist` and `runtime/dist`.

   `build/` (packaging) stays at the root for now.
7. **Nothing visible changes:**
   - the 39 service methods and payloads are unchanged (the E1 guard tests move with the app);
   - the demo build is byte for byte the same (`TestDemoBuildUnchanged`);
   - the app bundle's JS/CSS hashes may change (files moved), but the UI and behaviour don't;
   - all tests pass at their new homes.
8. **Boundaries, enforced by tests:**
   - Go: `engine/` and `runtime` import neither `kit/` nor `apps/`, and `kit/` does not import `apps/` (`engine/boundary_test.go`, extended);
   - TS: `runtime/` imports nothing else, `kit/ui` imports only `@zenmd/runtime`, and the app imports both (`kit/ui/test/boundary.test.ts`).

## Interfaces

```go
// kit/desktop
type Options struct {
	Name        string            // window title and app name
	Assets      fs.FS             // the app's frontend
	Session     *session.Session
	Services    []application.Service
	Menu        func(a *application.App, w *application.WebviewWindow) *application.Menu
	Dir, Page   string            // what to open at launch
	Mode        string            // read | edit
	OnStarted   func(a *application.App, w *application.WebviewWindow)
}
func Run(o Options) error
type Windows struct{ /* the window manager */ }
func Dialogs(a *application.App) *DialogSet // implements session.Prompter and the pickers
func NativePrint(w application.Window, title string) error
func Reveal(path string) error; func OpenFile(args ...string) error
```

```ts
// kit/ui (@zenmd/kit)
export interface KitHost extends RuntimeHost { /* page, source, preview, save, lint, openEditor, workspace events … as the kit needs */ }
export { createLayout, createStore, createHistory, createOpening, navFeature, pageFeature, editorFeature, lintView, … }
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1–3 | Go layout; `internal/app` gone; main is thin | `engine/boundary_test.go: TestPackageLayout` (updated), `TestBoundaries` (kit and apps rules) |
| 4–5 | TS layout and alias | `kit/ui/test/boundary.test.ts`; `tsc` in the app |
| 6 | build and CI paths | `task build`, `task package`, `ZEN_REQUIRE_BUNDLE=1 go test ./build/darwin`, `wails3 task test` |
| 7 | no visible change | `TestServiceMethodsUnchanged`, `TestPayloadShapes`, `TestDemoBuildUnchanged`, every moved test |

## UI

None (no change).

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A move drops a check (trust, close guard, CSP middleware) | Moves are `git mv` plus import rewrites; every existing test moves with its code | all tests, especially `middleware_test.go` and the windows and close tests |

## Notes

- **Out of scope:**
  - moving `build/` into the app;
  - `cmd/zenmd` as a separate CLI binary;
  - a second app.
- **Integration (lead):**
  - the changelog;
  - mark E2c done in ENGINE.md and specs/README.md;
  - after merge, the user runs `task install` again (the binary is the same, but the build paths moved).
- **Changelog:** `Changed (engine): kit/desktop (Go) and kit/ui (@zenmd/kit) hold the shared desktop shell; zen.md lives in apps/zenmd-app. No visible change.`

## Prompt

> Implement `specs/kit/desktop-ui/spec.md` following the rules in
> `specs/README.md`. Work in this order, with all tests green after each commit:
> 1. Go: `internal/app` → `apps/zenmd-app`, with no split yet.
> 2. Go: the reusable parts → `kit/desktop`.
> 3. `git mv frontend apps/zenmd-app/frontend`, and update every path (Taskfiles, Wails config, CI, runtime configs, .gitignore).
> 4. TS: the shared UI → `kit/ui`, with the alias and `KitHost`.
> 5. The boundary tests and docs.
>
> If the Wails binding generator or the Taskfiles fight the new layout, stop and report rather than inventing workarounds.
>
> No new dependencies, no downloads. Do not launch the app, and don't run `task install`.
>
> Done when these are clean:
> - `cd apps/zenmd-app/frontend && npx tsc --noEmit && npx vitest run`;
> - `task build`;
> - `go vet ./... && go test ./...`;
> - `go test -tags appstore ./...`;
> - `task package` + `ZEN_REQUIRE_BUNDLE=1 go test ./build/darwin`.
