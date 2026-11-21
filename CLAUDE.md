# Zenmark / zenmd

A macOS markdown app (Zenmark) and engine/CLI (`zenmd`), on Wails v3 with Go and
TypeScript. The repository is `zenmark-org/zenmd`, and `main` is the integration branch.
Start with docs/PLAN.md (decisions), specs/README.md (features, waves, rules),
docs/UI.md (look and tokens), docs/BUILD.md (assembly, features, export) and
docs/ENGINE.md (what is engine, what is app).

## How work is done

Claude is the **lead**. It writes specs, plans waves, starts and steers the
subagents, reviews every branch and merges. Subagents build. The lead does not
build features itself, apart from small review fixes.

1. **Spec first.** Every feature, and every issue too big for one file,
   gets a spec: `/add-spec <scope>/<feature>` from `specs/_template/spec.md`.
   Fill every section: Owns, numbered Behaviour, Interfaces, a Tests table,
   UI, Security, Notes with the Integration and Changelog lines, and the
   subagent Prompt. Add its row to specs/README.md. Small bugs and features
   stay as issues (`/add-issue`, `/check-feedback`, `/work-issues`).
2. **Waves in parallel.** Specs in the same wave run at the same time, one
   Sonnet subagent each (`Agent`, `model: sonnet`, `run_in_background: true`),
   all started in one message. Their *Owns* paths must not overlap. A change to
   a shared file goes in the spec's *Notes → Integration*, and the lead makes it
   at merge. A spec that needs another's result goes in a later wave.
3. **One worktree per subagent:**
   `git worktree add -b feature/<scope>-<feature> ../zen-worktrees/<scope>-<feature> main`.
   For issues the branch is `fix/<N>-<slug>` or `feature/<N>-<slug>`, in
   `../zen-worktrees/issue-<N>`.
4. **Subagent rules** (put them in every brief):
   - only its worktree and its owned paths;
   - tests first, committed failing;
   - no new dependencies or downloads;
   - never launch the app, run `task install`, `task package` or `task release`, start servers, push, or touch `main`;
   - commit messages end in `Fixes #N` when there is an issue.
5. **Review** each branch before merging:
   - read the diff (owned paths only, real tests);
   - rerun the checks in the worktree;
   - check the spec's Security rows;
   - send fixes back to the same subagent (`SendMessage`).
6. **Merge** in wave order:
   - `git merge --no-ff`;
   - apply the Integration lines;
   - add the Changelog line under `## [Unreleased]`;
   - set the spec's status to `done` in specs/README.md;
   - run `task test` and `task build`;
   - revert stray changes to `build/darwin/icons.icns`;
   - then `git worktree remove --force` and `git branch -d`.
7. **Push only when the user says so.** Merging into `main` needs the user's
   go-ahead, unless they have waived it for the current batch.
8. **UI changes:** package with `task package`, and the user opens
   `bin/Zenmark.app` (quit the running app with ⌘Q first). The browser
   preview `frontend-fake` in `.claude/launch.json` shows the frontend with
   the fake api.

## Checks

```sh
go vet ./... && go test ./...                       # needs apps/zenmark/frontend/dist (npx vite build)
cd apps/zenmark/frontend && npx tsc --noEmit && npx vitest run   # app, kit and runtime projects
task test && task build                             # the lead, after each merge
```

- Wails bindings are git-ignored. Regenerate them with
  `wails3 generate bindings -d apps/zenmark/frontend/bindings -clean=true -ts -i ./apps/zenmark`.
  `wails3` and `task` live in `$(go env GOPATH)/bin`.
- After `apps/<app>/app-assembly.yml` changes, run `go run ./kit/assembly/cmd/gen apps/zenmark`.
  `TestAssemblyGenerated` catches drift.
- Known flaky under load (timing limits): `TestLargeFile`, `TestLargeBlock` and `TestH2StreamCancelWithEscapedChild`. Rerun them alone.

## Conventions

- Zenmark is being rebuilt from a starting shell (`features: [reader, nav,
  inspector, developer]`). The roadmap is the GitHub Project "Zenmark roadmap"
  (#9 and its sub-issues). Don't cut a release from `main` during the roadmap.
  Hotfixes go on `release/0.6`.
- **UI consistency is a constraint** on every design (docs/UI.md, Principles).
  Reuse the existing pattern for each kind of view: the Settings frame (items
  on the left, details on the right, empty text in the left column), the
  panel columns, and one style per state. Fix the shared component, never one
  instance. Every spec's UI section names the pattern it reuses, and the lead
  checks it in review.
- UI: black and white tokens only (docs/UI.md), keyboard reachable, and
  `prefers-reduced-motion` respected.
- **Plugins and layouts** follow docs/PLUGIN-UI.md: interactive blocks use
  the plugin kit (`.zen-block`, `.zen-btn`, `.zen-btn-primary`, `.zen-btn-quiet`,
  `.zen-choice`, `.zen-field`) and the UI font at 14px, never their own buttons,
  choices or fonts. Put the guide in every plugin or layout brief, and check its
  review list before merging.
- Commits and PRs: plain messages with no Co-Authored-By lines. Never put
  personal data in issues.
- Never ask for passwords, keys or certificates in chat. The user runs
  interactive credential commands (`gh auth refresh …`) themselves.
