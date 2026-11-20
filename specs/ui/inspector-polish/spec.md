# ui/inspector-polish

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/ui-inspector-polish` |
| Wave | 28 |
| Depends on | #25 (Inspector, panel), ui/settings-layout (About) |
| Owns | `kit/ui/src/panel/**`, `kit/ui/src/inspector/**`, `kit/ui/src/page/index.ts` (the warnings bar) + `page/*.test.ts`, `kit/ui/src/shell/about.ts` + `about.test.ts` (removed), `kit/ui/src/index.ts`, `apps/zenmark/menu.go` + `menu_test.go`, `apps/zenmark/app.go`, `apps/zenmark/frontend/src/shell/shell.ts` + `shell.test.ts`, `apps/zenmark/frontend/src/api/{index,wails,fake}.ts` + tests (`menu:about` removed) |

## Purpose

The Inspector after the first round of testing. Its columns can be resized, its
padding is the same in every tab, and every warning about the page is in one
place, Problems. The About sheet goes away. The macOS About panel comes back,
and what the build contains moves into the Inspector next to the Console.

## Behaviour

1. **About is the standard macOS panel again.** Zenmark › About Zenmark uses `Role: "About"`. The `menu:about` event, `openAbout` and the About sheet are removed.
2. **Build tab.** The Inspector gets a **Build** tab after Console, shown under the same rule as Console: the `developer` feature and the "Show Console in the Inspector" switch.
   - Left column: the version and the features list, with the missing features greyed as today.
   - Right column: the add-ons, plugins, layouts, templates, dialects and theme.
   - It reuses `buildSection`'s content.
3. **Resizable columns.** In every two-column tab there is a vertical separator between the columns that can be dragged.
   - It is `role="separator"` with `aria-orientation="vertical"` and `aria-valuenow`, and it is focusable: ←/→ move it 5%, and Home and End go to the limits.
   - The left column keeps 25%–75% of the width.
   - The split is remembered per machine, one value per tab (`zen.panel.cols.<tab>`).
   - Each column scrolls on its own, and long text wraps (`overflow-wrap: anywhere`) instead of being cut.
   - Below 700 px the columns stack and the separator is hidden, as today.
4. **Consistent padding.** Every tab and both columns use the same padding (`--space-3` horizontal, `--space-2` vertical) and the same gap between rows. The header tools line up with the tab labels. A test reads `inspector.css` and `panel.css` and checks that there is one padding rule for `.zen-panel-col`, with no per-tab overrides.
5. **Page warnings in Problems.** The render warnings of the page (`Page.warnings`, for example "dialect myst is not in this build" or `unknown layout "slides"`) are Problems items:
   - **Kind:** `warning`, under the **Checks** filter.
   - **Right column:** the message, plus "Fix" help where one is known. For a dialect: "Settings › Markdown, or remove it from `.zen.yml`". For a layout: "Settings › Layouts, or change `layout:` in the front matter".
   - **Count:** they add to the count chip and the Inspector button's badge.
   - **The warnings bar:** when the build has `inspector`, the bar above the page (`zen-page-warnings`) no longer shows warnings. Without `inspector` it stays as today, so the warnings are never lost.
   - **The dialect hint:** it is a suggestion, not a warning, so it keeps its own bar.

## Architecture

- **`kit/ui/src/panel`:** `panelColumns(o?: { id: string })` adds the separator and the stored split. The drag logic is the sidebar resizer's, made generic or copied into the panel.
- **`kit/ui/src/inspector`:**
  - `problems.ts` gains the `warning` kind, fed from the `Page` the page feature already announces. Use the existing INSPECTOR events, or extend the state event with the page's warnings.
  - The Build tab is in `index.ts`.
- **`kit/ui/src/page/index.ts`:** it hides warnings in the bar when `hasFeature(host, "inspector")`, and still announces them to the Inspector.
- **Go:** `apps/zenmark/menu.go` puts back `Role: "About"`. `app.go` loses `About: showAndEmit(…)`; `EventAbout` and `menuActions.About` are removed.
- **`apps/zenmark/frontend/src/shell/shell.ts`:** loses the `menu:about` wiring and passes nothing new.

## Interfaces

```ts
// kit/ui/src/panel/index.ts
export function panelColumns(o?: { id?: string; doc?: Document }): { el: HTMLElement; left: HTMLElement; right: HTMLElement };
// kit/ui/src/inspector/problems.ts
export type ProblemKind = "check" | "spelling" | "warning";
```

`menu:about` is removed from `Api.on`, and `openAbout` from `@zenmd/kit`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | About is the role again; no `menu:about` | `apps/zenmark/menu_test.go: TestAboutMenu` (changed); `shell.test.ts` (the About tests removed or replaced) |
| 2 | The Build tab follows the Console's rule, and shows the version and the greyed features | `kit/ui/src/inspector/build.test.ts` (new) |
| 3 | Separator drag and keys, the 25–75% limits, stored per tab, hidden when stacked | `kit/ui/src/panel/panel.test.ts: "resizable columns"` |
| 4 | One padding rule | `kit/ui/src/inspector/css.test.ts: "consistent padding"` |
| 5 | Warnings in Problems (count, badge, Fix help); the bar is empty with inspector and stays without it | `kit/ui/src/inspector/problems.test.ts: "page warnings"`; `kit/ui/src/page/inspector.test.ts: "warnings go to Problems"` |

Edge cases:
- a page with only warnings;
- the warnings changing on live reload;
- the editor preview's warnings, which are not announced (the editor has its own console);
- a stored split outside 25–75%, which is clamped.

## UI

docs/UI.md tokens only. The separator is a 1 px `--rule` line with an 8 px hit area, and `--fg-muted` on hover and focus, like the sidebar's edge. Warnings use the amber lint signal colour only for their icon, as the lint marks do.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A warning message carries markup (a layout name taken from front matter) | Problems renders text with `textContent` only | `problems.test.ts: "warning text is text"` |

## Notes

- **Out of scope:** new kinds of checks.
- **Runs beside:** ui/shell-polish, which owns `kit/ui/src/nav/**`, `kit/ui/src/shell/{settings,layout}.ts`, `kit/ui/src/shell/settings.css`, `kit/ui/src/styles/shell.css` and the app's `settings.ts`. The `.zen-about*` rules in `settings.css` are removed there.
- **Integration (lead):**
  - docs/UI.md, the Inspector paragraph;
  - CHANGELOG: `Changed: the Inspector's columns can be resized and have the same padding in every tab; page warnings (an unknown layout, a dialect not in the build) are listed under Problems; what the build contains is the Inspector's Build tab, and Zenmark › About is the standard macOS panel again.`
- **App Store / sandbox:** none.

## Prompt

> Implement `specs/ui/inspector-polish/spec.md` following `specs/README.md` and CLAUDE.md. Context:
> - the Inspector is in `kit/ui/src/inspector` and the panel in `kit/ui/src/panel` (#25);
> - the About sheet came from ui/settings-layout (`kit/ui/src/shell/about.ts`, `menu:about` in `menu.go`, `app.go`, `shell.ts` and the api);
> - render warnings come in `Page.warnings` and are shown by `kit/ui/src/page/index.ts:88`.
>
> Work in `../zen-worktrees/ui-inspector-polish`. Tests first. Do not launch the app, push or touch main.
>
> Done when the table's tests pass, `go vet ./... && go test ./...` pass (run `npx vite build` first), and `npx tsc --noEmit && npx vitest run` pass in `apps/zenmark/frontend`.
