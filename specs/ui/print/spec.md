# ui/print

| | |
|---|---|
| Status | done |
| Target version | 0.2.0 |
| Branch | `feature/ui-print` |
| Wave | 12 |
| Depends on | core/static-build, core/layouts, app/menu |
| Owns | `internal/core/site/` (print.go new; site.go writes print.html), `internal/app/print.go` + `print_test.go` (new), `frontend/src/print/` (new), `frontend/src/page/print.css` (new, imported by `page.css`), the `@media print` block of `frontend/src/page/page.css`, `examples/demo/reading/printing.md` (new) |

## Purpose

Print what you read: **Print Page** prints the current page as it looks,
cleanly. **Print Site** prints the whole folder as one document: a cover, a
table of contents, then every page in sidebar order. Both use the macOS
print panel, so **Save as PDF** comes for free.

## Behaviour

> Since 0.6.0 ([app/export](../../app/export/spec.md)): File › Print Site… is **File › Export › PDF…** (⇧⌘P); the preview's primary button is **Export PDF…**, which saves the PDF without the print panel, and `zenmd export pdf` writes it from the command line.

### Print Page
1. **File › Print Page…** (⌘P), and ⌘P in the main window, prints the page shown, in its layout, through the macOS print panel. The editor window's ⌘P prints the preview of the unsaved text.
2. The printout is always light (black on white, whatever the app theme). It leaves out:
   - all app chrome: sidebar, toolbar, breadcrumb, warnings, lint marks;
   - interactive controls: buttons, the course's Next/Back, the quiz Check button, the sandbox Run button.

   It keeps what they show, such as quiz questions and the answers given so far.
3. Before the panel opens, zen waits until the page is ready: plugin scripts mounted (diagrams drawn), images and fonts loaded. The wait is 5 s at most; after that it prints anyway and logs which blocks weren't ready.
4. Print rules for content:
   - headings stay with the next block, and code blocks, tables, figures and diagrams aren't split where they fit on a page;
   - long code lines wrap;
   - external links print their URL after the text in small muted type, except when the text already is the URL;
   - internal links print as text;
   - `<details>` blocks print open.
5. Layouts keep their own print rules: `handout` prints its sheet, `slides` one slide per page, `course` all steps one after another. A layout without print rules prints like an article.

### Print Site
6. **File › Print Site…** (⇧⌘P) opens a **Print Preview** window, a separate window titled `<folder> — Print`, showing the whole folder as one document:
   - **Cover:** the folder's name (or the home page's title), and below it the date and the page count.
   - **Contents:** every page in sidebar order, indented like the sidebar, each linking to its section.
   - **Pages:** every page rendered as in the app, in its layout's print form, each starting on a new sheet. Each page carries a small running label with its breadcrumb above its title.
7. Links inside the document are rewritten: a link to another page becomes a link to that page's section (`#zen-print-<n>`), anchors become `#zen-print-<n>-<anchor>`, and a link to a page that isn't in the document prints as text. Heading ids are prefixed, so pages with the same headings don't clash.
8. The preview window has a small toolbar that isn't printed:
   - **Print…** (primary, ⌘P);
   - **Close** (⌘W, Esc);
   - an option **Include drafts** (off);
   - a choice of **Pages:** All, or This section (the current page and the pages under it in the sidebar).

   Changing an option renders the document again. The toolbar shows "Preparing 34 pages…" while it renders and mounts.
9. Pages with `draft: true` are left out unless Include drafts is on. The same plugin trust as the app applies: blocked plugins show their blocked box. Pages that fail to render appear as a short error section, and the print still goes ahead.
10. Limit: 500 pages per document. Beyond that, the preview asks you to print a section, and Print… is disabled.
11. **`zen build` also writes `print.html`:** the same single document (cover, contents, pages), using the build's runtime, so it can be printed from any browser. `--no-print` leaves it out. It is linked from no page, but its path is shown in the build summary.

### Shared
12. One Go function builds the single document for the app and for `zen build`: `site.PrintDocument(in Input, o PrintOptions) (string, Report, error)`. The app serves it at `/_print/site?drafts=0&section=<path>` to the preview window. That route only answers the preview window's requests, through the same asset middleware.
13. In the frontend, `print/ready.ts` holds `whenReady(root, {timeout})`. Print Page and Print Site both use it, then call `window.print()`.

## Architecture

- `internal/core/site/print.go`:
  - `PrintDocument` reuses `Input` (pages, tree, render) and the link rewriter, with a print mode that makes in-document anchors instead of `.html` files;
  - the cover, contents and page wrappers come from `html/template`;
  - CSS is `zen.css` plus the print CSS of each layout used;
  - scripts are inlined as in portable mode, so the same document works in the app window and in `print.html`.
- `internal/app/print.go`:
  - `PrintSite()`, the service and menu action, opens or focuses the preview window (`?window=print`) through the window manager, as editor windows are opened;
  - `PrintPage()` makes the main window run its print flow (event `menu:print-page`);
  - the `/_print/site` handler adapts the app state to `site.Input`, reusing the headless pieces of `build.go`.
- `frontend/src/print/`:
  - `ready.ts`;
  - `preview.ts`, the preview window: it fetches `/_print/site`, mounts it with the static runtime's mounting code, and runs the toolbar;
  - `print.css`, the shared print rules.

  The shell routes `?window=print` to `preview.ts`, as `?window=editor` goes to the editor.

## Interfaces

```go
// internal/core/site
type PrintOptions struct {
	Drafts  bool
	Section string // "" = all; else this page and the pages under it in the tree
	Title   string // cover title; "" = the home page's title or the folder name
	Date    string // shown on the cover; "" = none (keeps the build deterministic)
	Max     int    // 0 = 500
}
func PrintDocument(in Input, o PrintOptions) (html string, rep Report, err error)
var ErrTooManyPages = errors.New("too many pages to print at once; print a section")

// internal/app
func (z *Zen) PrintPage() error   // asks the main window to print (menu:print-page)
func (z *Zen) PrintSite() error   // opens/focuses the Print Preview window
const EventPrintPage = "menu:print-page"
```

```ts
// frontend/src/print/ready.ts
export function whenReady(root: HTMLElement, opts?: { timeout?: number }): Promise<{ late: string[] }>;
// frontend/src/print/preview.ts
export function mountPreview(root: HTMLElement, api: Api, doc?: Document): () => void;
// api: printSite(), printPage(); on("menu:print-page")
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1, 3 | ⌘P and the menu event wait for ready and then print; the editor window prints its preview | `frontend/src/print/ready.test.ts`; `frontend/src/page/page.test.ts: "print waits for plugins"` |
| 2, 4 | print CSS: light colours, chrome and controls hidden, breaks, wrapping code, URLs after external links | `frontend/src/page/css.test.ts: "print rules"` (parsed CSS assertions) |
| 5 | layouts keep their print CSS | `internal/core/site/print_test.go: TestPrintLayoutsCSS` |
| 6 | cover, contents in sidebar order and indent, a section per page, breadcrumb label | `print_test.go: TestPrintDocument` |
| 7 | in-document links, anchors, prefixed ids, missing pages as text | `print_test.go: TestPrintLinks` |
| 8 | preview toolbar: options re-render, Print, Close, states, not printed | `frontend/src/print/preview.test.ts` |
| 9 | drafts, blocked plugins, failed pages | `print_test.go: TestPrintDraftsAndErrors`; `internal/app/print_test.go: TestPrintSiteTrust` |
| 10 | the 500-page limit and the section option | `print_test.go: TestPrintLimitAndSection` |
| 11 | `zen build` writes `print.html` unless `--no-print`; deterministic | `internal/core/site/site_test.go: TestBuildPrintFile`; `internal/cli/cli_test.go: TestBuildNoPrint` |
| 12 | `/_print/site` serves only to the app; the same output as `PrintDocument` | `internal/app/print_test.go: TestPrintRoute` |

Edge cases to cover:
- an empty folder (a cover and "No pages");
- a single page;
- pages with the same heading text;
- a page that links to itself;
- a slides page in the middle of a site;
- a page with only a diagram, still drawing when the timeout hits;
- a very long table;
- dark mode on;
- the print preview window opened twice, which focuses the first one.

## UI

- **Print Preview window:**
  - a narrow toolbar at the top with the title, the page count, Include drafts (a checkbox), Pages (a select: All, This section), Close, and Print… (primary);
  - under it, the document on a light "paper" background with sheet-sized sections (A4/Letter width from the locale), scrolling;
  - states: preparing (a spinner and a count), ready, too many pages (a message plus the section select), an error (the message plus Retry);
  - tokens from docs/UI.md; the toolbar follows the app theme and the document is always light.
- **Accessibility:**
  - the toolbar is a `toolbar` role with arrow-key movement;
  - the document is a `document` landmark;
  - Contents is a `nav`;
  - focus starts on Print….
- **Menu (Integration):** File gets a separator after Open Folder…, then Build Website… ⇧⌘B (ui/static-build), Print Page… ⌘P and Print Site… ⇧⌘P.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| The print document runs scripts from plugins the user didn't allow | It is made from the same render as the app, so blocked plugins stay blocked; scripts are inlined only for plugins in use and allowed | `TestPrintSiteTrust` |
| `/_print/site` becomes a way to read files | It returns only the rendered document of the open folder; no path parameter except `section`, which must be a page in the tree | `TestPrintRoute` |
| Hostile markdown in the combined document | Same sanitised HTML; ids are prefixed, so a page can't target another page's elements by id; the template auto-escapes | `TestPrintLinks` |
| The CSP of `print.html` | The same CSP as portable pages | `TestBuildPrintFile` |

## Notes

- **Out of scope:**
  - writing a PDF without the print panel (`zen print -o file.pdf`, which needs a headless WebKit; maybe later through WKWebView `createPDF`);
  - page numbers in the contents (WebKit has no `target-counter()`);
  - running headers and footers via `@page` margin boxes (not supported in WebKit);
  - choosing paper size inside zen (the print panel does it).
- **Integration (done by the lead at merge):**
  - `internal/app/menu.go`: in File, after Open Folder…, a separator, then Print Page… (⌘P → `PrintPage`) and Print Site… (⇧⌘P → `PrintSite`); `menu_test.go` expectations.
  - The api: add `printPage`, `printSite` and `menu:print-page` in `index.ts`, `wails.ts` and `fake.ts`.
  - `frontend/src/shell/shell.ts`: route `?window=print` to `mountPreview`; the main window's ⌘P handler calls the print flow; and remove the page's own ⌘P handler in `frontend/src/page/index.ts`, which the print flow replaces.
  - `internal/app/assets.go`: route `/_print/` to `print.go`'s handler.
  - `internal/cli`: the `--no-print` flag and the usage line.
  - docs/BUILD.md: `print.html`; a demo link; the changelog.
- **App Store / sandbox:** none. Printing uses the system panel.
- **Changelog:** `Added: Print Page (⌘P) and Print Site (⇧⌘P) — the whole folder as one document with a cover and contents, through the print panel (Save as PDF); zen build writes print.html.`

## Prompt

> Implement `specs/ui/print/spec.md` following the rules in
> `specs/README.md`. Context:
> - `internal/core/site` already builds static pages (link rewriting in `rewrite.go`, portable inlined scripts in `portable.go`, the shell in `shell.go`).
> - `internal/app/build.go` adapts the app state to `site.Input`.
> - `frontend/src/static/static.ts` mounts plugin scripts outside the app.
> - Editor windows (`internal/app/windows.go`, `?window=editor`) are the model for the preview window.
> - Page print rules are in `frontend/src/page/page.css` (`@media print`).
> - Layouts with print CSS: `handout`, `slides`.
>
> Write `PrintDocument` first (test-first, with the site fakes), then the print CSS and `whenReady`, then the preview window and the app service.
>
> Do not edit `menu.go`, the api files, `shell.ts`, `page/index.ts`, `assets.go` or `internal/cli`: give exact diffs. Do not launch the app.
>
> Done when the table's tests pass and `go vet ./internal/... && go test ./internal/...` and `cd frontend && npx tsc --noEmit && npx vitest run` are clean. Report:
> - the Integration diffs;
> - what is unverified: the real print panel and how WebKit lays out the printout. Measure it with a print-media screenshot of the built `print.html` in the browser pane if you can.
