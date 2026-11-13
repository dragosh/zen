# app/services

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/app-services` |
| Wave | 3 |
| Depends on | core/workspace, core/config, core/plugins, core/render, core/index, core/watch, cli/commands, ui/shell |
| Owns | `internal/app/`, `frontend/src/api/wails.ts` |

## Purpose

Connects the Go core to the window: implements the Service API of PLAN.md so
the real app does what the fake API does.

## Behaviour

1. `Run(Options{Assets, Dir, Page, Mode})` opens the workspace for `Dir` (empty → none), creates one window, registers the `Zen` service and starts watching.
2. `Startup()` returns `{workspace, page, mode}`; `page` is `Options.Page`, else the workspace home.
3. `PickFolder()` shows the native directory dialog; on choice it replaces the workspace (stops the old watcher, rebuilds config, renderer and index, starts a new watcher) and returns it; cancel returns `null`.
4. `Page(path)` reads through the workspace, renders, adds the breadcrumb from the index. `Source`, `Preview`, `Save` map to workspace read, renderer, workspace write. Errors are returned as messages without absolute paths.
5. `OpenExternal(url)` accepts only `http`, `https` and `mailto`, shows a native confirm dialog with the full URL, and opens the default browser on confirmation; returns whether it opened.
6. Watch batches: a changed/created/removed markdown file re-indexes and emits `workspace:changed` only if the tree changed, and `page:changed {path}` per changed markdown file; a changed asset emits `page:changed` for pages that reference it (alpha: for the currently served page); `.zen.yml` change reloads config and emits `page:changed` for the current page.
7. HTTP middleware: `/_ws/<path>` serves `workspace.Asset` (correct content type, `X-Content-Type-Options: nosniff`, SVG served with `Content-Security-Policy: sandbox`); `/_plugins/<id>/<file>` serves the plugin's asset FS; everything else falls through to the embedded frontend. A CSP header on the frontend allows scripts and styles from `'self'` only, images from `'self'` and `https:`.
8. `frontend/src/api/wails.ts` implements `Api` and exports it as `export default` (ui/shell's `getApi()` loads it from there) over the generated bindings and Wails events.
9. Registers built-in plugins in order: `highlight`, `diagram` (those merged).
10. Removes `documents.go` and the `Files` option.
11. Closing the window with unsaved editor changes asks first: the window close hook asks the frontend (event `app:before-close`, answered by ui/editor) and only closes on Save/Discard.
12. The watch callback never calls `Watcher.Stop` (it would deadlock); swapping workspaces stops the old watcher from the service goroutine.

## Architecture

Only package importing Wails. `service.go` (bound methods, thin), `state.go`
(workspace + derived objects behind a mutex, swap on PickFolder),
`assets.go` (middleware), `app.go` (window, menu: File › Open Folder ⌘O).
Logic is tested through `state` and the middleware with `httptest`, without a window.

## Interfaces

Go service `Zen` with methods `Startup, PickFolder, Page, Source, Preview,
Save, OpenExternal`, JSON shapes exactly as PLAN.md "Service API". Events
`workspace:changed`, `page:changed`.

## Tests

| # | Behaviour | Test (`internal/app/*_test.go`) |
|---|---|---|
| 2 | startup state | `TestStartup` |
| 3 | swapping workspaces stops the old watcher | `TestSwapWorkspace` |
| 4 | page with breadcrumb, source, preview, save round trip, error text | `TestPageLifecycle` |
| 5 | scheme allow-list (dialog and opener injected) | `TestOpenExternal` |
| 6 | change → events (event sink injected) | `TestWatchEvents` |
| 7 | `/_ws`, `/_plugins`, traversal attempts, headers, CSP | `TestAssetMiddleware` |
| 8 | wails client maps calls and events (bindings mocked) | `frontend/src/api/wails.test.ts` |

## UI

Native: folder dialog, confirm dialog "Open https://… in your browser?", File menu.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Path traversal through `/_ws/` or service arguments | everything goes through `workspace.Resolve`/`Asset` | `TestAssetMiddleware`, `TestPageLifecycle` |
| Opening dangerous URL schemes (`file:`, custom handlers) | allow-list + user confirmation in Go | `TestOpenExternal` |
| SVG with script served as a document | `CSP: sandbox` + `nosniff`; pages embed it via `<img>` | `TestAssetMiddleware` |
| Script injection in the webview | CSP `script-src 'self'` | `TestAssetMiddleware` |
| Leaking absolute paths | error messages use relative paths | `TestPageLifecycle` |

## Notes

- **Out of scope:** multiple windows, recent folders, settings UI, single-instance.
- **Integration:** this spec *is* the Go-side integration; the lead reviews it line by line.
- **App Store / sandbox:** folder access comes from the dialog or CLI argument; needs `com.apple.security.files.user-selected.read-write` (packaging/macos).
- **Changelog:** Added: folder workspaces in the app with live reload.

## Prompt

> Implement `specs/app/services/spec.md` following the rules in `specs/README.md`. All
> core packages are merged in your base; read their specs for exact signatures. Keep
> Wails-specific code thin and inject dialogs, opener and event sink as small interfaces
> so tests need no window. `wails3 generate bindings` produces `frontend/bindings`; write
> `api/wails.ts` against it. Done when the tests pass, `task build` succeeds, and you have
> listed what could not be verified without a display.
