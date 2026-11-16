# core/extplugins

| | |
|---|---|
| Status | done |
| Target version | 0.2.0-alpha.1 |
| Branch | `feature/core-extplugins` |
| Wave | 6 |
| Depends on | core/plugins, core/render, app/services |
| Owns | `internal/core/extplugins/` (new), `internal/core/plugins/`, `internal/core/render/`, `internal/app/` (plugin loading, trust, services, asset serving), `examples/demo/.zen/plugins/note/`, `examples/demo/.zen/plugins/calc/`, `internal/app/demo_test.go` |

## Purpose

Anyone can add a plugin by dropping a folder with a `plugin.yml`: a program
zen runs (backend / tool), scripts and styles for the page (frontend), or
both. The contract is **docs/PLUGINS.md "Writing your own plugin"**; this spec
implements the Go side of it exactly.

## Behaviour

1. **Discovery.** On opening a folder, read `<root>/.zen/plugins/*/plugin.yml` and `<user config dir>/zen/plugins/*/plugin.yml` (workspace wins on the same id; a built-in id can never be replaced). A manifest is rejected with a warning (shown on every page until fixed) when: YAML invalid, `id` ≠ folder name or not `[a-z][a-z0-9-]*`, `apiVersion` ≠ 1, version not SemVer, unknown keys, a block/inline/layout name already owned, `backend.command` empty, a `./` command or a `frontend` file missing or resolving outside the plugin folder (symlinks included), `timeout` > 60s, or the plugin folder larger than 20 MB / 500 files.
2. **Trust.** An external plugin is used only when trusted. Trust key = plugin folder's absolute path + SHA-256 over the sorted relative paths and contents of all its files. Stored in `<user config dir>/zen/trusted-plugins.yml` (`allow` / `block` per key, mode 0600). When a folder opens with plugins that have no decision, `Dialogs.Confirm` asks once, listing each plugin id, where it lives and what it runs (the backend command line; "scripts in the page" for frontend JS). Declined or blocked plugins are not registered; pages that use their names render as plain markdown plus the warning `plugin "x" is not allowed (Settings › Plugins)`.
3. **Registry.** Trusted plugins register after the built-ins as `plugins.Plugin`s. `plugins.Manifest` gains `Layouts []string`; `Registry`/`Set` gain `Layout(name) Plugin`. The registry is rebuilt on folder open and when trust changes (emits `workspace:changed` + `page:changed` for the current page).
4. **Backend calls** follow the stdin/stdout/environment table in docs/PLUGINS.md exactly, including `ZEN_ATTR_*` (key upper-cased, non `[A-Z0-9_]` → `_`), `ZEN_ATTRS_JSON`, `ZEN_CONFIG_JSON`, `ZEN_CONFIG_*`, `ZEN_WORKSPACE`, `ZEN_PLUGIN_DIR`, `ZEN_DATA_DIR` (created on demand, `<user config dir>/zen/data/<sha256(root)[:16]>/<id>/files`). cwd = plugin folder. Environment otherwise only `PATH`, `HOME`, `TMPDIR`, `LANG`. Timeout (default 5s) kills the process group. stdout capped at 2 MB, stderr at 16 KB. `output: text` escapes stdout and wraps it in `<pre class="zen-plugin-output">`. Non-zero exit, timeout or cap → `plugins.ErrorBox` with stderr's first lines. At most 4 backend processes at once per app.
5. **Wrappers (core render change).** Every plugin-owned block/inline wrapper gets `data-zen-name` and `data-zen-attrs` (JSON object of string attrs, omitted when empty) next to `data-zen-plugin`, for built-ins too. A plugin that owns a name but has no `block` (or `inline`) backend hook gets default content: a `:::` container's body rendered as markdown through the same pipeline (nested plugin blocks work; depth limit 8), a fence's body in `<pre class="zen-plugin-source">`, an inline's text escaped.
6. **Layouts (core render change).** Frontmatter `layout: <name>` owned by an enabled plugin wraps the final page HTML in `<div data-zen-plugin="<id>" data-zen-layout="<name>">` (attrs from the frontmatter mapping `layoutOptions`, strings only), marks the plugin used (assets load) and sets `Page.Layout`. Unknown or disabled layout → warning, page renders normally.
7. **Sanitiser** keeps `data-zen-name` (`[a-z][a-z0-9-]*`), `data-zen-layout` (same), `data-zen-attrs` (any text, attribute-escaped) on `div`/`span`, `pre.zen-plugin-source`, `pre.zen-plugin-output`, and each plugin's `allow` rules (same hard denials as today).
8. **Assets.** External plugins implement `AssetProvider` from `frontend.js`/`css`; `/_plugins/<id>/<file>` serves only files listed in the manifest, from the plugin folder, with the existing headers (JS as `text/javascript`).
9. **Services** (bound, JSON shapes below): `Plugins()` lists built-in and external plugins; `PluginTrust(id, allow)` records a decision and reloads; `PluginStorageGet(id, key)` / `PluginStorageSet(id, key, json)` persist JSON values per plugin and folder in `<user config dir>/zen/data/<sha256(root)[:16]>/<id>/storage.json` (keys `[A-Za-z0-9._-]{1,64}`, 1 MB per plugin, atomic writes, mutex per file); `PluginInvoke(id, json)` runs the `action` hook (plugin must be trusted, enabled and declare the hook) and returns its stdout parsed as JSON. Storage and invoke are refused for built-in ids and unknown plugins.
10. **Demo plugins.** `examples/demo/.zen/plugins/note/` (shell backend for `:::note` / `:::warning`, CSS, as in PLUGINS.md) and `…/calc/` (tool: `bc -l`, `output: text`). `TestDemoFolder` trusts them through the fake dialogs and keeps passing.

## Interfaces

```go
// internal/core/extplugins
type Manifest struct{ /* plugin.yml, see docs/PLUGINS.md */ }
type Plugin struct{ /* implements plugins.Plugin, hook interfaces by declared hooks, AssetProvider */ }
func Discover(dirs ...string) (found []*Plugin, warnings []string)
func (p *Plugin) Dir() string
func (p *Plugin) Hash() (string, error)
func (p *Plugin) Runs() []string            // human lines for the trust prompt
func (p *Plugin) Action(ctx plugins.Context, payload []byte) ([]byte, error)
type Trust struct{ /* trusted-plugins.yml */ }
func LoadTrust(path string) *Trust
func (t *Trust) Decision(p *Plugin) (allowed, decided bool)
func (t *Trust) Set(p *Plugin, allow bool) error
```
```ts
// service JSON
type PluginInfo = { id: string; version: string; description: string; source: "builtin" | "folder" | "user";
                    status: "on" | "off" | "blocked" | "undecided" | "invalid"; runs: string[]; problem: string };
Plugins(): PluginInfo[]; PluginTrust(id: string, allow: boolean): void;
PluginStorageGet(id: string, key: string): string /* JSON text, "null" when absent */;
PluginStorageSet(id: string, key: string, json: string): void;
PluginInvoke(id: string, json: string): string /* JSON text */;
// Page gains: layout: string ("" when none)
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | manifest parsing and every rejection | `internal/core/extplugins/manifest_test.go` |
| 2 | hash changes with any file; decisions persist; file mode | `trust_test.go` |
| 4 | env, stdin/stdout per hook, text output, exit code, timeout kills children, caps, concurrency (tiny shell scripts in `t.TempDir()`) | `exec_test.go` |
| 3,5,6,7 | wrappers and attrs, default content (nested, depth limit), layouts, sanitiser, through `render.Renderer` | `internal/core/render/external_test.go` |
| 8 | only listed assets served, traversal refused | `internal/app/assets_test.go` (added) |
| 2,9 | prompt once, decline, re-prompt after a file changes; storage round trip, limits, key validation; invoke; refusals | `internal/app/plugins_test.go` |
| 10 | demo folder | `internal/app/demo_test.go` |

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Opening a downloaded folder runs its code | nothing external runs before an explicit, per-file-hash approval | `plugins_test.go: TestTrustPrompt` |
| Plugin swapped after approval | hash covers every file; change → undecided again | `trust_test.go` |
| Command or asset outside the plugin folder | resolved and checked, symlinks included | `manifest_test.go`, `assets_test.go` |
| Runaway or chatty backend | timeout on the process group, output caps, 4 at once | `exec_test.go` |
| Injection through attrs/env | values passed as env/stdin only, never through a shell; HTML sanitised | `exec_test.go`, `external_test.go` |
| Storage abuse | key pattern, 1 MB cap, per-plugin files outside the workspace | `plugins_test.go` |

## Notes

- **Out of scope:** streaming output from a backend (planned with plugins/sandbox), WASM, a plugin marketplace, hot reload of plugin files (reopen the folder).
- **Changelog:** Added: external plugins (backend programs, tools, frontend scripts, layouts) with a trust prompt.

## Prompt

> Implement `specs/core/extplugins/spec.md` following the rules in `specs/README.md`;
> docs/PLUGINS.md "Writing your own plugin" is the contract. Another agent builds the
> frontend (`frontend/src/**`, `examples/demo/plugins/*.md`, `examples/demo/.zen/plugins/progress/`)
> against the same contract: do not touch those. You may run `task build` to regenerate bindings.
