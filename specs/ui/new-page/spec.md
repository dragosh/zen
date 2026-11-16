# ui/new-page

| | |
|---|---|
| Status | done |
| Target version | next alpha |
| Branch | `main` (built with codeblock, slides keys, problems tabs) |
| Wave | 9 |
| Depends on | app/services, ui/shell, ui/editor-window, plugins/layouts |
| Owns | `internal/app/newpage.go`, `frontend/src/shell/newpage.ts`, File › New Page menu item, `Api.createPage`, event `menu:new-page` |

## Purpose

Start a new page from the app instead of a text editor or Finder, and pick its
layout (plain page, slides or course) up front so the frontmatter is right.

## Behaviour

1. **Menu:** File › New Page… (⌘N) brings the main window forward and emits `menu:new-page`. ⌘N in the main window's page does the same (for the browser build). Nothing happens on the welcome screen or in an editor window, or when the dialog is already open.
2. **Dialog:** a modal (`role=dialog`, `aria-modal`) with a **Layout** radio group of three cards (Page, Slides, Course, each with a one-line hint; ← → move) and a **Name** field prefilled with the shown page's folder plus `untitled` (the `untitled` part selected). Esc, Cancel or a click outside close it; Tab stays inside; focus returns where it was.
3. **Create:** `Zen.CreatePage(name, layout)` trims the name, adds `.md` unless it ends in `.md`/`.markdown`, refuses other extensions, `..`, absolute paths, NUL, and any path segment starting with a dot; creates missing folders inside the workspace; **never overwrites** (`a page with that name already exists`). Errors show in the dialog, which stays open.
4. **Templates:** title from the file name (`my-first_deck` → `My first deck`, quoted for YAML when needed).
   - Page: `title` frontmatter and a `#` heading.
   - Slides: `layout: slides`, `layoutOptions: { ratio: "16:9" }`, three slides with a speaker note.
   - Course: `layout: course`, `layoutOptions: { id: <slug of the title> }`, two steps, the first with a `:::check` list.
5. On success the main window shows the new page and opens it in the editor window.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 3,4 | paths, templates, refusals, no overwrite | `internal/app/newpage_test.go` `TestCreatePage`, `TestTitleOf`, `TestCreatePageNoWorkspace` |
| 4 | templates render with the real layout plugins, no warnings | `TestCreatePageTemplatesRender` |
| 2,3 | dialog, layout keys, create, error, close | `frontend/src/shell/newpage.test.ts` |
| 1,5 | menu event and ⌘N, navigate + open editor, not on welcome | `frontend/src/shell/shell.test.ts` |

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Writing outside the folder | `newPagePath` cleaning plus `Workspace.Resolve` | `TestCreatePage` (`../out`, `a/../../out`) |
| Replacing a user's file | `Lstat` before writing; existing → error | `TestCreatePage` (README) |
| Creating hidden or non-markdown files | dot segments and other extensions refused; `Workspace.Write` only writes markdown | `TestCreatePage` |
