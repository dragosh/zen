# app/logging

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/app-logging` |
| Wave | 30 |
| Depends on | ui/developer-tab (the log ring), #25 (Console tab) |
| Owns | `kit/desktop/**` (logs and the app lifecycle), `engine/session/**` (logging only: no behaviour change), `engine/watch/**` (logging only), `apps/zenmark/*.go` (logging only), `kit/ui/src/shell/{devpanel,console}.ts` + tests (the level filter, if needed) |

## Purpose

The Console tells the story of the app. It shows what started, what opened,
what was rendered or reloaded, and what went wrong, each at the right level.
That way someone testing the app, or reporting a bug, sees what happened. It
is empty today because the app logs almost nothing (about five `slog` calls).

## Behaviour

1. **Levels** follow `log/slog`. Every entry has a **source**: `app` for the Go side, `plugin` for plugin backends and helpers, `page` for the page's JS console (unchanged).

   | Level | Used for |
   |---|---|
   | debug | detail useful when tracing: each page render (path, ms), each file event, each cache hit or miss |
   | info | what a user did or the app did once |
   | warn | something went wrong but the app carries on |
   | error | an action failed |

   **info covers:**
   - the app started (version, channel, features);
   - a folder opened (name, pages, index time), closed, or was switched;
   - the config was reloaded (which file);
   - a dialect or layout was switched;
   - a plugin was allowed or blocked;
   - an export started or finished (pages, time, output);
   - a print;
   - Settings saved a value.

   **warn covers:**
   - render warnings (unknown layout, dialect not in the build);
   - a broken config value;
   - the legacy `.zen.yml` notice;
   - a plugin that failed and fell back;
   - a watcher error;
   - a slow index (over 2 s).

   **error covers:**
   - a service method that returned an error to the UI (the method and its message);
   - a failed save or export;
   - a panic recovered in a handler.
2. **Paths:** messages use folder-relative paths, never the user's home or absolute paths. The folder's name may appear, but its full path may not.
3. **No secrets:** plugin options with `scope: user`, tokens and file contents are never logged.
4. **Volume:**
   - debug entries are rate-limited per kind, at most 20 a second, with a single "… N more" entry;
   - the ring keeps its capacity;
   - the Console's level filter starts at **Info+**, and the choice is remembered per machine;
   - All levels shows debug.
5. **From the start:** entries logged before the window or the Console existed are in the ring and show when the Console opens. The startup info line is the first entry.

## Architecture

- **The helper:** `kit/desktop/logs.go` gets `LogApp(level, msg, attrs...)`, which uses `slog` with `source=app`. Attrs are rendered as `key=value` after the message, as today.
- **Where the logging goes:**
  - service methods log their errors in one place: a wrapper or a deferred helper in `apps/zenmark/service.go` and the Wails adapters;
  - the session logs open, close, reload and index;
  - the watcher logs its events at debug;
  - the export and print code logs start, finish and failure.
- **Paths:** a `relPath(root, abs)` helper keeps the paths folder-relative.
- **The Console:** `devpanel.ts` gets the default level and remembers it (`zen.console.level`).

## Interfaces

```go
// kit/desktop
func LogApp(level slog.Level, msg string, attrs ...any)
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Opening a folder logs info with the name and count; a failing service method logs error; render warnings log warn; a file event logs debug | `engine/session/logging_test.go`, `apps/zenmark/logging_test.go` (capture with a test handler) |
| 2 | No absolute paths or home directory in any entry from a folder open, render or export run | `logging_test.go: TestNoAbsolutePaths` |
| 3 | User-scoped options and file contents never logged | `TestNoSecretsLogged` |
| 4 | Debug rate limit; the Console defaults to Info+ and remembers the choice | `kit/desktop/logs_test.go: TestDebugRateLimit`; `kit/ui/src/shell/devpanel.test.ts: "default level"` |
| 5 | The startup line is the first entry; the backlog shows when the Console opens | `kit/desktop/logs_test.go`, `devpanel.test.ts` |

## UI

UI consistency: the Console is unchanged, apart from the level filter's default and its memory.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Logs leak paths or secrets into bug reports | Folder-relative paths only; no option values, file contents or tokens | `TestNoAbsolutePaths`, `TestNoSecretsLogged` |

## Notes

- **Runs beside:** ui/inspector-details (`kit/ui/src/inspector`, `page/index.ts`). The lead edits `apps/zenmark/frontend/src/shell/{settings,shell}.ts`.
- **Changelog:** `Added: the Console shows what the app does, at debug, info, warn and error levels (opening a folder, reloads, exports, failures), with folder-relative paths only.`

## Prompt

> Implement `specs/app/logging/spec.md` following `specs/README.md` and CLAUDE.md.
>
> Work in `../zen-worktrees/app-logging`. Tests first. Logging only: no behaviour change outside logging. Do not launch the app, push or touch main.
>
> Done when the table's tests pass, `go vet ./... && go test ./...` pass (run `npx vite build` first), and `npx tsc --noEmit && npx vitest run` pass in `apps/zenmark/frontend`.
