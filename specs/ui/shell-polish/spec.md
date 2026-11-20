# ui/shell-polish

| | |
|---|---|
| Status | specified |
| Target version | 0.7.0 |
| Branch | `feature/ui-shell-polish` |
| Wave | 28 |
| Depends on | ui/nav-panel (#24), ui/settings-layout (#26), ui/accessibility |
| Owns | `kit/ui/src/nav/**` (the old folder row; panel.css), `kit/ui/src/shell/settings.ts` (Appearance), `kit/ui/src/shell/settings.css` (and removing `.zen-about*`), `kit/ui/src/shell/layout.ts` (footer button states, `navSideRow`), `kit/ui/src/styles/shell.css` (footer button states), `apps/zenmark/frontend/src/shell/settings.ts` (Appearance items, Markdown) + `settings.test.ts` (+ the tests of all of these) |

## Purpose

The sidebar and Settings after the first round of testing. The folder's name
is shown once. The sidebar's footer shows which panel is open. Appearance and
Markdown use the same list on the left as every other section.

## Behaviour

1. **One heading in the sidebar.** The nav feature's old folder row (`.zen-nav-ws`, with the folder name) is removed. The sidebar header from ui/nav-panel is the only one. Whatever that row still did, such as the indexing status, keeps working below the tree or in the header's title tooltip, and is not lost.
2. **Active footer buttons.** A footer button whose panel or view is open is clearly marked in black and white:
   - a `--btn-bg` filled background, the icon in `--fg`, and a 2 px `--fg` bar on the side next to the content;
   - the Settings button while Settings shows, Inspector while the Inspector is open, Export while Settings › Export shows.

   The state is set as `aria-pressed="true"` on Export and `aria-expanded="true"` on the others, and it is visible at a glance in both themes and with Increase contrast. Hover stays lighter than the active state.
3. **Appearance like Accessibility.** The left column lists **Theme** and **Navigation**, in that order, vertically. The right column shows the selected one's control and preview: the theme radios and the theme preview, or the Left/Right radios. Without `nav`, there is only Theme. The `appearanceExtra` rows from the shell become items, each named by its row's label.
4. **Markdown like the rest.** The left column always has one item, **Dialects**, selected by default. Its right column shows:
   - the dialects of the build, each with its on/off for this folder, and its summary, sample and preview when selected;
   - or, with no dialects in the build, one short line: "No dialects in this build."

   The CommonMark/GFM sentence is removed.

## Architecture

- **`kit/ui/src/nav/index.ts`:** drop the `zen-nav-ws` header and its CSS. Move any status it carried (search for its uses) under the tree.
- **`kit/ui/src/shell/layout.ts`:** footer buttons get a `setActive(id, on)` method, or the existing `aria-expanded` updates plus `aria-pressed` for Export. The shell already tells the layout when Settings opens; Export is active while `settings.current === "export"`. If something has to be wired in `shell.ts` (owned by ui/inspector-polish in this wave), write it as an Integration line.
- **Styles:** `kit/ui/src/styles/shell.css` and `nav/panel.css`, tokens only.
- **`kit/ui/src/shell/settings.ts`:** `appearanceSection` gives `items` (`theme`, then one per extra row) and `render`.
- **`apps/zenmark/frontend/src/shell/settings.ts`:** the Markdown section's items are `[{ id: "dialects", label: "Dialects" }]`.

## Interfaces

```ts
// kit/ui/src/shell/layout.ts
// Layout += { setActive(button: "settings" | "export" | "inspector", on: boolean): void }
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | No `.zen-nav-ws`; the status is still shown | `kit/ui/src/nav/nav.test.ts: "one heading"` |
| 2 | Active states, and the CSS rule exists with tokens | `kit/ui/src/shell/layout.test.ts: "active footer buttons"`; `kit/ui/src/nav/panel.test.ts` or the shell.css test: the active rule uses `--btn-bg`/`--fg` |
| 3 | Appearance items Theme, Navigation; Theme only without nav | `kit/ui/src/shell/settings.test.ts: "appearance items"`; `apps/zenmark/frontend/src/shell/settings.test.ts` |
| 4 | Markdown: a Dialects item by default; the empty line; no CommonMark sentence | `apps/zenmark/frontend/src/shell/settings.test.ts: "markdown section"` (changed) |

Edge cases:
- Settings opened on the home screen, where there is no sidebar footer;
- switching sections while Settings is open moves the Export state;
- Increase contrast keeps the active state visible.

## UI

docs/UI.md tokens only; black and white. The active bar sits on the content side, so it mirrors when the sidebar is on the right.

## Security

None: no new input, and settings text goes in through `textContent` as before.

## Notes

- **Runs beside:** ui/inspector-polish, which owns `shell.ts`, the panel and the Inspector, `page/index.ts`, `menu.go`, `app.go` and the api.
- **Integration (lead):**
  - docs/UI.md, the sidebar and Settings paragraphs;
  - CHANGELOG: `Changed: the sidebar shows the folder's name once and marks the open panel's button; Settings › Appearance lists Theme and Navigation, and Settings › Markdown lists Dialects, like the other sections.`
- **App Store / sandbox:** none.

## Prompt

> Implement `specs/ui/shell-polish/spec.md` following `specs/README.md` and CLAUDE.md.
>
> Work in `../zen-worktrees/ui-shell-polish`. Tests first. Do not launch the app, push or touch main.
>
> Done when the table's tests pass, `go vet ./... && go test ./...` pass (run `npx vite build` first), and `npx tsc --noEmit && npx vitest run` pass in `apps/zenmark/frontend`.
