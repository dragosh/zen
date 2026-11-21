# plugins/myst-builtins

| | |
|---|---|
| Status | planned |
| Target version | 0.8.0 |
| Branch | one per phase: `feature/myst-<phase>` (phases below) |
| Wave | 36–40, one phase per wave |
| Depends on | engine/dialects, builtins/dialects/myst, ui/plugin-kit, ui/reader-settings |
| Owns | per phase (see each phase): new `builtins/plugins/<id>/**`, the registration lines for new built-ins, `builtins/dialects/myst/**` (roles and the non-directive syntax), `engine/dialects/**` (the inline scan only), the demo's MyST pages and the showcase's dialect page |

## Purpose

A folder written for MyST, Jupyter Book or Sphinx should read in Zenmark the
way it reads there, without being rewritten. The dialect already turns
```` ```{name} ```` and `:::{name}` into the container plugin that owns
`name`, but Zenmark has built-ins for only a few of those names. So a note, a
figure, a tab set or a `{kbd}` role comes out as a warning and its raw text.

This spec plans a built-in plugin for every common MyST directive and role,
grouped into small built-ins, with the rest of MyST's syntax in the dialect.
Each built-in also works without MyST, as a `:::name` container or a
`:name[…]` inline in plain Zenmark markdown, so nothing is MyST-only.

## Rules for every phase

1. **One owner per name.** A MyST name maps to exactly one built-in, by declaring it in that built-in's `Blocks` or `Inlines`. The dialect stays a pure syntax layer: it never renders anything itself.
2. **Options follow MyST.**
   - Every MyST option a directive accepts is read, through `:key: value` lines or a YAML block.
   - `:class:` and `:name:` work on every directive: `:name:` gives the block an `id` that links and `{ref}` can point to.
   - An option the built-in doesn't support shows a page warning and is otherwise ignored. It is never an error.
3. **The look comes from the plugin kit** (docs/PLUGIN-UI.md): `.zen-block`, the UI font at 14px, tokens only, no new kind of button, choice or field. Callout colours use the lint tokens (`--lint-error`, `--lint-warning`, `--lint-suggestion`) for the left rule only, so pages stay black and white.
4. **Rendered in Go** wherever possible, with output that a page without scripts can read. Scripts only add behaviour, such as tabs or dropdowns opening by keyboard.
5. **Safe:**
   - nothing runs code: `{code-cell}` and `{eval}` stay code;
   - nothing reads outside the folder: `{include}` and `{literalinclude}` read workspace-relative paths only, refuse symlinks out, and stop at 1 MB;
   - raw HTML from `{raw}` is never passed through.
6. **Part of the build.** Each new built-in is listed in the assembly catalog. Zenmark gets it once its phase is checked in the app. The demo's MyST page shows each one live, and `TestDemoMatchesAssembly` keeps the demo honest.
7. **Tests per built-in:**
   - the MyST and `:::` forms render the same;
   - each option works, and unknown options warn;
   - hostile text stays text;
   - the no-script output reads well;
   - the kit classes are present.

## Phases

### Phase 1: callouts, built-in `callout` (wave 36)

| MyST | Renders |
|---|---|
| `note`, `tip`, `hint`, `important`, `seealso` | a callout with the kind's title ("Note", "Tip"…) and its own one-letter icon, with a left rule in `--lint-suggestion` |
| `warning`, `caution`, `attention` | the same, with the rule in `--lint-warning` |
| `danger`, `error` | the same, with the rule in `--lint-error` |
| `admonition` | a callout with the title given as the argument (required; a missing one warns and uses "Note") |
| `versionadded`, `versionchanged`, `deprecated` | a one-line callout, "New in version X", "Changed in version X" or "Deprecated since version X", then the body |

**Options:**
- `:class: dropdown` makes the callout collapsed, a `details` element that opens by click or Enter;
- `:open:` starts it open;
- a title argument replaces the kind's default title.

### Phase 2: figures and tables, built-ins `figure` and `table` (wave 37)

| MyST | Built-in | Renders |
|---|---|---|
| `figure` | `figure` | `figure` > `img` + `figcaption`, with the body as the caption and any further paragraphs as the legend; `:alt:`, `:width:`, `:height:`, `:align:` (left, center, right), `:name:`, `:target:` |
| `image` | `figure` | an `img` with the same options and no caption |
| `table` | `table` | a titled table: the argument is the caption over the body's markdown table |
| `list-table` | `table` | a table from a two-level list; `:header-rows:`, `:stub-columns:`, `:widths:`, `:align:` |
| `csv-table` | `table` | a table from CSV in the body, or `:file:` in the folder; `:header:`, `:header-rows:`, `:delim:`, `:widths:` |

- Images resolve as page images do (folder-relative, missing ones listed in Problems).
- Numbering (Figure 1, Table 1) is per page, and `:name:` targets show the number.

### Phase 3: math, built-in `math` (wave 38)

| MyST | Renders |
|---|---|
| `math` directive, and `$$ … $$` blocks (dialect syntax) | display math; `:label:` numbers it and makes a target |
| `{math}` role, and `$ … $` inline (dialect syntax, only when the folder turns on MyST's `dollarmath`) | inline math |
| `{eq}` role | a link to a labelled equation, showing its number |

**Rendering: KaTeX, vendored** (decided 2026-11-21).
- **Where it lives:** the files go in `builtins/plugins/math/assets/`, as mermaid's do: `katex.min.js`, `katex.min.css` and the fonts. A `VENDOR.md` records the version, the source and the licence (MIT).
- **When it loads:** only on pages that have math, through `ctx.assetUrl`, the way the diagram plugin loads mermaid.
- **How it renders:** in the page, with `throwOnError: false`, `trust: false` and `strict: "warn"`. An error shows the TeX with the message under it.
- **Before KaTeX loads,** and in a page without scripts, the TeX shows as monospace text in a `.zen-block`.
- **Fetching it:** the lead downloads KaTeX once, with the user's go-ahead, before the subagent starts. Subagents never download.

### Phase 4: interactive layout blocks (sphinx-design), built-ins `tabs`, `dropdown` and `cards` (wave 39)

| MyST | Built-in | Renders |
|---|---|---|
| `tab-set` > `tab-item` (title argument) | `tabs` | a tab list (ARIA tabs: ←/→, Home/End) with one panel per item; `:sync:` keeps tabs with the same key in step across the page, and the last choice is remembered per folder |
| `dropdown` (title argument) | `dropdown` | a `details`/`summary` in the kit frame; `:open:`, `:color:` (ignored, black and white), `:icon:` (ignored) |
| `card` (title argument) | `cards` | a kit-framed card with header, body and footer (`^^^` and `+++` separators); `:link:` makes the whole card a link |
| `grid` > `grid-item` / `grid-item-card` | `cards` | a responsive grid; the column count argument as "1 2 3 4" is respected per width |
| `button-link`, `button-ref` | `cards` | a kit button that is a link |
| `{bdg-…}` roles | `cards` | a small text badge in the kit's muted style, whatever its colour |

Without scripts, every tab panel shows in order under its title, and a dropdown is a plain `details`.

### Phase 5: roles and the rest of MyST's syntax (wave 40)

**The dialect gains `ScanInline`** for roles written as `` {name}`content` ``. Each role maps to the inline plugin that owns `name`, the same way directives map to blocks. A new built-in, `text`, owns the small roles:

| Role | Renders |
|---|---|
| `{kbd}` | `<kbd>`, split on `+` and `-` |
| `{abbr}` | `<abbr title>`, written as `` {abbr}`HTML (HyperText Markup Language)` `` |
| `{sub}`, `{sup}` | subscript and superscript |
| `{ref}`, `{numref}` | a link to a `(label)=` target or a `:name:`, showing its text or number; an unknown label warns |
| `{doc}` | a link to another page of the folder, showing its title |
| `{term}` | a link to a `glossary` entry |
| `{download}` | a link to a folder file, opened in Finder in the app |

**Directives:**
- `glossary`: a definition list, each term a target;
- `epigraph`, `pull-quote`, `highlights`: quotes styled by the kit;
- `sidebar`, `margin`, `topic`: an aside in the kit frame, floated right when there is room;
- `toctree`: a list of links to its entries, with their titles; it does not change the sidebar;
- `include` and `literalinclude`: the folder file's markdown, or its text as code, with `:start-line:`, `:end-line:`, `:lines:` and `:language:`;
- `mermaid`: the existing `diagram` built-in, declared as one more block name.

**Non-directive syntax, in the dialect:**
- `(label)=` targets before a heading or block;
- `%` comment lines, dropped;
- `+++` block breaks, a thin rule;
- `{{ key }}` substitutions from frontmatter `substitutions:`;
- definition lists (`term` then `: definition`);
- field lists at the top of a page, shown like frontmatter in Page Info.

**Left as code, with one page warning each:** `code-cell`, `eval`, `raw`, `only`, `ifconfig`, and any directive no built-in owns. The warning names the directive and says it isn't supported.

## Interfaces

```go
// engine/dialects
type InlineScanner interface { // optional, phase 5
	ScanInline(src []byte) []plugins.InlineSpan
}
```

Each new built-in declares its MyST names in `Blocks` or `Inlines`. The help snippets show the `:::` form and the MyST form.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Every name in the phase tables renders through its built-in, in both MyST forms and as `:::` | each built-in's `*_test.go`, plus `builtins/dialects/myst/coverage_test.go` (a table of every MyST name in this spec mapped to its built-in id, checked against the registry) |
| 2 | Unsupported options warn; unsupported directives stay code with a warning | the same |
| 3 | Include confinement: no `..`, no absolute path, no symlink out, size limit | `include_test.go` |
| 4 | No raw HTML, no code runs | `myst_test.go` security cases |
| 5 | Kit classes, keyboard behaviour (tabs, dropdowns) | the runtime tests per built-in |
| 6 | The demo's MyST page uses each built-in live and passes the assembly guard | `engine/session/demo_test.go` |

## UI

UI consistency: every block is a plugin-kit block (docs/PLUGIN-UI.md).
Callouts, cards, dropdowns and tabs share the frame, type and buttons, and
colour appears only as a callout's left rule, from the lint tokens.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| `include` reads outside the folder | Workspace-relative only, symlinks refused, 1 MB cap | #3 |
| Raw HTML or code execution | `raw` and `eval` left as code; output sanitised as every page | #4 |
| Hostile titles or options | Text only; options parsed, never interpolated into HTML | each built-in's hostile-text test |

## Notes

- **Decided:** math is rendered by vendored KaTeX (phase 3).
- **Open:** whether the new built-ins join Zenmark's `plugins:` list one phase at a time, as in roadmap step 3. The plan is yes.
- **Out of scope:** Jupyter notebooks (`.ipynb`); executing `code-cell`; Sphinx cross-project references (`intersphinx`); bibliographies (`{cite}`); `myst.yml` project settings beyond detection.
- **Changelog per phase,** for example: `Added: MyST callouts (note, tip, warning, …, admonition, versionadded) as the callout built-in.`

## Prompt

> Implement phase N of `specs/plugins/myst-builtins/spec.md` following CLAUDE.md and docs/PLUGIN-UI.md, in `../zen-worktrees/myst-<phase>` on `feature/myst-<phase>`. Model each built-in on `builtins/plugins/quiz` (Go render, kit styles, runtime tests). Tests first, committed failing. Only that phase's paths. Do not launch the app, run task package/install/release, push or touch main.
