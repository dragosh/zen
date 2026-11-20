# zen — build plan

Source of truth for requirements: [design-draft-v010-alpha.md](design-draft-v010-alpha.md).
Companion documents: [UI.md](UI.md) (views, tokens, mockup), [PLUGINS.md](PLUGINS.md)
(plugin API and syntax), [VERSIONING.md](VERSIONING.md), [../specs/README.md](../specs/README.md)
(feature list, waves, rules for feature agents).

`Zenmark.app` is a desktop markdown reader and editor for **folders** of markdown.
`zen` is the same binary used from a terminal. Wails v3, macOS first.

## Product, v0.1.0-alpha

1. Open a folder (big button in the app, or `zen` in a terminal for the cwd).
2. The app indexes every markdown file, builds a navigation tree from the
   links between files, and shows it as a sidebar.
3. A root `README.md` is rendered by default; otherwise the user picks a page.
4. Internal links open in the same view, with a breadcrumb on top. External
   links ask before opening the default browser.
5. Pages render on the Go side through a pluggable pipeline. Images and SVG
   files display. Files reload live when they change on disk.
6. An editor (`zen edit [path]` or the Edit button) with a toolbar and a
   split view.
7. Plugins (built in for alpha: `highlight`, `diagram`) are switched on and
   off by a global config file, a workspace config file and page frontmatter.

## Decisions

| Topic | Decision |
|---|---|
| Rendering | Go only: goldmark (GFM, footnotes, heading ids) then bluemonday. No JS markdown or sanitiser libraries |
| Frontend | Vite + TypeScript, no UI framework, no runtime npm dependency except `@wailsio/runtime`. The frontend displays HTML it receives and sends user actions back |
| Workspace | One folder per window. All file access is confined to that folder |
| Navigation tree | Built from links, breadth-first from `README.md`; unlinked files are grouped by folder under "Other pages" |
| Breadcrumb | The page's path in the navigation tree, computed in Go |
| Plugins | Go interface with optional hooks. Built-ins are compiled in. External plugins (WASM, wazero) are designed for but ship in 0.2 |
| Plugin syntax | Fenced blocks ```` ```name ````, container blocks `:::name`, inline `:name[text]{k=v}` |
| Configuration | YAML. defaults < global < workspace `.zenmd.yml` < frontmatter |
| Editor | Dependency-free source editor with toolbar, live Go-rendered preview, split view. True WYSIWYG is **open question 1** |
| Versioning | SemVer, single `VERSION` file, see VERSIONING.md |
| Sandbox | None in the alpha, so `zen` can open any folder from a terminal. The App Store build (beta) needs a sandboxed app plus an unsandboxed CLI helper; entitlements kept in `build/darwin/*.sandbox.plist` |
| Distribution | One `Zenmark.app` bundling the `zen` binary and helper binaries. Bundle ID placeholder `org.zenmark.app` |

## Open questions (need the owner's answer, none block wave 1)

1. **WYSIWYG.** The draft asks for a WYSIWYG editor. Editing rendered content
   in place needs a rich-text engine (ProseMirror class), which means an npm
   dependency or months of work. Plan: alpha ships source editor + live
   preview + split view; WYSIWYG is a 0.2 spec. Say so if alpha must have it.
2. **Slides and polls** plugins are listed in the draft as examples. Planned
   for 0.2 on the same plugin API.
3. **Mermaid** needs its JavaScript file inside the app (vendored, pinned,
   checksummed; no npm). Needs a one-time download approval.

## Architecture

```
main.go                    entry: cli.Run -> app.Run
internal/cli               arguments and subcommands        (no Wails import)
internal/app               Wails wiring + services facade   (only package importing Wails)
internal/core/workspace    folder root, safe paths, read/write, file listing
internal/core/index        link graph, navigation tree, breadcrumbs
internal/core/render       markdown -> sanitised HTML, frontmatter, link rewriting
internal/core/plugins      plugin API, registry, directive syntax
internal/core/config       global/workspace/frontmatter configuration
internal/core/watch        file watching, change events
internal/plugins/<id>      built-in plugins, written against core/plugins
frontend/src/api           typed service client + in-memory fake
frontend/src/shell         window frame, welcome screen, routing between views
frontend/src/nav           sidebar tree, breadcrumb, link handling
frontend/src/page          rendered page view
frontend/src/editor        editor, toolbar, split view
frontend/src/styles        design tokens and base typography
build/darwin               bundle, entitlements, signing
specs/<scope>/<feature>    one spec per feature
```

Dependencies point inward: `app -> core`, `plugins/<id> -> core/plugins`,
`ui -> api`. `core` packages never import `app` or Wails and are tested
without a window.

### Render pipeline (Go)

```
file bytes
  1 frontmatter   split YAML header, resolve effective config
  2 source hooks  plugin: markdown -> markdown
  3 parse         goldmark + directive syntax -> AST
  4 block/inline  plugin: named block or inline node -> HTML fragment
  5 links         internal links, external links, image URLs rewritten
  6 html hooks    plugin: HTML -> HTML
  7 sanitise      bluemonday policy + elements declared by enabled plugins
Page{html, title, frontmatter, assets}
```

Sanitising is last and unconditional; no plugin output skips it.

### Service API (the contract between Go and the frontend)

Since engine E1 the Go side of this contract is `engine/session` (a headless
folder session, see docs/ENGINE.md and `go doc ./engine/session`); the app's
`Zen` service in `internal/app` binds it to Wails with one-line delegations.

Exposed by `internal/app`, consumed only through `frontend/src/api`.
UI features are built against the fake in `frontend/src/api/fake.ts`.

```ts
type NavNode   = { title: string; path: string; kind: "page" | "group"; children: NavNode[] };
type Workspace = { root: string; name: string; home: string; tree: NavNode[] };   // home: "" or "README.md"
type Crumb     = { title: string; path: string };
type Page      = { path: string; title: string; html: string;
                   frontmatter: Record<string, unknown>; breadcrumb: Crumb[];
                   assets: { css: string[]; js: string[] }; warnings: string[] };
type Alert      = { line: number; col: number; endCol: number; severity: "error" | "warning" | "suggestion";
                   check: string; message: string; match: string };
type LintResult = { available: boolean; alerts: Alert[] };   // available=false: no Vale or switched off
type Startup   = { workspace: Workspace | null; page: string; mode: "read" | "edit" };

interface Api {
  startup(): Promise<Startup>;
  pickFolder(): Promise<Workspace | null>;          // native folder dialog
  page(path: string): Promise<Page>;
  source(path: string): Promise<string>;
  preview(path: string, content: string): Promise<Page>;   // render unsaved text
  save(path: string, content: string): Promise<void>;
  openExternal(url: string): Promise<boolean>;      // Go shows the confirm dialog
  on(event: "workspace:changed", fn: (w: Workspace) => void): () => void;
  on(event: "page:changed", fn: (e: { path: string }) => void): () => void;
  on(event: "workspace:opened", fn: (s: Startup) => void): () => void; // Open With, File > Open Folder
  openEditor(path: string): Promise<void>;           // opens/focuses the page's editor window
  openConfig(which: "global" | "workspace"): Promise<void>; // config.yml / .zenmd.yml in the text editor
  lint(path: string, content: string): Promise<LintResult>; // Vale; "" content = file on disk
  exportPdf(title: string): Promise<string>;        // app/export: save panel + PDF; "" when cancelled
  revealExport(): Promise<void>;                    // Finder on the last exported PDF
  // page:changed also fires when another window saves (editor:saved)
}
```

Paths are always workspace-relative with `/` separators. Workspace files
(images) are served read-only at `/_ws/<path>`; plugin assets at
`/_plugins/<id>/<file>`.

### Rendered HTML conventions

| Markup | Meaning |
|---|---|
| `<a data-zen-link="internal" data-path="docs/a.md" data-anchor="intro">` | link to a workspace page; the frontend navigates in place |
| `<a data-zen-link="external" href="https://…">` | the frontend calls `openExternal` |
| `<a data-zen-link="broken">` | target not in the workspace; rendered muted |
| `<img src="/_ws/img/a.png">` | workspace image (png, jpg, gif, webp, svg, avif) |
| `<div data-zen-plugin="diagram" …>` | block owned by a plugin |

## Process

1. The lead (Fable) owns `docs/`, the spec files and integration.
2. Every feature is one spec in `specs/<scope>/<feature>/spec.md`, built by one
   subagent on branch `feature/<scope>-<feature>` in its own `git worktree`.
3. A feature edits only the paths its spec **owns**. Shared wiring files are
   edited by the lead at merge time from the spec's *Integration* notes.
4. Tests first: the tests in the spec's table are committed failing, then the
   implementation.
5. Features merge in waves (see `specs/README.md`); a wave starts when the
   wave before it is merged into the integration branch.

## Milestones

| Version | Content |
|---|---|
| `0.1.0-alpha.1` | Waves 1–4: folder workspace, navigation, Go rendering, plugins `highlight` + `diagram`, editor with split view, CLI, unsigned `Zenmark.app` |
| `0.1.0-beta.1` | Signing, notarisation, "Install command line tool", recent folders, search |
| `0.2.0` | External WASM plugins, `slides`, `polls`, WYSIWYG decision |
