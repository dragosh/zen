# plugins/codeblock

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/plugins-highlight` |
| Wave | 3 |
| Depends on | core/plugins, core/render |
| Owns | `internal/plugins/codeblock/`, `frontend/src/plugins/builtin-codeblock.test.ts` |

## Purpose

Code blocks are syntax-coloured entirely in Go, and every block has a Copy and a Wrap button. (Renamed from `highlight`.)

## Behaviour

1. Manifest: id `codeblock`, `Default: true`, options `style` (string, default `zen`), `lineNumbers` (bool, default false), `wrap` (bool, default false: blocks start wrapped).
2. Implements `HTMLHook`: every `<pre><code>` outside other plugins' `data-zen-plugin` elements is wrapped in `<div data-zen-plugin="codeblock" class="zen-codeblock">` (plus `zen-codeblock-wrap` when `wrap` is on). Blocks whose language chroma knows are replaced by chroma output using **CSS classes** (no inline styles); unknown languages and blocks without a language keep their text unchanged.
3. Ships `codeblock.css` and `codeblock.js` through `AssetProvider`: the `zen` style is monochrome (weight, italics and three gray levels from the UI tokens), defined for light and dark.
4. `lineNumbers: true` adds a non-selectable line-number gutter.
5. Declares `Allow` rules for `span[class]`, `pre[class]`, `code[class]` and `div[class]` only.
7. `codeblock.js` mounts on each wrapper and adds, top right inside the block, a **Wrap** button (toggles `zen-codeblock-wrap`, `aria-pressed`) and a **Copy** button (copies the text without the line-number gutter, shows "Copied" for 1.5 s; falls back to a hidden textarea when the async clipboard is refused). The buttons show on hover or keyboard focus, always on touch screens, never in print.
6. Highlighting a 1 MB code block finishes under 300 ms; blocks over 2 MB are skipped with a warning.

## Architecture

`github.com/alecthomas/chroma/v2` (in go.mod). `New() plugins.Plugin`; CSS
embedded with `embed.FS`.

## Interfaces

```go
package codeblock
func New() plugins.Plugin
```

## Tests

| # | Behaviour | Test (`internal/plugins/codeblock/codeblock_test.go`) |
|---|---|---|
| 1 | manifest registers | `TestManifest` |
| 2 | known / unknown / no language, other plugins' blocks untouched | `TestHighlight` |
| 2 | output has classes and no `style=` | `TestNoInlineStyles` |
| 3 | CSS asset present, covers emitted classes, both themes | `TestCSS` |
| 4 | line numbers | `TestLineNumbers` |
| 1,2 | `wrap` option adds the wrap class | `TestWrapOption` |
| 7 | buttons, wrap toggle, copy without gutter, unmount | `frontend/src/plugins/builtin-codeblock.test.ts` |
| 5,2 | survives the real sanitiser via `render.Renderer` | `TestThroughRenderer` |
| 6 | size guard | `TestLargeBlock` |

## UI

Monochrome code styling consistent with docs/UI.md tokens.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Code content breaking out as HTML | chroma escapes; output re-sanitised by the pipeline | `TestThroughRenderer` (code containing `<script>`) |
| Regex-heavy lexers stalling on hostile input | 2 MB cap | `TestLargeBlock` |

## Notes

- **Out of scope:** language label, user-defined styles.
- **Breaking:** configs that name `highlight:` under `plugins` must say `codeblock:`.
- **Integration (lead):** `registry.Register(codeblock.New())` in app/services.
- **Changelog:** Added: syntax highlighting plugin.

## Prompt

> Implement `specs/plugins/codeblock/spec.md` following the rules in `specs/README.md` and
> docs/PLUGINS.md. Use chroma v2 with class-based HTML output. Done when the tests pass.
