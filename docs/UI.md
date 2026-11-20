# zen — UI design

Visual reference: [ui/mockup.html](ui/mockup.html) (static, open in a browser;
the buttons at the top right switch between the three views and the theme).
The mockup is the reference for spacing and look; this file is the reference
for behaviour. Implemented by the `ui/*` specs.

## Principles

- **UI consistency (a constraint on every design).** The same kind of thing looks
  and behaves the same everywhere. Before adding a view, reuse the pattern that
  already exists for its kind; a new pattern needs a reason in its spec.
  - **Settings:** every section uses the two-column frame. The left column holds the items, one per row, vertically (Theme, Navigation; Zoom, Motion…). The right column holds the selected item's controls, preview, help and actions. A section with nothing to list shows its empty text, or its failure text, in the left column, where the items would be. Nothing that lists items lives in the right column.
  - **Panels:** every Inspector tab uses the panel's two resizable columns and the same padding and header tools.
  - **Sidebar footer:** its buttons are one group, so at most one is on and two views never overlap. Turning one on closes the view of the one that was on; clicking the one that is on closes its view. Settings and Export share one view and only move the mark. A new footer button joins the group through `addFooterButton` (kit/ui `layout.ts`).
  - **States:** the same `aria-*` state means the same look everywhere. Active, selected, disabled, empty and error each have one style.
  - **Fix the pattern, not the instance.** When feedback hits one place, change the shared component so that every place that uses it changes together, and add a test that holds the rule for all of them.
- **Black and white.** No hue anywhere in the chrome. Buttons are grayscale.
  Colour only comes from the user's content (images, diagrams) and the three
  lint signal colours (wavy squiggles: red error, amber warning, blue suggestion)
  and the code colours (`--code-*`) used for syntax highlighting in the reader
  and the editor.
- **Editorial.** Body text in a serif at a comfortable measure (max 68ch);
  chrome in the system sans at small sizes. Generous whitespace, thin rules.
- **Yin and yang.** Light and dark themes are exact inversions of each
  other; the logo mark is the ☯ idea reduced to two shapes. Follows the
  system theme.
- **Quiet chrome.** One toolbar row, no icons without labels on first
  appearance, nothing moves unless the user moved it.

## Tokens (`frontend/src/styles/tokens.css`)

| Token | Light | Dark |
|---|---|---|
| `--bg` | `#ffffff` | `#0b0b0b` |
| `--bg-subtle` (sidebar, code) | `#f5f5f4` | `#161616` |
| `--fg` | `#111111` | `#ededed` |
| `--fg-muted` | `#6b6b6b` | `#9a9a9a` |
| `--rule` | `#e2e2e0` | `#2a2a2a` |
| `--btn-bg` / `--btn-bg-hover` | `#ececeb` / `#dededc` | `#232323` / `#303030` |
| `--btn-primary-bg` / `-fg` | `#111111` / `#ffffff` | `#ededed` / `#0b0b0b` |
| `--focus` | `#111111` | `#ededed` (2px outline, 2px offset) |

Type: `--font-body: "New York", "Iowan Old Style", "Palatino", Georgia, serif`
(17px/1.65); `--font-ui: -apple-system, "SF Pro Text", system-ui, sans-serif`
(13px); `--font-mono: "SF Mono", ui-monospace, Menlo, monospace` (0.88em).
Headings: body serif, weight 600, h1 2.1rem with a thin rule below.
Spacing scale 4/8/12/16/24/32/48. Radius 6px. No shadows except the modal.

## Views

### 1. Welcome (no folder open)

```
┌──────────────────────────────────────────────────────────┐
│                                                          │
│                          ☯                               │
│                         zen                              │
│                                                          │
│                  ┌──────────────────┐                    │
│                  │   Open a folder  │   primary button   │
│                  └──────────────────┘                    │
│              or drop a folder here · ⌘O                  │
└──────────────────────────────────────────────────────────┘
```

### 2. Reader (main view)

```
┌────────────┬─────────────────────────────────────────────┐
│ ☯ my-docs  │ README › Guides › Install          [ Edit ] │  top bar 44px
├────────────┼─────────────────────────────────────────────┤
│ README     │                                             │
│ ▾ Guides   │        Install                              │
│   Install ◂│        ─────────────────────────            │
│   Usage    │        Body text in serif, max 68ch,        │
│ ▸ API      │        centred in the pane…                 │
│            │                                             │
│ OTHER PAGES│                                             │
│   notes/a  │                                             │
└────────────┴─────────────────────────────────────────────┘
  sidebar 260px (resizable 200–400, ⌘\ toggles)
```

- **Sidebar.** A 44px header with the folder's logo (`.zenmd.yml` `logo:`, else the mark), its title (the home page's first heading, else the folder name) and a × that closes the folder; a footer with Settings (⌘,), Export (⇧⌘B) and the Inspector (⌥⌘I), each only with its feature; Settings › Appearance › Navigation puts the sidebar on the left or the right (ui/nav-panel). The cog opens a Settings page in the main pane (the sidebar stays; Esc or Back returns): the section list (Appearance, Configuration, Markdown, Layouts, Plugins, Templates, Export, Developer, each only when its feature is in the build) on the left, and every section in two columns: its items on the left, the selected item's preview, help and actions on the right (ui/settings-layout). Export and the Developer switch work in place. What the build contains is the Inspector's Build tab. Tree of pages; a node with children has
  a disclosure triangle and is itself a page. Current page is marked with a
  filled background (`--btn-bg`) and `aria-current="page"`. "Other pages"
  lists files nothing links to. Arrow keys move, Enter opens, ←/→ collapse/expand.
  Right-click, ⌃-click, the context-menu key or ⇧F10 on a row opens an in-page
  menu (`role="menu"`: Open, Open in Editor, New Page Here…, Move Up/Down, Copy
  Path, Copy Link, Reveal in Finder, Hide from Sidebar; items the host lacks are
  left out). Pages drag within their siblings (a drop line, Esc cancels) and ⌥↑/⌥↓
  move one place; the order is saved as `nav.order` in the pages' frontmatter
  (ui/sidebar-menu).
- **Breadcrumb.** `Page.breadcrumb`, each crumb is a link, last one is plain
  text. Collapses the middle to `…` when it does not fit.
- **Page.** Rendered HTML in `<article class="zen-page">`. Internal links
  navigate in place and push history (⌘[ back, ⌘] forward). External links
  call `openExternal` (Go shows "Open https://… in your browser?"). Broken
  links are muted with a dotted underline and do nothing.
- **States.** Loading: keep the old page, show a 2px progress line under the
  top bar after 150 ms. Error: message in the page area with a Retry button.
  File changed on disk: page re-renders in place, scroll position kept.

### 3. Editor

The editor opens in its own window (Edit or ⌘E on a page); the main window
keeps reading and follows the unsaved text live.

```
┌──────────────────────────────────────────────────────────┐
│ Install ●                                  [Split] [Done] │
├──────────────────────────────────────────────────────────┤
│ B  I  S  H1 H2  “ ”  •  1.  ☐  </>  🔗  ▦                 │  toolbar 36px
├───────────────────────────┬──────────────────────────────┤
│ 1 # Install               │ Install                      │
│ 2                         │ ───────                      │
│ 3 Run **zen** …           │ Run zen …                    │
│   source (mono, numbered) │ preview (as reader)          │
├───────────────────────────┴──────────────────────────────┤
│ Problems  1 error, 1 warning                             │  full width
└──────────────────────────────────────────────────────────┘
```

- **Split.** The `Split` button in the header, or ⇧⌘P, shows a rendered
  preview to the right of the source (below it when the window is narrower
  than 900px). Off by default; hidden for settings files. The choice and the
  divider position are remembered for the editor window.
- **Divider.** A 1px rule like the sidebar's resize edge. Drag it, or focus it
  and press ←/→ (↑/↓ when stacked) to move 5%; Home and End jump to the
  limits. The source keeps 20% to 80% of the space.
- **Preview.** `api.preview()` of the unsaved text, debounced 150 ms; the
  latest answer wins. It mounts like the reader, so plugin colours, diagrams
  and scripts work. Links in the preview do nothing.
- **Scroll sync.** The preview follows the source by proportion.
- **Source.** Line numbers, syntax colours, and Vale squiggles. The Problems
  console spans the full width under both panes.
- **Toolbar.** Bold ⌘B, italic ⌘I, strikethrough, H1, H2, quote, bullet list,
  numbered list, task, code, link ⌘K, table. Buttons are grayscale text
  glyphs with `aria-label` and a tooltip showing the shortcut. Each wraps the
  selection or toggles the line prefix.
- **Saving.** ⌘S saves. A dot `●` after the title marks unsaved changes.
  `Done` closes the window; with unsaved changes it asks Save / Discard /
  Cancel. If the file changes on disk while dirty, a bar offers
  "Reload from disk" or "Keep mine".

## Accessibility

Landmarks: `nav` (sidebar), `header` (top bar), `main` (page/editor). All
controls reachable by keyboard with a visible focus ring. Contrast ≥ 7:1 for
text, ≥ 3:1 for rules that carry meaning. Respects `prefers-reduced-motion`,
`prefers-contrast: more` and `prefers-color-scheme`.

Settings › Accessibility (ui/accessibility, in every build) adds per-machine options for every window: Zoom (75–200%, also View › Zoom In/Out/Actual Size, ⌘+ ⌘− ⌘0), Reduce motion and Increase contrast (System or On), and, with the reader, the page's text size, font, line spacing, line width and link underlines. They are attributes on `<html>` (`data-reduce-motion`, `data-contrast`, `data-link-underline`) and the variables `--zen-text-size`, `--zen-text-font`, `--zen-line-height` and `--zen-measure`, in the app's `shell.css` only: built sites, print and PDF keep the document's own look.
