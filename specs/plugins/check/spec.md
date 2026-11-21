# plugins/check

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/12-builtin-plugins` (roadmap step 3, #12) |
| Wave | 31 |
| Depends on | core/plugin-extras (ctx.storage), plugins/layouts (course) |
| Owns | `builtins/plugins/check/**` (+ new `assets/check.{js,css}`, tests), `runtime/src/css/page.css` (task-list checkbox size only), `builtins/layouts/course/**` (only so the two never both handle a box), `examples/demo/plugins/check.md`, `engine/session/demo_test.go` + `testdata/demo-build.golden` (regenerated) |

## Purpose

A `:::check` block is a checklist you tick. Today, outside a course, its boxes
are disabled, so a reader can't use it. The ticks should work and be kept, so
a checklist in a page is a real to-do list.

## Behaviour

1. **Clickable.** Outside a page with `layout: course`, a `:::check` block's boxes are enabled. Clicking a box or its label ticks or unticks it. A plain `- [ ]` task list that isn't in a `:::check` block stays read-only (disabled), as on GitHub.
2. **Persistent.**
   - Ticks are saved through `ctx.storage` under `check-<page path>#<n>` (storage keys allow only letters, digits, `.`, `_`, `-` and 64 characters, so the key is `check-<path>.<n>` with other characters in the path replaced by `-`, and a long or altered path is shortened and given an 8-digit hash), where n is the block's index in the page (0-based), as `{ version: 1, ticked: [label, …] }`.
   - A box is matched by its label text (trimmed, whitespace collapsed), so editing other items keeps the ticks.
   - On mount, the saved ticks are applied, replacing the `[x]` written in the markdown. A block with nothing saved shows the markdown's own `[x]`.
   - The storage is the plugin's own, on this Mac in the app; in a built site it is the browser's (whatever `ctx.storage` uses there).
3. **Reset.** Once at least one box differs from the markdown, a small text button, "Reset", shows after the list. It restores the markdown's ticks and deletes the saved entry by writing `null`. The button is keyboard reachable and named "Reset checklist".
4. **In a course**, the course layout owns the boxes and their storage, as today. The check plugin's script does nothing when the block is inside `[data-zen-layout="course"]`, so the two never both handle a box.
5. **Bigger boxes.** Every task-list checkbox in a page, whether read-only or not, is drawn at `1.15em` square, aligned with the first line of its text. The list keeps its hanging indent so the text lines up, and `accent-color: var(--fg)` stays. A box you can click has `cursor: pointer`, and so does its label.
6. **Text only.** Labels are read with `textContent`; nothing is parsed as HTML.

## Architecture

- **`builtins/plugins/check/check.go`:**
  - embeds `assets/check.js` and `assets/check.css`, the way quiz does;
  - the manifest's `Help.Summary` says ticks are kept.
- **`assets/check.js`:**
  - `mount(el, ctx)`: return early inside a course;
  - otherwise enable the boxes, load the saved ticks, and listen for `change` to save;
  - add the Reset button;
  - cleanup removes the listener and the button, and disables the boxes again.
- **`runtime/src/css/page.css`:** the checkbox size rule (the shared pattern for every task list).
- **`check.css`:** the Reset button. It reuses the page's existing small text-button look (as in quiz or course); no new style.

## Interfaces

No contract change. Storage key: `check-<path>#<n>`. Value: `{ version: 1, ticked: string[] }`. Anything else counts as nothing saved.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Boxes enabled in a check block; a plain task list stays disabled | `builtins/plugins/check/assets/check.test.ts` (or a Go test on the rendered HTML plus a JS test, following quiz) |
| 2 | Ticks saved by label and restored on mount; an unknown stored value is ignored | `check.test.ts: "persists by label"`, `"bad stored value"` |
| 3 | Reset shows only when ticks differ, restores the markdown and clears storage | `check.test.ts: "reset"` |
| 4 | Inside a course, the script does nothing | `check.test.ts: "course owns it"` |
| 5 | Checkbox size rule present | a CSS test that reads page.css, run without a DOM |
| 6 | Hostile label text stays text | `check.test.ts: "labels are text"` |

## UI

UI consistency: a task-list checkbox has one look everywhere (fixed in page.css, not in the plugin), and the Reset button is the existing small text button. It is keyboard reachable, respects `prefers-reduced-motion` (no animation is added), and uses black and white tokens only.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Stored data shapes the DOM | Only booleans are applied, by matching labels; labels are compared as text | `"bad stored value"`, `"labels are text"` |

## Notes

- **Integration:** none beyond this branch. CHANGELOG line, added by the lead: `Changed: :::check checklists can be ticked outside a course, and the ticks are kept per page; task-list boxes are bigger.`
- **Demo:** `examples/demo/plugins/check.md` says the ticks are kept and shows Reset.

## Prompt

> Implement `specs/plugins/check/spec.md` following CLAUDE.md, in the existing worktree `../zen-worktrees/issue-12` on branch `feature/12-builtin-plugins`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
