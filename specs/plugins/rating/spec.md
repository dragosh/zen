# plugins/rating

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/13-layouts` (added during roadmap step 4, #13) |
| Wave | 32 |
| Depends on | core/plugin-extras (ctx.storage), plugins/quiz (the pattern: Go render plus page script, results list) |
| Owns | `builtins/plugins/rating/**` (new), `runtime/test/builtin-rating.test.ts` (new), the registration lines for a new built-in (`builtins/builtins.go`, `kit/assembly/generate.go` and the catalog or tests that list built-in ids), `apps/zenmark/app-assembly.yml` + `assembly_gen.go` (add `rating`), `examples/demo/plugins/{index,rating}.md`, `examples/demo/layouts/course.md`, `engine/session/demo_test.go`, `docs/PLUGINS.md` (the built-in list only) |

## Purpose

A reader can say how useful a page was. A `:::rating` block asks one
question, such as "How was this course?", and the reader picks 1 to 5 stars.
The rating is kept on this Mac, like quiz scores, so a course or a guide can end
with it and the reader can see later what they rated.

## Behaviour

1. **Syntax.** `:::rating{id=course title="How was this course?"}` opens the block and `:::` closes it.
   - `id` names the stored rating. It is required: 1–59 letters, digits, `.`, `_` or `-`, unique in the page. A missing or bad id renders a page warning and the block's text.
   - `title` is the question. The default is "Rate this page".
   - `max` is the number of stars, 3 to 10, default 5.
   - `labels` is optional, written as `labels="Poor|Fair|Good|Very good|Excellent"`: one label per star, shown beside the stars for the hovered or picked value.
   - Any markdown inside the block shows under the question as its text.
2. **Rendering (Go).** The block renders a `fieldset`. Its `legend` is the title. It holds `max` radio inputs named after the id, each with a visible star and a visually hidden label ("3 of 5", or the star's label). A status line follows.
   - Without the page script, as in a built site with scripts off, it is a plain radio group that still reads well.
3. **The page script:**
   - **On mount:** it reads `rating-<id>` from `ctx.storage` and checks the saved star.
   - **Picking a star:** clicking a star or pressing Space saves `{ version: 1, value, max, date }`. The status reads "You rated 4 of 5 · 21 Nov 2026".
   - **Keyboard:** ←/→ and ↑/↓ move the choice, as in any radio group, and Home and End go to 1 and `max`.
   - **Clear:** a Clear button appears once there is a rating. It removes the rating by writing `null`, and the status returns to "Not rated yet".
   - **Bad stored values:** a stored value that isn't version 1, or is out of range, counts as not rated.
4. **Results.** A `:::rating-results` block lists every rating in this folder: the page's title, linked to the page, then the question, the stars and the date. With none, it says "No ratings yet." It uses `ctx.storage.keys("rating-")`. The rating script stores the page path and title with each rating, so the list needs no extra call.
5. **Look.**
   - Stars are drawn with text glyphs (☆ ★) in `--fg`, and a picked star is filled.
   - Focus shows the `--focus` outline.
   - Hover fills up to the hovered star, with no animation when `prefers-reduced-motion` is on.
   - Tokens only. The status line and the Clear button reuse quiz's status and small-button styles, so the plugin adds no new kind of control.
6. **Text only.** The title, labels and stored strings go in through `textContent`.
7. **In a course,** the block works inside a step like any other block. It doesn't gate the step, which only `:::check` does.

## Architecture

- **`builtins/plugins/rating/rating.go`:**
  - the manifest: id `rating`, blocks `rating` and `rating-results`, `Default: true`, help with three snippets (a rating, labels, the results);
  - the block hook renders the fieldset and validates the attributes;
  - the assets are `assets/rating.js` and `assets/rating.css`, embedded as quiz does.
- **Registration:** add `rating` everywhere a built-in plugin is listed (builtins, assembly catalog and generator, their tests), then add `rating` to `apps/zenmark/app-assembly.yml` and regenerate.
- **Demo:**
  - `examples/demo/plugins/rating.md`, with the syntax, the options, a live block and a results block;
  - a row in `examples/demo/plugins/index.md`;
  - a last step in `examples/demo/layouts/course.md` before "Done": "Rate this course", with `:::rating{id=learn-by-doing title="How was this course?" labels="Poor|Fair|Good|Very good|Excellent"}`;
  - `engine/session/demo_test.go` gets the new plugin page in the Plugins list and `rating` in the guard's block map.

## Interfaces

Storage key `rating-<id>`. Value: `{ version: 1, value: int, max: int, date: string, page: string, title: string, question: string }`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Attributes, defaults, bad id, max range, labels count | `builtins/plugins/rating/rating_test.go` |
| 2 | Rendered fieldset, radios and hidden labels; text escaped | `rating_test.go` |
| 3 | Mount reads, pick saves, keys move, Clear writes null, bad stored value | `runtime/test/builtin-rating.test.ts` |
| 4 | Results list from storage keys, empty state, links | `builtin-rating.test.ts` |
| 5 | Registered and in the assembly; demo page renders with no warnings; the course has the rating step | existing assembly and demo tests, updated |
| 6 | Hostile title or labels stay text | both test files |

## UI

UI consistency: the rating is a radio group with the quiz's status line and small button, so no new control style. It is keyboard reachable, has a visible focus, uses black and white tokens only, and respects reduced motion.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Page text or stored values injected into the page | `textContent` only; stored values validated | #6, #3 |

## Notes

- **Changelog** (added by the lead): `Added: the rating plugin, where a reader gives a page or a course 1 to 5 stars, kept on this Mac, with a list of every rating in the folder.`

## Prompt

> Implement `specs/plugins/rating/spec.md` following CLAUDE.md, in the existing worktree `../zen-worktrees/issue-13` on branch `feature/13-layouts`. Model it on `builtins/plugins/quiz`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
