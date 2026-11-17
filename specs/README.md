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
| 11 | [core/static-build](core/static-build/spec.md) | `zen build`: folder → static website (hosted, or `--portable` for file://); `internal/core/site`, `internal/app/build.go`, `internal/cli` build command, `build:` config, `frontend/src/static` | core/layouts, cli/commands | specified |

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
