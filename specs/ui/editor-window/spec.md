# ui/editor-window

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.2 |
| Branch | `feature/ui-editor-window` |
| Wave | 5 |
| Depends on | app/services, ui/shell, ui/editor, ui/page |
| Owns | `internal/app/app.go`, `internal/app/windows.go` (new), `internal/app/service.go` (new `OpenEditor` method only), `frontend/src/api/` (adding `openEditor`), `frontend/src/shell/`, `frontend/src/editor/`, `frontend/src/page/index.ts` (Edit button only) |

## Purpose

Edit opens the page in its own editor window, connected to the main window:
the main window keeps reading and updates when the editor saves.

## Behaviour

1. Edit (button and ⌘E) in the main window calls `api.openEditor(path)`. Go opens a new window for that page, or focuses the existing one for the same path. One editor window per page path.
2. An editor window loads the app at `/?window=editor&path=<url-encoded path>`. The shell sees `window=editor` and mounts only the editor feature (toolbar, source, split preview) under a header with the page title, an unsaved `●`, Split and Done. No sidebar, no breadcrumb links, no history.
3. Window title: `<page title> — Edit — zen`. Default size 1000×720, cascaded 24px from the main window.
4. The main window no longer has an in-window edit mode. `mode: "edit"` from startup (`zen edit <path>`) opens the main window in read mode **and** an editor window for that page.
5. Saving in the editor writes the file; the main window updates through the existing `page:changed` event. Also, the editor window emits `editor:saved {path}` so the main window re-renders even if file watching is unavailable.
6. Done (or Esc) in an editor window closes it, with the existing Save / Discard / Cancel prompt when there are unsaved changes. Closing it with the window's close button does the same (per-window close handshake; reuse the app/services pattern, keyed by window).
7. Closing the main window first closes its editor windows, each with its own prompt; if any editor cancels, the main window stays open. ⌘Q follows the same path.
8. Opening a different folder in the main window closes its editor windows the same way (cancellable).
9. Navigating in the main window never affects editor windows (this also removes the old Back/Forward off-by-one after cancelling leaving the editor).
10. Links in the editor preview stay inert (as today).

## Architecture

Go: `windows.go` tracks `map[path]*WebviewWindow` for editors and the main
window; `OpenEditor` is a bound service method. Close handshake generalised
from app.go to any window. Frontend: `shell` reads `location.search` once at
start; `api.openEditor(path): Promise<void>` added to `Api`, the fake (records
the call) and `wails.ts`. The page feature's Edit button calls it instead of
setting `mode`.

## Interfaces

```ts
interface Api { /* existing */ openEditor(path: string): Promise<void>; }
// shell: export function windowKind(search: string): { kind: "main" } | { kind: "editor"; path: string };
```
```go
func (z *Zen) OpenEditor(path string) error // validated as a workspace markdown path
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | open / focus same path / reject non-markdown or outside paths | `internal/app/windows_test.go: TestOpenEditor` (window factory injected) |
| 4 | startup in edit mode opens an editor window | `windows_test.go: TestStartupEditOpensWindow` |
| 6–8 | close ordering and cancellation (pure state machine) | `windows_test.go: TestCloseOrder` |
| 1 | Edit button / ⌘E call `openEditor`, no in-window edit mode | `frontend/src/page/page.test.ts` (updated) |
| 2 | `windowKind` parsing, editor-window layout | `frontend/src/shell/window.test.ts` |
| 5,6 | save emits `editor:saved`; Done closes with prompt | `frontend/src/editor/editor.test.ts` (updated) |
| api | fake and wails client | `frontend/src/api/fake.test.ts`, `wails.test.ts` |

## UI

Editor window header: page title (plain text), `●` when dirty, Split, Done
(primary). Same tokens and toolbar as ui/editor. Main window header: Edit button
unchanged in look.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Opening arbitrary paths via `?path=` | Go validates through `workspace.Resolve` + markdown check in `OpenEditor` and in `Source`/`Save` | `TestOpenEditor` |
| Losing work when the main window closes | editors closed first, each with its prompt, cancellable | `TestCloseOrder` |

## Notes

- **Out of scope:** multiple main windows, editing several files in one editor window, tabs.
- **Integration (lead):** none beyond merging; ui/lint (wave 5b) builds on this.
- **Changelog:** Changed: Edit opens the page in its own editor window.

## Prompt

> Implement `specs/ui/editor-window/spec.md` following the rules in `specs/README.md`.
> Read the Wails v3 source in `$(go env GOMODCACHE)/github.com/wailsapp/wails/v3@v3.1.0`
> for windows, hooks and events. Inject the window factory so Go tests need no display.
> Remove the main window's in-window edit mode cleanly (store `mode`, page Edit button,
> shell mounting) and update the existing tests that relied on it. Report everything
> that needs a display to verify.
