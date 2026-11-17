# core/init

| | |
|---|---|
| Status | done |
| Target version | 0.2.0 |
| Branch | `feature/core-init` |
| Wave | 12 |
| Depends on | core/layouts, app/menu, ui/new-page, plugins/quiz |
| Owns | `internal/core/sitetemplates/` (new), `internal/sitetemplates/{techdocs,learning,planner}/` (new, embedded presets), `internal/cli` (the `init` command), `internal/core/config` (the `templates:` key), `internal/app/sitetemplates.go` + test (new), `frontend/src/shell/newsite.ts` + test (new), the welcome view in `frontend/src/shell/shell.ts`, `examples/demo/getting-started/new-site.md` (new) |

## Purpose

Start a new folder from a ready-made **site template**: technical
documentation laid out after [Diátaxis](https://diataxis.fr), a learning
path, or a planner. Templates are plain folders in a templates directory
that you can edit, copy or add to. Use them from the command line
(`zen init site --techdocs`), the welcome screen, or the File menu.

## Behaviour

### Templates
1. A site template is a folder with a `template.yml` and the files to copy:
   ```yaml
   id: techdocs
   name: Technical documentation
   description: Tutorials, how-to guides, reference and explanation (Diátaxis)
   version: 1.0.0
   order: 10            # position in lists; lower first
   ask:                 # values asked when the site is created
     title: { label: Site title, default: "{{folder}}" }
     author: { label: Author, default: "" }
   ignore: [".DS_Store"]
   ```
   Only `template.yml` is required. Every other file is copied as it is, except for the placeholders below.
2. **Placeholders**: in `.md`, `.yml`, `.yaml`, `.txt` and `.css` files and in file names, `{{title}}`, `{{author}}`, `{{date}}` (YYYY-MM-DD) and `{{folder}}` (the target folder's name) are replaced by their values. Any `ask:` key is replaced too. This is plain text replacement, not a template language: nothing is executed. An unknown `{{name}}` is left as it is. Other files (images) are copied byte for byte.
3. **Where templates live**: in `<templates dir>/sites/<id>/`. The default templates dir is `<zen config dir>/templates`, the folder next to `config.yml`. It can be changed in the global `config.yml` only:
   ```yaml
   templates:
     dir: ~/Documents/zen templates
   ```
   `~` expands to the home folder, and a relative path is relative to the config folder. `.zen.yml` cannot set it; the key there is a warning.
4. **Built-in presets are installed for editing**:
   - The first time templates are listed, zen copies the three built-ins (`techdocs`, `learning`, `planner`) into `<templates dir>/sites/`. It writes `<templates dir>/sites/.zen-templates.yml` recording which built-ins it installed and their version.
   - An installed template is never overwritten. A preset the user deleted is not installed again.
   - A newer built-in version is listed as "update available" and installed only on request (`zen init --update <id>`), into `<id>.new/`, beside the user's copy.
   - If the templates dir can't be written to, the built-ins are listed straight from the app bundle, with a note.
5. **Listing**: every folder under `sites/` with a valid `template.yml` is a template, sorted by `order`, then `name`. An invalid one is listed with its error and cannot be used. Two templates with the same `id`: the first in folder-name order wins, and a warning is shown.

### Creating a site
6. Creating a site copies the template into the target folder, then:
   - **The target may be a new folder or an existing one.** In an existing one, nothing is overwritten. If any file the template would write already exists, nothing is written at all, and the error lists up to 10 of the clashing files. Creating into a non-empty folder with no clashes is allowed.
   - Symlinks in a template are refused, and so is anything that would end up outside the target folder after placeholders in file names are applied (`..`).
   - Copies are atomic per file, and files are created with permissions 0644 and folders with 0755.
   - The result lists the files created, in order.
7. Limits: 2,000 files and 50 MB per template. Larger templates are refused before anything is written.

### Command line
8. `zen init site [--<template id> | --template <id>] [folder] [--title …] [--author …] [--set key=value]`:
   - `folder` defaults to the current folder.
   - `--techdocs`, `--learning`, `--planner`, and `--<id>` for any installed template are shortcuts for `--template <id>`.
   - Without a template, it prints the list and exits with 2.
   - Values not given on the command line take their `default`. The command never asks interactively, so it works in scripts.
9. On success it prints `Created 14 files in docs (Technical documentation). Open it with: zen docs` and exits 0. Clashes or an invalid template exit 1, and usage errors exit 2.
10. `zen init --list` prints the templates as `id  name — description`, marking invalid ones and those with an update. `zen init --update <id>` writes the newer built-in next to the user's copy (behaviour 4). `zen init --dir` prints the templates dir.

### In the app
11. **Welcome screen**: below Open a folder, a section **Start from a template** shows a card for each template (name, description), up to 6, plus **Show all…**.
    - Choosing one opens the **New Site** dialog: it asks for each `ask:` value (title first, pre-filled), then shows a **Choose folder…** button that opens the native folder panel with "New Folder" enabled.
    - Create makes the site there and opens it. The first page is the folder's README, as when opening any folder.
12. **File › New Site from Template…** (⇧⌘N) is the first item of the File menu, followed by a separator. It opens the same dialog with a template list first. It works from the welcome screen and from an open folder. Opening the new site replaces the current folder; unsaved editors are asked first, as with Open Folder.
13. The dialog states:
    - loading;
    - no templates ("No templates in <dir>", with a Reveal in Finder button);
    - an invalid template (shown greyed out with its error);
    - clashes (the list, and the folder choice stays open);
    - an error writing.

    The dialog has an **Open templates folder** link that reveals the templates dir in Finder.

### The three presets
14. **`techdocs`, technical documentation after Diátaxis:**
    - `README.md` is the home page with the `landing` layout. It has cards for the four kinds of documentation and one line on when to use each.
    - `tutorials/`, `how-to/`, `reference/` and `explanation/` each have an `index.md` that says what belongs there, plus one example page written for this site's topic (`{{title}}`).
    - `.zen.yml`: `layout: docs`, `layouts: { "README.md": landing }`, `diagram` and `codeblock` on, and a `build:` section.
    - A short `CONTRIBUTING.md` on where a new page goes. The text is original; it links to diataxis.fr and doesn't copy it.
15. **`learning`, a learning path:**
    - `README.md` (`landing`) lists the paths.
    - `paths/first-path/` holds a `course` layout with 4 steps: reading, a `:::quiz` check, an exercise with `:::check` items, and a recap.
    - A `glossary.md`, plus `.zen.yml` with `layouts: { "paths/**": course }`.
    - Progress is kept per reader through the course layout's storage, as today.
16. **`planner`:**
    - `README.md` (`landing`) is the dashboard, linking to the plan pages.
    - `roadmap.md` (`wide` layout) holds a mermaid gantt chart and a milestones table.
    - `weeks/{{date}}.md` is this week's plan, with goals, a `- [ ]` list and notes.
    - `decisions/0001-record-decisions.md` is a first decision record, from a short template.
    - `meetings/` holds a meeting notes template, and `ideas.md` is a parking lot.
    - `.zen.yml` sets `layouts: { "roadmap.md": wide }`.
17. Every preset builds with `zen build` with no warnings, and passes Vale with the demo's vocabulary except for the words its template values bring in.

## Architecture

- `internal/core/sitetemplates`:
  - `List(dir) ([]Template, []string)`, `Install(dir, builtins fs.FS) error`, `Create(t Template, target string, values map[string]string) (Result, error)`;
  - placeholder replacement, and the clash check done before any write;
  - no `internal/app` import.
- `internal/sitetemplates`: `embed.FS` with `techdocs/`, `learning/` and `planner/`, each holding a `template.yml`.
- `internal/core/config`: `(*Config).TemplatesDir(configDir string) (string, []string)`, global only.
- `internal/cli`: the `init` command calls an injected `Initer` func (like `Builder`) that `main.go` wires to `app.InitSite`.
- `internal/app/sitetemplates.go`:
  - the services `SiteTemplates() []SiteTemplate`, `PickSiteFolder() (string, error)` (native panel, New Folder allowed), `CreateSite(id, dir string, values map[string]string) (Startup, error)` (creates, then opens the folder like `openPath`), and `RevealTemplates() error`;
  - `InitSite(cli.Init, stdout, stderr) error` for the CLI.
- `frontend/src/shell/newsite.ts`: the dialog, a pattern like `newpage.ts`. The welcome section is built in `shell.ts` (`showWelcome`).

## Interfaces

```go
// internal/core/sitetemplates
type Ask struct{ Key, Label, Default string }
type Template struct {
	ID, Name, Description, Version, Dir string
	Order    int
	Ask      []Ask
	Builtin  bool   // installed from zen's presets
	Update   string // newer built-in version, "" if none
	Err      string // invalid: why
}
type Result struct{ Files []string } // target-relative, in creation order
func List(dir string) ([]Template, []string)
func Install(dir string, builtins fs.FS) error
func Create(t Template, target string, values map[string]string) (Result, error)
var ErrClash = errors.New("files already exist") // wrapped with the list

// internal/cli
type Init struct {
	Dir, Template string
	Values        map[string]string
	List, Update  bool
	ShowDir       bool
}
type Initer func(Init, io.Writer, io.Writer) error
// Run gains a trailing `initer Initer` argument (nil = "zen init" not available).

// internal/app
type SiteTemplate struct {
	ID, Name, Description, Version string
	Ask     []struct{ Key, Label, Default string }
	Builtin bool
	Update  string
	Error   string
}
func (z *Zen) SiteTemplates() []SiteTemplate
func (z *Zen) PickSiteFolder() (string, error)
func (z *Zen) CreateSite(id, dir string, values map[string]string) (Startup, error)
func (z *Zen) RevealTemplates() error
func InitSite(i cli.Init, stdout, stderr io.Writer) error
const EventNewSite = "menu:new-site"
```

```ts
// frontend/src/api: siteTemplates(), pickSiteFolder(), createSite(id, dir, values), revealTemplates(); on("menu:new-site")
// frontend/src/shell/newsite.ts
export function openNewSite(opts: { api: Api; doc: Document; onCreated(s: Startup): void; preselect?: string }): NewSiteDialog;
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | template.yml parsing, defaults, invalid manifests | `internal/core/sitetemplates/manifest_test.go: TestManifest` |
| 2 | placeholders in text files and names; binary untouched; unknown kept; nothing executed | `create_test.go: TestPlaceholders` |
| 3 | templates dir default, `~`, relative, global only | `internal/core/config/templates_test.go: TestTemplatesDir` |
| 4 | install once, record, never overwrite, deleted stays deleted, update to `<id>.new`, read-only dir lists embedded | `install_test.go: TestInstall` |
| 5 | listing order, invalid listed with error, duplicate ids | `list_test.go: TestList` |
| 6 | clash check before any write; non-empty folder allowed; symlinks and `..` refused; perms | `create_test.go: TestCreateClashes`, `TestCreateContainment` |
| 7 | size and count limits refused before writing | `create_test.go: TestLimits` |
| 8–10 | CLI parsing, shortcuts, list/update/dir, exit codes, messages | `internal/cli/cli_test.go: TestInitCommand`, `TestInitExitCodes` |
| 11, 13 | welcome cards, dialog states, ask fields, folder pick, create opens the folder | `frontend/src/shell/newsite.test.ts`; `shell.test.ts: "welcome lists templates"` |
| 12 | menu:new-site opens the dialog in both views; unsaved editors asked first | `newsite.test.ts: "menu event"`; `internal/app/sitetemplates_test.go: TestCreateSiteClosesEditors` |
| 14–16 | each preset creates, renders with no warnings, builds with no warnings | `internal/app/sitetemplates_test.go: TestPresets` |
| 17 | presets pass Vale (when Vale is present) | `internal/app/sitetemplates_test.go: TestPresetsLint` (skipped without the helper) |

Edge cases to cover:
- a target folder with spaces and unicode;
- the target being the templates dir itself, or inside it (refused);
- a template whose file names contain `{{date}}`;
- an empty `ask:`;
- a values map with extra keys (ignored);
- `--set title=…` and `--title …` both given (`--title` wins);
- creating into a folder that is currently open in zen.

## UI

- Welcome screen: the logo, Open a folder (primary), then **Start from a template**: a grid of up to 6 cards (name in the body font, description muted, 2 lines at most), then Show all…. Tokens from docs/UI.md; black and white.
- New Site dialog:
  - a modal like New Page: a template list (radio cards), the `ask:` fields, a Choose folder… button and the chosen path shown as text, then Cancel and **Create** (primary, disabled until a folder is chosen);
  - Esc cancels and Enter creates;
  - focus starts on the first field;
  - the template list is a `radiogroup` with arrow keys;
  - errors use `role="alert"`.
- File menu (Integration): New Site from Template… ⇧⌘N · ― · New Page… ⌘N · Open Folder… ⌘O.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A template writes outside the target (`..`, absolute names, symlinks) | Paths are cleaned after placeholders and must stay inside; symlinks and special files are refused; writes go through one helper | `TestCreateContainment` |
| Overwriting the user's files | Every target path is checked before the first write; nothing is overwritten, ever | `TestCreateClashes` |
| A template runs code | Placeholders are plain string replacement; no template engine and no hooks. Plugins a template brings into `.zen/` still go through trust when the folder opens | `TestPlaceholders`; `TestPresets` (no `.zen/plugins` in presets) |
| A huge or hostile template exhausts the disk or memory | Count and size limits are checked first; files are streamed | `TestLimits` |
| A folder's `.zen.yml` redirects the templates dir | `templates:` is read from the global config only | `TestTemplatesDir` |

## Notes

- **Out of scope:**
  - downloading templates from the internet;
  - page templates (New Page already uses layouts);
  - updating a site made from a template when the template changes;
  - a template editor in Settings (the folder is the editor).
- **Integration (done by the lead at merge):**
  - `main.go`: pass `app.InitSite` as the `initer` argument of `cli.Run`.
  - `internal/app/menu.go`: put New Site from Template… (⇧⌘N, `menuActions.NewSite`) first in File, followed by a separator; `menu_test.go` expectations.
  - The api: add `siteTemplates`, `pickSiteFolder`, `createSite`, `revealTemplates` and `menu:new-site` in `index.ts`, `wails.ts` and `fake.ts`.
  - The CLI usage text gets `zen init site [--techdocs|--learning|--planner] [folder]`.
  - Docs: `docs/PLAN.md` Service API; a `docs/TEMPLATES.md` how-to (format, placeholders, where they live); README link; changelog.
- **App Store / sandbox:** the templates dir is inside the container, so it works. The target folder comes from the folder panel. `zen init` from the CLI isn't in the App Store build (packaging/appstore).
- **Changelog:** `Added: site templates — zen init site --techdocs|--learning|--planner and File › New Site from Template… start a folder from editable presets kept in your templates folder.`

## Prompt

> Implement `specs/core/init/spec.md` following the rules in
> `specs/README.md`. Context:
> - New Page (`internal/app/newpage.go`, `frontend/src/shell/newpage.ts`) is the model for creating files and for the dialog.
> - `zen build` (`internal/cli` `runBuild`, `Builder`) is the model for a CLI command with an injected function.
> - Layouts available: `landing`, `docs`, `course`, `wide`, `post`, `handout`, `slides`. Plugins: `quiz`, `check`, `diagram`, `codeblock`.
>
> Build `internal/core/sitetemplates` first, test-first with fake file systems. Then write the three presets as real, useful starting sites in original words, with Diátaxis linked rather than copied. Then the CLI, the app services and the dialog.
>
> Do not edit `main.go`, `internal/app/menu.go` or the api files: give exact diffs for them. Do not launch the app.
>
> Done when the table's tests pass and `go vet ./internal/... && go test ./internal/...` and `cd frontend && npx tsc --noEmit && npx vitest run` are clean. Report:
> - the Integration diffs;
> - the file count of each preset;
> - anything unverified, such as the native folder panel and the look of the welcome cards.
