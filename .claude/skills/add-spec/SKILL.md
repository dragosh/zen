---
name: add-spec
description: Add or build one zen feature from a spec - write specs/<scope>/<feature>/spec.md from the template (behaviour, architecture, interfaces, tests, UI, security, notes, prompt), then build it on branch feature/<scope>-<feature> in its own git worktree, tests first. Use when the user runs /add-spec <scope>/<feature> or asks to add, specify, start or build a feature/spec.
argument-hint: <scope>/<feature> [short description]
---

# /add-spec

Arguments: `$ARGUMENTS` — `<scope>/<feature>` then an optional description.
If missing, ask. Scopes: `engine`, `cli`, `core`, `ui`, `plugins`, `app`, `packaging` (see docs/ENGINE.md for what belongs to the engine).

Always read first: `docs/PLAN.md`, `specs/README.md` (feature table, waves,
**Rules for every feature agent**), and, as relevant, `docs/UI.md`,
`docs/PLUGINS.md`, `docs/VERSIONING.md`.

## A. The spec does not exist yet → write it (lead work, on the integration branch)

1. Copy `specs/_template/spec.md` to `specs/<scope>/<feature>/spec.md`.
2. Fill **every** section. Requirements:
   - *Owns*: paths disjoint from every other spec in the same wave. If the
     feature needs a shared file, describe the change under *Notes →
     Integration* instead of owning the file.
   - *Behaviour*: numbered and testable. *Tests*: each behaviour maps to a
     named test; list edge cases.
   - *Interfaces*: exact signatures; must agree with PLAN.md "Service API"
     and docs/PLUGINS.md. Change those documents first if the contract changes.
   - *UI*: states (empty, loading, error, populated), keyboard, accessibility,
     tokens from docs/UI.md only. *Security*: risk → mitigation → test.
   - *Target version* per docs/VERSIONING.md, and the *Changelog* line.
   - *Prompt*: self-contained brief for the subagent that will build it.
3. Add the row to the table in `specs/README.md` (wave, owns, depends on).
4. New Go dependency? Add it to `go.mod` and `internal/deps/deps.go` now, on
   the integration branch, so the feature branch never edits `go.mod`.
5. Show the user the behaviour list and test table; get approval; commit
   `Add spec for <scope>/<feature>`.

## B. The spec exists and its dependencies are merged → build it

1. Create the worktree from the integration branch:
   ```bash
   git worktree add -b feature/<scope>-<feature> ../zen-worktrees/<scope>-<feature> <integration branch>
   ```
   If the branch exists, reuse it and continue from its last commit.
2. Hand the spec's *Prompt* to a subagent working in that worktree (or do the
   work yourself there), under the rules in `specs/README.md`: owned paths
   only, tests first and committed failing, contracts fixed, no new
   dependencies, no downloads.
3. Verify in the worktree: `go vet ./... && go test ./...`;
   `cd frontend && npx tsc --noEmit && npx vitest run`. Check every row of the
   spec's Security table against the code. For UI work, run the app and have
   the user confirm what they see if a screenshot is not possible.
4. Set the spec's Status to `done` on the feature branch. Report: branch,
   worktree path, test output, anything unverified, and the Integration lines.
   Do not merge or push without the user's go-ahead.

## C. Merging (lead)

Merge the feature branch into the integration branch, apply the spec's
*Integration* lines, add its *Changelog* line under `## [Unreleased]`, update
the status in `specs/README.md`, run `task test` and `task build` (`wails3`
and `task` live in `$(go env GOPATH)/bin`; revert incidental changes to
`build/darwin/icons.icns`), then
`git worktree remove ../zen-worktrees/<scope>-<feature>`.
