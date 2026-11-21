# ui/export-pdf-settings

| | |
|---|---|
| Status | in progress |
| Target version | 0.7.0 |
| Branch | `feature/19-export` (found while checking roadmap step 10, #19) |
| Wave | 42 |
| Depends on | engine/export-key (`export:`, exports/pdf), app/export, ui/print |
| Owns | `apps/zenmark/frontend/src/shell/{settings,buildsite}.ts` + new `pdfform.ts` + tests, `kit/ui/src/print/preview.ts` + tests (taking the options), `engine/config/**` (the `export.pdf` mapping), `engine/session/buildui.go` (Save as folder defaults for PDF) + tests, `apps/zenmark/{exportpdf,print,headless}.go` + tests (the options reach the export), `internal/cli/cli.go` (`--drafts` and `--section` for `export pdf`, if missing), the docs and demo text about the PDF export |

## Purpose

Settings › Export › Website is a full form: where the site goes, the choices
and a button. Settings › Export › PDF is only a button that opens the preview.
The PDF gets the same kind of form, so both exports are set up and run the
same way, and a folder can keep its PDF choices too.

## Behaviour

1. **Settings › Export › PDF** shows a form in the right column, built from the same parts as the Website form (`createBuildForm` and its field, checkbox and button helpers, which are shared rather than copied):

   | Field | Default | Does |
   |---|---|---|
   | Output file | `exports/pdf/<folder name>.pdf`, shown folder-relative like the website's out | a path field with **Choose…**, which opens a save panel |
   | Include drafts | off | adds pages with `draft: true` |
   | Pages | All | **All** or **This section**, meaning the page that was open and the pages under it. This section is disabled when no page is open. |
   | Cover and contents | on | the cover sheet and the table of contents before the pages |
   | Open when done | on | opens the PDF in the default viewer after writing it |

   - **Actions:**
     - **Export PDF** (primary) writes the file straight away, with no print panel, and shows progress and the result: the file name, **Show in Finder** and **Open**.
     - **Preview…** opens today's print preview with these options set.
     - **Save as folder defaults** writes the choices into `.zenmd.yml` under `export: pdf:`, as the Website form writes `export:`. It shows a preview of the change first.
   - **States:** with no folder open, the form is disabled and the left column says "Open a folder to export it.", as the Website form does. A failure shows in the form's status line.
2. **Folder defaults:** `export.pdf` in `.zenmd.yml` or `config.yml` holds `out`, `drafts`, `cover` and `open`. The form, File › Export › PDF…, the preview and `zenmd export pdf` all start from them. A choice made in the dialog wins for that run.
3. **File › Export › PDF…** (⇧⌘P) still opens the preview, now with the folder defaults. The preview's own Include drafts and Pages stay, and its Export PDF… saves to the output file. A different place is chosen through the form's Choose… or the save panel.
4. **CLI:** `zenmd export pdf` reads `export.pdf` and takes `--drafts`, `--section <page>` and `--no-cover`. `-o` wins over `export.pdf.out`.
5. **Consistency:** the two forms look and read the same: the same field order (where, then the choices), the same button style and order (primary first), the same status line and the same Save as folder defaults. Any piece both use moves into the shared form code.

## Interfaces

```yaml
# .zenmd.yml
export:
  out: exports/html
  pdf:
    out: exports/pdf/guide.pdf
    drafts: false
    cover: true
    open: true
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | The PDF form's fields, defaults, This section disabled without a page, disabled without a folder | `pdfform.test.ts`, `settings.test.ts` |
| 2 | Export PDF writes to the chosen file with the options; Preview opens with them | `pdfform.test.ts` (fake api), `apps/zenmark` exportpdf tests |
| 3 | Save as folder defaults writes `export.pdf` and keeps the rest of `export:` | `engine/session/buildui_test.go` |
| 4 | `export.pdf` is read by config, the preview and the CLI; flags win | `engine/config`, `internal/cli` and headless tests |
| 5 | Both forms share the helpers: same field order and button classes | `settings.test.ts` (compares the two forms' structure) |

## UI

UI consistency: the PDF form is the Website form's pattern, in the Settings frame's right column. The primary button comes first, the status line sits under the buttons, and Save as folder defaults sits last.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Writing outside where the user chose | Only to the output file shown, or a save-panel choice; a folder-relative default never escapes the folder | #2 |

## Notes

- **Changelog** (added by the lead): `Added: Settings › Export › PDF is a form like the website's (output file, drafts, pages, cover, open when done, Save as folder defaults), saved under export.pdf in .zenmd.yml.`

## Prompt

> Implement `specs/ui/export-pdf-settings/spec.md` following CLAUDE.md, in `../zen-worktrees/issue-19` on `feature/19-export`. Reuse the Website form's pieces (move them into shared code rather than copy). Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
