# engine/session

| | |
|---|---|
| Status | specified |
| Target version | 0.3.0 (engine `zenmd` 0.3.0) |
| Branch | `feature/engine-session` |
| Wave | 13 (E1) |
| Depends on | everything in `internal/app` today (waves 3–12) |
| Owns | `engine/session/` (new), `internal/app` (all files except `main.go`, which it does not touch), `frontend/src/api/wails.ts` (only if generated binding paths change), `docs/ENGINE.md` (the session section) |

## Purpose

Pull the **headless folder session** out of the Wails app into a public Go
package, `engine/session`. It has no windows, menus, dialogs or Wails import.
Today's zen.md app keeps working exactly as before, as a thin adapter over the
session. A second app, a test, or a tool like `zenmd serve` can then open a
folder, render pages, run plugins, build, print and make sites without the GUI.

## Behaviour

1. **No visible change:**
   - every service the frontend calls behaves as before;
   - events have the same names and payloads;
   - the generated bindings keep the same method names;
   - all existing tests still pass, moved or not.
2. `engine/session` imports no `github.com/wailsapp/...` package, nothing from `internal/app`, and nothing from `internal/cli`. It may import `internal/core/*` and the built-ins (moved to `engine/*` in E2).
3. **What moves into the session** (from `internal/app`):
   - opening a folder (progress, cancel, timeout, rollback, async watch);
   - the index and slugs;
   - page, preview, source, save, create page;
   - external and script plugins (trust, prompts through an interface, invoke, stream, storage);
   - layouts (trust, storage);
   - lint;
   - the asset handler (`/_ws/`, `/_plugins/`, `/_layouts/`, `/_print/`);
   - build (`BuildSite` core, progress, cancel, remembered options);
   - the print document;
   - site templates (list, create, update, dir);
   - the config file paths.
4. **What stays in `internal/app`:**
   - `Run`, windows, menus, the close handshake;
   - Wails dialogs (they implement the session's `Prompter`);
   - the `Zen` service (one-line delegations to the session plus the GUI-only actions: `OpenEditor`, `OpenConfig`, `OpenExternal`, the pickers, Print Page/Site windows, Reveal/Open in Finder);
   - the App Store channel and bookmarks;
   - `devBuild`;
   - `BuildSite` and `InitSite` for the CLI, as small adapters from `cli.Build` and `cli.Init` to the session.
5. **Events go out through an interface** (`session.Events`). The app forwards them to Wails. A nil `Events` is allowed, as in tests and the CLI.
6. **Questions to the user** (allow a plugin or layout, import a config) go through `session.Prompter`. A nil prompter means "never ask, decide nothing", which is today's headless build behaviour.
7. **Lifecycle:** `session.New(opts)` creates a session; `Close()` stops the watcher, plugin calls, streams and script helpers, as `Zen.close` does today. A session can open several folders one after another, as the app does.
8. **The JSON types** (`Workspace`, `NavNode`, `Crumb`, `Page`, `Startup`, `PluginInfo`, `LayoutInfo`, `OpenProgress`, `BuildOptions`, `BuildResult`, `SiteTemplate`, …) are defined in `engine/session`. `internal/app` re-exports them with type aliases, so the Wails models and `frontend/src/api/wails.ts` stay as they are. If the generator puts models under a new path, only import paths in `wails.ts` change.
9. **`engine/session` has a package doc and a runnable example** (`Example_renderFolder`): open `examples/demo`, list the tree, render `README.md`, build to a temporary folder. It runs in `go test`.

## Architecture

```
internal/app (Wails)                      engine/session (headless)
  Run, windows, menus  ─┐                   Session
  Zen service ──────────┼─ delegates ──►     Open/CancelOpen/Close
  wailsDialogs ─────────┼─ implements ─►     Prompter
  appSink ──────────────┘─ implements ─►     Events
                                              Page/Preview/Source/Save/CreatePage
                                              Plugins/Trust/Invoke/Stream/Storage
                                              Layouts/Trust/Storage
                                              Lint, Handler(), Build, PrintDocument, Templates
```

- `state` becomes `Session`, an unexported-fields struct with exported methods. Today's `state` methods keep their names and become exported where the app needs them.
- Event constants move to `session` (`session.EventWorkspaceChanged`, …), with aliases in `internal/app`.
- Test helpers (`tree`, the sinks, the fake dialogs) move to `engine/session/sessiontest` so both packages can use them.
- The move is mechanical first: one commit that moves files and fixes imports with no behaviour change, then commits that tidy the API. That keeps review possible.

## Interfaces

```go
package session

type Events interface{ Emit(name string, data any) }
type Prompter interface {
	Confirm(message string) bool
}
type Options struct {
	GlobalPath  string            // config.yml; "" for none
	ConfigDir   string            // trust, plugin data, user plugins/layouts, templates; default: dir of GlobalPath
	Registry    *plugins.Registry // built-in plugins; nil = the default set
	Layouts     *layouts.Registry // built-in layouts; nil = the default set
	Events      Events            // may be nil
	Prompter    Prompter          // may be nil: never asks
	Runtime     fs.FS             // the static runtime (frontend/dist) for Build and Print
	Watch       bool              // live reload (the app: true; the CLI: false)
	Version     string            // asset versioning
}
func New(o Options) *Session
func (s *Session) Open(dir string) error          // progress events, cancel, timeout, rollback
func (s *Session) CancelOpen()
func (s *Session) Close()
func (s *Session) Workspace() *Workspace
func (s *Session) Home() string
func (s *Session) Page(path string) (Page, error)
func (s *Session) Preview(path, content string) (Page, error)
func (s *Session) Source(path string) (string, error)
func (s *Session) Save(path, content string) error
func (s *Session) CreatePage(name, layout string) (string, error)
func (s *Session) Plugins() []PluginInfo
func (s *Session) PluginTrust(id string, allow bool) error
func (s *Session) Layouts() []LayoutInfo
func (s *Session) LayoutTrust(id string, allow bool) error
// storage, invoke, stream, cancel: today's signatures
func (s *Session) Lint(path, content string) LintResult
func (s *Session) Handler() http.Handler           // /_ws/ /_plugins/ /_layouts/ /_print/; others → 404
func (s *Session) Build(ctx context.Context, o BuildOptions, progress func(done, total int)) (BuildResult, error)
func (s *Session) PrintDocument(o PrintOptions) (string, error)
func (s *Session) Templates() TemplatesAPI          // List, Create, Dir, Update

const EventWorkspaceChanged, EventPageChanged, EventWorkspaceOpening, EventPluginsChanged, EventLayoutsChanged, EventBuildProgress /* … */ = …
```

```go
package app // after
type Zen struct {
	s *session.Session
	// GUI-only fields (windows, dialogs, open URL, launch page/mode)
}
func (z *Zen) Page(p string) (Page, error) { return z.s.Page(p) } // and so on
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | no visible change | every existing test in `internal/app`, moved to `engine/session` where it tests the session, passes unchanged; `frontend` vitest passes; the bindings diff contains no method changes (`internal/app/bindings_test.go: TestServiceMethodsUnchanged` against a list of today's 38 methods) |
| 2 | no Wails, app or cli imports | `engine/session/deps_test.go: TestNoGUIImports` (`go list -deps`) |
| 3–4 | the split | `internal/app/thin_test.go: TestAppIsThin` (each `Zen` method that has a session counterpart is a one-line delegation; source scan) |
| 5 | events through the interface; nil allowed | `engine/session/events_test.go: TestNilEvents`, `TestEventsForwarded` |
| 6 | the prompter; nil never asks | `engine/session/trust_test.go: TestNilPrompterNeverAllows` |
| 7 | Close stops everything; several opens in a row | `engine/session/lifecycle_test.go: TestCloseStopsWork`, `TestOpenTwice` (with `-race`) |
| 8 | JSON shapes unchanged | `internal/app/json_test.go: TestPayloadShapes` (golden JSON of Startup, Page, PluginInfo, LayoutInfo, BuildResult, SiteTemplate) |
| 9 | the example runs | `engine/session/example_test.go: Example_renderFolder` |

Edge cases to cover:
- the app quitting while an open, build or stream is running;
- two sessions in one process, so no package-level state is shared (today: `scriptplugins.Reset`, the print window map, `macOpen`), unless documented;
- the CLI build and the app build giving identical output.

## UI

None. The app must look and behave the same; check it by hand once after merge (open the demo, edit, build, print).

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Trust decisions bypassed in the move | The same trust code moves unchanged; a nil prompter allows nothing | `TestNilPrompterNeverAllows` and the moved trust and hardening tests |
| The asset handler widens what it serves | `Handler()` serves exactly the four routes, with the same checks; anything else is 404 | `engine/session/handler_test.go: TestHandlerRoutes` and the moved assets tests |

## Notes

- **Out of scope:**
  - moving `internal/core/*` to `engine/*` (E2);
  - the frontend runtime split (E2);
  - any behaviour change.
- **Integration (done by the lead at merge):**
  - `docs/ENGINE.md`: mark E1 session as done;
  - `docs/PLAN.md` "Service API": the session's Go API next to the frontend contract;
  - the changelog;
  - `specs/README.md`.
- **App Store / sandbox:** unchanged; the channel code stays in the app.
- **Changelog:** `Changed (engine): the folder session (opening, rendering, plugins, layouts, build, print, templates) is a headless Go package, engine/session; the app is a thin layer over it. No visible change.`

## Prompt

> Implement `specs/engine/session/spec.md` following the rules in
> `specs/README.md`. Context:
> - Only `internal/app/{app,menu,windows,print}.go` import Wails today.
> - `state` (`state.go`, `opening.go`) is the session; the other non-Wails files are its features.
> - `EventSink` and `Dialogs` are already interfaces.
>
> Work in this order:
> 1. Add the guard tests (deps, thin app, unchanged service methods, payload shapes) and commit them failing.
> 2. Do the mechanical move into `engine/session` (files, imports, aliases) with all existing tests passing, and commit.
> 3. Tidy the exported API and package doc, add the example, and commit.
>
> Don't change behaviour, event names or payloads. Don't touch `main.go`, `internal/cli` or docs other than ENGINE.md (another branch renames the CLI in parallel). Don't launch the app.
>
> Done when `go vet ./... && go test ./...` (after `cd frontend && npx vite build`, or with the dist stub), `go test -race ./engine/session ./internal/app`, `go test -tags appstore ./...` and `cd frontend && npx tsc --noEmit && npx vitest run` are clean. Report:
> - the final file split;
> - anything package-level that can't yet be per-session.
