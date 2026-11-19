# ui/find

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/4-find-in-page` (zenmark-org/zenmd#4) |
| Wave | 19 |
| Depends on | ui/page, ui/editor, app/menu |
| Owns | `kit/ui/src/find`, the find wiring in `kit/ui/src/{page,editor}/index.ts`, `menu:find*` in `kit/ui/src/host.ts` and `apps/zenmark/frontend/src/api`, Edit › Find in `apps/zenmark/{menu,menustate,aliases,app}.go` |

## Purpose

Search inside the open page like a browser's find: ⌘F opens a small bar over the top right of the window,
matches are highlighted and Enter steps through them. The macOS web view has no find bar of its own, so the
page draws it. Built websites (`zenmd build`) keep the browser's own find.

## Behaviour

1. ⌘F (Ctrl+F) and Edit › Find › Find… open a find bar over the top right of the window (left of the Inspector when it is open, so its tabs stay visible) with the field
   focused. The current selection (one line, up to 200 characters), if any, is filled in; otherwise the last
   query is kept.
2. Typing highlights every match in the page and makes the first one current (scrolled to the middle). The bar
   shows "3 of 12", or "No results"; nothing for an empty query.
3. Enter / ⌘G go to the next match, ⇧Enter / ⇧⌘G to the previous one, wrapping around. The current match is
   drawn apart from the others. The ↑ and ↓ buttons do the same. With the bar closed, ⌘G / ⇧⌘G open it.
4. Esc closes the bar, removes every highlight and gives the focus back. ⌘F with the bar open selects the
   field's text.
5. Case-insensitive by default; the "Aa" toggle (`aria-pressed`) matches case.
6. A match inside a closed `<details>`, a hidden slide or a hidden course step is shown when it becomes
   current: the `<details>` is opened; a slide is shown through its `#slide-N` anchor, a course step through its
   step link (a step that is still locked is just unhidden); any other `hidden` block is unhidden.
7. In the editor window ⌘F searches the markdown source: the current match is selected in the textarea and
   scrolled into view, the focus stays in the search field. The selection of the source is the starting query.
   Typing in the source counts again without moving the selection. (Replace is not part of this.)
8. Matches run across inline markup (`foo <strong>bar</strong>` finds "foo bar") but not across blocks;
   scripts, styles, SVG, form controls, a slide's notes and the slide overview are not searched.
9. Menu: Edit › Find with Find… (⌘F), Find Next (⌘G), Find Previous (⇧⌘G), enabled only while a folder is open.
10. The menu items send `menu:find`, `menu:find-next` and `menu:find-previous` to every window; the window that
    has the focus (`document.hasFocus()`) acts, so the shortcut reaches the main or the editor window,
    whichever is in front. ⌘F/⌘G are also handled on keydown, so the page works even when the web view sees the
    key before the menu does; a menu event right after the same key (150 ms) is ignored, so a press never steps twice.
11. The reader's bar searches again when the page is rendered again (live reload, another page) and keeps the
    current match number without scrolling.

## Architecture

`kit/ui/src/find/index.ts` has three layers:

- Pure: `findInText(text, query, matchCase)` returns start offsets of literal occurrences (the query is
  escaped before it becomes a `RegExp`).
- `Finder` (`find`, `show`, `clear`, `selected`): `domFinder(root, {skip, reveal, highlights})` builds `Range`s
  over the page's text and paints them with the CSS Custom Highlight API (`CSS.highlights`; names `zen-find` and
  `zen-find-current`, styled in `find.css`), so the page's DOM is not changed. Where the API is missing
  (macOS web views before Safari 17.2) the current match is selected instead. `textareaFinder(ta)` selects
  matches with `setSelectionRange`. The app's minimum is macOS 13 (`LSMinimumSystemVersion` 13.0).
- `createFind({doc, host, finder})` is the bar (`role="search"`, labelled field, `aria-live="polite"` count,
  `aria-pressed` Aa); `bindFind(doc, api, find, opts)` wires ⌘F/⌘G, the three menu events and the focus check.

The page mounts it in `pageFeature` (active in read mode with a page shown), the editor in `editorFeature`.
Go: `menuSpec` has a nested `Find` submenu in Edit; `emitAction(sink, EventFind…)` emits without showing a window.

## Interfaces

```ts
findInText(text: string, query: string, matchCase: boolean): number[]
domFinder(root: () => HTMLElement | null, opts?: DomFinderOptions): Finder
textareaFinder(ta: HTMLTextAreaElement): Finder
createFind(o: { doc: Document; host: HTMLElement; finder: Finder; label?: string }): Find
bindFind(doc: Document, api: FindApi, find: Find, opts?: { hasFocus?; active?; seed? }): () => void
// KitHost / Api
on(event: "menu:find" | "menu:find-next" | "menu:find-previous", fn: () => void): () => void
```

```go
const EventFind = "menu:find"; EventFindNext = "menu:find-next"; EventFindPrevious = "menu:find-previous"
menuActions{ Find, FindNext, FindPrevious func() }
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | literal, case-insensitive / match-case search; regex characters are literal; empty query | `kit/ui/src/find/find.test.ts: findInText` |
| 2 | page matches counted, highlighted, current apart; across inline markup; not across blocks / scripts | `find.test.ts: domFinder` |
| 3 | `<details>` opened; hidden slide goes through the reveal hook, other hidden blocks unhidden | `find.test.ts: domFinder` |
| 4 | no `CSS.highlights`: selection fallback; clear removes highlights; query never HTML | `find.test.ts: domFinder` |
| 5 | textarea: match selected, focus kept where it was | `find.test.ts: textareaFinder` |
| 6 | bar: role, labels, live region, focus; "n of m" / "No results"; Enter, ⇧Enter, buttons wrap | `find.test.ts: find bar` |
| 7 | Aa toggle; regex characters literal; `<details>` opens; Esc closes and clears; reopen selects text | `find.test.ts: find bar` |
| 8 | ⌘F / ⌘G / ⇧⌘G keys, other modifiers ignored; menu events act only in the focused window; no double step | `find.test.ts: bindFind` |
| 9 | reader: ⌘F opens, counts, menu event, next / previous, new page searched again, unmount cleans up | `kit/ui/src/page/find.test.ts` |
| 10 | editor: ⌘F, match selected in the source, selection as seed, literal regex characters, Esc does not leave | `kit/ui/src/editor/find.test.ts` |
| 10b | matches capped at 5000, count shown as "5000+" | `find.test.ts: match cap` |
| 11 | Edit › Find items and accelerators | `apps/zenmark/menu_test.go: TestMenuSpecProduction` |
| 12 | Find items are folder-only; actions only emit | `menu_test.go: TestFindMenuItemsAreFolderOnly, TestMenuFindEvents, TestMenuActions`; `menustate_test.go: TestFolderOnlyLabels` |

Edge cases: more than 5000 matches (capped), overlapping matches ("aaaa" / "aa" = 2), composition (IME) Enter.

## UI

A floating bar fixed 52 px from the top and 16 px from the right, over the page: field, count, "Aa", ↑, ↓, ✕.
Tokens only (`--bg`, `--bg-subtle`, `--rule`, `--btn-bg-hover`, `--btn-primary-*`, `--focus`, `--font-ui`), no
hue, no shadow, light and dark follow the theme; hidden in print. Matches are underlined on `--btn-bg-hover`; the
current match is inverted (`--btn-primary-bg` / `-fg`). Tab order: field, Aa, ↑, ↓, ✕; focus ring 2px. The count is
a polite live region. Reduced motion: no entrance animation.

## Security

- The query is text: it is only used through `RegExp` after escaping and through `Range`/selection, never as
  HTML or `innerHTML`. Tests: `find.test.ts` (regex characters, `<b>` query).
- Matches are capped (5000) so a one-letter query cannot freeze the page. Test: `find.test.ts: match cap`; the bar shows "1 of 5000+" when capped.
- No network, no file access; the page's DOM is not modified except opening a `<details>` or unhiding a block
  that holds the current match.

## Notes

- **Out of scope:** replace in the editor, find in built websites, find across pages, whole-word and regex modes.
- **Known limits:** a locked course step that holds a match is unhidden next to the current step (not
  unlocked); matches do not span blocks.
- **Integration:** none; the menu and events are wired in `app.go`.
- **App Store / sandbox:** none.
- **Changelog:** Find in page: ⌘F (Edit › Find) opens a find bar in the reader and the editor with match count, next/previous (⌘G, ⇧⌘G) and a match-case toggle (#4).

## Prompt

> Implement `specs/ui/find/spec.md` following `specs/README.md`.
