# Possible layouts

Ideas for page layouts beyond the ones in specs/core/layouts (article, docs,
post, landing, wide, handout, slides, course). A layout is the look of a
whole page; see specs/core/layouts for how they are built (template + CSS +
optional script).

## Writing and publishing

| Layout | Look |
|---|---|
| **book** | chapter page: running header, large drop cap, chapter number, Prev / Next as page turns; a `toc` page lists chapters with progress |
| **journal** | dated entry: big date, weather/mood fields, previous/next day; works with daily notes |
| **letter** / **memo** | sender, recipient, date, subject, signature block; print-ready |
| **changelog** | release notes: each `##` version becomes a card with date and badges (Added / Fixed / Security) |
| **newsletter** | issue number, date, sections as columns on wide screens |
| **resume** | two-column CV: sidebar with contact/skills fields, timeline for experience |
| **recipe** | ingredients panel beside steps, servings scaler, step checkboxes (storage) |

## Knowledge and work

| Layout | Look |
|---|---|
| **wiki** | infobox from frontmatter fields on the right, backlinks at the bottom |
| **meeting** | date, attendees, agenda; action items (`- [ ]`) collected into a box at the top |
| **decision** (ADR) | status badge (proposed/accepted/superseded), context / decision / consequences sections, links to superseding records |
| **runbook** | numbered steps with copy buttons and a "done" state per step, a "time spent" timer |
| **board** | each `##` is a column, list items are cards (kanban) |
| **timeline** | dated `##` headings on a vertical line |
| **dashboard** | grid of cards from `###` sections; good with `csv/table` and charts |
| **faq** | each `###` question collapses; search box filters questions |

## Learning and presenting

| Layout | Look |
|---|---|
| **flashcards** | each `##` is a card, flip on click, spaced repetition progress (storage) |
| **workshop** | slides on the left, notes / exercises on the right, timer |
| **poster** | one big page, scaled to fit, printable as A2/A3 |
| **gallery** | images as a grid with a lightbox; captions from alt text |
| **map** | a page with a big map from coordinates in frontmatter (needs a bundled map library) |

## Variations of the reading view

| Layout | Look |
|---|---|
| **focus** | larger type, narrower column, no sidebar (toggle) — for long reads |
| **two-column** | text in two columns on wide screens (newspaper) |
| **side-notes** | footnotes shown in the margin next to their reference (Tufte style) |
| **code** | wider column, code blocks full width, outline of code blocks on the side |
