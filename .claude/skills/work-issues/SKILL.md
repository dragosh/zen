---
name: work-issues
description: Work through the open GitHub issues of zenmark-org/zenmd - rank them in the right order (bugs that block use first, then what others build on, then features), group them into waves that can run in parallel without touching the same files, confirm the plan once, then give each issue its own subagent in its own git worktree, review each branch and merge in priority order after the user's go-ahead. Use when the user runs /work-issues, or asks to prioritise, triage, order, fix or work on the open issues, or to have subagents work on issues.
argument-hint: "[issue numbers, e.g. 2 6 8]  (default: every open issue)"
---

# /work-issues

Arguments: `$ARGUMENTS` — issue numbers to work on, or nothing for every open
issue. Repository: `zenmark-org/zenmd` (pass `-R zenmark-org/zenmd` to `gh`).

Read first: `specs/README.md` (**Rules for every feature agent**) and
`.claude/skills/add-spec/SKILL.md` (worktrees, verifying, merging); this skill
follows them.

## 1. Gather

```sh
gh issue list -R zenmark-org/zenmd --state open --limit 100 --json number,title,labels,body,createdAt
```

For each issue, write down in one line: type (`bug` / `enhancement` /
`documentation`), area labels, the files it will touch (from its **Where**
section; check them with a quick Grep), and the issues it depends on or
blocks (e.g. Export needs the greyed-out menu items first).

## 2. Rank

Put the issues in this order and say why for each:

1. **Bugs that break use or lose data**: the app does not open, files are
   lost, a release is broken.
2. **Other bugs**: wrong output on pages the user already has.
3. **Groundwork**: small changes that other issues build on.
4. **Enhancements**, smaller and self-contained ones first.
5. **Docs**, unless the docs describe something wrong.

When two are even, the older issue goes first. An issue whose dependency is
not merged yet waits for a later wave.

**Too big for an issue?** A feature that touches several layers (engine + CLI
+ app) or that changes a contract (`docs/PLAN.md` Service API,
`docs/PLUGINS.md`) gets a spec first: propose `/add-spec <scope>/<feature>`
for it, and leave it out of the waves until the spec is approved.

## 3. Plan the waves and confirm once

- A **wave** is a set of issues that run at the same time. Two issues in the
  same wave must not touch the same files (compare the file lists from step
  1), and none may depend on another in the wave. At most 4 per wave.
- Show the user one table: rank, issue link
  (`[zenmark-org/zenmd#N](https://github.com/zenmark-org/zenmd/issues/N)`),
  type/area, wave, branch, why it is at that rank. List any issue sent to
  `/add-spec` or held back, and why.
- Ask one question to start wave 1. Do nothing before a clear yes. The user
  may reorder, drop or move issues; follow their order.

## 4. One subagent per issue

For every issue in the wave, from `main` (or the integration branch):

```sh
git worktree add -b <branch> ../zen-worktrees/issue-<N> main
```

Branch: `fix/<N>-<slug>` for a bug, `feature/<N>-<slug>` for an enhancement,
`docs/<N>-<slug>` for docs (slug: 2–4 words from the title, kebab-case). If
the branch exists, reuse it and continue from its last commit.

Then start one `Agent` per issue, all in the same message so they run in
parallel (`subagent_type: general-purpose`, `model: sonnet`,
`run_in_background: true`). Each brief is self-contained:

> You are fixing zenmark-org/zenmd#N "<title>" in the worktree
> `<absolute path to ../zen-worktrees/issue-N>`, on branch `<branch>`.
> The issue: <its full body, and anything you found in step 1>.
> Files you may change: <the list>. Do not change anything else; if you
> must, stop and say why.
>
> Rules:
> - Work only in that worktree. Never touch the main checkout, `main`, or
>   another worktree. Do not push, merge or rebase.
> - Tests first: write the failing test(s) that show the bug or the new
>   behaviour, commit them, then make them pass.
> - Follow `specs/README.md` "Rules for every feature agent" and the spec
>   named in the issue; if the behaviour changes, update that spec.
> - No new dependencies, no downloads, no `go.mod` changes.
> - Do not launch the app, do not run `task install`, `task package`,
>   `task release` or anything that writes outside the worktree, and do not
>   start containers or servers.
> - Verify before you finish: `go vet ./... && go test ./...`, and for
>   frontend changes `npx tsc --noEmit && npx vitest run` in each changed
>   frontend package.
> - Add one line for CHANGELOG.md under `## [Unreleased]` (Fixed / Added /
>   Changed) in your report; do not edit CHANGELOG.md yourself.
> - Commit on the branch with a message ending in `Fixes #N`.
>
> Report: what changed (files), the tests added, the test output summary,
> the changelog line, and anything you could not verify (e.g. how it looks
> in the real app).

## 5. Review each branch

When a subagent finishes (do not poll; it reports back), check its work
before saying it is done:

- `git -C ../zen-worktrees/issue-<N> log --oneline main..` and the diff: only
  the allowed files, tests that really test the issue, no stray changes
  (revert incidental `build/darwin/icons.icns` changes).
- Run the tests in the worktree yourself.
- If something is wrong, send the fix back to the same subagent
  (`SendMessage`) rather than starting a new one.

Report each branch in one table: issue, branch, result, tests, what is
unverified (UI work: say the user should look at it in the app, or check it
in the browser preview where the fake api can show it).

## 6. Merge in rank order (after the user's go-ahead)

Only when the user says to merge. In rank order, per add-spec section C:

1. `git merge --no-ff <branch>` into `main`; resolve conflicts keeping both
   sides' intent.
2. Add the changelog line under `## [Unreleased]`.
3. `task test`, then `task build` (`task` and `wails3` live in
   `$(go env GOPATH)/bin`). Stop at the first failure and report it.
4. `git worktree remove ../zen-worktrees/issue-<N>` and
   `git branch -d <branch>`.

When the user should try the merged work in the app, run `task package` first:
`task build` only rebuilds `bin/zenmd`, and `bin/Zenmark.app` keeps the binary
from the last package, so opening it shows the old version. Tell the user to
quit a running Zenmark (⌘Q) before `open bin/Zenmark.app`.

Then start the next wave (back to step 4), since what it depended on is now
merged. Do not push; ask the user. The issues close on GitHub when the merge
commits reach `main` there (`Fixes #N`).
