# app/demo-guide

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/12-builtin-plugins` (roadmap step 3, #12) |
| Wave | 31 |
| Depends on | kit/shell-features (the starting shell), #12 (codeblock, diagram, check) |
| Owns | `examples/**`, the tests and goldens that read the demo (`engine/session/{demo,golden,build,example}_test.go`, `engine/session/doc.go`, `engine/session/testdata/**`, `engine/render/dialects_golden_test.go` + `engine/render/testdata/**`, `builtins/plugins/quiz/demo_test.go`, `internal/cli/docs_test.go`, `runtime/test/demo-progress.test.ts`), path mentions in `README.md`, `docs/{BUILD,ENGINE,LAYOUTS}.md`, `.claude/skills/add-issue/SKILL.md` |

## Purpose

`examples/demo` is the folder the user opens in Zenmark to check each roadmap
step. It shows only what the app's build has today, in a fixed order of
sections, and grows one step at a time. The complete feature tour moves to
`examples/showcase`, which the engine tests keep using.

## Behaviour

1. **The showcase.** Today's `examples/demo` moves, unchanged, to `examples/showcase` with `git mv`, so its history follows.
   - Every test, golden, doc and skill that pointed at `examples/demo` for the full tour points at `examples/showcase`.
   - The goldens' contents don't change apart from paths. If one does change, say why.
   - Specs keep their old text, because they are history.
2. **The new demo** lives in `examples/demo`. It uses only what `apps/zenmark/app-assembly.yml` has today:
   - features `reader`, `nav`, `inspector` and `developer`;
   - plugins `codeblock`, `diagram` and `check`;
   - layout `article`;
   - no dialects, add-ons or templates.

   Its sidebar has sections in this order. A section appears only once it has content; for now there are four:
   1. **Intro**, which is `README.md` and its pages:
      - what Zenmark is;
      - opening a folder;
      - the sidebar, links and breadcrumbs (with the deliberate broken link and the hidden or unlinked notes, as today);
      - frontmatter;
      - the Inspector (Page info and Problems) and the Developer panel (Console, Build), and how to turn it on;
      - keyboard shortcuts, listing only the keys the shell has;
      - the markdown showcase (CommonMark and GFM).
   2. **Plugins**: an index with the table of the built-in plugins in this build, then one how-to page each, for codeblock, diagram and check.
   3. **Layouts**: an index plus the `article` page.
   4. **Configuration**: the settings page (Settings sections as they are today) and the `.zenmd.yml` reference, limited to keys that do something in this build.

   Later roadmap steps add Dialects, Add-ons and Templates in the order Intro, Features, Dialects, Add-ons, Plugins, Layouts, Templates, Configuration. The README's table of contents follows that order.

   **Features (added on 2026-11-21, after #18).** The app's features have their own section, after Intro.
   - `features/index.md` lists the features this build has, in assembly order, with one line each.
   - Under it there is one page per feature:
     - `reader`: reading a page, live reload and the markdown showcase;
     - `nav`: the sidebar, links and breadcrumbs;
     - `find`;
     - `editor`;
     - `inspector`: Page Info and Problems;
     - `print`;
     - `developer`: the Console and Build tabs, and the switch.
   - A feature that isn't in the build has no page.
   - Intro keeps what isn't a feature: what Zenmark is, opening a folder, frontmatter and the keyboard shortcuts.
   - The demo test checks that the Features pages match the assembly's `features`.
3. **Nothing missing is named as present.** No page links to, uses or describes as available a plugin, layout, dialect, add-on, template or feature the build lacks. That covers the editor, find, print, export, newsite, Vale, MyST, quiz, sandbox, layouts other than article, and the `.zen/` example plugins and layouts. Those stay in the showcase.
   - `.zenmd.yml` in the demo sets only keys this build uses: `version`, `logo` (commented, as today), and `plugins` for codeblock and diagram.
   - There are no `layouts:` by path, no `build:` and no `markdown.dialects`.
4. **The demo opens clean.** Every page renders with no warnings (no unknown layout, no dialect warning). The only broken link is the deliberate one on the navigation page. Every page is reachable from the sidebar.
5. **A guard for later steps.** A test reads `apps/zenmark/app-assembly.yml` and fails when a demo page uses:
   - a plugin block outside the assembly's `plugins`;
   - a `layout:` outside its `layouts`;
   - a dialect it doesn't list.

   That way the demo can't get ahead of the build.
6. **Writing style:** short, plain pages, one feature each, each showing the feature as it explains it, as the showcase does today. Reuse the showcase's wording where it still fits.

## Architecture

- `git mv examples/demo examples/showcase`, then build `examples/demo` new. Copy the pages that still fit, cut to this build.
- `engine/session/demo_test.go` splits in two:
  - `TestShowcaseFolder`: the old checks, on the showcase;
  - `TestDemoFolder`: the new sidebar shape, no warnings, and the assembly guard. Parse the assembly with `kit/assembly` if that's possible without an import cycle; otherwise read the YAML.
- The golden for `TestDemoBuildUnchanged` stays on the showcase. Rename it if the name says demo, and keep a copy of the test name in the CLAUDE.md flaky list only if it was there.

## Interfaces

None.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Showcase moved; every old demo test passes on it | existing tests, paths changed |
| 2 | The new demo's sidebar is Intro, Plugins, Layouts, Configuration in that order, with the pages listed | `engine/session/demo_test.go: TestDemoFolder` |
| 3 | Nothing outside the build | `TestDemoMatchesAssembly` |
| 4 | No warnings; one deliberate broken link | `TestDemoFolder` |

## UI

None. The demo is content, rendered by the existing reader.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| None new: content only | – | – |

## Notes

- **Integration:** none. The lead adds the changelog line `Changed: examples/demo is the Zenmark guide for what the app has today and grows with each roadmap step; the full tour is examples/showcase.`
- The memory rule "demo follows roadmap" applies from now on: each roadmap step adds its pages to `examples/demo` and widens the guard.

## Prompt

> Implement `specs/app/demo-guide/spec.md` following CLAUDE.md, in the existing worktree `../zen-worktrees/issue-12` on branch `feature/12-builtin-plugins`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
