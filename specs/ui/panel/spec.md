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
3. A tab can show a count chip (`setCount(id, n, tone)`): hidden at 0 or null; tone neutral, warning or error (lint colours); the tab's accessible name gets the number ("Problems, 3").
4. With `resize`, the top edge is a separator: drag it, or ↑/↓ (16 px), Home (min) and End (70 % of the parent). The height is remembered under `resize.key`, never below `min`.
5. `panelEmpty(title, hint)` is the centred empty state; `panelProps(items)` the label-over-value grid that flows into as many columns as fit (`data-field` on each value, text only).
6. Storage that throws never breaks the panel: first tab, initial height.
7. The panel's height is `BOTTOM_PANEL_HEIGHT` (key `zen.panel.height`, 240 px by default, at least 120) and is re-read when the panel opens. There is one bottom panel, the Inspector (#25); the Developer panel is gone.
7b. **Tools per tab:** `toolsFor(id)` is the group of controls (a filter dropdown, actions) the header shows only while that tab is selected, before the chevron and ✕.
7c. **Hidden tabs:** `setTabVisible(id, false)` takes a tab out of the tab list, the arrow keys and the selection; the user's chosen tab returns when it is visible again. `visibleTabs` lists the rest.
7d. **Collapse:** a chevron (`.zen-panel-collapse`, `aria-expanded`, "Collapse/Expand Inspector") collapses the panel to its header (bodies and resizer hidden, height cleared) and expands it; choosing a tab expands. Remembered under `collapseKey` (localStorage `zen.panel.collapsed`).
7e. **Columns:** `panelColumns()` gives `{ el, left, right }`: a main column and a narrower details column (`.zen-panel-cols`), each scrolling on its own; under 700 px of panel width (a container query) they stack and the tab scrolls.
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
| 7b–7e | `panel.test.ts`: "each tab has its own tools…", "a tab that is not visible…", "collapse…", "panelColumns…"; `css.test.ts`: "two columns…" |

## UI

Header 34 px on `--bg-subtle`; tabs are text, the selected one bold with a 2 px underline in `--focus`; count chips are 16 px pills. Bodies on `--bg` with `--space-3`/`--space-4` padding. Tokens only (docs/UI.md).

## Notes

- **Users:** the Inspector (`kit/ui/src/inspector`, tabs Page Info, Problems, Console).
- Changed by #25
- **Changelog:** part of the Inspector line under Unreleased.
