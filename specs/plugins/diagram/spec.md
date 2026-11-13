# plugins/diagram

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/plugins-diagram` |
| Wave | 3 |
| Depends on | core/plugins, core/render; **blocked on PLAN.md open question 3** (approval to vendor mermaid) |
| Owns | `internal/plugins/diagram/` |

## Purpose

```` ```mermaid ```` blocks show as diagrams.

## Behaviour

1. Manifest: id `diagram`, `Default: true`, blocks `mermaid` and `diagram`, option `theme` (`neutral` default, or `dark`, `default`).
2. `RenderBlock` emits `<div data-zen-plugin="diagram" class="zen-diagram" data-theme="…"><pre>ESCAPED SOURCE</pre></div>`. Container form (`:::diagram`) is treated the same as fenced.
3. Assets: `diagram.js` (ES module, written here) and `mermaid.min.js` (vendored, pinned version, SHA-256 recorded in `VENDOR.md` and verified by a test), `diagram.css`.
4. `diagram.js` listens for `zen:page-rendered`, lazy-loads mermaid on first use, initialises it with `startOnLoad: false, securityLevel: "strict"`, theme from `data-theme` (forced to dark/neutral by the app theme), renders each not-yet-rendered `.zen-diagram` into an inline SVG replacing the `<pre>`, and marks it `data-rendered`.
5. A diagram with a syntax error keeps the source visible and shows mermaid's message in `.zen-plugin-error` below it.
6. Pages without diagram blocks load none of the assets.

## Architecture

Go: `New() plugins.Plugin` implementing `BlockHook` and `AssetProvider`,
assets embedded. JS: one small hand-written module; the only third-party code
is the vendored mermaid bundle. Not an npm dependency.

## Interfaces

```go
package diagram
func New() plugins.Plugin
```

## Tests

| # | Behaviour | Test (`internal/plugins/diagram/diagram_test.go`) |
|---|---|---|
| 1 | manifest | `TestManifest` |
| 2 | placeholder markup, source escaped, theme attr | `TestRenderBlock` |
| 2 | survives the sanitiser | `TestThroughRenderer` |
| 3 | vendored file checksum matches `VENDOR.md` | `TestVendorChecksum` |
| 6 | assets listed only when used | `TestAssetsOnlyWhenUsed` |
| 4,5 | `diagram.js` behaviour with a stub mermaid | `frontend/src/…` not owned — instead `internal/plugins/diagram/assets/diagram.test.mjs` run by `node --test`, invoked from `TestDiagramJS` (skipped when node is absent) |

## UI

Diagrams are centred, max-width 100%, monochrome `neutral` theme by default.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Diagram source injecting HTML/JS | source escaped in Go; mermaid `securityLevel: "strict"` (no HTML labels, no click handlers) | `TestRenderBlock`, `TestDiagramJS` |
| Tampered vendored bundle | checksum test | `TestVendorChecksum` |
| Remote loading | script only from `/_plugins/diagram/` | `TestDiagramJS` (no URL with a scheme in sources) |

## Notes

- **Out of scope:** other diagram engines, export to PNG, pan/zoom.
- **Integration (lead):** `registry.Register(diagram.New())`; the lead supplies the approved `mermaid.min.js` before this spec starts.
- **App Store:** bundled, static script; no remote code.
- **Changelog:** Added: mermaid diagram plugin.

## Prompt

> Implement `specs/plugins/diagram/spec.md` following the rules in `specs/README.md` and
> docs/PLUGINS.md. `internal/plugins/diagram/assets/mermaid.min.js` and `VENDOR.md` are
> provided by the lead; do not download anything. Done when the tests pass.
