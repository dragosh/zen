# plugins/layouts — slides and course (plan)

| | |
|---|---|
| Status | done |
| Target version | 0.2.0 |
| Depends on | core/extplugins (layouts), ui/plugins |
| Kind | two external plugins, frontend only (shipped as built-in folders) |

## slides

```markdown
---
layout: slides
layoutOptions: { ratio: "16:9", transition: fade }
---
# Title slide
---
## Second slide
- point
Note: speaker notes after "Note:" are hidden on the slide.
```

- **Split:** each `<hr>` in the rendered page starts a new slide; without any, each `h1`/`h2` does.
- **View:** one slide scaled to fit the pane at the chosen ratio; ← → Space PageUp/Down, Home/End, a progress bar, `n / total`; `F` (or the bar's Full page button) for full page: the deck covers the whole window and asks for full screen, which the main window's webview allows; `Esc` leaves. `O` for an overview grid, `P` for print (one slide per page, `@media print`). Keys reach the deck only when focus is not in a widget outside it (sidebar tree, menu, toolbar button) and no other handler used them; a click on the deck gives it focus.
- **State:** current slide in the URL anchor (`#slide-3`) through `ctx.navigate`, so Back/Forward and live reload keep the place.
- **Editing:** the editor window is unchanged; the main window follows live, staying on the current slide.

## course

```markdown
---
layout: course
layoutOptions: { id: intro-to-zen }
---
# Getting started
## Step 1: Install
…
:::check
- [ ] I ran `zen --version`
:::
## Step 2: Open a folder
```

- **Split:** each `h2` is a step; the layout shows a step list on the left (done / current / locked), the current step, and Previous / Next.
- **Progress:** per step done state and the last open step in `ctx.storage` under the course `id`; "3 of 7 steps" in the header and a Reset button. Optional `:::check` blocks gate Next until every box is ticked.
- **Skip step (#13):**
  - A quiet **Skip step** button sits next to Next on every step except a finished one. It marks the step as skipped and moves on. A skipped step unlocks the next one even when its checklist isn't ticked.
  - In the step list a skipped step shows **✗** in place of the number, and a done step keeps ✓. The skipped mark has its own `data-state="skipped"`, styled with the kit's muted text colour, not red.
  - **Counts:** "3 of 7 steps" counts done steps only, and the header adds " · 1 skipped" when there are any. The progress bar fills for done steps only. Skipped steps are saved with the progress, as `skipped: [index]` next to `done`.
  - **Going back:** a skipped step can be opened again. Ticking all its boxes, or pressing Next on a step with no checklist, turns it from skipped to done. Reset clears the skipped steps too.
  - **Finish:** Finish is allowed with skipped steps.
  - **Keyboard:** the button is reachable in tab order after Next. It has no shortcut.
  - **Tests:** `runtime/test/builtin-course.test.ts`: skip unlocks the next step and marks ✗; the count and bar ignore skipped steps; it is saved and restored; ticking a skipped step's boxes makes it done; Reset clears it; a stored `skipped` with bad entries is filtered.
- **Across pages:** a course can span files: `layoutOptions.pages: [a.md, b.md]`; the sidebar marks finished pages (needs the nav badge hook below).

## Needs from the core

| Need | Why | Change |
|---|---|---|
| Layouts (wrap + `Page.layout` + full-pane CSS) | both | in core/extplugins and ui/plugins (wave 6) |
| Anchor changes delivered to plugins | slide/step in the URL | `ctx.onAnchor(fn)` in the runtime |
| Nav badges | finished course pages in the sidebar | small API: `ctx.badge(path, text)` → store → ui/navigation renders a muted suffix |
| Print styles hook | slides to PDF | layout CSS may define `@media print`; add File › Print (⌘P) |

## Steps

1. `ctx.onAnchor`, `ctx.badge`, ⌘P (spec `ui/plugin-extras`).
2. `slides` plugin: split, fit, keys, overview, print CSS. Tests with happy-dom for split/keys/state.
3. `course` plugin: steps, storage, gating, reset, multi-page.
4. Ship both in the app as bundled plugin folders copied to the user plugins dir on first run (pre-trusted because they are part of the signed app).
5. Demo: `examples/demo/layouts/slides.md`, `course.md`.

## Risks

- Scaling slides with CSS `zoom`/`transform` and code blocks/diagrams inside: test with mermaid.
- Storage is per Mac and folder path; moving the folder resets progress (document it; later: optional `storage: folder`).
