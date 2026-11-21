---
name: add-issue
description: File one or more GitHub issues for Zenmark / zenmd (zenmark-org/zenmd) from the user's notes or from the note files in docs/improve/ - classify each (type and area labels), find the code it touches, write a clear typed title and body, check for duplicates, show the drafts for one confirmation, then create them with gh, return the links and delete the filed notes. Use when the user runs /add-issue, lists problems or ideas to "make issues" of, points at docs/improve, or asks to file, open, log or track issues/bugs/feature requests.
argument-hint: <one issue, or a list of them (one per line or bullet)>
---

# /add-issue

Arguments: `$ARGUMENTS` — one issue, or several (one per line, bullet or number),
or `docs/improve` (or a file in it).
If empty, look in `docs/improve/` for note files (every `*.md` except
`README.md`, which describes the process); if there are none, ask the user.

Notes in `docs/improve/` are files the user wrote while testing: one issue
per file. Read the folder's README.md first; it may add rules. A note that is
really a larger feature can go to `/add-spec` instead (say so in the drafts).

Repository: `zenmark-org/zenmd` (always pass `--repo zenmark-org/zenmd` to `gh`).

## 1. Split and understand

- One issue per distinct problem or idea. Split a bullet that holds two
  unrelated things; merge two bullets that are the same thing.
- Check what the note claims before calling it a bug (e.g. render the
  markdown with `bin/zenmd build` into the scratchpad): a note can be expected
  behaviour (CommonMark), a real bug, or both. Say which in the body.
- Classify each, with one **type** label and one or more **area** labels:
  - Type: `bug` (does not work as it should), `enhancement` (a new feature
    or a change), `documentation` (docs, the user guide `examples/demo`, the feature tour `examples/showcase`,
    README); `question` only when the note is a question, not a task.
  - Area: `engine` (engine/, builtins/, runtime/: format, rendering, plugin
    host, build, print), `kit` (kit/desktop, kit/ui), `app` (apps/zenmark:
    menu, service, windows, frontend), `plugin` (plugins and the plugin API),
    `layout` (layouts and the layout API), `packaging` (build/, signing, DMG,
    App Store, CI releases).
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

Title: starts with its type — `Bug:`, `Feature:` (enhancement) or `Docs:` —
then what it is, under ~75 characters, no trailing period:
"Bug: Inspector covers the editor toolbar in narrow windows",
"Feature: Close Folder in the sidebar menu".

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
gh issue create --repo zenmark-org/zenmd --title "<title>" --label <type> --label <area> --body-file <tmp file>
```

Write each body to a temp file in the scratchpad (not `--body` on the command
line, so quotes and backticks survive). Create them one by one and stop at the
first failure, reporting what was created.

Then, for notes from `docs/improve/`: delete each note file whose issue was
created (`git rm`, never README.md; a note that was skipped or failed stays)
and commit "Filed docs/improve notes as issues #N, #M".

## 6. Report

A table of the created issues with their links
(`[zenmark-org/zenmd#12](https://github.com/zenmark-org/zenmd/issues/12)`),
their labels, and any issue that was skipped as a duplicate. Offer, in one
line, to turn an enhancement into a spec with `/add-spec <scope>/<feature>`
or to start on a bug.
