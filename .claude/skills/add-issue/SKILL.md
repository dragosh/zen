---
name: add-issue
description: File one or more GitHub issues for Zenmark / zenmd (zenmark-org/zenmd) from the user's notes - classify each (bug, enhancement, documentation), find the code it touches, write a clear title and body, check for duplicates, show the drafts for one confirmation, then create them with gh and return the links. Use when the user runs /add-issue, lists problems or ideas to "make issues" of, or asks to file, open, log or track issues/bugs/feature requests.
argument-hint: <one issue, or a list of them (one per line or bullet)>
---

# /add-issue

Arguments: `$ARGUMENTS` — one issue, or several (one per line, bullet or number).
If empty, ask the user for the list.

Repository: `zenmark-org/zenmd` (always pass `--repo zenmark-org/zenmd` to `gh`).

## 1. Split and understand

- One issue per distinct problem or idea. Split a bullet that holds two
  unrelated things; merge two bullets that are the same thing.
- Classify each:
  - `bug`: something that does not work as it should ("doesn't", "broken",
    "not visible", "error", a screenshot of a failure).
  - `enhancement`: a new feature or a change to how something works.
  - `documentation`: docs, the user guide (`examples/demo`), README.
  - Add `question` only when the note is a question, not a task.
- Use only labels that exist (`gh label list --repo zenmark-org/zenmd`). Never
  create labels without asking.

## 2. Find where it lives (quick, read-only)

For each issue, spend a few searches (Grep/Glob, `specs/README.md`, the spec
of the feature) to name the area and the likely files, e.g.
`kit/ui/src/inspector/`, `apps/zenmark/frontend/src/shell/shell.ts`,
`engine/site/`. Name the spec if one covers it (`specs/ui/inspector/spec.md`).
Do not fix anything and do not run the app; if a cause is obvious from the
code, say so as a *likely cause*, not a fact.

## 3. Write each issue

Title: imperative or descriptive, under ~70 characters, no trailing period,
names the area: "Inspector covers the editor toolbar in narrow windows",
"Add Close Folder to the sidebar menu".

Body (Markdown), only the sections that have content:

```markdown
## Summary
One or two sentences.

## Steps to reproduce        <!-- bugs -->
1. …
## What happens               <!-- bugs -->
## What should happen         <!-- bugs -->

## Proposal                   <!-- enhancements: behaviour, UI, shortcuts -->

## Where
- Area: Inspector (kit/ui/src/inspector/), spec: specs/ui/inspector/spec.md
- Likely cause: …           <!-- only if seen in the code -->

## Environment                <!-- bugs -->
Zenmark <VERSION file>, macOS (app | built site | zenmd CLI)
```

Rules:
- Write from the user's words; do not invent steps, versions or details they
  did not give. Mark guesses as such ("probably", "likely cause").
- No personal data: no names, emails, local paths under the user's home
  (write `<folder>`), tokens or file contents that are not the user's to share.
- Screenshots the user attached cannot be uploaded by `gh`: write
  "Screenshot: to be attached" and tell the user to drag it into the issue.

## 4. Check for duplicates

`gh issue list --repo zenmark-org/zenmd --state all --search "<key words>" --limit 10`
for each. If one matches, propose a comment on it (or skipping) instead of a
new issue.

## 5. Confirm once, then create

Creating issues is public and outward-facing: show every draft first, in one
message (a table: #, type/label, title; then each body), and ask one question
to create them all (the user may edit, drop or merge some). Only after a clear
yes:

```sh
gh issue create --repo zenmark-org/zenmd --title "<title>" --label "<label>" --body-file <tmp file>
```

Write each body to a temp file in the scratchpad (not `--body` on the command
line, so quotes and backticks survive). Create them one by one and stop at the
first failure, reporting what was created.

## 6. Report

A table of the created issues with their links
(`[zenmark-org/zenmd#12](https://github.com/zenmark-org/zenmd/issues/12)`),
their labels, and any issue that was skipped as a duplicate. Offer, in one
line, to turn an enhancement into a spec with `/add-spec <scope>/<feature>`
or to start on a bug.
