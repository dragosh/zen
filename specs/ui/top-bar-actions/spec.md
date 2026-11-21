# ui/top-bar-actions

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/15-editor` (found while checking roadmap step 6, #15) |
| Wave | 34 |
| Depends on | ui/nav-panel (the sidebar footer group), ui/find, #25 (the Inspector), the editor feature |
| Owns | `kit/ui/src/shell/icons.ts` (new: the app's line icons) + test, `kit/ui/src/shell/{layout,dom}.ts` + tests (footer group, icons moved out), `kit/ui/src/page/index.ts` (the Edit button) + tests, `kit/ui/src/find/**` (the Find button's pressed state), `kit/ui/src/inspector/index.ts` (where its button lives) + tests, `kit/ui/src/styles/shell.css`, `apps/zenmark/frontend/src/shell/shell.ts` + tests, `docs/UI.md` (top bar, sidebar), `examples/demo/{intro/editor,intro/find,intro/inspector,layouts/course,configuration/settings}.md` |

## Purpose

The page's own actions sit together in the top bar as icons, the way the
sidebar footer has the cog: Find, Edit and the Inspector. The top bar now has
an "Edit" text button, while Find has no button at all and the Inspector sits
in the sidebar footer.

## Behaviour

1. **The top bar's actions**, right-aligned in the top bar of a page, in this order:

   | Button | Icon | Name and tooltip | Does | Shown with |
   |---|---|---|---|---|
   | Find | a magnifying glass | "Find (⌘F)" | opens the find bar, or closes it when open | `find` |
   | Edit | a pencil | "Edit (⌘E)" | opens the page in the editor, as the Edit button does today | `editor`, and a page that can be edited |
   | Print (#18) | a printer | "Print (⌘P)" | prints the page, as ⌘P and File › Print Page… do (the `zen:print-page` event to the shell's page print) | `print` |
   | Inspector | a window with a bottom panel showing a `>_` prompt | "Inspector (⌥⌘I)" | opens or closes the Inspector, as the footer button does today, with its red problem-count badge | `inspector` (or `developer` alone, as today) |

   Each button shows only with its feature, and a build without any of them has no action group.
2. **One look.** All icons use one set:
   - **Drawing:** 24×24 viewBox, drawn at 16px, stroke `currentColor`, width 1.5, no fill, round caps. That's the cog and export style.
   - **One module:** they live in `kit/ui/src/shell/icons.ts` with the existing cog and export icons, which move there from `layout.ts` and `dom.ts`.
   - **Buttons:** the top bar buttons are the same size and shape as the footer's icon buttons.
   - **On state:** a button that is on (the find bar open, the Inspector open) uses the footer's on style (`background: var(--fg); color: var(--bg)`), with `aria-pressed` for Find and `aria-expanded` for the Inspector.
   - **Text and focus:** there is no visible text, the name is the `aria-label`, the tooltip is the `title`, and the focus ring is as in the footer.
3. **The sidebar footer** keeps Settings and Export. The Inspector button leaves it.
4. **One group still.** Settings and the Inspector still never show together, so the group of ui/nav-panel stays. The Inspector's entry keeps its place in that group even though its button is in the top bar. Opening the Inspector closes Settings and Export, and opening Settings closes the Inspector, as today. The group's `addFooterButton` becomes a general `addViewButton(entry)`, with the button's place set apart from its group.
5. **Keys and menus are unchanged:** ⌘F, ⌘E, ⌥⌘I, ⌥⌘J and Edit › Find. A key that turns a button on or off updates its state.
6. **The editor window** is unchanged in this spec. Its header keeps its own buttons (Split, Done).
7. **Docs and demo:**
   - docs/UI.md: update the top bar sketch and the sidebar paragraph;
   - the demo pages that name these buttons say "the pencil in the top bar", "the magnifying glass", and "the Inspector button in the top bar".

## Architecture

- **`icons.ts`:** `icon(doc, name)` returns an SVG for `cog`, `export`, `search`, `pencil` and `panel`. The paths are made with DOM APIs, never innerHTML.
- **`layout.ts`:** owns the top bar action group (`setActions` or a `topActions` slot) and the view group.
- **The buttons move:** the page view's Edit button (`page/index.ts`) and the Inspector button (`inspector/index.ts`, which the shell adds) move into the top bar group. Find adds a button that toggles its bar.
- **The shell:** `shell.ts` wires the buttons, in Find, Edit, Inspector order.

## Interfaces

```ts
// kit/ui/src/shell/icons.ts
export type IconName = "cog" | "export" | "search" | "pencil" | "panel";
export function icon(doc: Document, name: IconName): SVGSVGElement;
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | The top bar has Find, Edit and Inspector in order, with icons, aria-labels and tooltips; each is missing without its feature; no group with none | `kit/ui/src/shell/layout.test.ts`, `apps/zenmark/frontend/src/shell/shell.test.ts` |
| 2 | One icon set: every icon from `icons.ts` has a 24 viewBox, 16px size, stroke only and aria-hidden | `kit/ui/src/shell/icons.test.ts` |
| 3 | The on state of Find (aria-pressed) and the Inspector (aria-expanded) follows the bar or panel, including via keys | `find` and `inspector` tests |
| 4 | Settings and the Inspector still exclude each other; the footer has only Settings and Export | `layout.test.ts` (the footer group tests, updated) |
| 5 | The badge moves with the Inspector button | the `inspector` tests |

## UI

UI consistency: the top bar actions reuse the sidebar footer's icon buttons, its on style and its group, with one icon set for the app.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| None new (UI only) | – | – |

## Notes

- **Changelog** (added by the lead): `Changed: Find, Edit and the Inspector are icons in the top bar (a magnifying glass, a pencil and a panel); the sidebar footer keeps Settings and Export.`

## Prompt

> Implement `specs/ui/top-bar-actions/spec.md` following CLAUDE.md, in the existing worktree `../zen-worktrees/issue-15` on branch `feature/15-editor`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
