# kit/assembly

| | |
|---|---|
| Status | done |
| Target version | 0.6.0 |
| Branch | `feature/kit-assembly` |
| Wave | 22 |
| Depends on | kit/addons (merged first), core/plugins, core/layouts, core/init (site templates) |
| Owns | `kit/assembly/**` (new), `apps/zenmark/app-assembly.yml` (new), `apps/zenmark/assembly_gen.go` (generated), `apps/zenmark/app.go`, `apps/zenmark/headless.go`, `apps/zenmark/addons*.go`, `builtins/builtins.go`, `builtins/layouts/layouts.go`, `builtins/sitetemplates/presets.go`, `builtins/sitetemplates/*/template.yml`, `Taskfile.yml`, `build/darwin/Taskfile.yml`, `build/Taskfile.yml`, `internal/cli/cli.go` (version), `kit/ui/src/shell/addons.ts` (becomes the build section) + tests, `apps/zenmark/frontend/src/shell/settings.ts` |

## Purpose

One file says what an app is made of: `apps/<app>/app-assembly.yml` lists its
add-ons, built-in plugins, layouts, site templates (blueprints) and theme. The
build includes exactly that and nothing else, so Zenmark can ship everything and
a niche app (HashDish, say) only a recipe layout and one template, from the same
repository.

```yaml
# apps/zenmark/app-assembly.yml
app: zenmark
addons: [vale]
plugins: [codeblock, diagram, check, quiz, sandbox]
layouts: [article, docs, post, landing, wide, handout, slides, course]
templates: [techdocs, learning, planner]
theme: default
```

## Behaviour

### The file
1. `apps/<app>/app-assembly.yml` has the keys `app` (the app id), `addons`, `plugins`, `layouts`, `templates` (each a list of ids, or `all`) and `theme` (an id). A missing list means none; `all` means every built-in of that kind, so a new built-in joins Zenmark without editing the file. Unknown keys and unknown ids fail with the known ids listed.
2. The order of `plugins`, `layouts` and `templates` is the order the app shows them in (the layout picker, New Site). `all` keeps the built-in order.
3. Dependencies are checked, and the build fails with a sentence naming both sides:
   - a template that needs a layout or plugin that is not in the assembly ("template learning needs layout course");
   - a layout that needs a plugin ("layout course needs plugin quiz"), when a layout declares one;
   - `layouts` empty while `templates` is not.
   `article` is always present (the fallback layout); listing it is allowed, leaving it out is not an error.
4. `theme` names the app's look (`default` is today's tokens, the only theme for now). An unknown theme fails the build.

### Building
5. `task build|package|run|dev|release APP=zenmark` (default `APP=zenmark`) reads `apps/$APP/app-assembly.yml`. `ADDONS=` on the command line still wins over the file's `addons` (for a one-off build); without it, the file's list is used. So `task package` builds Zenmark with Vale again, as the file says.
6. A generator (`go run ./kit/assembly/cmd/gen apps/zenmark`) writes `apps/zenmark/assembly_gen.go`: the plugin packages it imports and registers (others are not linked), the layout and template ids, and the theme. The generated file is committed; `task build` regenerates it, and a test fails when it does not match the YAML.
7. `zenmd version --verbose` prints the assembly: the app, add-ons, plugins, layouts, templates and theme, one line each. `zenmd version` keeps its one line (with the add-ons).

### Running
8. The app's session gets exactly the assembled sets: `session.Options.Registry` (plugins), `.Layouts`, `.Presets` (templates). The CLI (`zenmd build`, `export`, `init`) is the same binary and uses the same sets, so `zenmd init site --planner` fails in an app without the planner template, with the templates it has.
9. A folder that uses something the app was not built with behaves as it does today for an unknown name: a plugin's block renders as a plain code block with the usual "unknown plugin" warning, a layout falls back to `article` with its warning. Nothing crashes, nothing is lost from the file.
10. Settings › Add-ons becomes Settings › **This build**: the app's add-ons (as before), then the plugins, layouts and templates it includes and the theme, as short lists. Read-only.

### Built-ins
11. Each built-in site template declares what it needs in its `template.yml`: `requires: { layouts: [...], plugins: [...] }`. A test checks every template's files use only what it declares (the `layout:` of each page and `.zen.yml`, and the fenced blocks of the plugins).
12. A layout may declare required plugins in its layout manifest (`requires: [quiz]`); `course` declares `quiz` and `check` if its pages need them (check the layout's files).
13. `builtins` gains selection helpers: `PluginsFor(ids)`, `LayoutsFor(version, ids)`, `SiteTemplatesFor(ids)`; the existing `Plugins()`, `Layouts()`, `SiteTemplates()` stay (all of them) for embedders and the engine's defaults.

## Architecture

- `kit/assembly` (Go): `type Assembly struct { App string; Addons, Plugins, Layouts, Templates []string; Theme string }`, `Load(path) (Assembly, error)` (YAML, strict keys, `all` expanded against the catalog), `Check(a Assembly, cat Catalog) error` (ids and dependencies). `Catalog` lists the known ids and requirements: add-ons from `kit/addons/cmd/check`'s known list, plugins and layouts from `builtins`, templates from their `template.yml`.
- `kit/assembly/cmd/gen`: loads and checks the assembly, writes `assembly_gen.go` (package of the app, `// Code generated … DO NOT EDIT.`), and prints the add-on tags for the Taskfile (`-tags-only`), replacing the default of `ADDONS`.
- The generated file declares `assemblyPlugins() *plugins.Registry`, `assemblyLayouts(version) *layouts.Registry`, `assemblyTemplates() fs.FS`, `assemblyInfo` (for version and Settings). `app.go` and `headless.go` pass these to the session instead of `builtins.Plugins()` and the engine defaults.
- `builtins/layouts.Registry(version, ids...)` and `sitetemplates.Sub(ids...)` filter; plugin selection is by import in the generated file, so unused plugin packages are not linked.
- Theme: `theme` is carried to the frontend (`startup().build.theme`) and becomes `data-app-theme` on the root element; `default` needs no CSS. Later themes are token sets in `kit/ui/src/themes/<id>.css`, each a spec of its own.
- The add-ons spec's `ADDONS` default changes from empty to "the assembly's list"; the tag mechanism is unchanged.

## Interfaces

```go
// kit/assembly
type Assembly struct {
	App                                   string
	Addons, Plugins, Layouts, Templates   []string
	Theme                                 string
}
type Catalog struct {
	Addons, Plugins, Layouts, Templates, Themes []string
	LayoutNeeds, TemplateNeeds                  map[string]Needs
}
type Needs struct{ Layouts, Plugins []string }
func Load(path string, cat Catalog) (Assembly, error)
func Check(a Assembly, cat Catalog) error
func BuiltinCatalog() (Catalog, error)

// builtins
func PluginsFor(ids []string) (*plugins.Registry, error)
func LayoutsFor(version string, ids []string) (*enginelayouts.Registry, error)
func SiteTemplatesFor(ids []string) (fs.FS, error)

// engine/session: Startup.Build *BuildInfo
type BuildInfo struct {
	App     string      `json:"app"`
	Addons  []AddonInfo `json:"addons"`
	Plugins, Layouts, Templates []string
	Theme   string      `json:"theme"`
}
```

```ts
// kit/ui/src/host.ts
export type BuildInfo = { app: string; addons: AddonInfo[]; plugins: string[]; layouts: string[]; templates: string[]; theme: string };
// KitHost.build?: BuildInfo  (KitHost.addons stays, = build.addons)
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Keys, lists, `all`, unknown key or id fails with the known list | `kit/assembly/assembly_test.go: TestLoad`, `TestUnknown` |
| 2 | Order kept; `all` in built-in order | `TestOrder` |
| 3 | Missing layout/plugin for a template or layout fails with a clear sentence; article implied | `TestDependencies` |
| 4 | Unknown theme fails | `TestTheme` |
| 5 | ADDONS overrides the file; the file is the default | `kit/assembly/cmd/gen/gen_test.go: TestTagsFromAssembly`, `TestAddonsOverride` |
| 6 | Generated file matches the YAML (drift fails) | `apps/zenmark/assembly_test.go: TestAssemblyGenerated` |
| 6 | Only listed plugin packages are imported | `gen_test.go: TestOnlyListedPluginsImported` |
| 7 | `zenmd version --verbose` | `internal/cli/cli_test.go: TestVersionVerbose` |
| 8 | Session and CLI get the assembled sets; `init` with a missing template fails listing the available ones | `apps/zenmark/assembly_test.go: TestSessionUsesAssembly`; `TestInitMissingTemplate` |
| 9 | A folder using a plugin/layout outside the assembly: plain block + warning; article + warning | `engine/session/assembly_test.go: TestOutsideAssembly` |
| 10 | Settings › This build lists everything | `apps/zenmark/frontend/src/shell/settings.test.ts: "This build section"` |
| 11 | Each template uses only what it declares | `builtins/sitetemplates/requires_test.go: TestTemplatesDeclareNeeds` |
| 12 | Layout requirements declared and checked | `builtins/layouts/layouts_test.go: TestLayoutNeeds` |
| 13 | Selection helpers: order, unknown id error | `builtins/builtins_test.go: TestFor` |

Edge cases: an empty `plugins` list (allowed: without codeblock, code fences render as plain `<pre>` without highlighting); duplicate ids (error); `all` mixed with ids (error); a template listed twice; Windows/Linux line endings in the YAML.

## UI

Settings › This build: sections Add-ons (cards as today), Plugins, Layouts, Templates (comma lists of names, muted), Theme. Read-only. Tokens only.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| The generator writes Go code from YAML | Ids are checked against the catalog (and `^[a-z][a-z0-9-]{0,31}$`) before they reach the generated file; nothing else from the YAML is written into code | `gen_test.go: TestNoInjection` |
| A build silently leaves out what a folder relies on | Behaviour 9 keeps today's warnings; `zenmd version --verbose` and Settings show what is in the build | `TestOutsideAssembly` |

## Notes

- **Out of scope:** user-installed plugins and layouts (they keep working as today, per folder and trust); themes other than `default`; a second app (HashDish etc.) — this spec makes it possible, the app is its own spec; choosing the assembly at run time. Optional app features (`features:`, the editor first) are in [specs/kit/features](../features/spec.md).
- **Integration (lead, at merge):** docs/ADDONS.md and docs/BUILD.md: the assembly file; docs/ENGINE.md: the `builtins` helpers and `Startup.Build`; README; user guide golden hash only if the guide mentions it; CHANGELOG `Added: apps/<app>/app-assembly.yml chooses an app's add-ons, built-in plugins, layouts, site templates and theme; Settings › This build shows them.`
- **App Store / sandbox:** none beyond kit/addons.

## Prompt

> Implement `specs/kit/assembly/spec.md` following the rules in `specs/README.md`.
> It builds on kit/addons (merged): `ADDONS`, `addon_<id>` tags, `kit/addons/cmd/check`,
> Settings › Add-ons (`kit/ui/src/shell/addons.ts`), `Startup.Addons`. The built-ins are
> assembled in `builtins/builtins.go` (plugins: codeblock, diagram, check, quiz, sandbox;
> layouts in `builtins/layouts`; templates techdocs, learning, planner in
> `builtins/sitetemplates`), and the session takes `Options.Registry/Layouts/Presets`
> (defaults in `engine/session/session.go`). Start with `kit/assembly` (Load/Check,
> catalog) and the template/layout `requires` with their tests; then the builtins
> helpers; then the generator and `assembly_gen.go`; then the app wiring, the Taskfiles
> (APP, ADDONS default from the file), the version line and Settings › This build.
> Zenmark's assembly lists everything, so its behaviour must not change except that
> Vale is in the default build again. Done when the table's tests pass, both Go tag
> sets pass, and the frontend checks pass.
