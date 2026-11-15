# ui/lint

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.2 |
| Branch | `feature/ui-lint` |
| Wave | 5b |
| Depends on | core/lint, ui/editor-window, ui/page |
| Owns | `internal/app/lint.go` (new), `internal/app/service.go` (new `Lint` method only), `frontend/src/api/` (adding `lint`), `frontend/src/lint/` (new), `frontend/src/page/` (lint wiring), `frontend/src/editor/` (lint wiring) |

## Purpose

Spelling mistakes and style flags from Vale are visible where you read and
where you write.

## Behaviour

1. **Service.** `Lint(path, content)` returns `{available, alerts}`. `available` is false when no Vale binary was found (`lint.ErrUnavailable` at startup); then the UI shows nothing at all. Content is the editor's unsaved text, or the file on disk when `content` is empty and the reader asks.
2. **Settings.** `lint: false` in frontmatter, `.zen.yml` or global config turns linting off for that page/folder/everywhere (default on when Vale is available). Read through core/config like a plugin switch (`Settings.Plugin("lint")`), so no config schema change.
3. **Reader.** After a page renders, the page feature asks `api.lint(path, "")`. Each alert's `Match` is wrapped, inside text nodes of the article only (never inside `code`, `pre`, links' href or plugin blocks), in `<mark class="zen-lint zen-lint-<severity>" title="<Check>: <Message>">`. When a match occurs several times in the same block, the occurrence is chosen by order of alerts on that source line; when it cannot be placed, it is listed only in the panel.
4. **Reader panel.** A small count button in the header actions (`3 issues`, hidden when 0) toggles a panel listing alerts: line, severity, message, matched text. Clicking an item scrolls to and briefly highlights its mark. Esc closes.
5. **Editor.** Lints the unsaved text 600 ms after typing stops (latest response wins) and right after load and save. A backdrop layer behind the transparent textarea mirrors its text (same font, wrapping, padding, scroll) and draws the same marks at the exact line/column ranges; the caret and selection stay native. A gutter-free list under the toolbar shows the count; the panel works as in the reader, and clicking an item selects that range in the textarea.
6. **Look.** Wavy squiggles like every linter, never confused with a link's straight underline: errors `--lint-error` (red), warnings `--lint-warning` (amber), suggestions `--lint-suggestion` (blue), defined for light and dark. These three tokens are the only colours in the app (owner decision).
6a. **Problems console** (editor window): docked under the source, collapsible (remembered), lists severity, line:col, message and rule; selecting one jumps to it and shows why it was flagged (Vale's description) and a "Learn more" link (https only, opened through `openExternal`).
7. **Live reload.** `page:changed` re-lints the open page in the reader.
8. All text from Vale reaches the DOM through `textContent` / attributes; marks are built with DOM APIs, no `innerHTML`.

## Architecture

Go: `internal/app/lint.go` holds the `*lint.Linter` (or nil), wraps calls with
the workspace root and config switch. Frontend: `frontend/src/lint/` exports
`markText(root, alerts)`, `renderPanel(...)` and `backdrop(textarea)`; page and
editor features call them. `Api.lint(path, content): Promise<LintResult>`
(`{available: boolean; alerts: Alert[]}`), added to the fake (configurable
alerts) and `wails.ts`.

## Interfaces

```ts
type Alert = { line: number; col: number; endCol: number; severity: "error" | "warning" | "suggestion"; check: string; message: string; match: string };
type LintResult = { available: boolean; alerts: Alert[] };
interface Api { /* existing */ lint(path: string, content: string): Promise<LintResult>; }
```
```go
func (z *Zen) Lint(path, content string) (LintResult, error)
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1,2 | unavailable, config switch, path validation, disk vs unsaved content | `internal/app/lint_test.go` (fake linter) |
| 3 | marks placed in text, skipped in code/pre/plugin blocks, repeated words, unplaceable | `frontend/src/lint/mark.test.ts` |
| 4,5 | panel count, toggle, click to scroll/select, Esc | `frontend/src/lint/panel.test.ts` |
| 5 | backdrop mirrors text and ranges, debounce, latest wins | `frontend/src/lint/backdrop.test.ts`, `editor.test.ts` |
| 6 | no colours in lint.css | `frontend/src/lint/css.test.ts` |
| 8 | Vale text with `<script>` stays text | `mark.test.ts`, `panel.test.ts` |

## UI

See behaviour 4–6. Tokens only.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Vale output injecting markup | DOM APIs and textContent only | `mark.test.ts` |
| Linting arbitrary paths | `Lint` validates workspace markdown paths | `lint_test.go` |

## Notes

- **Out of scope:** quick-fix suggestions, adding words to a dictionary, Vale style package management.
- **Changelog:** Added: Vale spelling and style marks in the reader and editor.

## Prompt

> Implement `specs/ui/lint/spec.md` following the rules in `specs/README.md`. core/lint,
> core/navmeta and ui/editor-window are merged in your base. Start with the pure
> `frontend/src/lint/` module and its tests.
