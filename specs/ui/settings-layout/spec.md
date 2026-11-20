# ui/settings-layout

| | |
|---|---|
| Status | done |
| Target version | 0.7.0 |
| Branch | `feature/26-settings-layout` |
| Wave | 26 |
| Depends on | kit/shell-features, ui/developer-tab, ui/static-build, app/export, kit/assembly (This build) |
| Owns | `kit/ui/src/shell/settings.ts` (frame, Appearance, Developer), `kit/ui/src/shell/settings.css` (new, imported by settings.ts), `kit/ui/src/shell/about.ts` (new), `apps/zenmark/frontend/src/shell/{settings,pluginhelp,layouthelp,buildsite,templateshelp}.ts` (`templateshelp` is new), `apps/zenmark/menu.go` + `menu_test.go` (About), `apps/zenmark/frontend/src/api/{index,wails,fake}.ts` (only the `menu:about` event) (+ the tests of all of these) |

## Purpose

Settings is laid out the same way in every section: what you pick on the left,
and what it does on the right. That right column holds the preview, the
actions and the help. Export and Developer work in place, with no modal. What
the build contains moves to Zenmark › About, where people expect it (#26).

## Behaviour

### The frame
1. **The section list**, in order:
   - Appearance, Configuration, Markdown, Layouts, Plugins, Templates, Export, Developer.
   - A section whose feature is missing from the build is left out, as today (kit/shell-features).
   - Configuration needs `editor`; Markdown, Layouts and Plugins need `reader`; Templates needs `newsite`; Export needs `export`; Developer needs `developer`.
   - **This build** is no longer a section.
2. **Every section has two columns.** The left one is about 240 px: the setting or the list of items, keyboard navigable (↑/↓, Enter). The right one takes the rest: the selected item's preview, description and actions.
   - Below 720 px wide, the right column goes under the left.
   - The selected item is remembered per section for the session.

### Sections
3. **Appearance.**
   - Left: Theme (System, Light, Dark).
   - Right: a small preview of the reader in the selected theme (a page title, a paragraph, a link and a code line, drawn with the tokens).
   - Navigation (Left, the default, or Right) is added by ui/nav-panel (#24), which owns the sidebar side. This spec leaves a slot for it: `appearanceSection` takes optional extra rows.
4. **Configuration.**
   - Left: Global (`config.yml`, "all folders") and Local (`.zen.yml`, "this folder"; disabled with no folder open).
   - Right: a read-only preview of the selected file, in mono with no editing, plus its actions: Open in Editor and Reveal in Finder. "Create" appears when the file does not exist.
5. **Markdown.**
   - Left: the dialects of the build, each with its on/off for this folder.
   - Right: the selected dialect's summary, a syntax sample and a rendered preview of that sample (`api.preview`).
   - With no dialects in the build: "This build reads CommonMark and GFM only."
6. **Layouts, Plugins and Templates.**
   - Left: the list of names.
   - Right: the existing help for the selected item.
   - The words "on" and "Built in" are dropped from the list. Only exceptions show: a mark for off, blocked or invalid items, and "This folder" or "Your …" for items that are not built in.
7. **Export.**
   - Left: Website and PDF (PDF only with `print`).
   - Right: the options and actions in place, with the same fields, Build, progress with Cancel, warnings and results as today's File › Export › Website… sheet, and the PDF export as today.
   - The sheet is no longer opened from Settings. File › Export › Website… (⇧⌘B) opens Settings › Export › Website instead.
8. **Developer.**
   - Left: Console.
   - Right: the switch, now called **"Show Console in the Inspector"** (the same storage key, `developerEnabled()`), one line saying what the Console shows, and a **Open Console** action (⌥⌘J), enabled when the switch is on.

### About
9. **Zenmark › About Zenmark** opens an **About** sheet in the main window instead of the standard macOS panel. The sheet shows:
   - the logo, name, version and copyright line;
   - **This build** (today's section content: features, add-ons, plugins, layouts, templates, dialects, theme);
   - Close (Esc).

   It works in a bare shell and with no folder open. Its menu item sends `menu:about`.

## Architecture

- **`kit/ui/src/shell/settings.ts`:**
  - `createSettingsFrame` keeps its signature.
  - Each `SettingsSection` may now give `{ items, render(item) }` instead of a single `el`. The frame draws the two columns, the item list and the selection.
  - Old sections that give `el` render in the right column, under the section title.
  - New `settings.css` (tokens only) has the two-column grid and the list rows. It is imported from settings.ts.
- **`kit/ui/src/shell/about.ts`:** `openAbout(doc, { host, name, version, logoUrl })` uses `buildSection(host)` from `kit/ui/src/shell/addons.ts` (imported, not edited).
- **`apps/zenmark/frontend/src/shell/settings.ts`:**
  - builds the eight sections from the build;
  - moves the export form from the `buildsite.ts` sheet into the Export section (`buildsite.ts` keeps its form builder and loses its modal wrapper);
  - routes the `menu:build-website` event to Settings › Export;
  - drops This build.
- **`apps/zenmark/menu.go`:** About becomes `{Label: "About Zenmark", Event: "menu:about"}` instead of `Role: "About"`.

## Interfaces

```ts
// kit/ui/src/shell/settings.ts
export type SettingsItem = { id: string; label: string; note?: string; disabled?: boolean };
export type SettingsSection = {
  id: string; label: string;
  el?: HTMLElement;                                   // old single-column content (right column)
  items?: () => SettingsItem[] | Promise<SettingsItem[]>;
  render?: (item: SettingsItem, right: HTMLElement) => void | (() => void);
  show?: () => void; destroy?: () => void;
};
export function appearanceSection(doc: () => Document, extra?: HTMLElement[]): SettingsSection;
export function developerSection(o?: { openConsole?: () => void }): SettingsSection;

// kit/ui/src/shell/about.ts
export function openAbout(doc: Document, o: { host: Pick<KitHost, "addons" | "build">; name: string; version: string; logoUrl: string }): { close(): void };
```

Events: `menu:about` (Go → frontend), added to `Api.on`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | Section order, feature gating, no This build | `apps/zenmark/frontend/src/shell/settings.test.ts: "section list per build"` (full, starting shell, bare) |
| 2 | Two columns, keyboard selection, narrow stacking class | `kit/ui/src/shell/settings.test.ts: "two-column frame"` |
| 3 | Appearance preview follows the theme | `kit/ui/src/shell/settings.test.ts: "appearance preview"` |
| 4 | Configuration: preview, actions, Local disabled without a folder, Create | `settings.test.ts: "configuration section"` |
| 5 | Markdown: dialect toggle, sample preview, empty text | `settings.test.ts: "markdown section"` |
| 6 | Lists without "on"/"Built in", exceptions marked | `settings.test.ts: "plugin, layout and template lists"` |
| 7 | Export in place; ⇧⌘B opens it; no modal | `settings.test.ts: "export in place"`, `buildsite.test.ts` (updated) |
| 8 | Developer switch label, key kept, Open Console | `kit/ui/src/shell/settings.test.ts: "developer section"` |
| 9 | About sheet with This build, Esc, works bare; menu item sends `menu:about` | `kit/ui/src/shell/about.test.ts`; `apps/zenmark/menu_test.go: TestAboutMenu` |

Edge cases:
- a section with an empty list (no plugins) shows its empty text on the right;
- a build running when the user leaves Export keeps running, and the result shows when they come back;
- switching folders refreshes Local and the dialect states;
- About opened twice brings the same sheet to the front.

## UI

docs/UI.md tokens only. The list rows look like the sidebar rows: the selected one has `--btn-bg` and `aria-selected`. The right column has the item's title as `h3`, then its content. Thin `--rule` between the columns. Every list is a `listbox`; About is a `dialog` with `aria-modal`.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| The Configuration preview renders a config file as HTML | Shown as text (`textContent`) in a `pre`, never as HTML | `settings.test.ts: "configuration preview is text"` |
| The Markdown sample preview runs page scripts | It uses `api.preview` and mounts like the editor preview, so plugin scripts follow the existing trust rules | existing preview tests |

## Notes

- **Out of scope:** new settings beyond those listed. Accessibility (ui/accessibility, wave 27) adds its own section to this frame afterwards.
- **Shared with #25 (same wave):** #25 owns `kit/ui/src/shell/developer.ts` (the switch's storage) and opening the Inspector on Console. This spec calls `developerEnabled`/`setDeveloperEnabled` and an `openConsole` callback that the shell passes in.
- **Integration (lead, at merge):**
  - In `apps/zenmark/frontend/src/shell/shell.ts`: handle `menu:about` with `openAbout`, pass `openConsole` to the Developer section, and route `menu:build-website` to Settings › Export.
  - Remove the `.zen-settings-*` rules in `kit/ui/src/styles/shell.css` that settings.css replaces.
  - docs/UI.md Settings paragraph; CHANGELOG.
- **App Store / sandbox:** none.
- **Changelog:** `Changed: Settings has two columns in every section (the item on the left, its preview, help and actions on the right); Export and the Developer switch work in place; This build moved to Zenmark › About (#26).`

## Prompt

> Implement `specs/ui/settings-layout/spec.md` following the rules in `specs/README.md` and CLAUDE.md, for zenmark-org/zenmd#26 (`gh issue view 26`).
> Context:
> - Settings is `createSettingsFrame` and the sections in `kit/ui/src/shell/settings.ts`, plus the app's sections in `apps/zenmark/frontend/src/shell/settings.ts`.
> - Plugin and layout help is in `pluginhelp.ts` and `layouthelp.ts`; the export sheet is in `buildsite.ts`; This build is `buildSection` in `kit/ui/src/shell/addons.ts`.
> - #25 runs at the same time and owns the Inspector, the panel, `layout.ts`, `developer.ts`, `shell.ts` and `shell.css`. Don't edit them; anything needed there goes in your report as an Integration line.
>
> Work in `../zen-worktrees/issue-26` on `feature/26-settings-layout`. Tests first.
>
> Do not launch the app, run `task install`/`package`/`release`, push, or touch main.
>
> Done when the table's tests pass, `go vet ./... && go test ./...` pass (run `npx vite build` in `apps/zenmark/frontend` first), and `npx tsc --noEmit && npx vitest run` pass in `apps/zenmark/frontend`. The final commit message ends in `Fixes #26`.
