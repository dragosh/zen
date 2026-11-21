# engine/export-key

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/19-export` (found while checking roadmap step 10, #19) |
| Wave | 41 |
| Depends on | engine/config-name (the legacy-name pattern), app/export |
| Owns | `engine/session/build.go`, `internal/cli/cli.go` (the defaults and help), `apps/zenmark/{headless,exportpdf}.go` + tests (the PDF default), `apps/zenmark/frontend/src/{shell/buildsite.ts,api/fake.ts}` + tests, `engine/config/**` + tests (the key), `engine/session/buildui.go` + tests (Save as folder defaults), every place that documents or uses the key: `docs/{BUILD,ENGINE,PLAN}.md`, `README.md`, `examples/showcase/{.zenmd.yml,build/index.md}`, `examples/demo/features/export.md`, `engine/session/demo_test.go` (the guard's key list), the Problems Fix text if one names `build:` |

## Purpose

The website export's folder settings live under a `build:` key in
`.zenmd.yml`. That's a leftover from when exporting was called building.
Everything else now says Export: the menu, Settings and the `zenmd export`
command. So the key becomes `export:`, the way `.zen.yml` became `.zenmd.yml`:
old folders keep working, and they're told how to move on.

## Behaviour

1. **`export:` is the key.** It has the same sub-keys as `build:` today (`out`, `id`, `editURL`, `drafts`, `portable`, `exclude` and any others), in both the folder's `.zenmd.yml` and the global `config.yml`, wherever `build:` is read today.
2. **`build:` is still read**, as an old name:
   - With only `build:`, its values are used, and the config has a notice: "`build:` in .zenmd.yml is now `export:`; rename the key". The notice shows under Problems as a note, the same way the `.zen.yml` notice does.
   - With both keys, `export:` wins, `build:` is ignored, and the notice says so.
   - The global `config.yml` gets the same notice, naming that file.
3. **Save as folder defaults** (Settings › Export and the website dialog) writes `export:`. If the file has a `build:` mapping, the saved values go under `export:`. The old mapping's other values move with them, so nothing is lost, and the `build:` key is removed. That's the only time Zenmark renames the key by itself. The preview that shows what will change shows this too.
4. **Docs and examples** say `export:` everywhere, apart from the one sentence in docs/BUILD.md that says `build:` is still read.
   - The demo's guard (`TestDemoMatchesAssembly`) allows `export` in `.zenmd.yml` with the export feature. It allows `build` never.
   - A grep test, like `TestNoLegacyNameOutsideNotices`, fails on a `build:` config example outside the legacy notes.
5. **Default output: `exports/html`** (was `_site`).
   - The website goes to `exports/html` in the folder when neither `-o`, `export.out` nor the dialog names another place. That covers the CLI default and its help text, the dialog's default, Settings › Export and the fake api.
   - The output folder is never read as pages: no sidebar entries, and no live reload from writing it, as `_site` is today. Check how `_site` is kept out and make `exports/` work the same way.
   - A folder whose config says `out: _site` keeps it, because only the default changes.
   - The docs and the demo's Export page say `exports/html`.
6. **The PDF's default: `exports/pdf/<folder name>.pdf`** (was `<folder name>.pdf` at the top of the folder).
   - This covers `zenmd export pdf` without `-o` and its help text, and the save panel of File › Export › PDF…, which starts in `exports/pdf` with that name.
   - The folder is created when needed.
   - `-o` and anything chosen in the panel win.
   - Like `exports/html`, it is never read as pages.
7. **No other "build" changes.** The `zenmd build` command alias, the app's Build tab and `app-assembly.yml` keep their names, because they mean something else or are aliases already.

## Interfaces

None new. The config keeps its fields; only the YAML key changes.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | `export:` read, in the folder and in global | `engine/config` tests |
| 2 | `build:` alone is read and noted; both: `export:` wins and noted | `engine/config` tests |
| 3 | Save as folder defaults writes `export:`, moves an existing `build:` mapping, keeps comments and other keys | `engine/session/buildui_test.go` |
| 4 | No `build:` config examples outside the legacy notes | a grep test in `engine/config` |
| 5 | The notice shows under Problems | the existing notice path test, extended |
| 7 | The PDF's default is exports/pdf/<name>.pdf (CLI and the save panel's start); -o wins | `apps/zenmark` headless and exportpdf tests, `internal/cli` tests |
| 6 | The default output is exports/html (CLI, dialog, Settings); a configured out wins; exports/ is never pages or a reload | `engine/session/build_test.go`, `internal/cli` tests, `buildsite.test.ts`, a workspace or watch test |

## UI

UI consistency: the notice uses the existing legacy-name Problems note, the same as the `.zen.yml` one.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Rewriting the user's file loses settings | Only on Save as folder defaults, which already shows a preview; values move, comments kept | #3 |

## Notes

- **Changelog** (added by the lead): `Changed: the website export settings in .zenmd.yml are under export: (was build:, still read, with a note under Problems).`

## Prompt

> Implement `specs/engine/export-key/spec.md` following CLAUDE.md, in `../zen-worktrees/issue-19` on `feature/19-export`. Model the old-name handling on engine/config-name (`.zen.yml` to `.zenmd.yml`). Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
