# plugins/highlight

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/plugins-highlight` |
| Wave | 3 |
| Depends on | core/plugins, core/render |
| Owns | `internal/plugins/highlight/` |

## Purpose

Code blocks are syntax-coloured, in grayscale-friendly styles, entirely in Go.

## Behaviour

1. Manifest: id `highlight`, `Default: true`, options `style` (string, default `zen`), `lineNumbers` (bool, default false).
2. Implements `HTMLHook`: every `<pre><code class="language-X">` whose language chroma knows is replaced by chroma output using **CSS classes** (no inline styles). Unknown languages and blocks without a language are left unchanged. Blocks inside `data-zen-plugin` elements of other plugins are left alone.
3. Ships `highlight.css` through `AssetProvider`: the `zen` style is monochrome (weight, italics and three gray levels from the UI tokens), defined for light and dark.
4. `lineNumbers: true` adds a non-selectable line-number gutter.
5. Declares `Allow` rules for `span[class]` and `pre[class]`, `code[class]` only.
6. Highlighting a 1 MB code block finishes under 300 ms; blocks over 2 MB are skipped with a warning.

## Architecture

`github.com/alecthomas/chroma/v2` (in go.mod). `New() plugins.Plugin`; CSS
embedded with `embed.FS`.

## Interfaces

```go
package highlight
func New() plugins.Plugin
```

## Tests

| # | Behaviour | Test (`internal/plugins/highlight/highlight_test.go`) |
|---|---|---|
| 1 | manifest registers | `TestManifest` |
| 2 | known / unknown / no language, other plugins' blocks untouched | `TestHighlight` |
| 2 | output has classes and no `style=` | `TestNoInlineStyles` |
| 3 | CSS asset present, covers emitted classes, both themes | `TestCSS` |
| 4 | line numbers | `TestLineNumbers` |
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

- **Out of scope:** copy button, language label, user-defined styles.
- **Integration (lead):** `registry.Register(highlight.New())` in app/services.
- **Changelog:** Added: syntax highlighting plugin.

## Prompt

> Implement `specs/plugins/highlight/spec.md` following the rules in `specs/README.md` and
> docs/PLUGINS.md. Use chroma v2 with class-based HTML output. Done when the tests pass.
