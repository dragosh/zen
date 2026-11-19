---
name: check-feedback
description: Check the testing feedback in docs/improve/ - pull the notes the user added (often committed straight on GitHub), read each one, verify bug claims, decide issue or spec, file them with the /add-issue flow after one confirmation, delete the filed notes and commit. Use when the user runs /check-feedback, says they added an improvement, feedback, note or issue to docs/improve, or asks to go through the feedback.
argument-hint: "[file in docs/improve]  (default: every note)"
---

# /check-feedback

`docs/improve/` holds notes the user writes while testing Zenmark: one note per
file, `README.md` excepted (it describes the process and may add rules; read
it first). Arguments: `$ARGUMENTS` — one note file, or nothing for all of them.

## 1. Get the latest notes

The user often adds notes on GitHub directly, so look there too:

```sh
git fetch -q origin
git log --oneline main..origin/main          # commits that are not here yet
git diff --stat main...origin/main
```

- If `origin/main` is ahead and the working tree is clean, `git pull --ff-only origin main`.
- If the local tree has changes, or the branches have diverged, or the new
  commits touch more than `docs/improve/`, stop and tell the user what is
  there instead of merging.

Then list the notes: every `docs/improve/*.md` except `README.md`. If there
are none, say so in one line and stop.

## 2. Read and check each note

For each note, write down in one line what it asks for. Then:

- **A bug claim:** check it before calling it a bug. Reproduce it the cheap
  way (render the markdown with `bin/zenmd build <folder> -o <scratchpad>`,
  read the code, run a test); do not launch the app for this. Say what you
  found: expected behaviour (e.g. CommonMark), a real bug with its likely
  cause (file and line), or both.
- **A feature:** look for what already exists (Grep the code, `specs/README.md`)
  so the proposal builds on it.
- **Duplicates:** `gh issue list --repo zenmark-org/zenmd --state all --search "<words>"`.
- **Issue or spec:** an issue by default. Propose `/add-spec <scope>/<feature>`
  instead only for a larger self-contained feature the user wants to build
  next; say why in the drafts and let the user choose.

## 3. Draft and confirm once

Follow `.claude/skills/add-issue/SKILL.md` for the issue itself (typed title
`Bug:` / `Feature:` / `Docs:`, one type label plus area labels, the body
sections, no personal data). Show every draft in one message: a table
(note file, labels, title), then each body, and for each note what you found
when checking it. Ask one question to create them all.

## 4. File, clean up, commit

After a clear yes:

1. Create each issue (`gh issue create … --body-file`), stop at the first failure.
2. `git rm` each note whose issue was created (never `README.md`; a note that
   was skipped or failed stays).
3. Commit: `Filed docs/improve feedback as issues #N, #M`.
4. Do not push; ask the user.

## 5. Report

A table: note → issue link (`[zenmark-org/zenmd#N](https://github.com/zenmark-org/zenmd/issues/N)`),
labels, and anything skipped or left for a spec. Then one line on which issue
looks worth fixing first and why (a bug that loses data or blocks use comes
before a feature), and offer to start on it, or to run `/work-issues` for
all of them.
