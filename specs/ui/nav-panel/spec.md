# ui/nav-panel

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/24-nav-panel` |
| Wave | 27 |
| Depends on | #25 (Inspector with Console, removes the Developer button), #26 ui/settings-layout (Appearance takes extra rows), ui/sidebar-menu, kit/shell-features |
| Owns | `kit/ui/src/shell/layout.ts` (sidebar header, footer and side), `kit/ui/src/nav/panel.css` (new), `apps/zenmark/frontend/src/shell/shell.ts` (sidebar wiring, close, export button, the Navigation row passed to Appearance), `engine/config/config.go` (the `logo` key), `engine/session/session.go` (`Workspace.Logo`, `Workspace.Title`), `apps/zenmark/frontend/src/api/{index,wails,fake}.ts` (the two fields) (+ the tests of all of these) |

## Purpose

The sidebar says whose folder this is and holds the panel buttons. At the top
are the folder's logo, its title and a close button. In the middle is the
page tree, as today. At the bottom are Settings, Export and the Inspector.
The sidebar can sit on the left or the right (#24).

## Behaviour

### Header
1. **Logo.** It is the folder's logo when `.zen.yml` (or `config.yml`, which only sets the default) has `logo: <path>`, and Zenmark's mark otherwise.
   - The path is relative to the folder and must stay inside it.
   - It must be an image: `.svg`, `.png`, `.jpg`, `.jpeg`, `.gif` or `.webp`.
   - It is served like other folder files (`/_ws/…`) and shown at 24 px high.
   - A logo that is missing, outside the folder or not an image falls back to the mark, with one warning in the log.
2. **Title.** The first heading of the folder's home page (index or README), else the folder's name.
   - It is one line, cut with an ellipsis, and the full title is its tooltip.
   - It updates live when the home page changes.
3. **× (Close Folder).** The same as File › Close Folder: editor windows ask first. Its label is "Close folder" and its tooltip "Close Folder ⇧⌘W".

### Body
4. The page tree and its context menu are unchanged. Thin `--rule` separators sit between the header, body and footer.

### Footer
5. **Buttons**, in this order: **Settings** (⌘,), **Export** (⇧⌘B; it opens Settings › Export, #26) and **Inspector** (⌥⌘I, with its problem count badge).
   - Each button is there only when its feature is in the build: Export needs `export`; Inspector needs `inspector` or `developer` (#25).
   - The Developer button is gone (#25). Buttons are icons with tooltips and `aria-label`s, as today.

### Side
6. **Settings › Appearance › Navigation:** Left (default) or Right.
   - It is kept per machine (`zen.navSide`) and applies at once in the main window.
   - With Right, the sidebar, its resize edge and ⌘\ (toggle) work the same, mirrored, and the Inspector and the page are unchanged.
   - The row is given to `appearanceSection(doc, extra)` (#26).
7. Without `nav` there is no sidebar, as today (kit/shell-features), and the Navigation row is not shown.

## Architecture

- **`engine/config`:**
  - `Logo string` in the merged config, read from the `logo` key: global, then folder.
  - `logo` is a validated workspace-relative path: no `..` and no absolute paths. It is checked again on use.
- **`engine/session`:**
  - `Workspace` gains `Logo` (a `/_ws/` URL, or "") and `Title`.
  - `Title` is the home page's first heading, from the index the session already has (no extra file reads).
  - `workspace:changed` carries them, so live updates need no new event.
- **`kit/ui/src/shell/layout.ts`:**
  - the sidebar gets a header row (logo, title, close) above `navBody`;
  - the footer gets the Export button;
  - `data-nav-side="right"` on the frame mirrors the grid.
  - `LayoutHandlers` gain `onClose` and `onExport`, and the layout gets `setIdentity({ logo, title })`.
- **`apps/zenmark/frontend/src/shell/shell.ts`:** wires the handlers, feeds `setIdentity` from the workspace, passes the Navigation row to Appearance and applies the stored side.

## Interfaces

```go
// engine/session
type Workspace struct {
	Root  string    `json:"root"`
	Name  string    `json:"name"`
	Home  string    `json:"home"`
	Title string    `json:"title"` // home page's first heading, else Name
	Logo  string    `json:"logo"`  // "/_ws/<path>" or ""
	Tree  []NavNode `json:"tree"`
}
```

```ts
// kit/ui/src/shell/layout.ts
export type NavSide = "left" | "right";
export function storedNavSide(): NavSide;
export function setNavSide(side: NavSide, doc?: Document): void;
// LayoutHandlers += { onClose?(): void; onExport?(): void }
// Layout += { setIdentity(o: { logo: string; title: string }): void }
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | logo: valid path, outside the folder, not an image, missing → mark | `engine/config/config_test.go: TestLogo`, `engine/session/session_test.go: TestWorkspaceLogo` |
| 2 | Title from the home heading, else the name, live | `engine/session/session_test.go: TestWorkspaceTitle`; `kit/ui/src/shell/layout.test.ts: "identity"` |
| 3 | × closes the folder | `apps/zenmark/frontend/src/shell/shell.test.ts: "sidebar close"` |
| 5 | Footer buttons by feature, no Developer button | `layout.test.ts: "footer buttons per feature"`; `shell.test.ts: "export button opens Settings › Export"` |
| 6 | Navigation side stored, applied, ⌘\ and resize mirrored | `layout.test.ts: "nav side"`; `shell.test.ts: "Navigation row in Appearance"` |
| 7 | No nav: no sidebar, no row | `shell.test.ts: "bare shell"` (extended) |

Edge cases:
- an SVG logo with a script in it: it is shown through an `img`, so the script never runs;
- a very long title;
- a folder with no home page;
- switching folders replaces the logo and the title;
- the Right side with a narrow window.

## UI

- **Header:** 44 px high, the same as the top bar. Logo 24 px, title in `--font-ui` weight 600 at 13 px, and a ghost × button on the right.
- **Footer:** today's row of ghost icon buttons.
- **Separators:** 1 px `--rule`.
- **Accessibility:**
  - the header is part of the `nav` landmark;
  - × is a `button`;
  - the logo `img` has `alt=""` (the title names the folder).
- New styles go in `kit/ui/src/nav/panel.css`, with tokens only and its own reduced-motion rules if any.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| `logo:` points outside the folder (`../../etc`, absolute, symlink out) | The path is validated in config and resolved through the workspace's confined file access, so it falls back when outside | `TestLogo`, `TestWorkspaceLogo` |
| An SVG logo runs script | It is shown with `img` only, never inline SVG, and `/_ws/` keeps its CSP | `layout.test.ts: "logo is an img"` |

## Notes

- **Out of scope:** a logo for built sites (layouts can do that already), and changing the tree itself.
- **Integration (lead, at merge):**
  - docs/UI.md sidebar paragraph;
  - docs: the `logo:` key next to the other `.zen.yml` keys (the demo `examples/demo/.zen.yml` may show it);
  - engine apidiff: the added fields are compatible;
  - CHANGELOG.
- **App Store / sandbox:** none.
- **Changelog:** `Changed: the sidebar has a header with the folder's logo (.zen.yml logo:), its title and a close button, and a footer with Settings, Export and Inspector; Settings › Appearance › Navigation puts it on the left or the right (#24).`

## Prompt

> Implement `specs/ui/nav-panel/spec.md` following the rules in `specs/README.md` and CLAUDE.md, for zenmark-org/zenmd#24 (`gh issue view 24`).
> Context:
> - The sidebar is built in `createLayout` in `kit/ui/src/shell/layout.ts`. Its footer already holds the Settings and Inspector buttons.
> - The workspace reaches the frontend as `Workspace` (`engine/session/session.go`, `apps/zenmark/frontend/src/api`).
> - The config is in `engine/config/config.go`.
> - Appearance takes extra rows (#26).
>
> Work in `../zen-worktrees/issue-24` on `feature/24-nav-panel`, from main after #25 and #26 are merged. Tests first.
>
> Do not launch the app, run `task install`/`package`/`release`, push, or touch main.
>
> Done when the table's tests pass, `go vet ./... && go test ./...` pass (run `npx vite build` in `apps/zenmark/frontend` first), and `npx tsc --noEmit && npx vitest run` pass in `apps/zenmark/frontend`. The final commit message ends in `Fixes #24`.
