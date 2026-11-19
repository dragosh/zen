# zenmd: the engine

Status: E1, E2a, E2b and E2c done (November 2026): the command is `zenmd`, the module is `github.com/zenmark-org/zenmd`, the engine is public under `engine/`, the built-ins under `builtins/`, and the browser runtime is `runtime/` (`@zenmd/runtime`, embedded in Go), the desktop shell is `kit/` and Zenmark is `apps/zenmark`. This document plans the rest before it is specified in `specs/engine/*`.

Zenmark becomes two layers:

```
zenmd                    the engine: format, rendering, plugins, layouts, build, print, templates, CLI
 ├─ Zenmark (app)         reading, editing and publishing folders (today's app)
 └─ <language app>       lessons, speech, flashcards (later)
 └─ …                    more apps, all "built on zenmd"
```

The engine has no windows, menus or dialogs. Between the engine and the apps
sits a shared **kit**: the desktop shell parts every app needs (windows,
dialogs, the close handshake, the editor, the sidebar, settings). An app is
then its main, its menu and its own features.

```
apps/      Zenmark, <language app>, …          what makes each product itself
kit/       desktop (Go, Wails) + ui (TS)       shared desktop shell
engine/    + runtime/ + builtins/ + cmd/zenmd  zenmd: no windows, no Wails
```

Imports only point down: apps → kit → engine. The engine never imports the
kit or an app, and the kit never imports an app; tests enforce both.

## Goals

1. **One engine, many apps.** An app is a main, a window shell and its own
   features; everything that turns a folder into pages lives in zenmd.
2. **A format people can rely on.** The zenmd syntax (frontmatter, directives,
   plugin blocks, layouts) is written down, versioned and tested by a
   conformance suite, so content written today renders the same tomorrow.
3. **Stable APIs.** Go packages, the plugin API, the layout API and the CLI are
   versioned with SemVer, independent of any app's version.
4. **Solid before wide.** Fuzzed and benchmarked, with clear errors, before a
   second app is built on it.

## Where things are today

| Part | Today | Engine or app |
|---|---|---|
| `internal/core/*` (workspace, config, index, render, plugins, extplugins, scriptplugins, layouts, lint, watch, site, sitetemplates, atomicfile) | No dependency on the app | **Engine** (already clean) |
| `internal/plugins/*`, `internal/layouts/*`, `internal/sitetemplates/*` | Built-ins | **Engine** (default set; apps may add or leave out) |
| `internal/app/state.go`, `opening.go`, `build.go`, `buildui.go` (core part), `print.go` (document), `extplugins.go`, `layouts.go`, `stream.go`, `assets.go`, `slugs.go`, `sitetemplates.go` (core part) | Mixed with Wails types and events | **Engine:** becomes `session`, a headless folder session |
| `internal/app/windows.go`, dialogs, native print, Finder, `channel*`, bookmarks, the `Run` helper | Wails | **Kit** (`kit/desktop`) |
| `internal/app/app.go`, `menu.go`, `service.go` bindings, settings, New Site, Build Website | Wails | **App** (`apps/zenmark`) |
| `internal/cli` | One CLI for app and engine | **Engine CLI** (`zenmd`) plus a tiny app launcher |
| `frontend/src/plugins/runtime.ts`, `page/`, `static/`, `print/` | Mixed into the app bundle | **Engine runtime** (`@zenmd/runtime`), shared by apps and built sites |
| `frontend/src/shell` (frame, store, theme, keys, opening, settings frame), `nav`, `page`, `editor`, `lint`, `print/preview` | App | **Kit** (`kit/ui`, `@zenmd/kit`) |
| `frontend/src/main.ts`, `api/`, `shell/shell.ts`, `features.ts`, the dialogs and Settings sections | App | **App** (`apps/zenmark/frontend`) |

## The engine's shape

```
github.com/zenmark-org/zenmd (module)
  engine/                       public Go API (was internal/core + the headless half of internal/app)
    session/                    Open, Page, Preview, Save, Watch, Trust, Build, Print, Templates; events as a Go interface
    format/                     render, frontmatter, directives, sanitising
    plugin/  layout/            manifests, registries, external and Tengo plugins, the API versions
    site/  templates/  lint/  workspace/  config/ …
  builtins/                     default plugins, layouts and site templates
  cmd/zenmd/                    the engine CLI: build, init, check, render, serve (later)
  runtime/                      @zenmd/runtime: page runtime, static runtime, print (TypeScript)
  kit/
    desktop/                    Go: Run helper, window manager + close handshake, dialogs as a Prompter,
                                events to the frontend, print window, Finder, App Store channel, bookmarks
    ui/                         TS: frame, sidebar, editor, settings, dialogs, the opening loader, lint UI
  apps/
    zenmark/                  today's Zenmark app: main, menu, the Zen service, its features, packaging
    <next app>/
  spec/                         the format spec and the conformance fixtures
```

`internal/` stays for what no app should import (helpers, test rigs).

### `engine/session` (E1, built)

`engine/session` is the headless folder session. It imports no Wails, app or
CLI package (a test enforces it).

| Part | Where |
|---|---|
| `Session`: `New`, `Open`, `CancelOpen`, `Close`, `Workspace`, `Home`, `Page`, `Preview`, `Source`, `Save`, `CreatePage`, `Lint` | `session.go`, `opening.go`, `newpage.go`, `lint.go` |
| `SetLinter(find func() TextLinter)`: the text linter is installed by an add-on (`addons/vale`); without one `Lint` is unavailable and nothing is looked up. `Startup.Addons` (`AddonInfo`) lists the app's add-ons | `lint.go`, `session.go` |
| `builtins.PluginsFor`, `LayoutsFor`, `SiteTemplatesFor`: the built-ins an app assembly lists (kit/assembly); `requires:` in `template.yml` and `layout.yml` declares what a template or layout needs. `Startup.Build` (`BuildInfo`): the app, add-ons, plugins, layouts, templates and theme. `SiteEnv.Layouts`: the layouts a site build uses | `builtins/builtins.go`, `session.go`, `build.go` |
| Plugins and layouts: trust, storage, invoke, stream, cancel | `extplugins.go`, `layouts.go`, `stream.go` |
| `Handler()`: `/_ws/`, `/_plugins/`, `/_layouts/`, `/_print/`; anything else is 404 | `assets.go`, `print.go` |
| `Build`, `BuildOptions`, `SaveBuildDefaults`, `BuiltSite`; `WriteSite` is `zenmd build` | `build.go`, `buildui.go` |
| `PrintDocument`, `Templates()` (list, plan and create a site, update), config file paths | `print.go`, `sitetemplates.go`, `config.go` |
| Test helpers shared with apps: a folder tree, an event sink, a prompter | `sessiontest/` |

An app gives the session two optional things: `Events` (the app forwards them
to its frontend) and `Prompter` (the app implements it with native dialogs; a
nil prompter never asks and never allows). `Options.Watch` turns live reload
on, `Options.Runtime` is the browser runtime (`zen.js`, `zen.css`) that `Build`
and `WriteSite` copy into a site; nil means the embedded Go package `runtime`
(see below).

### The browser runtime (`runtime/`)

`runtime/` is `@zenmd/runtime`: the `ctx` contract for plugin and layout
scripts (`src/plugins.ts`), the runtime of built sites (`src/static/`), print
(`src/print/`) and the page, token and base CSS (`src/css/`). It needs a host
that implements `RuntimeHost` (`src/types.ts`): the app's `Api` extends it, a
built site gives it localStorage. It imports nothing from `frontend/`
(`runtime/test/boundary.test.ts`) and has no dependencies of its own; there is
one `node_modules`, the app frontend's (`apps/zenmark/frontend`), and the frontend imports the package
through a Vite alias and a `tsconfig` `paths` entry.

One build serves everyone: `npm run build` in `runtime/` (the frontend's build
runs it first) writes `runtime/dist/zen.js`, a classic IIFE, and `zen.css`.
The Go package `github.com/zenmark-org/zenmd/runtime` embeds them (`FS()`), which
is `engine/session`'s default `Options.Runtime`, so `zenmd build` needs no
frontend. `runtime/dist` is git-ignored: build it (`task build:frontend`)
before any Go build or test.

Zenmark's `apps/zenmark` keeps `Run`, the menu and the `Zen` service; the window
manager and close handshake, the pickers, Finder and the App Store channel are
`kit/desktop`. Its `Zen` service is one-line
delegations to the session, and the JSON types are aliases of the session's,
so the frontend bindings keep their method names and payloads.

A process may hold several sessions. Shared on purpose or by the packages the
session builds on: the Vale process limit, the script plugin helper pool
(closing a session retires the idle helpers of every session) and the storage
file locks.

## The Go API (E2a, built)

The module is `github.com/zenmark-org/zenmd`. What a program may import:

| Package | What |
|---|---|
| `engine/session` | the front door: open a folder, render, save, build, print |
| `engine/{workspace,config,index,render,plugins,extplugins,scriptplugins,layouts,site,sitetemplates,lint,watch}` | the parts session is built on |
| `engine/session/sessiontest` | test helpers for apps |
| `builtins` | the default set: `Plugins()`, `Layouts(version)`, `SiteTemplates()` |
| `builtins/{layouts,sitetemplates}`, `builtins/plugins/{check,codeblock,diagram,quiz,sandbox}` | the pieces of that set |

Private, under `internal/`: `cli` (the command) and `atomicfile` (a helper). The
Zenmark app is `apps/zenmark`. `engine/boundary_test.go` pins this list and the
rules: nothing under `engine/` or `builtins/` imports `apps/`, `kit/`,
`internal/cli` or Wails; `engine/` imports `builtins/` only in tests and
examples, with one exception: `engine/session` uses `builtins` for the
default set when an app gives it no registries.

Every exported name has a doc comment (`engine/doc_test.go` walks the AST), and
`go doc ./engine` is the map. `engine/example_test.go` (`Example_embed`) opens
`examples/demo`, lists the sidebar, renders a page and builds a site in about
20 lines. CI runs `apidiff` over `engine/...` and `builtins/...` against the
latest `v*` tag; an incompatible change fails unless a commit message or the
PR body says `api-break: <reason>`. `task api:check` does the same locally
when `apidiff` is installed. Before the first tag the job only reports.

## The kit and the app (E2c, built)

`kit/desktop` (Go, `github.com/zenmark-org/zenmd/kit/desktop`) is the window shell:
`Run(Options)` (the Wails application, the main window, maximising, Open With
and the startup hook), `Windows` (the window manager and the close handshake),
`Sink` (events to the frontend), `DialogSet` (native dialogs as a
`session.Prompter` and the folder pickers), `NativePrint` and `PrintWindows`,
`Reveal`/`OpenFile` (Finder and `open`), the App Store channel and bookmarks,
and `DevBuild`. It imports the engine and Wails, never `apps/`.

`kit/ui` (`@zenmd/kit`, one Vite alias and `tsconfig` path like the runtime) is
the shared UI: the frame, store, history, theme, keys, opening loader and
clipboard (`shell/`), the sidebar (`nav/`), the page view (`page/`), the editor
(`editor/`), the lint UI (`lint/`), the print preview (`print/`) and the
Settings frame with its Appearance section. It imports `@zenmd/runtime` and its
own files only; what it needs from an app is the `KitHost` interface it
declares in `src/host.ts`, which the app's `Api` extends. The kit's tests use
the app's fake api as a ready-made host, as the runtime's tests do.

`apps/zenmark` is Zenmark: `Run`, the menu, the `Zen` service with its
aliases and one-line delegations, New Site, Build Website, Print Page and Site,
settings, the CLI adapters (`BuildSite`, `InitSite`, `Launch`) and its own
frontend in `apps/zenmark/frontend` (main, brand, the Wails and fake api,
`shell/shell.ts` as the composition, the New Page, New Site and Build Website
dialogs, the plugin and layout help, the Settings sections). It embeds its
frontend (`//go:embed all:frontend/dist`); the root `main.go` is the `zenmd`
binary and only calls `app.Launch`, `app.BuildSite` and `app.InitSite`.
`build/` (packaging) stays at the root. `node_modules` is one install, in
`apps/zenmark/frontend`; `runtime/` and `kit/ui` use it through their
configs.

Tests pin the direction: `engine/boundary_test.go` (nothing under `engine/` or
`builtins/` imports `kit/` or `apps/`; `kit/` does not import `apps/`) and
`kit/ui/test/boundary.test.ts` (`runtime/` imports nothing else, `kit/ui`
imports only `@zenmd/runtime`, the app imports both).

## Waves

| Wave | Spec | What | Done when |
|---|---|---|---|
| E1 ✓ | `engine/cli-rename` | The command and executable become `zenmd`, and `zen` stays as an alias for one minor release, printing a notice. Updates the CLI install, Taskfile, docs and demo. | `zenmd build/init/version` work; `zen` still works with a notice |
| E1 ✓ | `engine/session` | Extract the headless session from `internal/app` into `engine/session`, with no Wails import. The app's `Zen` service becomes a thin adapter (bindings, dialogs, events). | `internal/app` contains only Wails glue; the session has its own tests; behaviour unchanged |
| E2a ✓ | `engine/public-api` | Rename the module to `github.com/zenmark-org/zenmd`; move `internal/core/*` to `engine/*` and the built-ins to `builtins/*`; document the Go API, add an `apidiff` check in CI and an "embed zenmd" example. | `go doc` reads well; an example renders a folder in 20 lines |
| E2b ✓ | `engine/runtime` | Split the frontend into `runtime/` (page, plugin runtime, static, print) and the app's UI. One build of the runtime is used by the app and by `zenmd build`. | Built sites and the app load the same runtime; the app bundle imports it |
| E2c ✓ | `kit/desktop` + `kit/ui` | Move the reusable desktop parts out of `internal/app` and `frontend/src` into `kit/`; today's app becomes `apps/zenmark` (main, menu, `Zen` service, its features, packaging). | A second app is a main, a menu and its features; Zenmark behaves the same |
| E3 | `engine/format-spec` | Write `spec/`: frontmatter keys, directives, inline attributes, plugin blocks, layouts and links. Add conformance fixtures (`.md` in, normalised HTML out) and `zenmd check` for a folder (render warnings, broken links, Vale). | Every documented feature has a fixture; `zenmd check examples/demo` is clean |
| E3 | `engine/hardening` | Fuzz the renderer, sanitiser, frontmatter and template placeholders. Add benchmarks with budgets (open 1,000 pages, render 5 MB, build 1,000 pages), and race tests in CI. Errors carry a code and a fix hint. | Fuzzers run in CI without findings; the budgets are enforced |
| E4 | `engine/serve` (optional) | `zenmd serve`: a local preview server with live reload, for editors other than the app (VS Code, the terminal). | `zenmd serve examples/demo` previews in a browser and reloads on save |

After E3, the engine gets its own version (`zenmd 1.0.0-beta`) and apps depend on it.

## Versioning

- **Engine:** SemVer, starting at `0.x`. A breaking change to the Go API, the CLI, the format, the plugin API (`apiVersion`) or the layout API bumps the minor version before 1.0 and the major version after.
- **Apps:** their own versions, each naming the zenmd version it is built on, for example "Zenmark 0.3 (zenmd 0.5)".
- **Content:** a folder may pin a minimum engine version in `.zen.yml` (`zenmd: ">=0.5"`), and older engines say so instead of rendering it wrongly.

## Decisions

1. **One repository**, holding the engine, the kit, the apps and the spec, with one Go module. Split when a second app has its own release rhythm.
2. **The repository is renamed to `zenmd`** on GitHub by the owner; the module path follows in E2a (`github.com/zenmark-org/zenmd`).
3. **A kit layer** between the engine and the apps (E2c), so a second app does not copy Zenmark's shell.
4. **The config folder.** `~/Library/Application Support/zen/` stays (a migration later, if ever), so existing settings, trust and plugin data keep working.
5. **App Store packaging** belongs to each app (packaging/appstore stays with Zenmark); the engine stays free of app-only entitlements.
