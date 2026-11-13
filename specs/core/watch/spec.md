# core/watch

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/core-watch` |
| Wave | 2 |
| Depends on | core/workspace |
| Owns | `internal/core/watch/` |

## Purpose

Tells the app when files in the workspace change so the open page and the
sidebar update by themselves.

## Behaviour

1. `Start(root, onEvent)` watches the root recursively (new subdirectories are added as they appear), skipping the same directories as `workspace.Files`.
2. Events are reported for markdown files, asset files and `.zen.yml` only, with workspace-relative `/` paths and a kind: `changed`, `created`, `removed` (rename = removed + created).
3. Bursts are debounced: events for the same path within 100 ms collapse into one; the callback receives a batch (slice) at most every 100 ms.
4. Editor save patterns (write to temp + rename over the file) produce a single `changed` for the real file and nothing for the temp file.
5. `Stop()` releases all watches, stops the goroutine and guarantees no callback afterwards. Calling it twice is safe.
6. If the watcher fails (limit reached, root removed) `onEvent` is not called again and `Err()` returns the cause.

## Architecture

`github.com/fsnotify/fsnotify` (in go.mod). No Wails; app/services turns
batches into `page:changed` / `workspace:changed` events.

## Interfaces

```go
package watch

type Kind string // "changed" | "created" | "removed"
type Event struct{ Path string; Kind Kind }

type Watcher struct{ /* unexported */ }
func Start(root string, onEvent func([]Event)) (*Watcher, error)
func (w *Watcher) Stop()
func (w *Watcher) Err() error
```

## Tests

| # | Behaviour | Test (`internal/core/watch/watch_test.go`) |
|---|---|---|
| 1,2 | create / modify / delete, new subdirectory, ignored dirs and extensions | `TestEvents` |
| 3 | debounce of rapid writes | `TestDebounce` |
| 4 | atomic-save pattern | `TestAtomicSave` |
| 5 | stop, double stop, no late callbacks | `TestStop` |
| 6 | root removed | `TestRootRemoved` |

Tests use `t.TempDir()` and wait with timeouts (max 2 s), never fixed sleeps longer than the debounce.

## UI

None.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Watching outside the workspace through symlinks | symlinked directories are not followed | `TestEvents` (symlink case) |
| Resource exhaustion on huge trees | ignored dirs skipped; watcher error surfaced, not retried in a loop | `TestRootRemoved` |

## Notes

- **Out of scope:** watching the global config file, polling fallback for network drives.
- **Integration:** none.
- **Changelog:** Added: live reload when files change on disk.

## Prompt

> Implement `specs/core/watch/spec.md` following the rules in `specs/README.md`, using
> fsnotify (already in go.mod). Keep the ignore rules consistent with
> `specs/core/workspace/spec.md` behaviour 3 (duplicate the small list; do not import
> workspace internals). Done when the tests pass reliably 20 times in a row
> (`go test -count=20 ./internal/core/watch`).
