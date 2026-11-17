# core/static-build

| | |
|---|---|
| Status | specified |
| Target version | 0.2.0 |
| Branch | `feature/core-static-build` |
| Wave | 11 |
| Depends on | core/layouts, core/script-plugins, core/plugin-hardening, cli/commands |
| Owns | `internal/core/site/` (new), `internal/app/build.go` + `internal/app/build_test.go` (new), `internal/cli` (the `build` command), `internal/core/config` (the `build:` section), `frontend/src/static/` (new), `examples/demo/build/**` (new demo pages) |

## Purpose

`zen build` turns a folder into a static website: one HTML page per markdown
page, rendered by the same Go pipeline as the app (plugins, layouts, sidebar,
breadcrumb), with its assets copied next to it. Put the output on any static
host. With `--portable`, it also opens straight from disk (`file://`) with a
double click.

## Behaviour

### Command
1. `zen build [folder] [-o <dir>] [--portable] [--drafts] [--clean]` builds `folder` (default: the current folder; a markdown file means its folder, as `zen [path]` resolves it) into `<dir>` (default: `build.out` from config, else `<folder>/_site`). It starts no window and asks nothing.
2. It prints one line per warning (`page.md: message`) to stderr and a summary to stdout: `Built 42 pages and 18 files into _site (3 warnings)`. Exit code 0 when built (warnings allowed), 1 on errors (nothing usable written), 2 on usage errors.
3. The output folder may not be the workspace root, a parent of it, or a folder inside `.zen/`. If it is inside the workspace, it is left out of the input (it is never read as pages or assets) and a warning says so.
4. `--clean` empties the output folder first, but only if it is empty or holds a `.zen-build` marker file written by an earlier build; otherwise it stops with an error naming the folder. Without `--clean`, files are overwritten and files no longer produced are left in place.
5. The same input gives byte-identical output: files and lists are sorted; no timestamps, random ids or absolute paths of this Mac appear in any written file.

### Pages
6. Every page the app would list (same workspace walk, file limits, `.zen.yml` excludes) is rendered with `render.Renderer` plus layouts, exactly as `Page.html` in the app, then wrapped in a static page shell: `<head>` (title, charset, viewport, CSP meta, CSS links), the sidebar, the breadcrumb, the page body, the page's warnings left out.
7. A page with `draft: true` in frontmatter is skipped unless `--drafts` (or `build.drafts: true`); links to a skipped page render as broken links.
8. `a/b.md` becomes `a/b.html`. The home page (README by default, as in the app) is also written as `index.html` at the output root; if the home is not at the root, `index.html` is a page that links to it and redirects with `<meta http-equiv="refresh">`.
9. Links are rewritten from the app conventions (PLAN.md › Rendered HTML conventions):
   - `data-zen-link="internal"` → relative `href` to the target's `.html`, plus `#anchor`; slugs resolve to their file first.
   - `data-zen-link="external"` → kept, with `target="_blank" rel="noopener noreferrer"`.
   - `data-zen-link="broken"` → no `href`, kept muted.
   - `data-zen-action="edit"` (docs layout) → `build.editURL` with `{path}` replaced by the page's path, or removed when not set.
10. Every `/_ws/<path>` URL (images, linked files) that resolves inside the workspace is copied to `<out>/<path>` and rewritten to a relative URL. URLs that do not resolve are left as broken images with a warning. Nothing outside the workspace is copied.
11. The sidebar is static HTML written into each page (the same tree, titles and order as the app; the current page marked `aria-current="page"`; groups as `<details>`, open along the current path). It works without JavaScript.

### Assets and scripts
12. Plugin and layout assets the built pages use are written to `<out>/_zen/plugins/<id>/<file>` and `<out>/_zen/layouts/<id>/<file>` (only files listed by the plugin or layout, same rules as the app's `/_plugins/` and `/_layouts/` routes). zen's own page CSS (tokens, base, page) and the static runtime go to `<out>/_zen/zen.css` and `<out>/_zen/zen.js`. Font files are copied only when the CSS references them.
13. The static runtime (`frontend/src/static`) mounts plugin and layout scripts with the **same contract** as the app (default export `{ mount(el, ctx) }`, docs/PLUGINS.md and docs/LAYOUTS.md), with a `ctx` that has:
    - `storage` kept in the reader's `localStorage` under `zen.site.<siteId>.<plugin|layout>.<id>.<key>` (`siteId` from `build.id`, else the folder name). If localStorage is unavailable, storage works for the session only.
    - `invoke` and `stream` that reject with `Error("runs in the zen app only")`; the block shows the usual plugin error box with that text.
    - `navigate`, `anchor` and `onAnchor` that follow the static links; `badge` that does nothing.
14. Render-time plugins (built-in Go, Tengo, and external backends that produce HTML) run during the build exactly as when the page is opened in the app; their output is static HTML.
15. **Trust:** built-in plugins and layouts are used. External and script plugins and folder or user layouts are used only if already allowed for this folder in the app (the same trust file, hash and fingerprint check). Anything else is left out, its blocks render as in the app when a plugin is blocked, and a warning names it with "allow it in the zen app first". The build never prompts and never writes trust.
16. Vale linting, the editor, Settings and trust prompts are not part of the output.

### Portable mode
17. `--portable` (or `build.portable: true`) makes the output work from `file://`: no `import()` of files. Each page carries the scripts it needs as text in `<script type="application/json" data-zen-module="<plugin|layout>:<id>">` elements, and the static runtime loads them through `Blob` URLs. Links stay relative, so the folder can be zipped and moved.
18. In portable mode a plugin or layout script that has its own `import` of another file is left out with a warning ("uses imports; not supported in portable mode"). Single-file modules, the norm for zen plugins, work.

### Configuration
19. `.zen.yml` and the global `config.yml` accept a `build:` section; command-line flags win over it:
    ```yaml
    build:
      out: _site            # relative to the folder
      id: my-notes          # storage namespace for readers; default: folder name
      editURL: https://github.com/me/notes/edit/main/{path}
      drafts: false
      portable: false
      exclude: ["private/**", "scratch.md"]   # globs, on top of the app's excludes
    ```
    Unknown keys give a warning; wrong types give an error naming the key.

## Architecture

```
zen build ──► internal/cli (parse) ──► Builder func (injected like Launcher) ──► internal/app/build.go
                                                                                    │ headless state: workspace,
                                                                                    │ config, plugins (trust read-only),
                                                                                    │ layouts, index, renderer
                                                                                    ▼
                                                                       internal/core/site.Build(site.Input)
                                                                         ├─ rewrite.go  links, /_ws/, edit action (x/net/html tokenizer)
                                                                         ├─ shell.go    static page shell, sidebar, breadcrumb (html/template)
                                                                         ├─ assets.go   collect + copy plugin/layout/workspace files
                                                                         ├─ portable.go module text → JSON script tags
                                                                         └─ write.go    out dir checks, marker, atomic writes, no symlinks
```

- `internal/core/site` does not import `internal/app`. It gets pages and file
  systems through `site.Input` (interfaces), so it is tested with fakes.
- `internal/app/build.go` builds a headless `state` (no events, no watcher,
  `trustMode: readOnly` instead of the dialog) and adapts it to `site.Input`.
  It reuses `snapLocked`/`render`, `loadExternal`, `loadLayouts` and the asset
  lookups behind `servePlugin`/`serveLayout`, so app and build cannot drift.
- `frontend/src/static/` is a second Vite entry (`static.ts`) built to
  `frontend/dist/static/zen.js` (classic IIFE, no imports) and
  `zen.css`. It reuses `frontend/src/plugins/runtime.ts` with a static `api`
  (no Wails bindings imported). The Go side embeds them through the existing
  `frontend/dist` embed.

## Interfaces

```go
// internal/cli
type Build struct {
	Dir      string // absolute folder
	Out      string // "" = from config / default
	Portable bool
	Drafts   bool
	Clean    bool
}
type Builder func(Build, io.Writer /*stdout*/, io.Writer /*stderr*/) error
func Run(args []string, cwd string, stdout, stderr io.Writer, launch Launcher, build Builder) int

// internal/app
func BuildSite(b cli.Build, stdout, stderr io.Writer) error   // wired in main.go

// internal/core/config
type Build struct {
	Out, ID, EditURL string
	Drafts, Portable bool
	Exclude          []string
}
func (c *Config) Build() (Build, []string /*warnings*/, error)

// internal/core/site
type Page struct {
	Path, Title, HTML string
	Breadcrumb        []Crumb
	Layout            string
	CSS, JS           []string // app URLs: /_plugins/<id>/<f>?v=, /_layouts/<id>/<f>?v=
	Draft             bool
	Warnings          []string
}
type Crumb struct{ Title, Path string }
type NavNode struct {
	Title, Path, Kind string // "page" | "group"
	Children          []NavNode
}
type Input interface {
	Pages() ([]string, error)                 // sorted workspace-relative paths
	Home() string
	Tree() []NavNode
	Render(path string) (Page, error)
	Slug(slug string) (string, bool)
	WorkspaceFile(path string) (fs.File, error) // inside the root only, no symlinks out
	Asset(url string) (fs.File, error)          // /_plugins/… and /_layouts/… as the app serves them
	Runtime() (js, css []byte)                  // frontend/dist/static
}
type Options struct {
	Out              string
	SiteID, EditURL  string
	Drafts, Portable bool
	Clean            bool
	Exclude          []string
}
type Report struct {
	Pages, Files int
	Warnings     []string // "path: message", sorted
}
func Build(in Input, o Options) (Report, error)
func CheckOut(root, out string) error // behaviour 3
```

```ts
// frontend/src/static/static.ts — reads <script type="application/json" id="zen-page">
// ({ path, siteId, assets: { css, js }, portable }) and mounts plugins/layouts.
export function start(doc?: Document): void;
export function staticApi(siteId: string, store?: Storage): Pick<Api, "pluginStorageGet" | "pluginStorageSet" | "pluginStorageKeys" | "pluginInvoke">;
```

## Tests

Written first and committed failing.

| # | Behaviour | Test |
|---|---|---|
| 1 | build parses flags, folder/file args, defaults | `internal/cli/cli_test.go: TestBuildCommand` |
| 2 | summary on stdout, warnings on stderr, exit codes 0/1/2 | `internal/cli/cli_test.go: TestBuildExitCodes`; `internal/app/build_test.go: TestBuildSiteReport` |
| 3 | out dir refused at root/parent/.zen; excluded when inside | `internal/core/site/write_test.go: TestCheckOut`, `TestOutInsideWorkspaceExcluded` |
| 4 | `--clean` only with marker or empty dir | `internal/core/site/write_test.go: TestCleanNeedsMarker` |
| 5 | byte-identical output across two builds | `internal/core/site/site_test.go: TestDeterministic` |
| 6 | page HTML equals the app's `Page.html` inside the shell | `internal/app/build_test.go: TestBuildMatchesApp` |
| 7 | drafts skipped, links to them broken; `--drafts` keeps them | `internal/core/site/site_test.go: TestDrafts` |
| 8 | `.md` → `.html`, `index.html` for home at root and redirect otherwise | `internal/core/site/site_test.go: TestOutputPaths` |
| 9 | link rewriting: internal + anchor + slug, external, broken, edit action with/without editURL | `internal/core/site/rewrite_test.go: TestRewriteLinks` |
| 10 | `/_ws/` files copied and relative; missing → warning; `..` and symlinks out refused | `internal/core/site/assets_test.go: TestWorkspaceFiles` |
| 11 | static sidebar: order, current page, open groups, no JS needed | `internal/core/site/shell_test.go: TestSidebar` |
| 12 | only used, listed plugin/layout files copied; zen.css/zen.js written | `internal/core/site/assets_test.go: TestPluginAssets` |
| 13 | static ctx: storage in localStorage by site id, fallback memory; invoke/stream reject; error box | `frontend/src/static/static.test.ts: storage`, `invoke and stream`, `mounts like the app` |
| 14 | Tengo and backend render-time output is in the page | `internal/app/build_test.go: TestBuildRunsRenderPlugins` |
| 15 | unapproved external plugin/layout left out with warning; trust file unchanged | `internal/app/build_test.go: TestBuildTrustReadOnly` |
| 16 | no lint, editor or app chrome in the output; CSP meta on every page | `internal/core/site/shell_test.go: TestShellContents`, `TestShellCSP` |
| – | user-scoped plugin options are not given to render-time plugins | `internal/app/build_test.go: TestBuildNoUserOptions` |
| 17 | portable: module JSON tags, Blob loading, no `import()` of files | `internal/core/site/portable_test.go: TestPortableModules`; `frontend/src/static/static.test.ts: portable` |
| 18 | portable: scripts with imports left out with warning | `internal/core/site/portable_test.go: TestPortableRejectsImports` |
| 19 | `build:` config parsing, flags win, warnings and errors | `internal/core/config/build_test.go: TestBuildSection` |
| – | the demo folder builds with no errors | `internal/app/build_test.go: TestBuildDemo` |

Edge cases to cover:
- empty folder (only `index.html` with an empty state);
- a single file argument;
- page names with spaces, unicode and `#`;
- two pages whose output names would collide case-insensitively (`A.md`, `a.md`): warning, the first in sort order wins;
- a self-link and a link to `#anchor` only;
- an image used by many pages, copied once;
- a plugin used on only one page, so its assets load only there;
- an output folder that is a symlink: refused;
- a file in the output that is a symlink: replaced, not followed;
- an external backend that times out: the same warning as in the app, and the build continues;
- the file limits from the global config.

## UI

There is no app UI; the feature is command line only. Output pages:

- They use the app's tokens (`docs/UI.md`): same editorial font, black and white, light and dark through `prefers-color-scheme`.
- The sidebar sits on the left. Below 720px it collapses behind a `<details>` "Contents" toggle, so it works without JS.
- Accessibility:
  - landmarks `<nav aria-label="Pages">`, `<nav aria-label="Breadcrumb">` and `<main>`;
  - a skip link "Skip to content";
  - `aria-current="page"` on the current page;
  - visible focus;
  - `lang` set from frontmatter `lang`, else `en`.
- Print: the sidebar and breadcrumb are hidden, which keeps the handout layout's print CSS intact.
- States: an empty folder gives an `index.html` that says "No pages yet". A plugin that needs the app shows the standard plugin error box with "runs in the zen app only".

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A built page carries script injection from markdown | Same sanitised HTML as the app; the shell is built with `html/template`; rewrites go through the tokenizer, never through string concatenation of attribute values | `TestRewriteLinks` (hostile hrefs, quotes, `javascript:` stay out), `TestBuildMatchesApp` |
| Unapproved plugin code ends up on a public site | Trust is read-only: only allowed plugins and layouts are used, re-checked by hash and fingerprint | `TestBuildTrustReadOnly` |
| Files outside the workspace are copied (`../`, symlinks, absolute `/_ws/`) | Workspace reads go through `workspace` (root-confined, no symlinks out); asset paths are cleaned and must stay inside | `TestWorkspaceFiles` |
| `--clean` or overwrite deletes the user's files | Marker file required; out dir refused at the root or a parent; the out dir is not followed through symlinks; atomic temp+rename writes | `TestCleanNeedsMarker`, `TestCheckOut` |
| Private data leaks into output (absolute paths, user-scoped plugin options such as tokens) | No absolute paths written (`TestDeterministic` greps for the root path). Options with `scope: user` are not available to render-time plugins during a build, and a warning is shown if a plugin declares them. | `TestDeterministic`, `TestBuildNoUserOptions` |
| Built site runs third-party scripts | Each page has a CSP meta tag: `default-src 'self'; script-src 'self'` (portable adds `blob:`); `object-src 'none'`; `base-uri 'none'`; `frame-src` only as the app allows | `TestShellCSP` |
| Reader storage collides between sites on one host | Storage keys are namespaced by `siteId` | `static.test.ts: storage` |

## Notes

- **Out of scope:**
  - an app menu item (File › Build Site…), which is a follow-up `ui/static-build`;
  - single-file export of one page;
  - running `invoke`/`stream` at build time;
  - search in the static site;
  - pretty URLs (`a/b/`);
  - sitemaps and RSS;
  - a watch/serve mode.
- **Integration (done by the lead at merge):**
  - `go.mod`: before the branch starts, make `golang.org/x/net` a direct dependency (it is already required).
  - `main.go`: pass `app.BuildSite` as the new `build` argument of `cli.Run`.
  - `frontend/vite.config.ts`: add the `static` entry (`build.rollupOptions.input` + a library build to `dist/static/` as IIFE), or a second config run by `task build:frontend`.
  - `Taskfile.yml`: make sure `task build` builds the static bundle before `go build`.
  - `cli` usage text: the line `zen build [folder]   build a static website (-o dir, --portable, --drafts, --clean)`.
  - `docs/BUILD.md`: a new how-to, linked from README and `examples/demo`. Also: the `build:` keys in the config docs, and a `build:` example in `examples/demo/.zen.yml`.
  - `.gitignore`: `_site/`.
- **App Store / sandbox:** none. It's a command-line feature that writes only where the user points it.
- **Changelog:** `Added: zen build — export a folder as a static website (hosted or --portable for file://), rendered with the same plugins and layouts as the app.`

## Prompt

> Implement `specs/core/static-build/spec.md` following the rules in
> `specs/README.md`. Context:
> - The app renders pages in Go: `internal/app/state.go` (`open`, `rebuildLocked`, `reindexLocked`, `snapLocked`, `renderSnap.render`) uses `internal/core/render` with layouts (`internal/core/layouts`).
> - Plugin and layout assets are served by `internal/app/assets.go` (`servePlugin`, `serveLayout`).
> - Trust lives in `internal/app/extplugins.go` and `layouts.go`.
> - The page runtime is `frontend/src/plugins/runtime.ts` (module contract `export default { mount(el, ctx) }`).
>
> Build:
> - `internal/core/site` (pure: Input → files) first, with fakes;
> - then `internal/app/build.go`, a headless state with read-only trust and no events or watcher;
> - then the `build` command in `internal/cli`, the `build:` section in `internal/core/config`, and the static runtime in `frontend/src/static`.
>
> Reuse the app's rendering and asset code; do not copy it. No new dependencies (use `golang.org/x/net/html` and `html/template`). Do not launch the app, start containers or run `task install`.
>
> Done when every test in the table passes, `go vet ./internal/... && go test ./internal/...` and `cd frontend && npx tsc --noEmit && npx vitest run` are clean, and `go run . build examples/demo -o <scratch dir>` builds the demo with no errors. Report:
> - the page and file counts;
> - the warnings;
> - the Integration lines;
> - what is unverified (opening the output in a real browser, both hosted and `file://`).
