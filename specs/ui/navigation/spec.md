# ui/navigation

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/ui-navigation` |
| Wave | 2 |
| Depends on | ui/shell |
| Owns | `frontend/src/nav/` |

## Purpose

The sidebar tree and the breadcrumb: how the user moves between pages.

## Behaviour

1. Renders `workspace.name` and `workspace.tree` in the `nav` slot as a tree (`role="tree"`); nodes with children show a disclosure triangle. Clicking a page navigates; clicking a group or a triangle expands/collapses.
2. The current page has `aria-current="page"`, its ancestors are expanded, and it is scrolled into view when it changes. Other expansion state is kept per workspace root in `localStorage`; first-level nodes start expanded.
3. Keyboard: ↑/↓ move focus through visible nodes, → expands or moves to first child, ← collapses or moves to parent, Enter/Space opens, Home/End jump. One tab stop (roving tabindex).
4. `workspace:changed` re-renders the tree keeping expansion and focus where the nodes still exist.
5. Renders the breadcrumb in the `breadcrumb` slot from the current `Page.breadcrumb` (store field `page`): every crumb but the last is a link that navigates; the last is plain text with `aria-current="page"`. When wider than its slot, middle crumbs collapse into a `…` button that lists them in a menu.
6. In `edit` mode with unsaved changes (store flag `dirty`), a `●` with `title="Unsaved changes"` follows the breadcrumb.
7. An empty tree shows "No markdown files in this folder".

## Architecture

`nav/index.ts` exports the `Feature` (`id: "nav"`); `nav/tree.ts`,
`nav/breadcrumb.ts`, `nav/nav.css`. Uses only `Ctx` from ui/shell. Reads
breadcrumb data from the store fields `page` (set by ui/page after a load)
and `dirty` (set by ui/editor), both defined in ui/shell's `AppState`.

## Interfaces

```ts
export const navFeature: Feature;
export function renderTree(el: HTMLElement, tree: NavNode[], opts: { current: string; onOpen(path: string): void; expanded: Set<string>; onToggle(path: string, open: boolean): void }): void;
export function renderBreadcrumb(el: HTMLElement, crumbs: Crumb[], opts: { onOpen(path: string): void; dirty: boolean }): void;
```

## Tests

| # | Behaviour | Test (`frontend/src/nav/*.test.ts`) |
|---|---|---|
| 1 | tree structure, click navigation, expand/collapse | `tree.test.ts` |
| 2 | current page marked, ancestors expanded, persistence | `tree.test.ts` |
| 3 | keyboard model | `tree.keyboard.test.ts` |
| 4 | live update keeps state | `nav.test.ts` |
| 5 | breadcrumb links, last crumb, collapse | `breadcrumb.test.ts` |
| 6 | dirty marker | `breadcrumb.test.ts` |
| 7 | empty state | `tree.test.ts` |

## UI

docs/UI.md "Reader": sidebar and breadcrumb; mockup for spacing. Indent 14px
per level, row height 26px, ellipsis on overflow with full title in `title`.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Page titles and paths containing markup | all text via `textContent`; paths only in `dataset` | `tree.test.ts: escapes titles` |

## Notes

- **Out of scope:** search/filter, drag to reorder, context menus, creating files.
- **Integration (lead):** in `frontend/src/shell/features.ts` replace the nav placeholder with `navFeature`; import `nav/nav.css` there.
- **Changelog:** Added: sidebar navigation and breadcrumb.

## Prompt

> Implement `specs/ui/navigation/spec.md` following the rules in `specs/README.md`. Build
> against the fake API and the `Feature`/`Ctx` types from `frontend/src/shell/types.ts`
> (ui/shell is merged in your base). Look at docs/ui/mockup.html for the look; use tokens
> from `styles/tokens.css` only, no new colours. Done when the tests in the table pass.
