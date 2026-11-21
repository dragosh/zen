# engine/templates-blueprints

| | |
|---|---|
| Status | planned |
| Target version | 0.7.0 (blueprints, page templates phase 1), 0.8.0 (the rest) |
| Branch | `feature/20-blueprints`, then one branch per phase |
| Wave | 46–49 |
| Depends on | core/init (site templates, now blueprints), core/layouts (`fields:`), ui/inspector-details (Problems and marking), engine/export-cli-parity (CLI first) |
| Owns | per phase (below): `engine/structure/**` (new: the checker), `builtins/pagetemplates/**` (new), `builtins/layouts/*/layout.yml` (the `structure:` block), `builtins/sitetemplates` → `builtins/blueprints` (renamed), `engine/config/**` (`templates:` rules, `blueprints:`), `kit/assembly/**` (`blueprints:` and `templates:` keys), `internal/cli/**` (`zenmd check`, `zenmd new`, `zenmd init`), the Problems tab's new kind, the New Page and New Site dialogs, Settings › Templates and Blueprints, docs and demo |

## Purpose

Zenmark has one kind of template today: a whole folder you start a site from.
It needs two kinds, with clear names:

- **Page templates** describe what a *page* should contain: its frontmatter
  fields, its sections in order, the blocks it needs, and a few writing
  rules. A page that follows a template is **checked against it**, and every
  gap shows in the Inspector's **Problems** tab, the way a broken link does.
  A template also gives a new page its skeleton. Each **layout** brings a
  structure of its own, so a post needs a date and a course needs steps.
- **Blueprints** are what "site templates" are today: a folder you start from
  (technical docs, a learning path, a planner). They're renamed so that
  "template" means the page kind. A blueprint can bring page templates with
  it, so a learning blueprint ships its "lesson" template.

**CLI first:** `zenmd check`, `zenmd new page --template` and
`zenmd init --blueprint` come first, and the app calls the same engine code.

## Page templates

### 1. What a template is

A YAML file. Built-ins live in `builtins/pagetemplates/<id>.yml`, a folder's
own in `.zen/templates/<id>.yml`, and a layout's structure in its
`layout.yml` under `structure:`.

```yaml
id: how-to
name: How-to guide
description: Steps that get the reader to one result (Diátaxis)
layout: docs                  # the layout a new page gets; checking works with any layout
fields:                       # frontmatter
  title:    { required: true }
  audience: { type: string }
  updated:  { type: date, required: true, severity: warning }
structure:                    # the body, top to bottom
  - heading: { level: 1, count: 1 }                   # exactly one H1 (or a title: field)
  - paragraph: { min: 1, max-words: 60, label: Summary }
  - section: { heading: "Before you start", optional: true }
  - section:
      heading: "Steps"
      contains: [ { list: ordered, min-items: 2 } ]
  - section: { heading: "/^(Troubleshooting|Next steps)$/", optional: true, repeat: true }
  - end: {}                   # nothing else at the top level after these
rules:
  max-words: 1500
  heading-order: true         # no skipped levels (H2 to H4)
  image-alt: required
  code-lang: suggestion       # fenced code should name its language
skeleton: |                   # the body a new page starts with ({{title}} and fields filled)
  # {{title}}

  One paragraph: what the reader will have done at the end.

  ## Steps

  1. …
  2. …
```

### 2. The structure vocabulary (one checker for all)

| Element | Matches | Options |
|---|---|---|
| `heading` | a heading | `level`, `text` (a string, or `/regex/`), `count`, `optional` |
| `section` | a heading and everything up to the next heading of the same or a higher level | `heading` (as above, level defaults to 2), `contains: [elements]`, `optional`, `repeat`, `min`, `max` |
| `paragraph` | a paragraph | `min`, `max`, `max-words`, `label` |
| `list` | a list | `ordered`, `unordered` or `task`, `min-items`, `max-items` |
| `code` | a fenced code block | `lang` (required, or a value) |
| `table` | a table | `min-rows`, `columns: [names]` |
| `image` | an image | `alt: required` |
| `block` | a plugin block, by name | `name` (`quiz`, `check`, `rating`, `mermaid`…), `min`, `max`, `ids: unique` |
| `slide` | a slide (slides layout) | `heading: required`, `max-bullets` |
| `any` | anything | `min`, `max` (for free text between required parts) |
| `end` | the end of the page, or of a `contains:` list | none |

Fields reuse the layouts' `fields:` types (`string`, `date`, `image`, `list`, `bool`, `number`) and add `required`, `enum: [..]`, `pattern` and `severity`.

Every rule and element has a **severity**: `error`, `warning` (the default) or `suggestion`, shown with the lint colours, as Vale's are.

### 3. Which template a page follows

From strongest to weakest:
1. the page's frontmatter: `template: how-to`, or `template: false` to opt out;
2. a path rule in `.zenmd.yml`, as for layouts: `templates: { "guides/**": how-to, "blog/**": post }`;
3. the page's layout's own `structure:`.

A page follows at most one template, plus the structure of its layout. Both are checked, and the template's rules win where the two disagree.

### 4. Each layout's structure

The structures are built in, and each one is small, so a page that follows no template is still checked for what its layout needs.

| Layout | Fields | Body | Rules |
|---|---|---|---|
| `article` | `title` or one H1 | none | `heading-order`, `image-alt` |
| `docs` | `title` or one H1 | a first paragraph as the summary (suggestion); H2 and H3 sections | `heading-order`; a page over 800 words without H2s gets a suggestion |
| `post` | `date` (required, a date), `author` (warning), `tags` (a list), `cover` (an image in the folder) | a summary paragraph of at most 60 words (used for cards and link previews) | `image-alt`; the cover has alt text or the post a `cover-alt:` |
| `landing` | `tagline` (suggestion), `cover` | a short body of at most 200 words (suggestion) | a warning when the page has no child pages, because the cards would be empty |
| `wide` | none | at least one wide element: a table, a diagram or a grid (suggestion) | none |
| `handout` | `date`, `author` | fits one sheet: at most about 600 words (suggestion); no slide breaks | `image-alt` |
| `slides` | none | at least 2 slides; each slide has a heading; at most 6 bullets a slide (suggestion) | speaker notes optional |
| `course` | `layoutOptions.id` (required) | at least 2 steps (H2 sections); each step but the last ends with a `check` block (warning); quiz ids unique | an optional final `rating` step |

### 5. Built-in page templates

| Id | For |
|---|---|
| `tutorial` | learning by doing |
| `how-to` | one task |
| `reference` | facts to look up |
| `explanation` | understanding (the Diátaxis four) |
| `post` | a blog post or a note |
| `lesson` | one course step |
| `adr` | an architecture decision record: Context, Decision, Consequences |
| `meeting` | meeting notes: Attendees, Agenda, Decisions, Actions (a task list) |
| `release-notes` | Added, Changed, Fixed |
| `readme` | a folder's home: Summary, Getting started, Contents |

### 6. Where the results show

- **Problems (Inspector):** a new kind, **Structure**, next to broken links, images and spelling.
  - **Each row:** the rule's message ("Missing section 'Steps'", "date is required by the post layout", "Step 2 has no checklist"), the template or layout it comes from, its severity, and a **Fix** line, such as "Add a '## Steps' section after the summary".
  - **Clicking a row** marks the element in the page, the way problems are marked today. A missing element marks where it should go: the heading before the gap, or the top of the page for a missing field.
- **The sidebar:** a page with structure errors (not warnings or suggestions) gets the error mark that pages with problems use today.
- **Page Info:** shows the template the page follows and why (frontmatter, path rule or layout).
- **The editor window:** the same problems under it as Vale's, with the outline of the template's sections as a checklist (✓ or missing) in a side strip that you can collapse.

### 7. New pages

- **The New Page dialog (⌘N)** gets a **Template** picker. It lists the templates for this folder (built-in, the folder's own, and those a path rule would apply where the page is being created), with **None** first.
- **The new page** gets the template's `layout`, its required fields filled with defaults, and its `skeleton`. A path rule picks the template by itself, so "New page in guides/" starts as a how-to.

### 8. CLI first

| Command | Does |
|---|---|
| `zenmd check [folder] [--format text\|json] [--severity error\|warning\|suggestion] [--template id]` | checks every page and prints the problems by page (text or JSON). It exits with 1 on errors, so it can run in CI. |
| `zenmd new page <path> [--template id] [--title text] [--set key=value]` | writes a new page from a template |
| `zenmd templates [--list] [--show id]` | lists the page templates, or prints one |

## Blueprints (what "site templates" are today)

1. **The rename everywhere a user sees it:**
   - **The menu:** File › New Site from Template… becomes **File › New from Blueprint…** (⇧⌘N).
   - **Settings:** Settings › Templates splits in two, **Settings › Blueprints** and **Settings › Templates** (page templates).
   - **The CLI:** `zenmd init site --template <id>` becomes **`zenmd init --blueprint <id>`**, and the old spelling still works.
   - **The files and keys:**
     - `template.yml` in a blueprint folder becomes **`blueprint.yml`**;
     - the global `templates:` key becomes **`blueprints:`**;
     - the assembly key `templates:` becomes **`blueprints:`**;
     - the built-ins move to `builtins/blueprints/`.

     Every old name is still read. The old config key shows a Problems note, as `build:` does.
   - **The feature id** `newsite` stays, because it's an internal name.
2. **What a blueprint can bring:** what it does today (files, `ask:` questions, `requires:` layouts and plugins), plus:
   - page templates in its `.zen/templates/`, copied with the folder;
   - path rules in its `.zenmd.yml` (`templates: { "lessons/**": lesson }`), so pages created later follow the blueprint's structure;
   - its starting pages, which follow those templates, so `zenmd check` is clean on a fresh folder (tested).
3. **The built-in blueprints** say which templates they bring:
   - `techdocs`: tutorial, how-to, reference and explanation;
   - `learning`: lesson, with course pages;
   - `planner`: meeting, adr and release-notes.

## Settings

- **Settings › Templates** (page templates): the Settings frame, with the templates on the left (built-in, the folder's own, and each layout's structure) and the selected one on the right. The right column shows its description, a readable outline of its structure, its skeleton as a preview, and the path rules that use it. It also has **Copy to this folder**, which writes `.zen/templates/<id>.yml` to edit, and **Open in Editor** for the folder's own templates.
- **Settings › Blueprints:** what Settings › Templates shows today.

## Phases

1. **Wave 46:**
   - the rename to blueprints, everywhere, with the old names still read;
   - the checker engine (`engine/structure`), with `fields`, `heading`, `section`, `paragraph`, `list`, `block` and `end`, and the rules `heading-order`, `image-alt` and `max-words`;
   - the layouts' `structure:` blocks;
   - Problems › Structure and the marking;
   - `zenmd check`.
2. **Wave 47:** the page templates, built-in and from the folder, the path rules, `template:` in frontmatter, the New Page Template picker and `zenmd new page`.
3. **Wave 48:** Settings › Templates and Blueprints, Page Info, the sidebar mark, and the editor's outline strip.
4. **Wave 49:** the blueprints bring their templates and rules; the built-in blueprints are updated, and their starting pages are checked clean.

## Interfaces

```go
// engine/structure
type Template struct { ID, Name, Description, Layout string; Fields map[string]Field; Structure []Element; Rules Rules; Skeleton string }
type Problem struct { Rule, Message, Fix, Severity string; Line int; Anchor string /* heading id or element ref */ }
func Check(page *render.Page, t *Template, layout *Template) []Problem
```

The Problems kind `structure` uses `ProblemTarget` with `{ kind: "structure", line, text, heading }` for marking.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Each element and rule matches and reports (table-driven, with a page per case) | `engine/structure/*_test.go` |
| 2 | Template choice: frontmatter, then path rule, then layout; `template: false` | `engine/structure` and `engine/session` tests |
| 3 | Each layout's structure on the demo's layout pages: clean; a broken copy reports what the table says | `builtins/layouts` tests |
| 4 | Problems › Structure rows, Fix text and marking | `kit/ui/src/inspector/problems.test.ts`, `page/inspector.test.ts` |
| 5 | `zenmd check` output and exit codes; `zenmd new page`; `zenmd init --blueprint` and the old spelling | `internal/cli` tests |
| 6 | The rename: the old file, key and flag names read, with notes; the menu and Settings labels | `engine/config`, `kit/assembly`, `apps/zenmark` tests |
| 7 | Built-in blueprints start clean under `zenmd check` | `builtins/blueprints` tests |
| 8 | The New Page picker writes the skeleton and fields; a path rule preselects | `newpage.test.ts` |

## UI

UI consistency:
- Problems › Structure is one more kind in the existing Problems list, with its details and marking.
- Settings › Templates and Blueprints use the Settings frame.
- The New Page picker is a field of the existing dialog.
- Severities use the lint colours.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A template's regex makes the checker slow | Go `regexp` (RE2, linear time); patterns capped at 200 characters | #1 |
| A folder template or blueprint writing outside the folder | Skeletons are written only to the new page's path, inside the folder; blueprint copying keeps today's checks | #5, #7 |
| Template text injected into the page | Messages and skeletons are text; skeletons go through the normal render | #4 |

## Notes

- **Naming:** "template" means a page template, and "blueprint" means a folder you start from. The docs, the demo (its Templates section becomes Templates and Blueprints) and the user-facing strings follow this.
- **Out of scope:** templates for one part of a page (snippets cover that); AI-written content; checking pages outside the open folder.
- **Changelog lines** per phase, for example: `Added: page templates check a page's structure (fields, sections, blocks) and list what's missing under Problems › Structure; zenmd check runs the same checks.` and `Changed: site templates are now blueprints (File › New from Blueprint…, zenmd init --blueprint); the old names still work.`

## Prompt

> Implement phase N of `specs/engine/templates-blueprints/spec.md` following CLAUDE.md (CLI first) and docs/UI.md, in `../zen-worktrees/blueprints-<phase>` on `feature/blueprints-<phase>`. Tests first, committed failing. Only that phase's paths. Do not launch the app, run task package/install/release, push or touch main.
