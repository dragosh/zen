# ui/reader-settings

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/22-dialects` (roadmap step 13, #22) |
| Wave | 35 |
| Depends on | engine/dialects, ui/settings-layout, app/demo-guide (the Features section) |
| Owns | `apps/zenmark/app-assembly.yml` + `assembly_gen.go` (`dialects: all`), `apps/zenmark/frontend/src/shell/settings.ts` + tests (the Markdown section), `apps/zenmark/dialects.go` (comment), `kit/ui/src/host.ts` (comments), `kit/ui/src/inspector/problems.ts` + test (the Fix text), wherever the app opens Settings on the Markdown section (the MyST project hint) + tests, `docs/UI.md` (Settings paragraph), `examples/demo/**`, `engine/session/demo_test.go` |

## Purpose

The reader is the feature that reads markdown, so its settings are where a
folder chooses its syntaxes. Settings › Markdown becomes **Settings › Reader**,
and the demo guide's Reader page holds the markdown pages. Turning on the
build's dialects (MyST), roadmap step 13, gives that section its first dialect.

## Behaviour

1. **The build:** `dialects: all` in `apps/zenmark/app-assembly.yml`, regenerated.
2. **Settings › Reader** replaces Settings › Markdown, in the same place in the section list. It is shown with the `reader` feature.
   - **The left column:**
     - a small heading, "Markdown", over the items;
     - "CommonMark and GFM", with the note "Default";
     - each dialect of the build, with the note On or Off.
   - **Opening the section:** "CommonMark and GFM" is selected when nothing has been picked, so the section never opens on an empty right column.
   - **The right column:** each item shows what it shows today (the summary, and for a dialect its switch, sample and preview).
   - **Other names:**
     - the section id becomes `reader`;
     - every place that opens Settings on that section opens `reader`, including the MyST project hint's button and the Problems Fix text for "dialect … is not in this build", which now reads "Settings › Reader, or remove it from .zenmd.yml";
     - code comments that say "Settings › Markdown" say "Settings › Reader".
   - **Look:** the heading uses the Settings frame's existing style, if there is one for groups. Otherwise it is the `--fg-muted` small text the frame uses for notes, with no new style.
3. **The demo, Features › Reader:**
   - The reader page has a "Markdown" part that links two sub-pages:
     - `features/markdown.md`, titled just "Markdown", never "markdown showcase": every place in the demo that says "markdown showcase" says "Markdown". It opens by saying that CommonMark and GFM are the default and are always on;
     - `features/myst.md`, "MyST": how to turn it on (Settings › Reader, or `markdown: dialects: [myst]` in `.zenmd.yml`), the fence and colon forms, and options, with live examples.
   - The demo's `.zenmd.yml` turns MyST on, so the live examples render.
   - Use only plugins this build has, such as notes written as quiz or check blocks. If MyST needs a block the build lacks, show it as a code sample, not live.
   - The Settings page in Configuration says Reader in place of Markdown.
   - The demo test expects the new reader sub-pages. `TestDemoMatchesAssembly` passes, because MyST and dialects are now in the build.
4. **No new section for Dialects** in the demo: the dialects live under Reader. The section order becomes Intro, Features, Add-ons, Plugins, Layouts, Templates, Configuration.

## Interfaces

None new. The Settings section id changes from `markdown` to `reader`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | The assembly has every dialect | `TestAssemblyGenerated`, the assembly tests |
| 2 | The Reader section: label, the Markdown heading, CommonMark and GFM first with "Default", selected when nothing is picked, dialects with On or Off; no "Markdown" section; hidden without `reader` | `apps/zenmark/frontend/src/shell/settings.test.ts` |
| 3 | The hint and Problems Fix open or name Settings › Reader | their tests |
| 4 | The demo's reader sub-pages render with no warnings; a MyST fence renders as its container | `engine/session/demo_test.go` |

## UI

UI consistency: the Settings frame, with items on the left and details on the right. The heading reuses an existing style, and a default item is selected, as in the other sections.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| None new | – | – |

## Notes

- **Changelog** (added by the lead):
  - `Added: MyST markdown is back in Zenmark (Settings › Reader, or markdown.dialects in .zenmd.yml) (#22).`
  - `Changed: Settings › Markdown is now Settings › Reader, with CommonMark and GFM as the default and the dialects under it.`

## Prompt

> Implement `specs/ui/reader-settings/spec.md` following CLAUDE.md and docs/PLUGIN-UI.md, in the worktree `../zen-worktrees/issue-22` on branch `feature/22-dialects`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
