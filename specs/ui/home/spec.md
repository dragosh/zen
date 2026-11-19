# ui/home

| | |
|---|---|
| Status | done |
| Target version | 0.5.0 |
| Branch | `feature/ui-home` |
| Wave | 18 |
| Depends on | ui/shell, ui/navigation, core/static-build, core/init |
| Owns | `apps/zenmark/frontend/src/assets/logo.svg`, the welcome rules in `kit/ui/src/styles/shell.css`, the group rules in `runtime/src/static/static.css` |

## Purpose

A calmer home screen when no folder is open, no redundant Open… button in the
sidebar, and disclosure chevrons on the groups of a built site's sidebar, as in
the app.

## Behaviour

1. The welcome screen shows the Zenmark logo (decorative, `alt=""`), the
   "Zenmark" heading, a one-line tagline, the primary "Open a folder" button
   with a ⌘O hint, the status line, and (when templates exist) "Start from a
   template" cards with "Show all…".
2. The templates sit in a grid: one column on narrow windows, two from 560 px,
   three from 860 px.
3. A small hint at the top-left of the welcome view points up to the menu bar:
   "↖ File menu: Open Folder… · New Site… (⇧⌘N)", followed by the Settings ⌘,
   link (`.zen-welcome-settings`). There is no footer line. The app has no
   recent-folders or version API, so neither is shown (nothing is invented).
3a. In the New Site dialog, "Open templates folder" (and the empty state's
   "Reveal in Finder") show only when developer mode is on (`developerEnabled()`).
4. All existing welcome behaviour stays: the status message for a launch folder
   that failed, template cards, Show all…, Open a folder.
5. The sidebar has no "Open…" button. Opening stays on the welcome screen, ⌘O
   and File › Open Folder; `LayoutHandlers.onOpenFolder` stays in the type.
6. In a built site, each group's `<summary>` shows a chevron before the title:
   pointing right when closed, down when open; leaves are indented to line up.
   CSS only (no JavaScript, works from file://); the browser's own marker is
   hidden (`list-style: none` and `::-webkit-details-marker`). Print is
   unchanged.

## Architecture

`showWelcome` in `apps/zenmark/frontend/src/shell/shell.ts` builds the view and
imports `src/assets/logo.svg` (the bare mark, copied from
`build/appicon.icon/Assets/zen_mark.svg`) as a URL. Styles in
`kit/ui/src/styles/shell.css`. The chevron mirrors `kit/ui/src/nav/nav.css`
(`.zen-nav-twisty`) and lives in `runtime/src/static/static.css`.

## Interfaces

No API change. New classes: `.zen-welcome-logo`, `.zen-welcome-tagline`,
`.zen-welcome-menuhint`. Removed: `.zen-welcome-foot`, `.zen-sidebar-open`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | logo, heading, tagline, button, hint | `apps/zenmark/frontend/src/shell/shell.test.ts: welcome has the logo, tagline and footer` |
| 1a | menu hint with ↖, File menu, ⇧⌘N and the Settings link; no footer | `shell.test.ts: welcome has a hint pointing up to the menu bar, and Settings stays reachable` |
| 1b | templates button hidden by default, shown in developer mode | `newsite.test.ts: Open templates folder is hidden unless developer mode is on`, `empty state: Reveal in Finder only in developer mode` |
| 2 | existing welcome behaviour | the existing welcome tests in the same file |
| 3 | no Open… in the sidebar | `kit/ui/src/shell/layout.test.ts: the sidebar has no Open button; the cog at the bottom reports itself` |
| 4 | chevron, marker hidden, open state | `runtime/src/static/static.test.ts: group summaries get a chevron and no native marker` |
| 5 | built demo | `engine/session: TestDemoBuildUnchanged` (golden: only zen.css changes) |

## UI

Tokens only; light and dark follow the tokens; the logo is a white disc with a
black mark and reads on both. No animation beyond the chevron's 120 ms turn,
removed under `prefers-reduced-motion`. Buttons and cards are real buttons with
the focus ring.

## Security

None: static markup and CSS, a bundled image, no network access.

## Notes

- **Out of scope:** recent folders and a version string (no API), the user guide link (it is a folder, not shipped in the app).
- **Integration:** none. `.zen-nav-ws` loses the 76 px right padding that made room for Open….
- **Changelog:** Added: a calmer home screen; disclosure arrows in built sites. Changed: no Open… button in the sidebar.

## Prompt

> Implement `specs/ui/home/spec.md` following `specs/README.md`.
