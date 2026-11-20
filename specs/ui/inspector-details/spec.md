# ui/inspector-details

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/ui-inspector-details` |
| Wave | 30 |
| Depends on | ui/inspector-polish, ui/lint (Vale marks), ui/find (Custom Highlight API) |
| Owns | `kit/ui/src/inspector/**`, `kit/ui/src/panel/**`, `kit/ui/src/page/index.ts` (marking a problem in the page) + `page/*.test.ts`, `kit/ui/src/lint/**` only if the Vale marks' code is reused there (+ tests) |

## Purpose

Problems and Build work like every other list in the app (docs/UI.md, UI
consistency): pick an item on the left, and its details are on the right. In
Problems, picking an item also shows the problem in the page, the way the
spelling checker marks a word.

## Behaviour

1. **Problems: one click.** Clicking a problem, or pressing Enter or Space on it, does three things:
   - it selects it, and its details (message, rule or kind, line or element, Fix help) show on the right;
   - the page scrolls straight to it;
   - the problem's text is marked in the page.

   The Show button goes away. ↑/↓ move the selection, and each move shows that problem in the page.
2. **The mark** is the same for every kind of problem, styled like the Vale marks: a wavy underline in the problem's lint colour (error red, warning amber, suggestion blue), plus a brief background flash of `--btn-bg` that fades unless Reduce motion is on.
   - It uses the CSS Custom Highlight API, as find does, so the page's DOM is not changed.
   - A broken link marks the link text, a missing image its `img` (an outline instead of an underline), a spelling problem the word (from the Vale alert's range), and a page warning the element it names, or nothing if it names none.
   - Only one problem is marked at a time. The mark goes away when the selection changes, the page changes or the Inspector closes.
   - Where the Highlight API is missing, the element is still scrolled to and focused.
3. **Build: a list.** The Build tab's left column is a list of items:
   - Version, Features, Add-ons, Plugins, Layouts, Templates, Dialects and Theme;
   - each with a short note: the version number, or a count such as "4 of 9" or "1".

   The right column shows the selected item's details:
   - Version: the app version, the engine version if it differs, and the channel;
   - Features: every feature in catalog order, the missing ones greyed and marked "not in this build";
   - the others: their ids and descriptions, or "None in this build.".

   Version is selected first. It uses the panel's columns and its list style (`role=listbox`), like Problems and Console.
4. All text goes in through `textContent`.

## Architecture

- **`kit/ui/src/inspector/problems.ts`:**
  - selection drives the details and a new `showInPage(problem)`;
  - the Show button is removed.
- **`showInPage`:**
  - it sends an `INSPECTOR_MARK` event with `{ kind, href?, src?, alert?, text? }`;
  - `kit/ui/src/page/index.ts` resolves it to a `Range` in the article and scrolls to it (smooth unless reduced motion);
  - it sets the `zen-problem` highlight;
  - `INSPECTOR_MARK` with `null` clears it.
- **Styles:** `::highlight(zen-problem)` in inspector.css, plus the image outline.
- **`kit/ui/src/inspector/index.ts`:** the Build tab becomes items and details through `panelColumns` and the list helpers.

## Interfaces

```ts
// kit/ui/src/inspector/events.ts
export const INSPECTOR_MARK = "zen:inspector-mark"; // detail: ProblemTarget | null
export type ProblemTarget = { kind: ProblemKind; severity: "error" | "warning" | "suggestion"; href?: string; src?: string; text?: string; offset?: number; length?: number };
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | One click selects, shows the details, scrolls and marks; the keys move it; no Show button | `kit/ui/src/inspector/problems.test.ts: "click shows in page"` |
| 2 | Each kind resolves to the right range or element; one mark at a time; cleared on page change and close; fallback without Highlight | `kit/ui/src/page/inspector.test.ts: "problem marks"` |
| 3 | Build items, notes, Version first, details per item | `kit/ui/src/inspector/build.test.ts` (rewritten) |
| 4 | Text only | the existing text tests, extended to Build |

## UI

UI consistency (docs/UI.md):
- the Build list is the Problems/Console list pattern;
- the problem mark is the Vale mark pattern;
- no new style for selection.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A problem's `href` or text is used in a selector | No selector strings: matching compares attribute values in JS | `problems.test.ts: "hostile href"` |

## Notes

- **Runs beside:** app/logging (Go and the Console). The lead edits `apps/zenmark/frontend/src/shell/{settings,shell}.ts` in this wave.
- **Changelog:** `Changed: clicking a problem jumps to it in the page and marks it like a spelling mark, with its details on the right; the Inspector's Build tab is a list with details.`

## Prompt

> Implement `specs/ui/inspector-details/spec.md` following `specs/README.md` and CLAUDE.md.
>
> Work in `../zen-worktrees/ui-inspector-details`. Tests first. Do not launch the app, push or touch main.
>
> Done when the table's tests pass, `go vet ./... && go test ./...` pass (run `npx vite build` first), and `npx tsc --noEmit && npx vitest run` pass in `apps/zenmark/frontend`.
