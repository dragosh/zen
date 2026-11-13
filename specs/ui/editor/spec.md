# ui/editor

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/ui-editor` |
| Wave | 2 |
| Depends on | ui/shell |
| Owns | `frontend/src/editor/` |

## Purpose

Edit a page's markdown with a formatting toolbar and a live preview beside it.

## Behaviour

1. In `edit` mode, loads `api.source(path)` into a `<textarea>` (monospace, soft wrap, tab inserts two spaces, Shift-Tab outdents, Enter continues list / quote / task prefixes and removes an empty one).
2. Toolbar (`role="toolbar"`, arrow-key navigation) commands, each a pure function `(text, selStart, selEnd) → {text, selStart, selEnd}`: bold ⌘B, italic ⌘I, strikethrough, H1, H2, quote, bullet list, numbered list, task, inline code / fenced block when the selection spans lines, link ⌘K, table (3×2 skeleton). Wrapping commands toggle (applying twice restores the text); line commands toggle the prefix on every selected line.
3. Edits go through `document.execCommand("insertText")` or an equivalent that keeps native undo/redo working (⌘Z / ⇧⌘Z).
4. "Split" (⌘⇧P) toggles the preview pane; the choice and the divider position (20–80%) are remembered in `localStorage`. Default: split on.
5. The preview shows `api.preview(path, text)` using `mountPage` styling (`.zen-page`), debounced 150 ms; the latest response wins; links in the preview do not navigate.
6. Preview scroll follows the source proportionally.
7. `store.dirty` is true when the text differs from the last loaded/saved text. ⌘S calls `api.save`; success clears dirty; failure shows the message in a bar and keeps dirty.
8. "Done" (Esc when the toolbar or nothing is focused) returns to `read` mode; when dirty it asks Save / Discard / Cancel in an accessible dialog. Navigating to another page while dirty asks the same.
9. `page:changed` for the current path: not dirty → reload silently keeping the caret; dirty → bar with "Reload from disk" and "Keep mine".
10. Closing the window while dirty is prevented with the same dialog (`beforeunload`).

## Architecture

`editor/index.ts` exports the `Feature` (`id: "editor"`); `editor/commands.ts`
(pure functions), `editor/toolbar.ts`, `editor/preview.ts`, `editor/editor.css`.
No editor library. Preview HTML comes from Go, already sanitised; it is
mounted with the same code path as ui/page (copy the 5-line mount; do not
import from `page/` so the two features stay independent).

## Interfaces

```ts
export const editorFeature: Feature;
export type Edit = { text: string; selStart: number; selEnd: number };
export const commands: Record<"bold"|"italic"|"strike"|"h1"|"h2"|"quote"|"bullet"|"numbered"|"task"|"code"|"link"|"table", (e: Edit) => Edit>;
export function continueList(e: Edit): Edit | null; // Enter key behaviour
```

## Tests

| # | Behaviour | Test (`frontend/src/editor/*.test.ts`) |
|---|---|---|
| 1 | load, tab, list continuation | `editor.test.ts`, `commands.test.ts: continueList` |
| 2 | every command: apply, toggle off, multi-line, empty selection | `commands.test.ts` (table-driven) |
| 4 | split toggle and persistence | `editor.test.ts` |
| 5 | debounce, latest wins, inert links | `preview.test.ts` |
| 7 | dirty tracking, save success / failure | `editor.test.ts` |
| 8 | done / navigate with unsaved changes | `editor.test.ts` |
| 9 | external change, clean and dirty | `editor.test.ts` |
| 2 | toolbar roles, labels, shortcuts | `toolbar.test.ts` |

## UI

docs/UI.md "Editor" and the mockup. Toolbar 36px, grayscale glyph buttons with
`aria-label` and tooltip including the shortcut.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Unsaved text rendered unsanitised | preview only via `api.preview` (Go sanitiser) | `preview.test.ts: uses api.preview only` |
| Preview link navigating away | clicks inside preview are cancelled | `preview.test.ts` |
| Losing work | dirty guards on done, navigate, close, external change | `editor.test.ts` |

## Notes

- **Out of scope:** WYSIWYG editing (PLAN.md open question 1), syntax colouring in the source pane, image paste, find/replace, exact scroll sync, creating/renaming files.
- **Integration (lead):** in `features.ts` replace the editor placeholder with `editorFeature`; import `editor/editor.css`.
- **Changelog:** Added: markdown editor with toolbar, live preview and split view.

## Prompt

> Implement `specs/ui/editor/spec.md` following the rules in `specs/README.md`. Start with
> `commands.ts` and its table-driven tests; they are pure and carry most of the logic.
> Build against the fake API. Tokens only, no new colours, no packages. Done when the
> tests in the table pass.
