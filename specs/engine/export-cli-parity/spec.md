# engine/export-cli-parity

| | |
|---|---|
| Status | in progress |
| Target version | 0.7.0 |
| Branch | `feature/19-export` (asked while checking roadmap step 10, #19) |
| Wave | 45 |
| Depends on | engine/export-key, ui/export-pdf-settings, ui/site-topbar; the first slice of engine/export-theme (the scheme) |
| Owns | `internal/cli/**` + tests, `engine/config/**` (`export.theme.scheme`), `engine/site/**` (the scheme in the shell), `engine/session/buildui.go` (saving defaults) + tests, `apps/zenmark/{headless,exportpdf}.go` + tests, `apps/zenmark/frontend/src/shell/{buildsite,pdfform,exportform}.ts` + tests, new `apps/zenmark/export-options.json` (the parity table), `docs/BUILD.md`, the demo's Export page |

## Purpose

**CLI first.** Everything you can choose for an export in Settings must also
be a `zenmd export` flag, with the same name, the same default and the same
folder setting behind it. The app's forms are a view of the CLI, not the other
way round. That way scripts, CI and the app always make the same site and the
same PDF.

This also adds the first piece of export theming: a **light (white) or dark
(black) website**, in Settings and as a flag.

## Behaviour

1. **The scheme of an exported website.** `export.theme.scheme` takes:
   - `auto`, the default: the page follows the reader's system setting, as today;
   - `light`: the black-on-white look;
   - `dark`: the white-on-black look.

   A forced scheme sets `data-theme` on every page's `<html>`, which `tokens.css` already supports, and drops the `color-scheme` switch. The PDF is always light, whatever the scheme. This is behaviour 4 of engine/export-theme without the switch. That spec's later phases build on the same key.
2. **Settings › Export › Website** gets a **Theme** row: Auto, Light (white) or Dark (black), as a radio group made from the shared form pieces (`radioGroup`). It's saved by Save as folder defaults.
3. **One table of options.** `apps/zenmark/export-options.json` lists every export option once, for each export (`site`, `pdf`). Each entry gives:
   - its flag, such as `--drafts`, plus its negative flag if it has one, such as `--no-drafts`;
   - its config key, such as `export.drafts`;
   - its form field id;
   - its default;
   - its help line.

   The CLI help, the Go flag parser and both Settings forms are checked against it, so an option can't exist in one place only.
4. **Flags, after this change:**

   | `zenmd export site` | `zenmd export pdf` |
   |---|---|
   | `-o, --out <dir>` | `-o, --out <file>` |
   | `--hosted` / `--portable` | `--drafts` / `--no-drafts` |
   | `--drafts` / `--no-drafts` | `--section <page>` / `--all` |
   | `--clean` | `--cover` / `--no-cover` |
   | `--print` / `--no-print` | `--open` / `--no-open` |
   | `--scheme auto\|light\|dark` | |
   | `--save-defaults` | `--save-defaults` |

   - **Precedence:** a flag wins over the folder's `export:` settings, and those win over the global `config.yml` and then the built-in default. The app's forms work the same way: a choice made in the form wins for that run.
   - **`--open`:** the CLI opens the PDF with macOS `open` once it's written. The default is off on the command line and on in the app. The option table records the two defaults.
   - **`--save-defaults`:** writes the flags given on the command line into the folder's `.zenmd.yml` under `export:` or `export.pdf`, exactly as Save as folder defaults does (shared code, comments kept). It prints what changed, then exports. Used with `--dry-run`, it prints the change and writes nothing.
   - **Help:** the heading "Build options (export site, build)" becomes "Export site options". Each line comes from the option table.
5. **Folder defaults in the CLI.** `zenmd export site` and `zenmd export pdf` read `export:` and `export.pdf` from the folder and from `config.yml`, as the app does. They print the config notices too: a legacy `build:` key, or `.zen.yml`.
6. **The rule for later work.** CLAUDE.md gets a CLI-first rule: every new option of an export, a build or any folder operation is designed as a `zenmd` flag and a config key first, and the app's UI then calls the same engine code. Specs list the flag in their Interfaces section. The lead adds the rule.

## Interfaces

```json
// apps/zenmark/export-options.json (excerpt)
{ "site": [ { "flag": "--scheme", "values": ["auto","light","dark"], "key": "export.theme.scheme", "field": "scheme", "default": "auto", "help": "the website's colours: auto follows the reader's system, light is black on white, dark is white on black" } ],
  "pdf":  [ { "flag": "--open", "negative": "--no-open", "key": "export.pdf.open", "field": "open", "default": { "cli": false, "app": true }, "help": "open the PDF when it is written" } ] }
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | The scheme: config, flag, `data-theme` on every page, PDF always light | `engine/config`, `engine/site/shell_test.go`, `internal/cli` tests |
| 2 | Every option in the table is parsed by the CLI, with its negative, and is in the help | `internal/cli/options_test.go` (reads the JSON) |
| 3 | Every option in the table is a field in its Settings form, with the same default | `apps/zenmark/frontend/src/shell/options.test.ts` (reads the JSON) |
| 4 | Precedence: flag, then folder, then global, then default | `internal/cli` tests |
| 5 | `--save-defaults` writes like the app's Save as folder defaults; `--dry-run` writes nothing | `internal/cli` and `engine/session/buildui_test.go` |
| 6 | The CLI prints the config notices | `internal/cli` tests |
| 7 | The Theme radio in Settings › Export › Website, saved with the defaults | `buildsite.test.ts` |

## UI

UI consistency: the Theme row is a radio group made from the export form's shared pieces. Nothing else in the forms changes, apart from checking them against the option table.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| `--save-defaults` rewrites the folder's settings | Same shared code as the app's Save as folder defaults (keeps comments, shows the change); `--dry-run` to look first | #5 |

## Notes

- **Changelog** (added by the lead):
  - `Added: a light (white) or dark (black) website export: export.theme.scheme, Settings › Export › Website › Theme, and --scheme.`
  - `Added: every export choice in Settings is a zenmd export flag too (--no-drafts, --all, --cover, --open, --save-defaults, …), checked against one table of options.`
- **Integration (lead):** the CLAUDE.md CLI-first rule.

## Prompt

> Implement `specs/engine/export-cli-parity/spec.md` following CLAUDE.md, in `../zen-worktrees/issue-19` on `feature/19-export`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
