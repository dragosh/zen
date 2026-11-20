# engine/dialects

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/engine-dialects` |
| Wave | 24 |
| Depends on | kit/features (merged first: both change kit/assembly, This build and the version line), kit/assembly, core/plugins, core/render, core/config |
| Owns | `engine/dialects/**` (new), `builtins/dialects/**` (new, `myst` first), `builtins/builtins.go` (`DialectsFor`), `engine/render/{render,plugins}.go`, `engine/config/{config.go,zenyml_edit.go}` (new edit helper), `engine/session/dialects.go` (new) + `session.go` options, `kit/assembly/**` (`dialects` key), `apps/zenmark/{app-assembly.yml,assembly_gen.go,dialects.go}`, `internal/cli/cli.go` (version line), `kit/ui/src/host.ts`, `kit/ui/src/page/index.ts` (the hint), `kit/ui/src/shell/addons.ts` (This build), `apps/zenmark/frontend/src/shell/settings.ts`, `apps/zenmark/frontend/src/api/{index,wails,fake}.ts`, `examples/demo/plugins/dialects.md` (new) (+ the tests of all of these) |

## Purpose

Markdown written for another tool should render in Zenmark without being rewritten.
A **dialect** is another syntax for zen's container plugins. The first is
**MyST**, the syntax of MyST, Jupyter Book and Sphinx
([zenmark-org/zenmd#3](https://github.com/zenmark-org/zenmd/issues/3)):

````markdown
```{note} Remember
Body text.
```
````

renders exactly like

```markdown
:::note{title="Remember"}
Body text.
:::
```

Two layers decide whether it applies. The **build** decides which dialects an
app can read (`app-assembly.yml`). The **folder** decides which ones it uses
(`.zen.yml`). Syntax belongs to the content, so a folder renders the same way in
the app, in `zenmd export`, on another Mac and in CI.

## Behaviour

### Choosing
1. `.zen.yml` gains `markdown: { dialects: [myst] }`. The default is none: a folder that does not list a dialect renders exactly as today, so a ```` ```{note} ```` fence stays a code block. An id the build doesn't have is ignored, with one warning per folder ("dialect mkdocs is not in this build"). Only the folder config chooses dialects; `config.yml` (global) and front matter cannot.
2. `app-assembly.yml` gains `dialects:`, a list of ids or `all`. A missing key means none, as for plugins. Zenmark gets `dialects: all`. `zenmd version --verbose` prints a `dialects:` line and Settings › This build lists them.
3. The same rules apply in the CLI (`zenmd export site|pdf`), built sites and print, because they all render through `engine/render`.

### MyST (`myst`)
4. **Backtick and tilde fences.** A fence of 3 or more backticks or tildes whose info string starts with `{name}` is the container plugin that owns block `name`.
   - The text after `}` on the opening line is the **argument**. It becomes the attribute `title`, unless an option sets `title`, and is also kept in `arg`.
   - The fence closes on a line of the same character, at least as long, with nothing after it.
   - An outer directive written with more backticks can hold inner ones, as in MyST.
5. **Colon fences.** `:::{name} argument` works too (MyST's colon fence) and closes on a line of at least as many colons.
   - `:::name{attrs}` without braces around the name is zen's own syntax and is unchanged.
6. **Options.** Lines at the top of the body of the form `:key: value` become attributes until the first line that is not one.
   - A key that is not a valid attribute key (`[A-Za-z][A-Za-z0-9_-]*`) is dropped with a warning.
   - An empty value is the flag `"true"`.
   - The rest of the body is the container's markdown, as with `:::`.
7. **Code.** `{code-block} lang` and `{code} lang` render as an ordinary fenced code block in `lang`, so the codeblock plugin highlights them. `{code-cell}` renders the same way, without running anything.
8. **Unknown names.** A `{name}` that no enabled plugin owns stays an ordinary code block (with `{name}` as its info string), shown as today, with no warning. Denied plugins (trust) behave as they do for `:::`.
9. **Names.** MyST's admonition names (`note`, `tip`, `warning`, `important`, `caution`, `attention`, `danger`, `error`, `hint`, `seealso`, `admonition`) are ordinary block names: a plugin answers to them by listing them in its manifest's `blocks`. No new manifest field is needed. The demo's `note` plugin gains `tip`, `important` and `caution` to show this.
10. Inside an ordinary code fence nothing is scanned, as for `:::` today. Nesting limits and warnings are the same as for `:::` containers (`maxNesting`).

### Suggesting it
11. When a folder opens, the session looks for signs of a MyST project:
    - a `myst.yml`;
    - a Jupyter Book `_config.yml` with a `parse:` or `myst_` key;
    - a `conf.py` that mentions `myst_parser`.

    If it finds one, the build has `myst`, and `.zen.yml` doesn't list it, `Startup.DialectHint` names it. Detection reads only those files at the folder's root and never runs anything.
12. The reader shows a one-line hint above the page: "This folder looks like a MyST project. Turn on MyST syntax?" It has **Turn on** and **Not now**.
    - **Turn on** adds `myst` to `markdown.dialects` in `.zen.yml`, creating the file if needed and keeping its comments and other keys. The pages then re-render.
    - **Not now** hides the hint for this folder; the choice is kept in app state, not in the folder.
    - There is no hint in a read-only app (kit/features), or once the folder lists the dialect.
13. Settings has a **Markdown** section for the open folder: one checkbox per dialect in the build, with its name and a one-line summary. Changing it writes `.zen.yml` in the same way. Without a folder, or in a read-only app, the checkboxes are shown disabled with the reason.

## Architecture

- `engine/dialects` (no dependencies beyond `engine/plugins`):
  - `Dialect` (id, name, summary, `Scan`, `Detect`) and `Registry` (ordered, ids checked like add-on ids).
  - `Scan` returns `plugins.Span`s, the type `:::` containers already use. For code it returns `Code` spans with the language.
- `builtins/dialects/myst`: the MyST scanner and detector. `builtins.DialectsFor(ids)`; `builtins.Dialects()` returns all of them.
- `engine/render`:
  - `Options.Dialects []dialects.Dialect`, the ones the folder enabled.
  - `replaceContainers` merges the dialect spans with `plugins.ScanContainers` in source order, before the `:::` early return. It keeps "outermost wins" and skips spans whose name no enabled plugin owns.
  - Code spans are rewritten to a plain fence (```` ```lang ````) before goldmark parses.
  - With no dialects the path is byte-for-byte today's.
- `engine/config`:
  - `Markdown.Dialects []string`, from `.zen.yml` only.
  - `SetDialects(src []byte, ids []string) ([]byte, error)` in `zenyml_edit.go` edits the `markdown:` block in place and keeps comments, the way `frontmatter_edit.go` edits `nav:`.
- `engine/session`:
  - `Options.Dialects *dialects.Registry`; the session passes the folder's enabled ones to render.
  - `Detect` runs on open and sets `Startup.DialectHint`.
  - `SetDialect(id string, on bool) error` writes `.zen.yml`, reloads the config and re-renders. It goes through the `writable()` guard from kit/features.
  - `DismissDialectHint(id)` stores the dismissal in app state.
- kit/assembly: the `dialects` key and the catalog list. The generator writes `assemblyDialects()` and the ids into `BuildInfo.Dialects`.

## Interfaces

```go
// engine/dialects
type Dialect interface {
	ID() string      // "myst"
	Name() string    // "MyST"
	Summary() string // one line for Settings
	// Scan finds the dialect's directives in src (a page body), outside ordinary code fences.
	Scan(src []byte) []Span
	// Detect reports whether the folder looks like it was written for this dialect (root files only).
	Detect(root fs.FS) bool
}
type Span struct {
	plugins.Span
	Code bool   // a code directive: render as a fence in Lang
	Lang string
}
type Registry struct{ /* ordered */ }
func NewRegistry(ds ...Dialect) (*Registry, error)
func (r *Registry) Get(id string) Dialect
func (r *Registry) List() []Dialect

// builtins
func Dialects() *dialects.Registry
func DialectsFor(ids []string) (*dialects.Registry, error)

// engine/config
type Markdown struct{ Dialects []string }
func SetDialects(src []byte, ids []string) ([]byte, error)

// engine/session
// Options.Dialects *dialects.Registry; Startup.DialectHint string ("" or an id)
func (s *Session) SetDialect(id string, on bool) error
func (s *Session) DismissDialectHint(id string) error
// BuildInfo.Dialects []string `json:"dialects"`
```

```ts
// kit/ui/src/host.ts
export type DialectInfo = { id: string; name: string; summary: string; on: boolean };
// KitHost.dialects?(): Promise<DialectInfo[]>; setDialect?(id, on); dismissDialectHint?(id)
// BuildInfo.dialects: string[]
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Off by default: ```` ```{note} ```` is a code block; `.zen.yml` turns it on; an unknown id warns once; front matter/global cannot | `engine/render/dialects_test.go: TestDialectOff`, `TestDialectOn`; `engine/config/config_test.go: TestMarkdownDialects` |
| 2 | Assembly key, `all`, unknown id; version line; This build | `kit/assembly/assembly_test.go: TestDialects`; `internal/cli/cli_test.go: TestVersionVerbose`; `settings.test.ts: "This build section"` |
| 3 | Built site renders MyST the same as the app | `engine/session/dialects_test.go: TestExportUsesDialects` |
| 4 | Backtick/tilde fences, argument → title (+arg), longer outer fence nests | `builtins/dialects/myst/myst_test.go: TestFence`, `TestArgument`, `TestNesting` |
| 5 | `:::{name}` colon fence; `:::name` unchanged | `myst_test.go: TestColonFence`; existing `engine/plugins/syntax_test.go` |
| 6 | Option lines → attributes; invalid key dropped with warning; flags | `myst_test.go: TestOptions` |
| 7 | `{code-block}`/`{code}`/`{code-cell}` → highlighted code | `engine/render/dialects_test.go: TestMystCode` |
| 8 | Unknown name stays code, no warning; denied plugin as for `:::` | `TestMystUnknown`, `TestMystDenied` |
| 9 | A plugin answers MyST names through `blocks` | `TestMystAdmonitionNames` (demo note plugin) |
| 10 | Not scanned inside code fences; nesting limit | `myst_test.go: TestInsideCode`; `TestMystNestingLimit` |
| 11 | Detection: myst.yml, _config.yml, conf.py; nothing outside the root; hint only when not on | `builtins/dialects/myst/detect_test.go`; `engine/session/dialects_test.go: TestDialectHint` |
| 12 | Hint: Turn on writes .zen.yml (comments kept, file created), Not now remembered; none read-only | `engine/config/zenyml_edit_test.go: TestSetDialects`; `TestSetDialectReadOnly`; `kit/ui/src/page/page.test.ts: "dialect hint"` |
| 13 | Settings › Markdown checkboxes write the folder; disabled without folder / read-only | `settings.test.ts: "Markdown section"` |
| – | No dialects: rendering byte-for-byte unchanged | `engine/render/dialects_test.go: TestNoDialectsUnchanged` (golden over examples/demo); `TestDemoBuildUnchanged` |

Edge cases:
- An unclosed fence runs to the end of the page, as CommonMark does.
- CRLF line endings.
- A fence indented by 1 to 3 spaces.
- Option lines after a blank line: the blank line ends the options.
- `{name}` with spaces, like `{ note }`: not a directive.
- An info string `{name}` with no argument.
- A MyST fence inside a `:::` container, and the reverse.
- A huge page (no quadratic scan; time-boxed test with 10,000 directives).
- A `.zen.yml` with `markdown:` written as a flow mapping.

## UI

- **Hint:** one quiet line above the page (the style of the reader's warnings strip, an info tone, not a warning). The two buttons are ghost buttons, and Esc dismisses the hint like Not now. It is announced politely (`role="status"`).
- **Settings › Markdown:** a checkbox list (name, then the summary muted underneath). The note under it reads "Saved in this folder's .zen.yml, so the folder renders the same everywhere."
- Tokens only (docs/UI.md).

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Option values or arguments inject HTML | They become plugin attributes, which go through the same sanitiser as `:::` attributes | `TestMystOptionsSanitised` |
| Detection reads or runs the folder's files | Only three root file names are read, at most 64 KB each, as text; `conf.py` is never executed | `detect_test.go: TestDetectReadsOnly` |
| A page turns on a dialect for itself | Only `.zen.yml` can turn one on; front matter `markdown:` is ignored with a warning | `TestMarkdownDialects` |
| A crafted page makes the scanner slow | A linear scan with the same limits as `ScanContainers` | `myst_test.go: TestLinear` |
| Writing `.zen.yml` in a read-only app | `SetDialect` goes through the session's write guard (kit/features) | `TestSetDialectReadOnly` |

## Notes

- **Out of scope:**
  - other dialects: MkDocs `!!! note` admonitions, GitHub `> [!NOTE]` alerts, Pandoc fenced divs, each one more dialect with its own small spec;
  - MyST roles (`` {ref}`x` ``), cross-references, YAML option blocks (`---` inside a directive), and `{eval-rst}`;
  - a built-in admonition plugin (today `note` comes from a folder plugin; a built-in one would be its own issue);
  - editor syntax highlighting of MyST fences;
  - Vale rules for directives.
- **Answers to #3's questions:**
  - the colon form `:::{note}` is in (behaviour 5);
  - aliases need no new manifest field, because a plugin lists the MyST names in `blocks` (behaviour 9);
  - a built-in admonition plugin is a separate issue.
- **Integration (lead, at merge):**
  - `docs/PLUGINS.md`: "Other syntaxes (dialects)", pointing here.
  - `docs/ENGINE.md`: `engine/dialects`, `render.Options.Dialects`, `builtins.DialectsFor`.
  - `docs/BUILD.md`: the `dialects` key.
  - `docs/ADDONS.md`: the table row "dialect" next to add-on, plugin and layout.
  - The user guide page `examples/demo/plugins/dialects.md` and its golden hash.
  - README.
  - Close #3 with the merge (`Fixes #3` on the merge commit).
- **App Store / sandbox:** none; detection only reads files the user opened.
- **Changelog:** `Added: Markdown dialects: a folder can turn on MyST syntax (```{note} and :::{note} directives) in .zen.yml; the app suggests it for MyST projects.`

## Prompt

> Implement `specs/engine/dialects/spec.md` following the rules in `specs/README.md`.
>
> Context:
> - Containers are scanned at the source level today: `plugins.ScanContainers` in `engine/plugins/syntax.go`, used by `replaceContainers` in `engine/render/plugins.go`, with code-fence tracking in `codeFence`.
> - Fenced code blocks go through `engine/render/nodes.go`.
> - Plugins own block names through `Manifest.Blocks` (extplugins: `blocks:` in plugin.yml).
> - Config is in `engine/config/config.go`; in-place YAML editing that keeps comments is in `engine/config/frontmatter_edit.go`.
> - The assembly is in `kit/assembly` plus the generator `kit/assembly/cmd/gen`; the read-only guard `writable()` comes from kit/features.
> - The demo's note plugin is `examples/demo/.zen/plugins/note`.
>
> Order of work:
> 1. `engine/dialects` and the MyST scanner, with their tests (the largest part).
> 2. The render merge, with `TestNoDialectsUnchanged` first.
> 3. Config and `SetDialects`.
> 4. Session options, detection, `SetDialect`.
> 5. Assembly and generator.
> 6. The frontend hint and Settings › Markdown.
> 7. The demo page.
>
> Do not launch the app. Done when the table's tests pass with and without `-tags addon_vale`, and the frontend checks pass in kit/ui and apps/zenmark/frontend.
