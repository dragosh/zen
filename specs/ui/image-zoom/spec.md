# ui/image-zoom

Branch `feature/ui-image-zoom` · Owns `runtime/src/zoom/`, the zoom rules in `runtime/src/css/page.css`, one line each in `kit/ui/src/page/index.ts` and `runtime/src/static/static.ts` · Depends on engine/runtime, ui/page · Version: MINOR (0.5.0)

## Purpose

Screenshots and diagrams in a page are shrunk to the column width. A click on an
image should show it large, over the page, and let the reader zoom in and pan,
in the app and in built websites alike.

## Behaviour

- A click on an image in the page (`.zen-page img`) opens it in a zoom view over
  the whole window, fitted to the window (never enlarged past 100% by the fit).
- Not zoomable, the click does what it did before: an image inside a link, a
  button or a plugin's output (`[data-zen-plugin]`), one marked
  `data-zen-nozoom`, an image that has not loaded or is broken, and small images
  (both sides 48 px or less: icons, badges). Clicks with ⌘, ⌃, ⇧ or ⌥, other
  buttons, and clicks a handler already took (`defaultPrevented`) are ignored.
- In the zoom view:
  - a click on the image switches between fit and a closer look: 100% when the
    image is larger than the window, otherwise twice the fit; the point clicked
    stays in view;
  - `+`/`=` zoom in (×1.25), `-` zooms out, `0` fits, `1` is 100%; a pinch on the
    trackpad (wheel with ⌃) zooms; the scale stays within 10%–800%;
  - a larger image scrolls (pans) inside the view;
  - a bar shows −, the scale in %, +, Fit and ✕ (Close);
  - Esc, ✕, or a click outside the image closes it, and focus returns to where
    it was.
- One zoom view at a time; navigating to another page closes it.
- Images in pages get a zoom-in cursor; the view is never printed.

## Architecture

`runtime/src/zoom/zoom.ts`: `installImageZoom(root)` listens for clicks on
`root` (event delegation, so it holds for every render) and builds the view
(plain DOM, appended to `document.body`). The app's page feature
(`kit/ui/src/page`) installs it on the article; built sites (`static.ts`) on
`.zen-page`. Styles are in `runtime/src/css/page.css`, which both load.

## Interfaces

```ts
export const ZOOM_MIN = 0.1, ZOOM_MAX = 8;
export function zoomableImage(target: EventTarget | null, root: HTMLElement): HTMLImageElement | null;
export interface ImageZoom { open(img: HTMLImageElement): void; close(): void; readonly isOpen: boolean; destroy(): void }
export function installImageZoom(root: HTMLElement): ImageZoom;
```

## Tests

`runtime/src/zoom/zoom.test.ts` (happy-dom; natural sizes are stubbed):

1. a click on a page image opens the view with a copy of the image, fitted;
2. images in links, buttons, plugins, `data-zen-nozoom`, broken and small ones do not open it; modifier clicks do not;
3. click toggles fit ↔ 100% for a large image and fit ↔ 2× for a small one;
4. `+`, `-`, `0`, `1` and ⌃-wheel change the scale within the limits; the bar shows it;
5. Esc, ✕ and a click outside close it and restore focus; `destroy()` closes and stops listening;
6. the CSS hides `.zen-zoom` in print (kit `css.test.ts`).

## UI

A dimmed backdrop (`--zen-overlay` or 85% of the background), the image
centred, the bar at the bottom centre in the style of the other floating
controls (tokens only, light and dark). Motion: none when reduced motion is set.

## Security

The view copies `src` and `alt` of an image the sanitiser already allowed; no
HTML is parsed (no `innerHTML`).

## Notes

A gallery (next/previous image) and zoom on SVG diagrams are possible later.

## Prompt

Build `runtime/src/zoom/zoom.ts` and its tests as specified, wire it into the
page feature and the static runtime, add the CSS, then run `task test`.
