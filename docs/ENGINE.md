# zenmd: the engine

Status: proposal (November 2026). This document plans the work before it is
specified in `specs/engine/*`.

zen.md becomes two layers:

```
zenmd                    the engine: format, rendering, plugins, layouts, build, print, templates, CLI
 ├─ zen.md (app)         reading, editing and publishing folders (today's app)
 └─ <language app>       lessons, speech, flashcards (later)
 └─ …                    more apps, all "built on zenmd"
```

The engine has no windows, menus or dialogs. Each app is a thin shell (Wails,
or anything else) over the engine's session API and frontend runtime.

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
| `internal/app/app.go`, `windows.go`, `menu.go`, `service.go` bindings, dialogs, `channel*`, bookmarks | Wails | **App** (zen.md) |
| `internal/cli` | One CLI for app and engine | **Engine CLI** (`zenmd`) plus a tiny app launcher |
| `frontend/src/plugins/runtime.ts`, `page/`, `static/`, `print/` | Mixed into the app bundle | **Engine runtime** (`@zenmd/runtime`), shared by apps and built sites |
| `frontend/src/shell`, `nav`, `editor`, `lint` UI | App | **App** (zen.md); a second app reuses what it wants |

## The engine's shape

```
github.com/dragosh/zen (module; renamed to …/zenmd when the repo is)
  engine/                       public Go API (was internal/core + the headless half of internal/app)
    session/                    Open, Page, Preview, Save, Watch, Trust, Build, Print, Templates; events as a Go interface
    format/                     render, frontmatter, directives, sanitising
    plugin/  layout/            manifests, registries, external and Tengo plugins, the API versions
    site/  templates/  lint/  workspace/  config/ …
  builtins/                     default plugins, layouts and site templates
  cmd/zenmd/                    the engine CLI: build, init, check, render, serve (later)
  runtime/                      @zenmd/runtime: page runtime, static runtime, print (TypeScript)
  apps/
    zenmd-app/                  today's zen.md app: Wails main, windows, menus, shell UI
    <next app>/
  spec/                         the format spec and the conformance fixtures
```

`internal/` stays for what no app should import (helpers, test rigs).

## Waves

| Wave | Spec | What | Done when |
|---|---|---|---|
| E1 | `engine/cli-rename` | The command and executable become `zenmd`, and `zen` stays as an alias for one minor release, printing a notice. Updates the CLI install, Taskfile, docs and demo. | `zenmd build/init/version` work; `zen` still works with a notice |
| E1 | `engine/session` | Extract the headless session from `internal/app` into `engine/session`, with no Wails import. The app's `Zen` service becomes a thin adapter (bindings, dialogs, events). | `internal/app` contains only Wails glue; the session has its own tests; behaviour unchanged |
| E2 | `engine/public-api` | Move `internal/core/*` and the builtins to `engine/*` and `builtins/*`. Document the Go API, add `apidiff` checks in CI, and add a small "embed zenmd" example program. | `go doc` reads well; an example renders a folder in 20 lines |
| E2 | `engine/runtime` | Split the frontend into `runtime/` (page, plugin runtime, static, print) and `apps/zenmd-app/frontend` (shell, nav, editor). One build of the runtime is used by the app and by `zenmd build`. | Built sites and the app load the same runtime; the app bundle imports it |
| E3 | `engine/format-spec` | Write `spec/`: frontmatter keys, directives, inline attributes, plugin blocks, layouts and links. Add conformance fixtures (`.md` in, normalised HTML out) and `zenmd check` for a folder (render warnings, broken links, Vale). | Every documented feature has a fixture; `zenmd check examples/demo` is clean |
| E3 | `engine/hardening` | Fuzz the renderer, sanitiser, frontmatter and template placeholders. Add benchmarks with budgets (open 1,000 pages, render 5 MB, build 1,000 pages), and race tests in CI. Errors carry a code and a fix hint. | Fuzzers run in CI without findings; the budgets are enforced |
| E4 | `engine/serve` (optional) | `zenmd serve`: a local preview server with live reload, for editors other than the app (VS Code, the terminal). | `zenmd serve examples/demo` previews in a browser and reloads on save |

After E3, the engine gets its own version (`zenmd 1.0.0-beta`) and apps depend on it.

## Versioning

- **Engine:** SemVer, starting at `0.x`. A breaking change to the Go API, the CLI, the format, the plugin API (`apiVersion`) or the layout API bumps the minor version before 1.0 and the major version after.
- **Apps:** their own versions, each naming the zenmd version it is built on, for example "zen.md 0.3 (zenmd 0.5)".
- **Content:** a folder may pin a minimum engine version in `.zen.yml` (`zenmd: ">=0.5"`), and older engines say so instead of rendering it wrongly.

## Open decisions

1. **One repository or several.** The recommendation is one repository for now, holding the engine, the apps and the spec, with one Go module. Split when a second app has its own release rhythm.
2. **The module and repository name.** Renaming `github.com/dragosh/zen` to `…/zenmd` is a GitHub change the owner makes; the code follows. It can wait until E2.
3. **The config folder.** `~/Library/Application Support/zen/` stays (a migration later, if ever), so existing settings, trust and plugin data keep working.
4. **App Store packaging** belongs to each app (packaging/appstore stays with zen.md); the engine stays free of app-only entitlements.
