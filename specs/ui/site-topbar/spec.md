# ui/site-topbar

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/19-export` (found while checking roadmap step 10, #19) |
| Wave | 43 |
| Depends on | core/static-build (the site shell), engine/runtime (static.ts and static.css), ui/top-bar-actions (the icon set) |
| Owns | `engine/site/shell.go` + tests (the page shell), `runtime/src/static/**` + tests, `runtime/src/css/print.css` (hiding the bar when printing), the build goldens (regenerated), the demo's Export page text |

## Purpose

A page of an exported website should feel like the app. Today its breadcrumb
scrolls away with the text, and there's no way to hide the sidebar or to go
back and forward. The exported page gets the app's fixed top bar: a button
that closes and opens the sidebar, ‹ and › for back and forward, and the
breadcrumb.

## Behaviour

1. **A fixed top bar** sits at the top of the content column, the same height and look as the app's top bar (44px, `--bg`, a bottom rule). It stays in place while the page scrolls. It holds, left to right:
   - **The sidebar button.** Its icon is the app's sidebar icon, if the icon set has one; if not, add a `sidebar` icon to `kit/ui/src/shell/icons.ts` and use the same drawing here. It closes the sidebar, so the page takes the whole width, and opens it again. `aria-expanded` follows the sidebar, and its name is "Hide sidebar" or "Show sidebar". The choice is remembered per site in localStorage (`zen.site.<siteId>.sidebar`). <kbd>⌘\</kbd> does the same, as in the app.
   - **‹ and ›**, named "Back" and "Forward", with tooltips "Back (⌘[)" and "Forward (⌘])". They go back and forward in the browser's history. Each is disabled when there is nowhere to go: Back at the first page of the visit, and Forward when nothing was undone. ⌘[ and ⌘] work too.
   - **The breadcrumb**, as today, after the buttons.
2. **Same look as the app:** the same icon drawing (24 viewBox, 16px, stroke 1.5), the same icon button size, focus ring and on style, and the app's design tokens. The site's CSS takes these from the shared definitions instead of copying them, if the runtime can import them. Otherwise it uses one shared stylesheet that both the app and the site use.
3. **Narrow windows (under 720px):** the sidebar button takes the place of today's "Contents" summary and opens the sidebar over the page. The bar stays fixed.
4. **Without scripts:** the bar still shows, fixed, with the breadcrumb. The buttons are hidden, because they need scripts, and the sidebar stays open, as today.
5. **Printing:** the bar is left out, as the breadcrumb is today.
6. **Portable sites:** the bar works when a page opens from disk (`file://`) as well as from a server. Back and Forward use `history`, which works in both.

## Interfaces

None new for plugins. The site shell's HTML gains `header.zen-site-top` with `.zen-site-sidebar-toggle`, `.zen-site-back`, `.zen-site-forward` and the existing `.zen-site-crumbs`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | The shell writes the bar with its buttons and the breadcrumb; no script needed to show it | `engine/site/shell_test.go` |
| 2 | The toggle hides and shows the sidebar, updates aria and remembers per site; ⌘\ does the same | `runtime/src/static/static.test.ts` |
| 3 | Back and Forward use history, are disabled when there's nowhere to go, ⌘[ and ⌘] | `static.test.ts` |
| 4 | Fixed position, icon sizes and tokens; hidden in print; narrow layout | a CSS test that reads static.css and print.css, with no DOM |
| 5 | Goldens updated for the new shell | `TestDemoBuildUnchanged` (regenerated) |

## UI

UI consistency: the exported page's top bar is the app's top bar pattern, with the same icon set, button style and order (sidebar, back, forward, breadcrumb).

## Security

| Risk | Mitigation | Test |
|---|---|---|
| None new: no new data is read; localStorage keys are per site | – | – |

## Notes

- **Changelog** (added by the lead): `Added: an exported website has the app's fixed top bar: a button to hide the sidebar, back and forward, and the breadcrumb.`

## Prompt

> Implement `specs/ui/site-topbar/spec.md` following CLAUDE.md and docs/UI.md, in `../zen-worktrees/issue-19` on `feature/19-export`. Tests first, committed failing. Only the Owns paths (and `kit/ui/src/shell/icons.ts` if a sidebar icon is needed). Do not launch the app, run task package/install/release, push or touch main.
