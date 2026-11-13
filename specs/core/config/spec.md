# core/config

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/core-config` |
| Wave | 1 |
| Depends on | none |
| Owns | `internal/core/config/` |

## Purpose

One place that answers "which settings apply to this page", merging defaults,
the global file, the workspace file and the page's frontmatter.

## Behaviour

1. `GlobalPath()` returns `<os.UserConfigDir()>/zen/config.yml`.
2. `Load(globalPath, workspaceRoot)` reads the global file and `<root>/.zen.yml`. A missing file is not an error; an unreadable or invalid file yields defaults for that layer plus a warning.
3. Schema (YAML): `version: 1`, `theme: system|light|dark`, `plugins: { <id>: bool | map }`. Unknown top-level keys are kept in `Extra` and produce a warning.
4. `SplitFrontmatter(src)` returns the YAML mapping between a leading `---` line and the next `---` or `...` line, and the remaining body. No frontmatter returns an empty map and the unchanged source. Invalid YAML returns an empty map, the body without the block, and a warning. Frontmatter larger than 64 KB is treated as body.
5. `ForPage(frontmatter)` returns the effective `Settings`: defaults < global < workspace < frontmatter (`plugins` merged per plugin id; a map merges option by option; `false` disables).
6. `Settings.Plugin(id)` returns `(enabled bool, options map[string]any, mentioned bool)`; `mentioned` is false when no layer names the plugin (the registry then uses the plugin's default).
7. `version` greater than 1 yields a warning and the layer is ignored.

## Architecture

Pure Go + `gopkg.in/yaml.v3`. No Wails, no dependency on other core packages.
`core/render` calls `SplitFrontmatter` and `ForPage`.

## Interfaces

```go
package config

type Config struct{ /* merged global + workspace */ }
type Settings struct {
    Theme    string
    Warnings []string
    /* unexported plugin layers */
}
func GlobalPath() (string, error)
func Load(globalPath, workspaceRoot string) *Config            // never nil
func Default() *Config
func (c *Config) Warnings() []string
func (c *Config) ForPage(frontmatter map[string]any) Settings
func (s Settings) Plugin(id string) (enabled bool, options map[string]any, mentioned bool)
func SplitFrontmatter(src []byte) (meta map[string]any, body []byte, warning string)
```

## Tests

| # | Behaviour | Test (`internal/core/config/config_test.go`) |
|---|---|---|
| 1 | global path | `TestGlobalPath` |
| 2,7 | missing, invalid, future version | `TestLoadTolerant` |
| 3 | schema, unknown keys | `TestSchema` |
| 4 | frontmatter split variants | `TestSplitFrontmatter` |
| 5 | precedence across four layers | `TestPrecedence` |
| 6 | bool vs map vs absent | `TestPluginLookup` |

Edge cases: CRLF line endings, BOM, `---` appearing later in the body, empty frontmatter block, non-map YAML (a list) in frontmatter.

## UI

None.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| YAML bombs (aliases) / huge frontmatter | 64 KB cap; decode into plain maps | `TestSplitFrontmatter` (oversize case) |
| Config executing anything | config is data only; no paths to executables, no includes | by design |

## Notes

- **Out of scope:** writing config, a settings UI, watching config files.
- **Integration:** none.
- **Changelog:** Added: global `config.yml`, workspace `.zen.yml` and page frontmatter settings.

## Prompt

> Implement `specs/core/config/spec.md` following the rules in `specs/README.md`.
> Create package `internal/core/config` with exactly the interface above, using
> `gopkg.in/yaml.v3` (already in go.mod). See docs/PLUGINS.md "Enabling, disabling,
> configuring" for the file format. Done when the tests in the table pass.
