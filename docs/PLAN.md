# zen — build plan

Desktop application (Wails v3) that reads, edits and live-reloads markdown
files. App name `zen`, binary and CLI name `zen`. macOS first.

## Decisions

| Topic | Decision |
|---|---|
| Editing model | Split view: CodeMirror 6 source pane with a formatting toolbar, live preview beside it |
| Plugins | JS plugins (frontend) and Go/WASM plugins (backend), enabled, disabled and reordered at runtime |
| Built-in plugins | `gfm`, `highlight`, `mermaid`, written against the public plugin API |
| Distribution | One build flavour for direct download and the App Store |
| External plugins | Users author plugins locally against the manifest/API; the app never downloads plugin code |
| Bundled binaries | The `zen` CLI ships inside the `.app`, plus a resource manifest for helper binaries |
| Previous PoC | Discarded; this is a clean start |
| Apple account, bundle ID, signing | Deferred. Placeholder ID `io.github.dragosh.zen` |

## Architecture

One `zen` binary. With no arguments or with file paths it starts the GUI;
flags and subcommands run headless.

```
main.go                 entry: CLI dispatch -> GUI
internal/cli            argument parsing, commands
internal/app            Wails application wiring (windows, menus, services)
internal/core/document  open, save, recent files, file access
internal/core/watch     file watching, live-reload events
internal/core/render    pipeline orchestration
internal/core/plugins   registry, manifests, JS and WASM hosts
internal/core/resources bundled-binary manifest, lookup, exec
frontend/src/editor     CodeMirror, toolbar commands
frontend/src/preview    rendered view, scroll sync
frontend/src/plugins    JS plugin host and built-in plugins
build/darwin            bundle, entitlements, signing, dmg, pkg
specs/<scope>/<feature> spec, prompt and tests, written before the code
```

Scopes depend inward only: `ui -> core`, `plugins -> plugin API`, never the
reverse. Each `core` package is a Wails service with a narrow interface, so it
is testable without a window.

### Render pipeline

Every stage is a plugin hook. A change to the plugin set or its config
triggers a re-render of open documents.

1. `source` — markdown text to markdown text (Go/WASM)
2. `parse` — markdown-it rules and renderers (JS)
3. `html` — HTML string to HTML string (Go/WASM or JS), then sanitised
4. `dom` — enhance the mounted preview, e.g. render mermaid diagrams (JS)

### Plugin model

- A plugin is a folder with `plugin.json`: `id`, `version`, `apiVersion`,
  `hooks`, `permissions`, `config` schema, and entry points (`main.js`
  and/or `main.wasm`).
- JS plugins load as ES modules and receive a restricted API object.
- WASM plugins run in wazero with no ambient filesystem or network; all host
  access goes through declared permissions.
- The registry holds order and enabled state, persists them, and emits change
  events.

### Resources

`resources.yml` lists binaries to place in the bundle per OS and arch. The
build copies them to `Contents/Helpers`, signs them, and `core/resources`
resolves and executes them at runtime. The CLI is the first such resource
(same binary, exposed through an "Install command line tool" action).

## Process per feature

1. `specs/<scope>/<feature>/spec.md` — behaviour and acceptance criteria.
2. `specs/<scope>/<feature>/prompt.md` — the build prompt for the feature.
3. Failing tests next to the code (Go `_test.go`, Vitest, Playwright e2e).
4. Implementation, until the tests pass.

Spec, prompt and tests are committed before the implementation commit.

## Phases

| # | Phase | Outcome |
|---|---|---|
| 1 | Scaffold | Wails v3 skeleton, task runner, CI, spec templates, CLI entry point |
| 2 | Read | Open, render and live-reload a markdown file |
| 3 | Plugin framework | Registry, manifest, pipeline hooks, runtime toggling |
| 4 | Built-in plugins | `gfm`, `highlight`, `mermaid` on the public API |
| 5 | Editor | Source pane, toolbar, save, dirty state, scroll sync |
| 6 | External plugins | User-authored JS and WASM plugins, permissions UI, plugin dev docs |
| 7 | Resources | Binary manifest, bundled CLI, command line tool install |
| 8 | Packaging | Signed and notarised `.dmg` and `.pkg`, then App Store submission |

## App Store constraints

These shape the design from phase 2 on, even though signing is deferred.

- **Sandbox file access.** Files are reachable only through user selection
  (open panel, drag and drop, Finder "Open With"). Recent files need
  security-scoped bookmarks. Images referenced relative to a document need
  access to its folder, so the app asks for the folder once and bookmarks it.
- **Executable code (guideline 2.5.2).** The app must not download code. zen
  loads plugins only from a folder the user picks, has no plugin marketplace
  or installer, and runs JS in WebKit and WASM in an interpreter. This is the
  defensible reading, not a guarantee; a `noexternalplugins` build tag is kept
  as a fallback if review rejects it.
- **WASM execution.** wazero runs in interpreter mode so no JIT entitlement
  is needed.
- **Helper binaries.** Each is signed with `com.apple.security.inherit` and
  lives inside the bundle. A sandboxed app cannot write to `/usr/local/bin`,
  so the CLI install action shows the symlink command for the user to run.
- **CLI to app hand-off.** `zen file.md` from a terminal runs outside the
  sandbox; it forwards the path to the running app through the system open
  mechanism rather than reading the file itself.

## Open items

- Apple Developer account, final bundle identifier, signing identities.
- macOS CI runner for build, signing and dmg verification.
- App icon and dmg artwork (Wails defaults are in place).
