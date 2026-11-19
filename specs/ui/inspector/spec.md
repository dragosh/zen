# ui/inspector

| | |
|---|---|
| Status | done |
| Target version | 0.5.0 (MINOR) |
| Branch | `feature/ui-inspector` |
| Wave | 18 |
| Depends on | ui/page, ui/lint, engine/session, kit/desktop-ui |
| Owns | `kit/ui/src/inspector/`, `engine/session/pageinfo*.go`, `apps/zenmark/inspector.go`; additive lines in `kit/ui/src/{host.ts,index.ts,page/index.ts,lint/panel.ts,styles/shell.css}`, `apps/zenmark/{service.go,thin_test.go}`, `apps/zenmark/frontend/src/api/*` |

## Purpose

A panel on the right of the reading view that shows what is known about the page
that is open: its Vale issues, its file and its content. One icon button next to the Settings cog
opens it; the issue count stays visible on that button when the panel is closed.

## Behaviour

1. An icon button (`.zen-inspector-button`, title "Inspector ⌥⌘I") sits in the sidebar
   footer right after the Settings cog, same style as its neighbours. It shows while a
   page can be inspected and toggles the panel of the page shown (`aria-expanded`);
   ⌥⌘I does the same. The editor window has no sidebar: its toolbar keeps a toggle
   (`.zen-inspector-toggle`, same icon and badge, `aria-controls`).
2. The panel is closed by default: every launch starts with it closed. Open or closed
   lasts while the app runs (`sessionStorage` key `zen.inspector.open`, so it follows you
   from page to page), every access in try/catch; unavailable storage means closed. The last
   tab is remembered too (`zen.inspector.tab`: `issues`, `file` or `page`; anything else means Linting).
3. The panel is the tabbed bottom panel of [ui/panel](../panel/spec.md): full width under the page,
   the same height as the Developer panel (240 px by default, resizable from its top edge, shared as `zen.panel.height`),
   with a ✕ close button and a count chip on Linting (red with errors).
   Only one bottom panel shows at a time: opening the Inspector closes the Developer panel, and
   opening the Developer panel closes the Inspector (`zen:inspector-close`). It takes
   room from the page and sits above the Developer panel when both are open. Esc closes it when focus is inside it and returns focus to the Inspector button.
4. Neither the button nor the panel exists while no page is open (home screen,
   Settings), and they follow the open page: navigation, live reload
   (`page:changed`) and editor drafts (`page:draft`) all refresh them.
5. The panel is tabbed like the Developer panel: a `role=tablist` of Linting, File, Page
   (Linting first; its id stays `issues`), one `role=tabpanel` each, ←/→ (wrapping), Home and End move between
   tabs, roving tabindex. The tabs are a list of definitions in `inspector/index.ts`, so a
   later tab (Spelling) is one more entry.
6. **Linting** (was Issues) lists the Vale alerts of the page with the same items as before
   (line, severity, message, match). Choosing one scrolls to its mark and flashes
   it. States: "No issues" (Vale ran, nothing found) and "Linting off" (Vale
   unavailable or linting disabled). The button shows a red count badge ("3") when
   there are issues, none at 0, also with the panel closed, and its accessible name says
   it ("Inspector, 3 issues"); the old floating "N issues"
   button and panel are gone from the reader (the editor's console is unchanged).
7. **File** shows: path in the folder, full path with *Copy path* (copies the
   full path, button text becomes "Copied" briefly) and *Reveal in Finder* (only
   when the host offers it), size ("1.5 KB"), modified and created (created only
   when the system knows it). While loading it shows "…"; when the call fails it
   shows "Not available" and never throws. The data is fetched when the panel is
   open (and on opening), once per refresh; stale answers are dropped.
8. **Page** shows: title, layout (name and source: built-in, folder, user),
   words, reading time (words / 200, rounded, at least 1 min when there are
   words), headings, links (internal and external), images, plugins used (ids of
   `[data-zen-plugin]` in the article) and front matter keys. All counted in the
   frontend from `Page` and the rendered article.
9. All text goes in through `textContent`; nothing from the page is parsed as HTML.
10. Backend: `PageInfo(path)` returns size, modification time, creation time
    (macOS; 0 elsewhere) and the absolute path of a markdown file inside the
    open folder. `RevealInFinder(path)` shows that file in Finder.

## In the editor

The editor window (ui/editor) shows the same Inspector: toolbar toggle, panel, tabs, ⌥⌘I.

E1. The Inspector toggle (icon with badge, `aria-label` "Inspector" or "Inspector, N issues") is in the editor's actions, before Split and Done. ⌥⌘I toggles the panel. Open or closed uses the same key as the reader (`zen.inspector.open`, sessionStorage): a new editor window starts with the panel closed. Settings and config YAML files (`isConfigPath`) get no button and no panel, like no Split.
E2. **Linting** shows the alerts of the editor's own lint of the unsaved text, the ones `lintView` and the problems console show: the console stays. "Checking…" shows until the first lint answers (not on every keystroke, so the list does not flash while typing), "Linting off" when Vale is unavailable or the text is empty. Choosing an issue selects it in the source (`lintView.select(i)`), as the console does.
E3. **File** is as in the reader (path, size, dates, Copy path, Reveal in Finder, optional host methods) and is fetched again after a successful save.
E4. **Page** statistics come from the unsaved text. With Split on they are taken from the preview pane's rendered article (no extra request). Otherwise `api.preview(path, text)` is called 500 ms after typing stops, only while the Inspector is open (also once when it opens and after the file loads), a stale answer is dropped, and the HTML is parsed into a detached document (no assets, no scripts, nothing shown) for `pageStats`. With the Inspector closed nothing is rendered for it.
E5. Layout: the panel docks at the bottom of the editor window under the panes (`.zen-editor-window` is a flex column), so the toolbar and Done stay visible.
E6. Unmounting the editor destroys the Inspector (button, panel, timers, pending answers).

Tests: `kit/ui/src/editor/inspector.test.ts` (E1 to E6), `kit/ui/src/inspector/inspector.test.ts` (`onOpenChange`), `kit/ui/src/inspector/css.test.ts` (editor window rule), `kit/ui/src/editor/split.test.ts` (action order).

## Architecture

`kit/ui/src/inspector/` holds `stats.ts` (pure counting), `format.ts` (size, date,
reading time), `index.ts` (`createInspector`, the DOM) and `inspector.css`. The
page feature (`kit/ui/src/page/index.ts`) creates it with `external: true`, puts the panel in `.zen-content` (the parent of the main pane;
the panel is hidden whenever the main pane is, for Settings) and feeds it after
each render, lint and navigation. The sidebar button (`shell/layout.ts`) and the page do not know each other. Two events on
the document in `inspector/events.ts`: the inspector sends `zen:inspector-state`
(`{available, open, count}`) on every change and when destroyed (available false), and the
button sends `zen:inspector-toggle`, which an available inspector answers. The editor's
inspector is created without `external`, uses its own button and sends nothing, so the two
never clash when an editor is shown in the main window. The list of issues is `issueList()`, extracted
from `lint/panel.ts` and shared with `renderPanel`, which stays for other hosts.
The kit stays independent of the app: file data comes from the optional
`KitHost.pageInfo` and `KitHost.revealInFinder`; without them the File section
shows the path in the folder only.

Go: `engine/session/pageinfo.go` (`Session.PageInfo`, resolves the path through
the workspace like `Page` and `Source`), `pageinfo_darwin.go` / `pageinfo_other.go`
(creation time). The app: `Zen.PageInfo` is a one-line delegation in `service.go`;
`Zen.RevealInFinder` in `apps/zenmark/inspector.go` asks the session for the
validated path and calls `open -R` through `kit/desktop`.

## Interfaces

```go
// engine/session
type PageInfo struct {
	Path      string `json:"path"`      // as asked, slash separated, relative to the folder
	AbsPath   string `json:"absPath"`   // symlinks resolved
	Size      int64  `json:"size"`      // bytes
	ModTime   int64  `json:"modTime"`   // Unix milliseconds
	BirthTime int64  `json:"birthTime"` // Unix milliseconds, 0 when unknown
}
func (s *Session) PageInfo(path string) (PageInfo, error)

// apps/zenmark
func (z *Zen) PageInfo(path string) (PageInfo, error)
func (z *Zen) RevealInFinder(path string) error
```

```ts
// kit/ui/src/host.ts
export type PageInfo = { path: string; absPath: string; size: number; modTime: number; birthTime: number };
interface KitHost {
  pageInfo?(path: string): Promise<PageInfo>;
  revealInFinder?(path: string): Promise<void>;
}
// kit/ui/src/inspector
export type PageStats = { words: number; minutes: number; headings: number; links: { internal: number; external: number }; images: number; plugins: string[]; frontmatter: string[] };
export function pageStats(root: ParentNode, page: Page): PageStats;
export const formatSize: (bytes: number) => string;
export const readingMinutes: (words: number) => number;
export function createInspector(o: InspectorOptions): Inspector;
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | PageInfo size, times, absolute path, nested page | `engine/session/pageinfo_test.go: TestPageInfo` |
| 2 | PageInfo rejects `..`, absolute, NUL, non-markdown, missing, symlink out, no folder; errors carry no absolute path | `TestPageInfoValidation` |
| 3 | JSON shape | `TestPageInfoJSON` |
| 4 | `Zen.PageInfo` delegates; `RevealInFinder` reveals the validated file only | `apps/zenmark/thin_test.go`, `inspector_test.go: TestRevealInFinder` |
| 5 | Counting: words, headings, links, images, plugins, keys | `kit/ui/src/inspector/stats.test.ts` |
| 6 | Size, reading time | `format.test.ts` |
| 7 | Toggle, aria, persistence, broken storage, Esc, ⌥⌘I | `inspector.test.ts` |
| 8 | Tabs: roles, arrow keys, Home/End, last tab remembered | `inspector.test.ts` |
| 14 | Button in the footer after the cog, badge, none at 0, name, toggle event | `shell/layout.test.ts`, `page/inspector.test.ts` |
| 9 | Issues: list, select, "No issues", "Linting off", badge while closed | `inspector.test.ts`, `page/lint.test.ts` |
| 10 | File: fields, copy, reveal present/absent, failure, stale answer dropped | `inspector.test.ts` |
| 11 | Page section values; updates on navigation, `page:changed`, `page:draft`; hidden without a page | `page/inspector.test.ts` |
| 12 | Wails adapter calls and maps `PageInfo`, `RevealInFinder` | `api/wails.test.ts` |
| 13 | CSS uses tokens only; overlay rule under 900 px | `inspector/css.test.ts` |

Edge cases: page with zero words, missing front matter, a file without creation
time, storage that throws, a page changed while the file call is in flight,
a path with spaces or non-ASCII characters.

## UI

States: closed (button only, red count badge when there are issues), open with loading
file data, populated, file data unavailable. Compact rows (12 px, label left in
`--fg-muted`, value right), tabs look like the Developer panel's (`.zen-inspector-tab`).
Tokens from docs/UI.md only (`--bg`, `--bg-subtle`, `--fg`, `--fg-muted`, `--rule`,
`--btn-bg`, `--focus`, `--space-*`, `--radius`, fonts); the lint severity tokens
for the issue list as today. Keyboard: ⌥⌘I, Tab through the tabs and buttons,
Esc closes. The panel is `aside[aria-label="Inspector"]`; each tab has a
`role=tabpanel` labelled by its tab. ⌥⌘I is also the *Toggle DevTools*
accelerator of the Develop menu in dev builds only (`apps/zenmark/menu.go`); there
the menu takes it, in release builds the shortcut is free.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A path from the frontend escapes the folder | `PageInfo` resolves through `workspace.Resolve` and accepts markdown files only | test 2 |
| `RevealInFinder` shows or opens an arbitrary path | takes a workspace-relative path, validated by `PageInfo`, passes `-R` and the resolved path as separate arguments | test 4 |
| Error messages leak absolute paths | errors go through `cleanErr` | test 2 |
| Page data (title, plugin ids, front matter keys) as HTML | `textContent` only | test 7, 11 |

## Notes

- **Out of scope:** resizing, editing front matter from the panel, a table of contents.
- **Integration (done by the lead at merge):** `PageInfo` joins `delegating` in `apps/zenmark/thin_test.go` (done here); other branches adding methods to `service.go`, bindings or `fake.ts` merge additively.
- **App Store / sandbox:** `open -R` goes through `kit/desktop` like the other Finder calls and only receives paths inside the folder the user opened.
- **Changelog:** Inspector: a side panel with the page's issues, file details and content statistics (⌥⌘I).

## Prompt

> Implement `specs/ui/inspector/spec.md` following the rules in `specs/README.md`.
> Done when `task test` passes and `npx tsc --noEmit -p apps/zenmark/frontend` is clean.
