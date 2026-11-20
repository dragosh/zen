# ui/accessibility

| | |
|---|---|
| Status | specified |
| Target version | 0.7.0 |
| Branch | `feature/ui-accessibility` |
| Wave | 27 |
| Depends on | kit/shell-features (the bare shell, `hasFeature`), #23 intro screen (merged), ui/settings-layout (#26, the two-column frame), #25 (the Inspector panel) |
| Owns | `kit/ui/src/shell/{a11y,a11y.test}.ts` (new), `kit/ui/src/shell/settings.ts` (the new section), `kit/ui/src/shell/keys.ts` (zoom keys), `runtime/src/css/{tokens,page}.css` (reading variables), the `prefers-reduced-motion` and contrast rules in `runtime/src/css/base.css`, `kit/ui/src/styles/shell.css`, `kit/ui/src/{panel/panel,print/preview,find/find,nav/nav,inspector/inspector}.css`, `kit/ui/src/shell/settings.css`, `apps/zenmark/menu.go` (View menu) + `menu_test.go`, `apps/zenmark/frontend/src/{main.ts,shell/settings.ts}` (wiring) (+ the tests of all of these). Runs beside ui/nav-panel, which owns `layout.ts`, `shell.ts`, the api files and `nav/panel.css` |

## Purpose

People who need larger text, calmer screens or stronger contrast can set the
app up for themselves in Settings › Accessibility. The window zooms as a
whole with the usual ⌘+ / ⌘− / ⌘0. The reader's text size, font, spacing and
width can be changed without zooming the chrome. These settings belong to the
person, not the folder, so they are kept per machine and apply to every window.

## Behaviour

### Settings › Accessibility
1. A new section, **Accessibility**, follows Appearance. Like Appearance, it is part of the bare shell (`features: []`). Its **Reading** group appears only when the build has `reader`.
2. Every option takes effect at once, in every open window (main and editor), and is kept per machine (`localStorage`, `zen.a11y.*`). **Reset to defaults** puts every option back. Each option shows its default.

### Zoom (the whole window)
3. **Zoom**: 75%, 90%, 100% (default), 110%, 125%, 150%, 175% and 200%, as a stepper with the value shown.
   - The whole web view scales: chrome, sidebar, page, panels and dialogs (`Window.SetZoom`).
   - Every window uses the same value.
4. **View menu**, new in the shell:
   - Actual Size (⌘0), Zoom In (⌘+, also ⌘=) and Zoom Out (⌘−) step through the same list.
   - The ends do nothing and the menu item is disabled there.
   - The same keys work when focus is in the page.
   - It is the macOS View menu, between Edit and Window.

### Reading (with `reader`)
5. **Text size**: 14–28 px in steps of 1, default 17. It sets the reader's body text and everything sized in `em` from it: headings, code and tables. The chrome is unaffected.
6. **Font**: Serif (default, `--font-body`) or Sans (`--font-ui`). These are system fonts only; nothing is downloaded or bundled.
7. **Line spacing**: Normal (1.65, default), Relaxed (1.9) or Loose (2.2).
8. **Line width**: Narrow (60ch), Normal (68ch, default) or Wide (84ch).
9. **Underline links**: off by default. When on, every link in a page is underlined, including links in plugin output, not only on hover.
10. The reading options apply to the reader and the editor's preview. They do **not** apply to print, Print Preview, PDF export or built sites, which keep the document's own look. The editor's source text follows Zoom only.

### Motion and contrast (always)
11. **Reduce motion**: System (default) or On. On does what `prefers-reduced-motion: reduce` does today, everywhere it is used: the welcome fade, panel slides, the opening bar, card transitions, image zoom and find.
12. **Increase contrast**: System (default) or On. It is also on when macOS asks for it (`prefers-contrast: more`). With it on:
    - `--fg-muted` becomes `--fg`;
    - `--rule` is at least 3:1 against `--bg`;
    - focus rings are 3 px.

    This holds in both themes, and nothing else changes colour.

### Applying
13. The options are attributes on `<html>`, as the theme is:
    - `data-reduce-motion="on"` and `data-contrast="more"`;
    - the reading options as CSS variables: `--zen-text-size`, `--zen-text-font`, `--zen-line-height`, `--zen-measure`;
    - `data-link-underline="on"`.

    They are applied before the first paint, so a window never shows the defaults first. `followAccessibility(doc)` keeps them in step with the other windows through the `storage` event, like `followTheme`.
14. Values that are stored but invalid or out of range fall back to their defaults. A missing `localStorage` works for the window it is in only.

## Architecture

- **`kit/ui/src/shell/a11y.ts`**, new:
  - the option model and its ranges;
  - read and write in `localStorage`;
  - `applyAccessibility(doc)`, which sets the attributes and variables;
  - `followAccessibility(doc)`;
  - `zoomStep(dir)` with the list from behaviour 3.

  Zoom calls an injected `setZoom(n)`, so tests need no Wails. The app passes `Window.SetZoom`.
- **`kit/ui/src/shell/settings.ts`**:
  - `accessibilitySection(doc, { reader, setZoom })`;
  - it uses the existing `zen-settings-segment` radio groups and a stepper (role `spinbutton`).
- **`runtime/src/css/tokens.css`**: the four reading variables, with today's values as defaults. `page.css` uses them (`font: var(--zen-text-size)/var(--zen-line-height) var(--zen-text-font)`, `max-width: var(--zen-measure)`). Built sites and print get the defaults, so their output is unchanged byte for byte. The print stylesheet and the print preview reset the variables.
- **Reduce motion:** each `@media (prefers-reduced-motion: reduce) { … }` block gets the same rules for `:root[data-reduce-motion="on"]`, written once per file as a shared selector list. No new animation.
- **Contrast:** `tokens.css` has the `prefers-contrast: more` and `[data-contrast="more"]` overrides for both themes.
- **Menu:** `apps/zenmark/menu.go` gets a View menu. Its items send `menu:zoom-in`, `menu:zoom-out` and `menu:zoom-reset`. The shell steps the zoom and saves it, so the menu and the keys share one code path. `keys.ts` handles ⌘= / ⌘+ / ⌘− / ⌘0 when the menu does not (the editor window, tests).
- **Wiring:**
  - `apps/zenmark/frontend/src/main.ts` calls `followAccessibility` next to `followTheme`, before the shell mounts.
  - Settings adds the section after Appearance.
  - On start, each window sets its zoom from the stored value.
- No Go service changes and no new bindings: zoom is a Wails runtime call made from the window itself.

## Interfaces

```ts
// kit/ui/src/shell/a11y.ts
export type Accessibility = {
  zoom: number;            // 0.75 | 0.9 | 1 | 1.1 | 1.25 | 1.5 | 1.75 | 2
  textSize: number;        // 14..28 (px)
  font: "serif" | "sans";
  lineSpacing: "normal" | "relaxed" | "loose";
  lineWidth: "narrow" | "normal" | "wide";
  underlineLinks: boolean;
  reduceMotion: "system" | "on";
  contrast: "system" | "more";
};
export const A11Y_DEFAULTS: Accessibility;
export const ZOOM_STEPS: readonly number[];
export function storedAccessibility(): Accessibility;
export function setAccessibility(patch: Partial<Accessibility>, doc?: Document): Accessibility;
export function resetAccessibility(doc?: Document): Accessibility;
export function applyAccessibility(a: Accessibility, doc?: Document): void;
export function followAccessibility(doc?: Document, setZoom?: (z: number) => void): () => void;
export function zoomStep(current: number, dir: 1 | -1 | 0): number; // 0 = Actual Size

// kit/ui/src/shell/settings.ts
export function accessibilitySection(doc: () => Document, o: { reader: boolean; setZoom?: (z: number) => void }): SettingsSection;
```

Events (Go → frontend): `menu:zoom-in`, `menu:zoom-out` and `menu:zoom-reset`, added to `Api.on` in `apps/zenmark/frontend/src/api/{index,wails,fake}.ts`.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | The section is in the bare shell, and Reading only with `reader` | `apps/zenmark/frontend/src/shell/settings.test.ts: "Accessibility section"` (bare and full) |
| 2 | Applies at once, stored, synced by the storage event, Reset | `kit/ui/src/shell/a11y.test.ts: "stores, applies and follows other windows"`, `"reset"` |
| 3–4 | Zoom steps, the ends, the View menu items and keys | `a11y.test.ts: "zoomStep"`; `kit/ui/src/shell/keys.test.ts: "zoom keys"`; `apps/zenmark/menu_test.go: TestViewMenu` (the items, the shortcuts, and that the bare shell has it) |
| 5–9 | The reading variables and the underline attribute are set | `a11y.test.ts: "reading options set the variables"` |
| 10 | Print and built sites unchanged | `runtime` test: the print and static CSS reset or omit the variables; `engine/site` golden build output unchanged (existing goldens pass) |
| 11 | Reduce motion On matches every reduced-motion block | `a11y.test.ts: "every reduced-motion rule also matches data-reduce-motion"` (reads the owned CSS files and checks each `@media (prefers-reduced-motion` block has the `[data-reduce-motion="on"]` twin) |
| 12 | Contrast overrides in both themes | `kit/ui/src/shell/tokens.test.ts: "increase contrast"` (muted = fg; rule contrast ≥ 3:1) |
| 13 | Applied before first paint | `apps/zenmark/frontend/src/shell/shell.test.ts: "accessibility applied before mount"` |
| 14 | Invalid stored values fall back; storage missing | `a11y.test.ts: "invalid values"`, `"no storage"` |

Edge cases:
- zoom at 200% plus text size 28: the page still has no horizontal scroll at a width of 1100 px;
- an editor window opened after a change starts with the new values;
- Print Preview opened at 150% zoom prints at 100%;
- the keyboard can reach every option, and the stepper takes ↑/↓, Home and End;
- `prefers-contrast: more` with the option on System turns contrast on, and with On it stays on.

## UI

Settings › Accessibility, in the same frame and controls as Appearance (docs/UI.md tokens only):

```
Accessibility
  Zoom              [ − ]  100%  [ + ]          ⌘−  ⌘0  ⌘+
  Reduce motion     ( System | On )
  Increase contrast ( System | On )
  Reading                                 (only with reader)
    Text size       [ − ]  17 px  [ + ]
    Font            ( Serif | Sans )
    Line spacing    ( Normal | Relaxed | Loose )
    Line width      ( Narrow | Normal | Wide )
    Underline links [ ]
  Sample paragraph in the reading style   (live preview)
                                       [ Reset to defaults ]
```

- **States:**
  - Defaults: Reset is disabled.
  - Changed: Reset is enabled.
  - Storage unavailable: a muted note, "Not saved: applies to this window only."
- **Accessibility:**
  - each group is a labelled `radiogroup` with arrow keys;
  - steppers are `spinbutton` with `aria-valuetext` ("125 percent", "17 pixels");
  - the sample paragraph has `aria-hidden="true"`.
- When #26 (Settings in two columns) lands, this section moves into its two-column layout like every other section, with the sample in the right column.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A stored value injects CSS (for example a crafted `--zen-text-font`) | Values are parsed into the closed types above and mapped to fixed CSS strings. Nothing from storage is written into a style without that check. | `a11y.test.ts: "invalid values"` (strings with `;`, `url(` and `}` fall back) |
| A page or plugin script changes the user's settings | The settings live in the app origin's `localStorage`, and page scripts run in the existing plugin sandbox and get no `localStorage` of the app. | existing plugin sandbox tests; no new surface |

## Notes

- **Out of scope:**
  - a reading-size control in built sites (readers have the browser's zoom);
  - per-folder accessibility settings;
  - VoiceOver audits beyond the controls added here;
  - dyslexia fonts or any downloaded font;
  - colour-blind themes (the chrome has no hue);
  - the editor's source font size apart from Zoom.
- **Interplay with the shell issues:**
  - #26 restyles Settings into two columns and adds Appearance › Navigation. This spec only adds one section to today's frame.
  - #25 (Inspector) and #24 (Nav panel) keep these variables and the reduced-motion twin rules.
- **Integration (lead, at merge):**
  - `apps/zenmark/frontend/src/api/{index,wails,fake}.ts`: the `menu:zoom-in`, `menu:zoom-out` and `menu:zoom-reset` events (ui/nav-panel owns those files in this wave); until then `main.ts` listens with the Wails `Events.On` directly.
  - `kit/ui/src/nav/panel.css` (ui/nav-panel): add the reduced-motion twin if it has motion rules.
  - docs/UI.md: the Accessibility paragraph gains the section, the attributes, View › Zoom and "Contrast more".
  - docs/BUILD.md: Accessibility is in the bare shell.
  - README: a features line.
  - CHANGELOG under Added.
- **App Store / sandbox:** none.
- **Changelog:** `Added: Settings › Accessibility: zoom for the whole window (View › Zoom In/Out/Actual Size, ⌘+ ⌘− ⌘0), the reader's text size, font, line spacing, line width and link underlines, and Reduce motion and Increase contrast that also follow macOS.`

## Prompt

> Implement `specs/ui/accessibility/spec.md` following the rules in `specs/README.md`.
> Context:
> - Settings sections live in `kit/ui/src/shell/settings.ts`. `appearanceSection` and `kit/ui/src/shell/theme.ts` (`localStorage`, `followTheme`, the `storage` event) are the model to copy.
> - The reader's text is set in `runtime/src/css/page.css:57` (`font: 17px/1.65 var(--font-body)`). The tokens are in `runtime/src/css/tokens.css`.
> - The features of a build come from `hasFeature(api, id)`.
> - The native menu is `menuSpecFor(dev, features)` in `apps/zenmark/menu.go`.
> - Wails zoom from the frontend: `Window.SetZoom(n)` in `@wailsio/runtime`.
>
> Order of work:
> 1. `a11y.ts` and its tests (model, storage, apply, follow, zoomStep, invalid values).
> 2. The CSS: the reading variables with today's defaults, the reduced-motion twins in every owned CSS file, and the contrast overrides. Add the test that checks every reduced-motion block has its twin.
> 3. The Settings section and the keys.
> 4. The View menu and its events.
> 5. The wiring in `main.ts` and `shell.ts`, before the first paint.
>
> Check that built sites and print output do not change: the existing engine/site goldens and runtime tests must pass unchanged.
>
> Do not launch the app, run `task install`/`package`/`release`, push, or touch main.
>
> Done when the table's tests pass, `go vet ./... && go test ./...` pass, and `npx tsc --noEmit && npx vitest run` pass in `apps/zenmark/frontend` (which also runs the kit and runtime projects).
