# Specs

Every feature is specified here before it is built. One feature = one
`specs/<scope>/<feature>/spec.md` (copy of [`_template/spec.md`](_template/spec.md))
= one branch `feature/<scope>-<feature>` = one `git worktree` = one subagent.

Read first: [docs/PLAN.md](../docs/PLAN.md), [docs/UI.md](../docs/UI.md),
[docs/PLUGINS.md](../docs/PLUGINS.md), [docs/VERSIONING.md](../docs/VERSIONING.md).

## Features

| Wave | Spec | Owns | Depends on | Status |
|---|---|---|---|---|
| 0 | [cli/entrypoint](cli/entrypoint/spec.md) | `internal/cli` (superseded by cli/commands) | – | done |
| 1 | [core/workspace](core/workspace/spec.md) | `internal/core/workspace` | – | done |
| 1 | [core/config](core/config/spec.md) | `internal/core/config` | – | done |
| 1 | [core/plugins](core/plugins/spec.md) | `internal/core/plugins` | – | done |
| 1 | [ui/shell](ui/shell/spec.md) | `frontend/src/{main.ts,api,shell,styles}`, `frontend/index.html` | – | done |
| 1 | [packaging/version](packaging/version/spec.md) | `VERSION`, `CHANGELOG.md`, `build/**/Taskfile.yml`, `Taskfile.yml` | – | done |
| 2 | [core/render](core/render/spec.md) | `internal/core/render` | core/plugins, core/config, core/workspace | done |
| 2 | [core/index](core/index/spec.md) | `internal/core/index` | core/workspace | done |
| 2 | [core/watch](core/watch/spec.md) | `internal/core/watch` | core/workspace | done |
| 2 | [cli/commands](cli/commands/spec.md) | `internal/cli`, `main.go` (+ `Options` fields in `internal/app/app.go`) | packaging/version | done |
| 2 | [ui/navigation](ui/navigation/spec.md) | `frontend/src/nav` | ui/shell | done |
| 2 | [ui/page](ui/page/spec.md) | `frontend/src/page` | ui/shell | done |
| 2 | [ui/editor](ui/editor/spec.md) | `frontend/src/editor` | ui/shell | done |
| 3 | [plugins/codeblock](plugins/codeblock/spec.md) (was highlight) | `internal/plugins/codeblock` | core/render | done |
| 3 | [plugins/diagram](plugins/diagram/spec.md) | `internal/plugins/diagram` | core/render | done |
| 3 | [app/services](app/services/spec.md) | `internal/app`, `frontend/src/api/wails.ts` | all core, cli/commands | done |
| 4 | [packaging/macos](packaging/macos/spec.md) | `build/darwin` | everything | done |
| 5 | [ui/editor-window](ui/editor-window/spec.md) | `internal/app/{app,windows,service}.go`, `frontend/src/{api,shell,editor}`, page Edit button | app/services, ui/* | done |
| 5 | [core/navmeta](core/navmeta/spec.md) | `internal/core/index`, `internal/app/{state,slugs}.go` | core/index, app/services | done |
| 5 | [core/lint](core/lint/spec.md) | `internal/core/lint`, `resources.yml`, `build/darwin` helper fetch | packaging/macos | done |
| 5b | [ui/lint](ui/lint/spec.md) | `internal/app/lint.go`, `frontend/src/lint`, page/editor wiring | ui/editor-window, core/lint | done |
| 6 | [core/extplugins](core/extplugins/spec.md) | `internal/core/{extplugins,plugins,render}`, `internal/app`, demo plugins `note`, `calc` | core/plugins, core/render | done |
| 6 | [ui/plugins](ui/plugins/spec.md) | `frontend/src/plugins`, api, page wiring, Settings › Plugins, demo how-to and `progress` plugin | ui/page, contract in docs/PLUGINS.md | done |
| 7 | [core/plugin-extras](core/plugin-extras/spec.md) | actions and streams for built-ins and external plugins, `ctx.stream/onAnchor/badge/storage.keys`, user-scoped options, ⌘P | wave 6 | done |
| 8 | [plugins/layouts](plugins/layouts/spec.md) | `internal/plugins/{slides,course}`, demo pages | core/plugin-extras | done |
| 8 | [plugins/quiz](plugins/quiz/spec.md) | `internal/plugins/quiz`, demo page | core/plugin-extras | done |
| 8 | [plugins/sandbox](plugins/sandbox/spec.md) | `internal/plugins/sandbox`, demo page | core/plugin-extras | done |
| 9 | [core/plugin-hardening](core/plugin-hardening/spec.md) | fixes for the Nov 2026 plugin review | waves 6–8 | done |
| 9 | [core/script-plugins](core/script-plugins/spec.md) | one-file Tengo plugins, run in a helper process | core/plugin-hardening | done |
| 10 | [core/layouts](core/layouts/spec.md) | layouts as their own concept: `.zen/layouts/<id>/layout.yml`, Go templates, built-ins in `internal/layouts` | core/script-plugins | done |
| 9 | [ui/new-page](ui/new-page/spec.md) | `internal/app/newpage.go`, `frontend/src/shell/newpage.ts`, File › New Page | app/services, ui/shell | done |
| 11 | [core/static-build](core/static-build/spec.md) | `zen build`: folder → static website (hosted, or `--portable` for file://); `internal/core/site`, `internal/app/build.go`, `internal/cli` build command, `build:` config, `frontend/src/static` | core/layouts, cli/commands | done |
| 11 | [app/menu](app/menu/spec.md) | slim native menu: `internal/app/menu.go`, ⌘W in `frontend/src/shell/keys.ts` | ui/new-page, ui/editor-window | done |
| 12 | [packaging/appstore](packaging/appstore/spec.md) | zen DMG art and icons (no Wails), Developer ID + Mac App Store channels: `build/darwin/**`, `build/config.yml`, `internal/app/channel*.go`, `internal/core/extplugins/channel*.go` | packaging/macos, app/menu | done |
| 12 | [core/init](core/init/spec.md) | site templates: `zen init site --techdocs|--learning|--planner`, editable presets in the templates folder, welcome cards, File › New Site from Template…; `internal/core/sitetemplates`, `internal/sitetemplates`, `internal/app/sitetemplates.go`, `frontend/src/shell/newsite.ts` | core/layouts, app/menu, ui/new-page | done |
| 12 | [ui/print](ui/print/spec.md) | Print Page (⌘P) and Print Site (⇧⌘P, one document with cover and contents), `print.html` in builds; `internal/core/site/print.go`, `internal/app/print.go`, `frontend/src/print` | core/static-build, core/layouts, app/menu | done |
| 12 | [ui/static-build](ui/static-build/spec.md) | File › Build Website… (⇧⌘B) and Settings › Website: build the open folder from the app with progress and results; `internal/app/buildui.go`, `frontend/src/shell/buildsite.ts` | core/static-build, app/menu | done |
| 13 (E1) | [engine/cli-rename](engine/cli-rename/spec.md) | the command and executable become `zenmd` (`zen` alias until 0.4.0): `internal/cli`, `main.go`, Taskfiles, `installcli`, command examples in docs/demo/templates | cli/commands, core/static-build, core/init | done |
| 13 (E1) | [engine/session](engine/session/spec.md) | the headless folder session moves from `internal/app` to the public `engine/session`; the app becomes a thin Wails adapter, no visible change | waves 3–12 | done |
| 14 (E2a) | [engine/public-api](engine/public-api/spec.md) | module `github.com/zenmark-org/zenmd`; `internal/core/*` → public `engine/*`, built-ins → `builtins/*`; docs, embed example, apidiff in CI; runs alone (all imports) | engine/session, engine/cli-rename | done |
| 15 (E2b) | [engine/runtime](engine/runtime/spec.md) | the browser runtime (plugin mounting, built sites, print, page CSS) moves to `runtime/` (`@zenmd/runtime`), embedded in Go as package `runtime`; one build for the app and `zenmd build` | engine/public-api | done |
| 16 (E2c) | [kit/desktop-ui](kit/desktop-ui/spec.md) | `kit/desktop` (Go) and `kit/ui` (`@zenmd/kit`) hold the shared desktop shell; `internal/app` and `frontend/` move to `apps/zenmd-app`; runs alone | engine/public-api, engine/runtime | done |
| 17 | [ui/image-zoom](ui/image-zoom/spec.md) | click an image to see it large: zoom, pan, fit, close; in the app and built sites; `runtime/src/zoom`, page.css, built pages with images load zen.js | engine/runtime, ui/page | done |
| 18 | [ui/home](ui/home/spec.md) | a calmer home screen with the new logo, no Open… button in the sidebar, chevrons on the groups of built sites: `apps/zenmark/frontend/src/assets`, welcome rules in `kit/ui` shell.css, group rules in `runtime/src/static/static.css` | ui/shell, core/static-build | done |
| 18 | [ui/inspector](ui/inspector/spec.md) | the Inspector: a right-hand panel with the page's Vale issues, file details and content statistics, ⌥⌘I; `kit/ui/src/inspector`, `engine/session/pageinfo*.go`, `apps/zenmark/inspector.go` | ui/page, ui/lint, engine/session | done |
| 18 | [ui/developer-tab](ui/developer-tab/spec.md) | Developer tab: a bottom log panel (App, Plugins, Console) behind Settings › Developer, ⌥⌘J; `kit/desktop/logs.go`, `kit/ui/src/shell/{developer,devpanel,console}.ts`, `Zen.Logs` | kit/desktop-ui | done |
| 18 | [ui/sidebar-menu](ui/sidebar-menu/spec.md) | sidebar context menu and reordering by drag / Move Up and Down: `kit/ui/src/nav`, `engine/config/frontmatter_edit.go`, `engine/session/nav.go`, `apps/zenmark/nav.go` | engine/session, kit/desktop-ui, core/navmeta, ui/new-page | done |
| 19 | [ui/find](ui/find/spec.md) | Find in page: ⌘F, ⌘G, ⇧⌘G and Edit › Find open a find bar in the reader (highlights via the CSS Custom Highlight API) and the editor (match selected in the source); `kit/ui/src/find`, page/editor wiring, `apps/zenmark/menu.go` | ui/page, ui/editor, app/menu | done |
| 20 | [app/export](app/export/spec.md) | Export: File › Export › Website… / PDF…, Settings › Export, `zenmd export site\|pdf`; the PDF saved through macOS printing without the panel, also headless from the CLI; `kit/desktop/pdf*.go`, `apps/zenmark/{exportpdf,headless}.go`, `internal/cli/export.go`, print preview, buildsite/settings | ui/static-build, ui/print, app/menu, cli/commands | done |
| 20 | [ui/panel](ui/panel/spec.md) | the tabbed bottom panel (tabs with count chips, close, resizable top edge, empty state and property grid) the Inspector is built on; `kit/ui/src/panel` | ui/inspector, ui/developer-tab | done |
| 21 | [kit/addons](kit/addons/spec.md) | add-ons: optional capabilities chosen at build time (`ADDONS=vale`, tags `addon_<id>`, helpers bundled per add-on); Vale linting becomes the first add-on, off by default; Settings › Add-ons; `kit/addons`, `addons/vale`, `engine/session.SetLinter`, Taskfiles, resources | core/lint, ui/lint, ui/inspector, ui/panel, packaging/macos | done |
| 22 | [kit/assembly](kit/assembly/spec.md) | `apps/<app>/app-assembly.yml`: the add-ons, built-in plugins, layouts, site templates and theme an app is built with; checked dependencies, a generated `assembly_gen.go`, Settings › This build; `kit/assembly`, builtins selection helpers, Taskfiles | kit/addons, core/plugins, core/layouts, core/init | done |
| 23 | [kit/features](kit/features/spec.md) | optional app features in `app-assembly.yml` (`features:`, default all); the first is `editor`: without it the app is read-only, every write refused by the session (`Options.ReadOnly`, `ErrReadOnly`) and the edit actions removed from the UI; `hasFeature`, Settings › This build | kit/assembly, ui/editor-window, ui/sidebar-menu, ui/new-page | done |
| 24 | [engine/dialects](engine/dialects/spec.md) | markdown dialects: other syntaxes for container plugins, chosen per folder (`.zen.yml` `markdown.dialects`) and per build (`dialects:` in app-assembly.yml); MyST first (```` ```{note} ````, `:::{note}`, options, code directives), a hint for MyST projects, Settings › Markdown; `engine/dialects`, `builtins/dialects/myst`, render merge (#3) | kit/features, kit/assembly, core/plugins, core/render, core/config | done |
| 25 | [kit/shell-features](kit/shell-features/spec.md) | every big piece of the app becomes a feature (reader, nav, find, editor, inspector, print, export, newsite, developer) with checked needs; `features: []` is a bare shell; Zenmark restarts from it and regains each piece on the roadmap | kit/features, engine/dialects, kit/assembly | done |
| 26 | [ui/inspector-console](https://github.com/zenmark-org/zenmd/issues/25) (#25) | one Inspector panel: Page Info, Problems (checks, spelling) and Console tabs in two columns, collapsible; the Developer panel becomes its Console tab; `kit/ui/src/{inspector,panel}`, `kit/ui/src/shell/{developer,devpanel,console,layout}.ts`, `shell.ts` | ui/inspector, ui/panel, ui/developer-tab | done |
| 26 | [ui/settings-layout](ui/settings-layout/spec.md) (#26) | Settings in two columns, Configuration/Markdown/Templates/Export/Developer views in place, This build moves to Zenmark › About; `kit/ui/src/shell/{settings,about}.ts`, app settings, `buildsite.ts`, `menu.go` | kit/shell-features, ui/static-build, app/export | done |
| 27 | [ui/nav-panel](ui/nav-panel/spec.md) (#24) | the sidebar's header (folder logo `logo:`, title, close) and footer (Settings, Export, Inspector), Left/Right side; `layout.ts`, `shell.ts`, `engine/config` logo, `Workspace.Title/Logo` | #25, ui/settings-layout | done |
| 27 | [ui/accessibility](ui/accessibility/spec.md) | Settings › Accessibility: whole-window zoom (View › Zoom, ⌘+ ⌘− ⌘0), the reader's text size, font, line spacing, line width and link underlines, Reduce motion and Increase contrast; `kit/ui/src/shell/a11y.ts`, settings, reading variables in `runtime/src/css`, the View menu | ui/settings-layout, #25 | done |
| 28 | [ui/inspector-polish](ui/inspector-polish/spec.md) | Inspector after testing: resizable columns, one padding, page warnings under Problems, a Build tab; About back to the macOS panel; `kit/ui/src/{panel,inspector}`, `page/index.ts`, `menu.go`, `app.go`, `shell.ts` | #25, ui/settings-layout | done |
| 28 | [ui/shell-polish](ui/shell-polish/spec.md) | sidebar and Settings after testing: one heading, active footer buttons, Appearance and Markdown as item lists; `kit/ui/src/nav`, `settings.ts`, `layout.ts`, `shell.css` | ui/nav-panel, ui/settings-layout, ui/accessibility | done |
| 29 | [engine/config-name](engine/config-name/spec.md) | the folder settings file becomes `.zenmd.yml`; `.zen.yml` is still read, shown under Problems and renamed on the first folder-settings write; templates, demo and docs follow | core/config, engine/session, ui/settings-layout | done |
| 30 | [ui/inspector-details](ui/inspector-details/spec.md) | a problem's click jumps to it and marks it like a Vale mark, details on the right; the Build tab as a list with details; `kit/ui/src/{inspector,panel}`, `page/index.ts` | ui/inspector-polish, ui/lint, ui/find | done |
| 30 | [app/logging](app/logging/spec.md) | the Console tells the app's story at debug/info/warn/error, folder-relative paths only, rate-limited debug, Info+ by default; `kit/desktop`, logging in `engine/session`, `apps/zenmark` | ui/developer-tab, #25 | done |
| 31 | [plugins/check](plugins/check/spec.md) | `:::check` boxes tickable outside a course, ticks kept per page with Reset; bigger task-list boxes; `builtins/plugins/check`, `runtime/src/css/page.css` | core/plugin-extras, plugins/layouts | done |

Features in the same wave own disjoint paths and can be built in parallel.

## Rules for every feature agent

1. **Worktree.** Work only in your worktree:
   `git worktree add -b feature/<scope>-<feature> ../zen-worktrees/<scope>-<feature> <integration branch>`.
2. **Ownership.** Create and edit only the paths in your spec's *Owns* row,
   plus your own spec's Status. Never edit `go.mod`/`go.sum`,
   `frontend/package.json`, `docs/`, other specs, or files another spec owns.
   All Go dependencies are already in `go.mod` (kept alive by
   `internal/deps/deps.go`). If you need anything else, stop and report.
3. **Tests first.** Write the tests in your spec's table, confirm they fail
   for the right reason, commit (`Add failing tests for <scope>/<feature>`),
   then implement and commit (`Implement <scope>/<feature>`).
4. **Contracts are fixed.** Types and signatures in your spec's *Interfaces*
   and in PLAN.md's *Service API* are what other features code against. If
   one is wrong, stop and report instead of changing it.
5. **Checks before reporting done:** `go vet ./internal/... && go test ./internal/...` for Go
   (the root package embeds `frontend/dist`, which a fresh worktree lacks);
   `cd frontend && npx tsc --noEmit && npx vitest run` for frontend (copy the
   git-ignored `frontend/bindings` from the main checkout first, or vitest won't start). Frontend
   tests use `happy-dom` and the fake API; no test needs a window or network.
6. **Security.** Follow your spec's Security section; every item has a test.
   No `eval`, no network access, no file access outside the workspace root.
7. **No new dependencies**, no npm packages, no downloads.
8. **Report** at the end: what you built, test output, anything unverified,
   and the *Integration* lines the lead must apply. Do not merge or push.

## Lead (integration)

- Merges each wave into the integration branch, applies *Integration* notes
  to shared files (`frontend/src/shell/features.ts`, `internal/app`), runs
  `task test` and `task build`, then starts the next wave.
- Adds the *Changelog* line of each merged spec to `CHANGELOG.md`.
- `internal/deps/deps.go` was deleted after wave 3; new Go dependencies are
  added by the lead on the integration branch before a feature starts.
