# ui/panel

| | |
|---|---|
| Status | done |
| Target version | 0.6.0 |
| Branch | `main` (built by the lead) |
| Wave | 20 |
| Depends on | ui/inspector, ui/developer-tab |
| Owns | `kit/ui/src/panel/` |

## Purpose

One tabbed bottom panel for everything that sits under the page: the Inspector
today, more tabs (Spelling, Outline, …) later. A new tab is one entry in a list
and the content of its body.

## Behaviour

1. `createTabbedPanel` builds a panel with a header (tabs on the left, a tools slot and an optional ✕ close button on the right) and one scrolling body per tab.
2. Tabs follow the ARIA tab pattern: `role=tablist/tab/tabpanel`, linked ids, one tab in the tab order, ←/→ wrapping, Home and End. The chosen tab is remembered under `tabKey` (localStorage); an unknown value means the first tab.
3. A tab can show a count chip (`setCount(id, n, tone)`): hidden at 0 or null; tone neutral, warning or error (lint colours); the tab's accessible name gets the number ("Linting, 3").
4. With `resize`, the top edge is a separator: drag it, or ↑/↓ (16 px), Home (min) and End (70 % of the parent). The height is remembered under `resize.key`, never below `min`.
5. `panelEmpty(title, hint)` is the centred empty state; `panelProps(items)` the label-over-value grid that flows into as many columns as fit (`data-field` on each value, text only).
6. Storage that throws never breaks the panel: first tab, initial height.
7. One bottom panel at a time: the Inspector and the Developer panel close each other (`zen:inspector-close`, `zen:inspector-state`).
8. The panel is never printed; transitions are off with reduced motion.

## Tests

| # | Test |
|---|---|
| 1–2 | `kit/ui/src/panel/panel.test.ts`: "tabs and panels are linked with roles…", "a click selects; ←/→ wrap…" |
| 3 | "count chips: hidden at 0 and null, a tone…" |
| 1 | "the close button appears only with onClose…" |
| 4 | "resizable: starts at the remembered height…" |
| 6 | "broken storage: the panel still works…" |
| 5 | "panelProps…", "panelEmpty…" |
| 7 | `kit/ui/src/panel/css.test.ts` |

## UI

Header 34 px on `--bg-subtle`; tabs are text, the selected one bold with a 2 px underline in `--focus`; count chips are 16 px pills. Bodies on `--bg` with `--space-3`/`--space-4` padding. Tokens only (docs/UI.md).

## Notes

- **Users:** the Inspector (`kit/ui/src/inspector`, tabs Linting, File, Page; height key `zen.inspector.height`). The Developer panel keeps its own header for now (it has filters and Clear in its head); moving it onto this panel is a follow-up.
- **Changelog:** part of the Inspector line under Unreleased.
