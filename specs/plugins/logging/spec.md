# plugins/logging

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/12-builtin-plugins` (roadmap step 3, #12) |
| Wave | 31 |
| Depends on | app/logging (the Console, sources and levels), core/plugin-extras (invoke and streams) |
| Owns | `engine/plugins/log.go` (new) + test, `engine/session/{extplugins,stream}.go` (logging only) + `engine/session/plugin_logging_test.go`, `builtins/plugins/{sandbox,quiz,diagram}/**` (logging only) + tests, `runtime/test/builtin-{diagram,quiz}*.test.ts` if they need a case |

## Purpose

The Console tells the story of the built-in plugins too. Today it shows almost
nothing for them: a sandbox run, a quiz check or a broken diagram leaves no
entry at Info+, and a successful stream logs only at debug.

## Behaviour

1. **Source `plugin`.** Every entry below has source `plugin`, and its message starts with the plugin id: `sandbox: …`.
2. **The session, for every plugin** (built-in or external):
   - **Each action call** logs at debug: `<id>: action <op> (<ms> ms)`. Here `<op>` is the payload's `op` field when it is a string of 1–20 letters, digits or `-`, and is left out otherwise.
   - **Each stream** logs at debug when it starts and when it ends, the end with its duration.
   - Failures keep their current warn entries.
   - The payload and the answer are never logged.
3. **sandbox** logs the steps of a run at info, in order, and never the program's output, which is in the block (changed after the first check in the app):
   - `sandbox: run <image key>: <cmd>`, with the command cut to 200 characters;
   - `checking the container service`, then `the container service is running`, or a warn if it is stopped or missing;
   - `wrote <file> (<n> bytes) to a temporary folder, mounted at /work`;
   - `starting container zen-<id> (image <key>)`. Image references and limits are user-scoped options, so they are not logged;
   - `the program is running (first output after <s> s)`;
   - `exit <code> in <s> s`, or a warn for a timeout or for output over the limit, or `stopped`;
   - `removed the temporary folder`.

   A status probe logs at debug.
4. **quiz:**
   - A Check logs at info: `quiz <id>: checked, <score>% (passed|not passed)`.
   - Reading or saving results that fails is a warn, as today in the page's console.
   - Answers and questions are never logged.
5. **diagram:** a diagram that fails to draw logs `diagram: <mermaid's error, first line, 200 chars>` as a warn in the page's console. The Console already shows the page's console as source `console`, so this needs nothing new.
6. **Volume:** debug entries go through the existing rate limit. Info output lines are capped by item 3.

## Architecture

- **`engine/plugins/log.go`:** `func Log(id string, level slog.Level, msg string, attrs ...any)`. It wraps `slog` with `"source", "plugin"` and the `"<id>: "` prefix, so builtins don't import `kit/desktop`.
- **The session:** `PluginInvoke` and `PluginStream` time the call and log through `plugins.Log`. A small `opOf(payload)` helper reads `op` with `encoding/json` into a struct that has only `Op string`.
- **sandbox.go:** logs start, the capped output lines and the end from its `Stream` and `Action`.
- **quiz.go:** logs the score from its `check` action.
- **diagram.js:** adds the `console.warn` where it shows the error in the block.

## Interfaces

```go
// engine/plugins
func Log(id string, level slog.Level, msg string, attrs ...any)
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Action and stream calls log at debug with op and ms; the payload never appears | `engine/session/plugin_logging_test.go` (a capture handler) |
| 2 | Sandbox start, output (capped at 20 lines, 200 chars), exit, timeout, stop and stopped service | `builtins/plugins/sandbox/logging_test.go` (fake runner as in the existing tests) |
| 3 | Quiz logs the score, not the answers | `builtins/plugins/quiz/logging_test.go` |
| 4 | A failing diagram warns in the page's console | the diagram runtime test |
| 5 | No code, answers or option values in any entry | `TestNoSecretsLogged` cases added in the tests above |

## UI

None. The Console is unchanged.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Logs leak code, answers or config | Only ids, keys, counts, exit codes, durations and capped program output | #5 |

## Notes

- **Changelog:** `Added: the Console shows what the built-in plugins do: sandbox runs with their output and exit, quiz scores, diagram errors.`

## Prompt

> Implement `specs/plugins/logging/spec.md` following CLAUDE.md, in the existing worktree `../zen-worktrees/issue-12` on branch `feature/12-builtin-plugins`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
