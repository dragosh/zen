# core/render

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/core-render` |
| Wave | 2 |
| Depends on | core/plugins, core/config, core/workspace (for `workspace.Link`) |
| Owns | `internal/core/render/` |

## Purpose

Turns a markdown file into the sanitised HTML page the reader shows, running
the plugin pipeline from PLAN.md on the Go side.

## Behaviour

1. `Renderer.Render(path, src)` runs the seven pipeline stages of PLAN.md and returns a `Page`.
2. Frontmatter is removed from the output and returned as `Page.Frontmatter`. `Page.Title` is frontmatter `title`, else the first `h1` text, else the file name without extension.
3. Markdown: CommonMark + GFM (tables, strikethrough, task lists, autolinks) + footnotes; headings get stable ids.
4. Fenced blocks whose info-string name is owned by an enabled plugin, container blocks and inline directives are rendered by that plugin's hook, wrapped in an element with `data-zen-plugin="<id>"`. Unknown or disabled names render as plain code block / literal text. A hook error or panic renders `plugins.ErrorBox`.
5. `SourceHook`s run before parsing and `HTMLHook`s before sanitising, each in registry order.
6. Links are rewritten with the `Resolver`: `internal` → `href="#"`, `data-zen-link="internal"`, `data-path`, `data-anchor`; `external` → `data-zen-link="external"`, original `href`, `rel="noopener noreferrer"`; `broken` → `data-zen-link="broken"`, no `href`; same-page `#anchor` links stay as they are. Images with `asset` targets get `src="/_ws/<escaped path>"`; images with external URLs are kept only for `https:`; anything else is dropped to its alt text.
7. The sanitiser runs last: base policy (current `render.HTML` policy plus `data-zen-*` attributes) extended by the `Allow` rules of enabled plugins. `<script>`, event handlers, `javascript:` URLs, `<iframe>`, `<object>`, `<style>` never survive.
8. `Page.Assets` lists CSS/JS of plugins that actually rendered something on this page. `Page.Warnings` collects config and plugin warnings.
9. Rendering is safe for concurrent use and has no global mutable state.

## Architecture

goldmark + bluemonday (already used in the current `render.go`, which this
spec replaces). Imports `core/plugins`, `core/config`, and only the `Link`
types from `core/workspace`. Directive syntax is parsed with
`plugins.ScanContainers` / `ScanInlines`.

## Interfaces

```go
package render

type Resolver interface{ ResolveLink(from, href string) workspace.Link } // *workspace.Workspace satisfies it

type Page struct {
    Path, Title, HTML string
    Frontmatter map[string]any
    Assets      plugins.Assets
    Warnings    []string
}

type Renderer struct{ /* unexported */ }
func New(reg *plugins.Registry, cfg *config.Config, links Resolver) *Renderer
func (r *Renderer) Render(path string, src []byte) (Page, error)
func (r *Renderer) Links(path string, src []byte) []workspace.Link // resolved links in document order, for core/index
```

## Tests

| # | Behaviour | Test (`internal/core/render/*_test.go`) |
|---|---|---|
| 2 | title sources, frontmatter stripped | `TestTitleAndFrontmatter` |
| 3 | GFM elements, heading ids | `TestMarkdown` |
| 4 | fenced / container / inline plugin blocks, disabled fallback | `TestPluginBlocks` |
| 4 | hook error and panic → error box | `TestPluginFailureIsContained` |
| 5 | hook order | `TestHookOrder` |
| 6 | link and image rewriting, all kinds | `TestLinks`, `TestImages` |
| 7 | XSS corpus (script, onerror, javascript:, iframe, style, svg script, data: URLs, plugin emitting script) | `TestSanitise` |
| 7 | plugin `Allow` rule honoured only when enabled | `TestAllowRules` |
| 8 | assets only when used; warnings | `TestAssetsAndWarnings` |
| 9 | parallel renders | `TestConcurrent` (run with `-race`) |

Edge cases: empty file, only frontmatter, 5 MB file under 500 ms, nested containers, directive inside code block.

## UI

None (page typography is ui/page).

## Security

| Risk | Mitigation | Test |
|---|---|---|
| XSS from markdown or raw HTML | sanitiser last, unconditional | `TestSanitise` |
| XSS from plugin output | same sanitiser; `Allow` additive and scoped | `TestSanitise`, `TestAllowRules` |
| Local file disclosure through links/images | only `/_ws/` URLs from resolver `asset` links; `file:` dropped | `TestImages` |
| Remote tracking images | only `https:` kept (config switch to block is a beta spec) | `TestImages` |
| Plugin crash taking the page down | recover per hook | `TestPluginFailureIsContained` |

## Notes

- **Out of scope:** syntax highlighting and diagrams (plugins), caching, incremental rendering.
- **Integration:** none; app/services constructs the renderer.
- **Changelog:** Added: Go-side markdown rendering with plugin pipeline and sanitising.

## Prompt

> Implement `specs/core/render/spec.md` following the rules in `specs/README.md`.
> Replace the existing `internal/core/render/render.go` and its test (keep its sanitiser
> cases in the new corpus); keep a package-level `HTML(src []byte) (string, error)` helper
> working because `internal/app/documents.go` still calls it. Use fake plugins and a fake
> `Resolver` in tests. Done when the tests in the table pass with `go test -race`.
