# zen — UI design

Visual reference: [ui/mockup.html](ui/mockup.html) (static, open in a browser;
the buttons at the top right switch between the three views and the theme).
The mockup is the reference for spacing and look; this file is the reference
for behaviour. Implemented by the `ui/*` specs.

## Principles

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

- **Sidebar.** Workspace name on top. Tree of pages; a node with children has
  a disclosure triangle and is itself a page. Current page is marked with a
  filled background (`--btn-bg`) and `aria-current="page"`. "Other pages"
  lists files nothing links to. Arrow keys move, Enter opens, ←/→ collapse/expand.
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

```
┌────────────┬─────────────────────────────────────────────┐
│ ☯ my-docs  │ README › Guides › Install ●    [Split] [Done]│
├────────────┼─────────────────────────────────────────────┤
│            │ B  I  S  H1 H2  “ ”  •  1.  ☐  </>  🔗  ▦   │  toolbar 36px
│  (sidebar  ├──────────────────────┬──────────────────────┤
│   as in    │ # Install            │ Install              │
│   reader)  │                      │ ───────              │
│            │ Run **zen** …        │ Run zen …            │
│            │  source (mono)       │  preview (as reader) │
└────────────┴──────────────────────┴──────────────────────┘
```

- **Modes.** `Split` toggles between source-only and source + preview
  (50/50, draggable divider). Preview is `api.preview()` debounced 150 ms.
- **Toolbar.** Bold ⌘B, italic ⌘I, strikethrough, H1, H2, quote, bullet list,
  numbered list, task, code, link ⌘K, table. Buttons are grayscale text
  glyphs with `aria-label` and a tooltip showing the shortcut. Each wraps the
  selection or toggles the line prefix.
- **Saving.** ⌘S saves. A dot `●` after the breadcrumb marks unsaved changes.
  `Done` returns to the reader; with unsaved changes it asks Save / Discard /
  Cancel. If the file changes on disk while dirty, a bar offers
  "Reload from disk" or "Keep mine".
- **Scroll sync.** Preview follows the source by proportion (alpha).

## Accessibility

Landmarks: `nav` (sidebar), `header` (top bar), `main` (page/editor). All
controls reachable by keyboard with a visible focus ring. Contrast ≥ 7:1 for
text, ≥ 3:1 for rules that carry meaning. Respects `prefers-reduced-motion`
and `prefers-color-scheme`.
