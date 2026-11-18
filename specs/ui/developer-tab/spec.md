# ui/developer-tab

| | |
|---|---|
| Status | done |
| Target version | 0.5.0 |
| Branch | `feature/ui-developer` |
| Wave | 18 |
| Depends on | kit/desktop-ui, engine/session |
| Owns | `kit/desktop/logs.go`, `kit/ui/src/shell/{developer,console,devpanel}.ts`, the footer and bottom slot of `kit/ui/src/shell/layout.ts`, the Developer section of `kit/ui/src/shell/settings.ts`, `apps/zenmark/logs.go` |

## Purpose

A developer can see what Zenmark and its plugins are doing: a bottom panel with
the Go side's log, plugin errors and the frontend console, switched on with a
setting that is off by default.

## Behaviour

1. Settings has a **Developer** section with a checkbox "Show developer tab", off by default. It is remembered on this Mac (`localStorage` key `zen.developer`, like the theme) and followed live by every window.
2. When on, a **Developer** button (terminal icon, tooltip "Developer ⌥⌘J") sits in the sidebar footer next to the settings cog. When off it is absent and the panel is closed.
3. ⌥⌘J toggles the panel while the setting is on; it does nothing while it is off.
4. The panel is a bottom panel across the content area, below the page (and below Settings). Its height is resizable with a drag handle (and arrow keys on the handle), clamped to 120 px to 70% of the content, and remembered (`zen.developer.height`). ✕ closes it; Esc closes it when focus is inside it.
5. Source tabs: **All**, **App** (Go side), **Plugins** (plugin errors, stderr and stream failures) and **Console** (frontend `console.*`, uncaught errors, unhandled rejections). Each entry shows time, level (debug, info, warn, error), source and message; levels use colour tokens.
6. A level filter (minimum level), a text filter (case-insensitive substring of the message), **Clear** and **Copy all** (the visible entries as text lines).
7. The list scrolls to the newest entry; when the user scrolls up it stops following, and a "Jump to latest" control resumes.
8. At most 2000 entries are kept in the frontend and in the Go ring; the oldest drop first.
9. Console capture wraps `console.debug/log/info/warn/error` and listens for `error` and `unhandledrejection`. The real console still receives every call unchanged. Capture starts when the window starts, so the panel shows what happened before it was opened.
10. Go: a bounded ring buffer (capacity 2000, sequence numbers from 1) holds log entries. `log/slog` is the app's logger: a handler writes every record to stderr (as before) and to the ring; the standard `log` package goes through it too. A record's `source` attribute ("app" by default, "plugin") says which tab it belongs to. Messages are truncated to 2000 bytes.
11. The Zen service has `Logs(since int) []LogEntry` (entries with `seq` > since, oldest first) and emits `log:entry` with a batch (array) of entries. Emission is batched: at most one event per 250 ms with at most 100 entries, so a chatty plugin cannot flood the window.
12. Plugin failures (a backend that exits non-zero, times out or cannot start; a failed stream) are logged with source "plugin". The lint "file too large" message goes through slog.
13. The host's `logs` and `onLog` are optional: an app without them shows only the Console tab.
14. Never logged: page contents, secrets, plugin storage values. Only paths, plugin ids and error text that the UI already shows.

## Architecture

`kit/desktop/logs.go` (package `desktop`; imports only the standard library and the sink interface): `LogRing`, `NewLogHandler`, `LogRing.Stream`. Layering: apps → kit → engine; the engine only calls `log/slog`, it never imports the ring. `apps/zenmark/app.go` builds the ring, installs the slog default and starts the streamer. `Zen.Logs` returns `ring.Since`.

Frontend: `kit/ui/src/shell/console.ts` (capture), `devpanel.ts` (the panel's DOM), `developer.ts` (setting, key, button, mount into the layout). `layout.ts` gets a footer button slot and a bottom slot below `main`. The panel reads `host.logs` once when opened, then `host.onLog`; entries from the backend are de-duplicated by `seq`.

## Interfaces

```go
// kit/desktop
type LogEntry struct {
	Seq     int    `json:"seq"`
	Time    int64  `json:"time"` // Unix milliseconds
	Level   string `json:"level"` // debug | info | warn | error
	Source  string `json:"source"` // app | plugin
	Message string `json:"message"`
}
const LogCapacity = 2000
func NewLogRing(capacity int) *LogRing
func (r *LogRing) Add(level, source, message string) LogEntry
func (r *LogRing) Since(seq int) []LogEntry
func (r *LogRing) Stream(sink Emitter, event string, every time.Duration, batch int) (stop func())
func NewLogHandler(r *LogRing, tee slog.Handler) slog.Handler
// apps/zenmark
func (z *Zen) Logs(since int) []LogEntry
// event "log:entry": []LogEntry
```

```ts
// kit/ui/src/host.ts (KitHost, optional)
export type LogEntry = { seq: number; time: number; level: LogLevel; source: "app" | "plugin" | "console"; message: string };
logs?(since: number): Promise<LogEntry[]>;
onLog?(fn: (entries: LogEntry[]) => void): () => void;
// kit/ui/src/shell/developer.ts
export function developerEnabled(): boolean; export function setDeveloperEnabled(on: boolean): void;
export function createDeveloper(layout: Layout, host: KitHost, doc?: Document): { destroy(): void; toggle(): void; isOpen(): boolean };
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | setting off by default, remembered, live | `kit/ui/src/shell/developer.test.ts: setting` |
| 2 | button only when on; tooltip | `developer.test.ts: button` |
| 3 | ⌥⌘J toggles only when on | `developer.test.ts: shortcut` |
| 4 | height clamp, remembered, ✕ and Esc | `developer.test.ts: resize and close` |
| 5-6 | tabs, level/text filter, Clear, Copy all | `kit/ui/src/shell/devpanel.test.ts` |
| 7 | follow pauses when scrolled up | `devpanel.test.ts: follow` |
| 8 | frontend cap | `devpanel.test.ts: cap` |
| 9 | console capture keeps real console | `kit/ui/src/shell/console.test.ts` |
| 10 | ring bound, sequence, handler tees, source attr, truncation | `kit/desktop/logs_test.go: TestLogRing*, TestLogHandler*` |
| 10 | concurrent use (race detector) | `logs_test.go: TestLogRingConcurrent` |
| 11 | `Logs(since)`, batching and rate limit | `logs_test.go: TestStream*`, `apps/zenmark/logs_test.go` |
| 12 | plugin failure logged | `engine/extplugins/log_test.go` |
| 13 | host without logs: Console only | `developer.test.ts: no host logs` |

Edge cases: `since` beyond the newest entry; `since` older than the ring; empty message; a burst of 10000 records.

## UI

Empty: "No log entries yet." Panel header with tabs, filters and buttons; entries in a monospace list (`role="log"`, `aria-live` off so a flood is not announced). Keyboard: ⌥⌘J, Esc, arrows on the resize handle, all controls are buttons or inputs. Level colours: `--lint-error`, `--lint-warning`, muted for info and debug; everything else from the tokens in docs/UI.md.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Secrets or page text in logs | only paths, ids and error text are logged; stderr of a plugin is limited as in its error message | review, `engine/extplugins/log_test.go` |
| Unbounded memory | ring and UI caps; message truncation | `TestLogRing*`, `devpanel.test.ts: cap` |
| UI flood | batch and rate limit | `TestStream*` |
| Markup in a message | textContent only | `devpanel.test.ts: text` |

## Notes

- **Out of scope:** persisting logs to disk, the log in built sites, a Network tab.
- **Integration:** `Logs` added to `serviceMethods` in `apps/zenmark/bindings_test.go`; one call in `shell.ts` mounts the developer controller after the layout; `main.ts` installs console capture.
- **App Store / sandbox:** none; nothing leaves the process.
- **Changelog:** Developer tab: a bottom panel (⌥⌘J, Settings › Developer) with the app's log, plugin errors and the console.

## Prompt

Implement this spec following specs/README.md: tests first, then the Go ring and handler, the service method, the frontend panel, and the docs.
