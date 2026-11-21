# ui/plugin-kit

| | |
|---|---|
| Status | in progress |
| Target version | 0.7.0 |
| Branch | `feature/13-layouts` (found while checking roadmap step 4, #13) |
| Wave | 33 |
| Depends on | plugins/rating, the built-in plugins and layouts |
| Owns | `runtime/src/css/plugin-kit.css` (new) and how the runtime ships it, `runtime/src/css/page.css` (only to drop rules the kit replaces), `builtins/plugins/{check,quiz,rating,sandbox,codeblock,diagram}/**` (styles and class names only), `builtins/layouts/course/**` (styles and class names only), their tests, `kit/ui/src/page/css.test.ts` or a new `runtime/test/plugin-kit.test.ts`, `docs/PLUGIN-UI.md` (new), `docs/PLUGINS.md` (a link to it) |

## Purpose

Every interactive block in a page should look like one app. Today each plugin
styles its own controls:
- quiz questions use the serif heading font while their answers use the UI font;
- quiz's Check button doesn't look like the sandbox's Run;
- a course's checklist box sits outside its frame;
- the course's step list doesn't align when a title wraps.

A shared kit of classes, and a style guide that every future plugin and layout
follows, fixes the pattern once.

## Behaviour

1. **The kit.** The runtime ships `plugin-kit.css` on every page, in the app and in built sites, with these classes. They use only the docs/UI.md tokens, the sandbox's current look is the reference, and every control has a `:focus-visible` ring.

   | Class | Is |
   |---|---|
   | `.zen-block` | the frame of an interactive block: `1px solid var(--rule)`, `var(--radius)`, `var(--bg-subtle)` background, `var(--font-ui)` at 14px, margin `1em 0` |
   | `.zen-block-head` | the block's title row: 14px UI font, weight 600, with an optional status on the right (`.zen-block-status`, `--fg-muted`, 13px) |
   | `.zen-block-body` | padding `10px 12px` |
   | `.zen-block-bar` | the action row, as the sandbox bar is now: flex, gap 8px, padding `6px 12px`, a top rule |
   | `.zen-btn` | a button: 13px UI font, padding `3px 12px`, no border, `var(--radius)`, `var(--btn-bg)`, hover `var(--btn-bg-hover)`, disabled at 0.45 opacity |
   | `.zen-btn-primary` | the main action (Run, Check, Next, Finish): `var(--btn-primary-bg)` and `var(--btn-primary-fg)` |
   | `.zen-btn-quiet` | a secondary text action (Clear, Reset): transparent, `--fg-muted`, hover background |
   | `.zen-choice` | an option row with a radio or checkbox: the input (16px, `accent-color: var(--fg)`) and its text on one baseline, in the 14px UI font, with the input inside the frame and a hanging indent so wrapped text lines up after it |
   | `.zen-field` | a text input or textarea: 14px UI font, `var(--bg)`, `1px solid var(--rule)`, `var(--radius)` |

2. **Text inside blocks.** Interactive blocks use one type style. Question titles, option text, labels and checklist items are all the UI font at 14px; titles are weight 600. The page's serif and heading styles stay for the page's own text and are no longer used inside blocks.
3. **Adopt the kit everywhere.** Each of these moves to the kit classes and drops its own copies of the same rules:
   - **sandbox:** frame, bar, Run as primary and Stop, the Check again note button;
   - **quiz:** frame, question titles as `.zen-block-head`-style text rather than serif legends, options as `.zen-choice`, the free-text field, Check as primary, the best-score line as the status;
   - **rating:** frame, title, Clear as quiet;
   - **check:** each item as `.zen-choice` inside the frame, with the box inside the border;
   - **course:** the step list, the step frame, Next, Back, Finish as primary and Reset as quiet.

   Each plugin keeps only the styles that are its own, such as the sandbox console or the rating stars.
4. **Course step list.** Each item is a grid: the mark (✓ or the number) in a fixed-width first column, the title in the second. A wrapped title starts under the title, not under the mark.
5. **No regressions.** Each plugin's existing tests still pass, and each test that checks a class name is updated. The demo pages render with no warnings.

## Style guide

`docs/PLUGIN-UI.md` is the rule for every future plugin and layout. It covers:
- **use the kit:** never restyle buttons, choices, fields or frames;
- **type:** the UI font at 14px inside blocks, and only the documented tokens;
- **states:** empty, loading, error, done, each with the same status style;
- **keyboard and focus;**
- **reduced motion;**
- **text only:** set text through `textContent`;
- **what a layout may change:** the page around blocks, never the blocks' controls;
- **checklist for review:** a short list the lead checks.

`docs/PLUGINS.md` links to it. CLAUDE.md points to it; the lead adds that line.

## Architecture

- **The runtime:** find where `page.css` is bundled and shipped, in the app and in `zenmd build` output, and ship `plugin-kit.css` the same way, so a plugin doesn't need to list it.
- **Plugins:** the plugin scripts that build controls (quiz, rating, sandbox, course) add the kit classes next to their own.

## Interfaces

The class names in Behaviour 1 are a public contract for plugin authors, documented in `docs/PLUGIN-UI.md`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Kit CSS exists, has every class, uses tokens only, and has a focus-visible rule for `.zen-btn`, `.zen-choice input` and `.zen-field` | `runtime/test/plugin-kit.test.ts` (reads the CSS, no DOM) |
| 2 | Shipped on every page (app and build) | the runtime asset test and the build golden |
| 3 | Each plugin's controls carry the kit classes: quiz's Check is `.zen-btn.zen-btn-primary`, rating's Clear is `.zen-btn-quiet`, course's Next is primary, check items are `.zen-choice` | the plugin runtime tests |
| 4 | Plugins no longer define their own button or choice rules; no serif font inside blocks | a CSS test over `builtins/**/*.css` |

## UI

This spec defines the pattern: UI consistency for every interactive block.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| None new (styles and class names only) | – | – |

## Notes

- **Changelog** (added by the lead): `Changed: every interactive block (quiz, rating, checklists, sandbox, course) uses one set of buttons, choices and text styles, documented in docs/PLUGIN-UI.md for future plugins and layouts.`
- **Integration** (lead): the CLAUDE.md line pointing to docs/PLUGIN-UI.md.

## Prompt

> Implement `specs/ui/plugin-kit/spec.md` following CLAUDE.md, in the existing worktree `../zen-worktrees/issue-13` on branch `feature/13-layouts`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
