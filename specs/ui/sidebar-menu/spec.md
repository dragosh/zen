# ui/sidebar-menu

| | |
|---|---|
| Status | done |
| Target version | 0.5.0 |
| Branch | `feature/ui-sidebar-menu` |
| Wave | 18 |
| Depends on | engine/session, kit/desktop-ui, core/navmeta (`nav.order`, `nav.hidden`), ui/new-page |
| Owns | `kit/ui/src/nav/{menu,entries,siblings,reorder}.ts` (+ tests), `engine/config/frontmatter_edit.go`, `engine/session/nav.go`, `apps/zenmark/nav.go`, `specs/ui/sidebar-menu` |

## Purpose

Right-click a page in the sidebar for Open, Open in Editor, New Page Here…,
Move Up/Down, Copy Path, Copy Link, Reveal in Finder and Hide from Sidebar;
drag a page within its siblings to put it in another order. The order and
the hiding are saved in the pages' own frontmatter (`nav.order`,
`nav.hidden`), nothing else in the files changes.

## Behaviour

Context menu

1. Right-click, ⌃-click, the context-menu key and ⇧F10 (on the focused row) open a menu at the pointer (or
   under the row for the keyboard). The row becomes the menu's target and the tree's active row; the page is not opened.
2. The menu is `role="menu"` with `role="menuitem"` items. It is built with DOM APIs (text through `textContent`).
3. ↑/↓ move (wrapping, skipping disabled items), Home/End jump, Enter/Space run the item, Esc closes and
   returns focus to the row, Tab closes. A click outside, window blur, a scroll of the tree and a resize close it.
4. The menu is placed inside the window: it flips left/up when it would leave the viewport.
5. Items for a **page** node: Open; Open in Editor (when `host.openEditor`); New Page Here… (when the shell offers it;
   the new page's folder is the folder of the item); Move Up / Move Down (when `host.reorderNav`; disabled at the ends
   of the page siblings); Copy Path (the workspace-relative path); Copy Link (`[Title](relative/path.md)`, relative to the page
   open now, or to the folder when none is open); Reveal in Finder (when `host.revealInFinder`); Hide from Sidebar
   (when `host.setNavHidden`).
6. A **group** node (a folder in "Other pages") has New Page Here… (folder = the group), Copy Path and Reveal in Finder
   only. The "Other pages" group itself (no path) has no menu.
7. Items the host does not provide are left out (no disabled placeholders); separators sit between the
   open, create/move, copy and destructive groups and never at an end or twice in a row.
8. Hide from Sidebar asks first in an in-page dialog (`role="alertdialog"`, Cancel is the default): it names the page, says its
   child pages go with it and how to bring it back (remove `nav: hidden` from the page). Confirm calls `host.setNavHidden(path, true)`.
9. A failed call shows its message in a small status line at the bottom of the tree (`role="alert"`) and changes nothing.

Reordering

10. A page row can be dragged (pointer events, after 4px of movement) among its **page siblings**: a drop line shows
    where it will land; dropping on the line calls `host.reorderNav(parent, orderedPaths)` with all page siblings in the new order.
    Esc, or releasing outside the group, cancels. Rows of other groups are not drop targets.
11. Move Up / Move Down do the same with one step. After the call the moved row stays selected and focused.
12. The tree refreshes from the next `workspace:changed` (the session emits it after the write), keeping expansion,
    selection and focus.
13. `Alt+↑` / `Alt+↓` on a focused row are the keyboard shortcuts for Move Up / Move Down.

Saving (engine)

14. `ReorderNav(parent, paths)` takes the page siblings under `parent` (the parent node's path, "" for the top level
    and for the "Other pages" group) in the wanted order. It fails when a path is not a page in the sidebar, repeated,
    not under that parent, or when `paths` is not all the page siblings. It numbers them 10, 20, 30… and writes
    `nav.order` only into the pages whose number changes.
15. The write changes the frontmatter block only: the body, line endings (LF or CRLF), a BOM, other keys, comments and
    their order stay byte for byte. A page without frontmatter gets a new block; a page without `nav` gets a `nav:` mapping; an
    existing `order` value is replaced in place, keeping a trailing comment.
16. Files are written one after the other with the workspace's atomic write (temp file and rename). When one write fails
    the ones already written are put back and the error is returned.
17. `SetNavHidden(path, hidden)` sets `nav.hidden: true`; with `false` it removes the key (and an empty `nav:`).
18. After the writes the session re-indexes at once and emits `workspace:changed` when the tree changed (the watcher then
    finds nothing new), so the sidebar updates without the watcher and without reloading.
19. A frontmatter that is not valid YAML, `nav` that is not a block or one-line mapping, or a result that would
    not parse back to the same data apart from the changed key: the file is left untouched and the error says so.
20. `RevealInFinder(path)` (app service) resolves the path inside the open folder (a page or a folder) and runs
    `open -R`; anything outside, absolute or with `..` is refused.

## Architecture

- `engine/config/frontmatter_edit.go`: `SetNavKey(src []byte, key string, value *string) ([]byte, error)`, a line-based editor
  with a parse-back check (behaviours 15, 17, 19). No new dependency (yaml.v3 is used already).
- `engine/session/nav.go`: `ReorderNav`, `SetNavHidden`, `AbsPath`; they hold `s.mu`, write through `s.ws.Write`, call
  `reindexLocked` and emit `EventWorkspaceChanged`.
- `apps/zenmark/nav.go`: the service methods `ReorderNav`, `SetNavHidden`, `RevealInFinder` (one line each, plus Finder).
- `kit/ui/src/nav/menu.ts`: the menu (position, keyboard, dismiss) and the confirm dialog; `reorder.ts`: the drag
  controller; `tree.ts` raises `onContextMenu(path, x, y | row)` and `onMove`; `index.ts` wires host methods to items.
- `kit/ui/src/host.ts`: optional `reorderNav?`, `setNavHidden?`, `revealInFinder?`; `Ctx.newPage?(folder)` is set by the
  app's shell (it opens the existing New Page dialog with that folder).
- Data flow: drag/menu → `host.reorderNav` → Go `ReorderNav` → files → `workspace:changed` → store → `renderTree`.

## Interfaces

```go
// engine/config
func SetNavKey(src []byte, key string, value *string) ([]byte, error) // value nil removes the key
// engine/session
func (s *Session) ReorderNav(parent string, paths []string) error
func (s *Session) SetNavHidden(path string, hidden bool) error
func (s *Session) AbsPath(rel string) (string, error)
// apps/zenmark (service)
func (z *Zen) ReorderNav(parent string, paths []string) error
func (z *Zen) SetNavHidden(path string, hidden bool) error
func (z *Zen) RevealInFinder(path string) error
```

```ts
// KitHost (all optional)
reorderNav?(parent: string, paths: string[]): Promise<void>;
setNavHidden?(path: string, hidden: boolean): Promise<void>;
revealInFinder?(path: string): Promise<void>;
// Ctx
newPage?(folder: string): void;
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | 15, 17, 19 frontmatter edit cases | `engine/config/frontmatter_edit_test.go: TestSetNavKey*` |
| 2 | 14, 16, 18 | `engine/session/nav_test.go: TestReorderNav, TestReorderNavRejects, TestReorderNavRollsBack, TestSetNavHidden` |
| 3 | 20 | `engine/session/nav_test.go: TestAbsPath`, `apps/zenmark/nav_test.go: TestRevealInFinder` |
| 4 | service exposes the methods | `apps/zenmark/nav_test.go: TestNavService` |
| 5 | 1-4 open, position, keyboard, dismiss | `kit/ui/src/nav/menu.test.ts` |
| 6 | 5-9 items, omitted items; confirm | `kit/ui/src/nav/entries.test.ts`, `menu.test.ts` |
| 7 | 10-13 drag, Move Up/Down | `kit/ui/src/nav/reorder.test.ts` |
| 8 | 12 tree refresh keeps focus; fake api reorders | `apps/zenmark/frontend/src/shell/navmenu.test.ts`, `api/fake.test.ts` |

Edge cases: CRLF files, BOM, `nav:` with a comment, `nav: { title: X }`, a file with `---` but no close, empty frontmatter
block, `order` with a trailing comment, nav with deeper nested lines, read-only file, case-different paths.

## UI

States: menu open; item disabled; confirm open; status line error; drag with drop line. Keyboard as in 3 and 13.
Accessibility: `role=menu`/`menuitem`, `aria-disabled`, focus returns to the row, dialog traps Tab, drop line `aria-hidden`
with a live-region message ("Moved Install above Usage"). Tokens only (`--bg`, `--rule`, `--btn-bg`, `--fg`, `--fg-muted`,
`--focus`, `--radius`, `--space-*`), nothing hard-coded; light and dark follow the tokens.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Writing outside the folder | every path goes through `workspace.Write`/`Resolve`; paths must be sidebar pages | `TestReorderNavRejects` |
| Half-written file | atomic write; failure restores the earlier files | `TestReorderNavRollsBack` |
| Corrupting a page | edit is frontmatter-only and parsed back; otherwise untouched | `TestSetNavKeyRefuses` |
| `open -R` on any path | path resolved inside the folder first | `TestRevealInFinder` |
| Text of titles/paths into the DOM | `textContent` only | `menu.test.ts` |
| Copy Link breaking markdown | `[ ] ( )` in titles and spaces in paths are escaped | `siblings.test.ts` |

## Notes

- **Out of scope:** dropping into another group (changing `nav.parent`), reordering folder groups, drag in editor windows, undo.
- Pages and groups mixed in "Other pages": only pages are numbered; numbered pages sort before groups (existing rule of `orderGroups`).
- **Integration (done by the lead at merge):** `specs/README.md` row; additive lines in `apps/zenmark/service.go` are in `nav.go`.
  `RevealInFinder` may also be added by the inspector spec: keep one.
- **App Store / sandbox:** Reveal goes through the same `macOpen` as the other Finder calls; only paths in the folder the user opened.
- **Changelog:** under 0.5.0 Added: sidebar context menu, reordering by drag and Move Up/Down, Hide from Sidebar.

## Prompt

> Implement `specs/ui/sidebar-menu/spec.md` following the rules in `specs/README.md`. Tests first. Done when `task test`
> and `npx tsc --noEmit -p apps/zenmark/frontend` pass.
