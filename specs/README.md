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
| 2 | [core/render](core/render/spec.md) | `internal/core/render` | core/plugins, core/config, core/workspace | specified |
| 2 | [core/index](core/index/spec.md) | `internal/core/index` | core/workspace | specified |
| 2 | [core/watch](core/watch/spec.md) | `internal/core/watch` | core/workspace | specified |
| 2 | [cli/commands](cli/commands/spec.md) | `internal/cli`, `main.go` (+ `Options` fields in `internal/app/app.go`) | packaging/version | specified |
| 2 | [ui/navigation](ui/navigation/spec.md) | `frontend/src/nav` | ui/shell | specified |
| 2 | [ui/page](ui/page/spec.md) | `frontend/src/page` | ui/shell | specified |
| 2 | [ui/editor](ui/editor/spec.md) | `frontend/src/editor` | ui/shell | specified |
| 3 | [plugins/highlight](plugins/highlight/spec.md) | `internal/plugins/highlight` | core/render | specified |
| 3 | [plugins/diagram](plugins/diagram/spec.md) | `internal/plugins/diagram` | core/render | specified |
| 3 | [app/services](app/services/spec.md) | `internal/app`, `frontend/src/api/wails.ts` | all core, cli/commands | specified |
| 4 | [packaging/macos](packaging/macos/spec.md) | `build/darwin` | everything | specified |

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
- Deletes `internal/deps/deps.go` once wave 3 is merged (`go mod tidy` then
  keeps only what is used).
