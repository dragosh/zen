# core/plugins

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/core-plugins` |
| Wave | 1 |
| Depends on | none |
| Owns | `internal/core/plugins/` |

## Purpose

The plugin API (version 1), the registry that holds plugins, and the parser
for the directive syntax, so that renderers can be added without touching the
render pipeline.

## Behaviour

1. `Registry.Register(p)` validates the manifest: id pattern `[a-z][a-z0-9-]*`, `APIVersion == plugins.APIVersion`, valid SemVer `Version`, no duplicate id, no block or inline name already owned by another plugin. Violations return an error naming the plugin.
2. `Registry.Enabled(lookup)` returns, in registration order, the plugins enabled for a page. `lookup(id)` reports `(enabled, options, mentioned)`; when not mentioned, `Manifest.Default` decides. Options are merged over the manifest's option defaults; unknown options and unknown ids mentioned by config are returned as warnings.
3. `Set.Block(name)` / `Set.Inline(name)` return the owning enabled plugin or nil.
4. `ParseAttrs("{a=1 b=\"two words\" flag}")` returns `map[a:1 b:two words flag:true]`; malformed input returns an error and callers treat the text as literal.
5. `ScanContainers(src)` finds `:::name{attrs}` … `:::` blocks (opening fence at line start, optional attrs, closing `:::` alone on a line, nesting by matching depth, ignored inside fenced code blocks) and returns their spans.
6. `ScanInlines(line)` finds `:name[text]{attrs}` (attrs optional), not inside code spans, and only when `name` is a known inline name.
7. `ErrorBox(pluginID, err)` returns the HTML shown in place of a failed block, with the message HTML-escaped.
8. `Set.Assets()` returns the CSS/JS URLs (`/_plugins/<id>/<file>`) of enabled plugins that implement `AssetProvider`; `Registry.AssetFS(id)` returns that plugin's `fs.FS`.

## Architecture

Pure Go, standard library only. Defines types only; `core/render` drives the
hooks. Full API and syntax: docs/PLUGINS.md (authoritative for type shapes).

## Interfaces

All types from docs/PLUGINS.md (`Manifest`, `Plugin`, `Context`, `Block`,
`Inline`, the five hook interfaces, `APIVersion`), plus:

```go
type Option    struct{ Type string; Default any; Description string } // Type: "string"|"bool"|"int"
type AllowRule struct{ Element string; Attrs []string }
type Assets    struct{ CSS, JS []string }

type Registry struct{ /* unexported */ }
func NewRegistry() *Registry
func (r *Registry) Register(p Plugin) error
func (r *Registry) All() []Manifest
func (r *Registry) AssetFS(id string) (fs.FS, bool)
func (r *Registry) Enabled(lookup func(id string) (bool, map[string]any, bool), mentionedIDs []string) (*Set, []string)

type Set struct{ /* unexported */ }
func (s *Set) Plugins() []Plugin
func (s *Set) Context(id, pagePath string) Context
func (s *Set) Block(name string) Plugin
func (s *Set) Inline(name string) Plugin
func (s *Set) Allow() []AllowRule
func (s *Set) Assets() Assets

type Span struct{ Start, End int; Name, Body string; Attrs map[string]string }
func ParseAttrs(s string) (map[string]string, error)
func ScanContainers(src []byte) []Span
func ScanInlines(line []byte, known func(string) bool) []Span
func ErrorBox(pluginID string, err error) string
```

## Tests

| # | Behaviour | Test (`internal/core/plugins/*_test.go`) |
|---|---|---|
| 1 | manifest validation, duplicates | `TestRegisterValidates` |
| 2 | defaults, config override, option merge, warnings | `TestEnabled` |
| 3 | block/inline ownership respects enablement | `TestLookupByName` |
| 4 | attribute grammar | `TestParseAttrs` |
| 5 | containers, nesting, inside code fences | `TestScanContainers` |
| 6 | inline directives, code spans, unknown names, prose colons | `TestScanInlines` |
| 7 | error box escapes | `TestErrorBoxEscapes` |
| 8 | asset URLs and FS | `TestAssets` |

Edge cases: unclosed container (not a container), `:::` with trailing spaces, attrs with `=` in quoted values, `note: text` and `10:30[am]` in prose.

## UI

None (error box style belongs to ui/page: class `zen-plugin-error`).

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Plugin message injecting HTML | `ErrorBox` escapes | `TestErrorBoxEscapes` |
| Plugin claiming another's block | duplicate ownership rejected at register | `TestRegisterValidates` |
| API drift | exact `APIVersion` match required | `TestRegisterValidates` |

## Notes

- **Out of scope:** running hooks (core/render), external WASM plugins (0.2), persistence of enabled state (config decides).
- **Integration:** none.
- **Changelog:** Added: plugin API v1 with fenced, container and inline directive syntax.

## Prompt

> Implement `specs/core/plugins/spec.md` following the rules in `specs/README.md`.
> docs/PLUGINS.md is authoritative for type shapes and syntax. Standard library only.
> Write a small fake plugin in the test files to exercise the registry. Done when the
> tests in the table pass.
