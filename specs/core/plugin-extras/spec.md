# core/plugin-extras

| | |
|---|---|
| Status | done |
| Target version | 0.2.0-alpha.1 |
| Branch | `feature/core-plugin-extras` |
| Wave | 7 |
| Depends on | core/extplugins, ui/plugins |
| Owns | `internal/core/plugins/`, `internal/core/extplugins/`, `internal/app/` (plugin services, events, menu), `frontend/src/plugins/`, `frontend/src/api/`, `frontend/src/nav/` (badges), `frontend/src/page/index.ts` (anchor + print wiring), `docs/PLUGINS.md` (document the additions) |

## Purpose

The small core additions the slides, course, quiz and sandbox plugins need,
so each of those can then be built in its own folder without touching the core.

## Behaviour

1. **Built-in plugins can act.** New optional Go interfaces in `core/plugins`:
   `ActionHook { Action(ctx Context, payload []byte) ([]byte, error) }` and
   `StreamHook { Stream(c context.Context, ctx Context, payload []byte, emit func(chunk []byte)) error }`.
   `Context` gains `DataDir string` (the plugin's writable folder, same location rule as external plugins' `ZEN_DATA_DIR`) and `Workspace string`. `PluginInvoke`, `PluginStorageGet/Set` work for a built-in plugin when it is enabled (invoke: and implements `ActionHook`); they stay refused for unknown or switched-off plugins.
2. **Streams.** `PluginStream(id, page, json) (callID string, error)` (`page` as for `PluginInvoke`) starts the plugin's stream (built-in `StreamHook`, or an external plugin declaring hook `stream`: its stdout is forwarded as it arrives, split on newlines, stdin = payload, `ZEN_HOOK=stream`). Chunks arrive as event `plugin:stream` `{call, chunk}`, **batched**: at most one chunk event per call every 50 ms, `chunk` the lines produced since the last event joined with `\n` (no trailing newline); the 4 MB limit counts every line once, newline and empty lines included; the end as `{call, done: true, error: ""}` (non-zero exit / returned error → `error` text). `PluginCancel(callID)` cancels the context / kills the process group; a cancelled stream ends with `error: "cancelled"`. Limits: timeout from the manifest for external (`stream` may be up to `10m`, default 60s), 10 minutes for built-ins unless their context ends earlier; 4 MB total output per stream then it is cut with `error: "output limit"`; at most 4 streams at once; all streams are cancelled when the folder changes or the app quits, and a plugin's streams when it is blocked or switched off.
3. **User-scoped options.** An option with `scope: user` (`Option.Scope == "user"`, in `plugin.yml`: `options: { images: { type: map, scope: user } }`) takes its value from the global `config.yml` only: values in `.zen.yml` or frontmatter are ignored with a warning. Option types gain `map` (string → string) and `list` (strings).
4. **Storage keys.** `PluginStorageKeys(id, prefix) []string` (sorted).
5. **Runtime additions** (`ctx` in `frontend/src/plugins/runtime.ts`):
   - `ctx.stream(payload, onChunk) → { done: Promise<void>, cancel(): void }` (rejects with the stream's error text),
   - `ctx.storage.keys(prefix?) → Promise<string[]>`,
   - `ctx.anchor()` (current anchor, "" when none) and `ctx.onAnchor(fn) → off` (called when the page's anchor changes without a reload; handlers removed on unmount),
   - `ctx.badge(path, text | null)` sets or clears a short label for a page in the sidebar (kept in the store for the window's lifetime, max 12 chars, shown muted after the title with `title` = text),
   - streams started by a block are cancelled when it unmounts.
6. **Print.** File › Print… (⌘P) in the main window calls `window.print()`; `page.css` gets a minimal `@media print` (no sidebar/header/panels, article full width). Plugins may add their own print rules.
7. `docs/PLUGINS.md` documents `stream` (backend table + `ctx.stream`), `scope: user`, `map`/`list` option types, `ctx.storage.keys`, `ctx.anchor/onAnchor`, `ctx.badge`, and that built-in plugins use the same `ctx`.

## Interfaces

```go
// service additions
func (z *Zen) PluginStream(id, page, json string) (string, error)
func (z *Zen) PluginInvoke(id, page, json string) (string, error)
func (z *Zen) PluginCancel(call string) error
func (z *Zen) PluginStorageKeys(id, prefix string) ([]string, error)
const EventPluginStream = "plugin:stream" // {call string, chunk string, done bool, error string}
```
```ts
interface Api { pluginStream(id: string, page: string, json: string): Promise<string>; pluginCancel(call: string): Promise<void>;
  pluginStorageKeys(id: string, prefix: string): Promise<string[]>;
  on(event: "plugin:stream", fn: (e: { call: string; chunk: string; done: boolean; error: string }) => void): () => void; }
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | built-in action/storage allowed when enabled, refused when off | `internal/app/plugins_test.go` (added) |
| 2 | external and built-in streams: chunks in order, done, error, cancel kills children, limits, concurrency | `internal/core/extplugins/stream_test.go`, `internal/app/stream_test.go` |
| 3 | user scope, map and list types, warnings | `internal/core/plugins/options_test.go`, `internal/core/extplugins/manifest_test.go` (added) |
| 4 | keys | `internal/app/plugins_test.go` (added) |
| 5 | stream/cancel/unmount, keys, anchor, badge (frontend: see core/plugin-hardening part B) | `frontend/src/plugins/runtime.test.ts` (added), `frontend/src/nav/tree.test.ts` (badge) |
| 6 | print rules present; ⌘P handler | `frontend/src/page/css.test.ts`, `page.test.ts` |
| api | fake and wails client | `frontend/src/api/*.test.ts` |

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A stream that never ends or floods | timeout, 4 MB cap, 4 at once, cancel on unmount/folder change/quit | stream tests |
| A page widening a user-only option (e.g. sandbox images) | `scope: user` ignores folder and page values | `options_test.go` |
| One window reading another plugin's stream | call ids are random 128-bit; the runtime only delivers a call's chunks to the ctx that started it | `runtime.test.ts` |

## Notes

- **Changelog:** Added: plugin streams, user-only plugin options, sidebar badges, Print (⌘P).

## Prompt

> Implement `specs/core/plugin-extras/spec.md` following the rules in `specs/README.md`.
> Read docs/PLUGINS.md and the wave-6 code first (`internal/core/extplugins`, `internal/app/extplugins.go`,
> `frontend/src/plugins/runtime.ts`). Keep every existing test green.
