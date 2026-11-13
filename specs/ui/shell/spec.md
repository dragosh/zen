# ui/shell

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/ui-shell` |
| Wave | 1 |
| Depends on | none |
| Owns | `frontend/index.html`, `frontend/src/main.ts`, `frontend/src/api/` (except `wails.ts`), `frontend/src/shell/`, `frontend/src/styles/`; deletes `frontend/src/shell.ts`, `shell.test.ts`, `style.css` |

## Purpose

The window frame: design tokens, the welcome screen with "Open a folder", the
layout slots the other UI features mount into, and the typed API client with
a fake so every UI feature can be built and tested without Go.

## Behaviour

1. On start the shell calls `api.startup()`. No workspace → welcome view (logo, "Open a folder" primary button, hint `⌘O`). Workspace → frame view, then navigates to `startup.page` (or `workspace.home`; if both empty, the page slot shows "Select a page from the sidebar").
2. "Open a folder" and ⌘O call `api.pickFolder()`; `null` (cancelled) changes nothing; a workspace switches to the frame view.
3. The frame has three slots: `nav` (sidebar, 260px, resizable 200–400px by dragging its edge, ⌘\ toggles, width remembered in `localStorage`), `header` (top bar 44px: breadcrumb slot + actions slot), `main`.
4. The shell owns app state `{workspace, path, mode: "read"|"edit"}` in a small store (`get`, `set`, `subscribe`) and a history stack: `navigate(path, anchor?)`, `back()` ⌘[, `forward()` ⌘].
5. Features are registered in `shell/features.ts` as `{ mount(slots, ctx) }` where `ctx = { api, store, navigate }`. The shipped file registers placeholder features so the app runs before the other UI specs merge.
6. `mode` switches what is mounted in `main`: page feature for `read`, editor feature for `edit`.
7. `styles/tokens.css` defines every token in docs/UI.md for light and dark (`prefers-color-scheme`, overridable with `data-theme`); `styles/base.css` holds resets, buttons, focus ring.
8. `api/index.ts` exports the `Api` interface and types exactly as in PLAN.md "Service API", `getApi()` (returns the Wails implementation when `window._wails` exists and `api/wails.ts` is present, else the fake), and `api/fake.ts`: an in-memory implementation with a three-page sample workspace, `emit()` for events, and call recording for tests.
9. A failed `startup()` shows an error screen with the message and a Retry button.

## Architecture

Plain TypeScript and DOM, no framework, no new packages. `main.ts` only boots
the shell. Other UI features import from `api/` and `shell/` types, never the
reverse (the shell knows features only through `features.ts`).

## Interfaces

```ts
// shell/types.ts
export type AppState = { workspace: Workspace | null; path: string; anchor: string; mode: "read" | "edit";
                         page?: Page;      // set by ui/page after a load
                         dirty?: boolean }; // set by ui/editor
export type Store = { get(): AppState; set(p: Partial<AppState>): void; subscribe(fn: (s: AppState) => void): () => void };
export type Slots = { nav: HTMLElement; breadcrumb: HTMLElement; actions: HTMLElement; main: HTMLElement };
export type Ctx = { api: Api; store: Store; navigate(path: string, anchor?: string): void };
export type Feature = { id: "nav" | "page" | "editor"; mount(slots: Slots, ctx: Ctx): () => void }; // returns unmount
// shell/features.ts
export const features: Feature[];
```

## Tests

| # | Behaviour | Test (`frontend/src/shell/*.test.ts`, `api/fake.test.ts`) |
|---|---|---|
| 1 | welcome vs frame on startup | `shell.test.ts: shows welcome without workspace / frame with workspace` |
| 2 | pick folder, cancel | `shell.test.ts: open folder` |
| 3 | sidebar toggle and width clamp | `layout.test.ts` |
| 4 | store and history | `store.test.ts`, `history.test.ts` |
| 5,6 | features mounted per mode, unmounted on switch | `shell.test.ts: mounts features by mode` |
| 7 | every UI.md token defined in both themes | `tokens.test.ts` (parses the CSS file) |
| 8 | fake implements Api, events, recording | `fake.test.ts` |
| 9 | startup error and retry | `shell.test.ts: startup error` |

## UI

docs/UI.md views 1 and frame of 2; mockup `docs/ui/mockup.html`. Landmarks
`nav`, `header`, `main`. All shortcuts also work via visible controls.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Injecting markup through workspace name or errors | text set with `textContent` only; no `innerHTML` in shell | `shell.test.ts: escapes workspace name` |

## Notes

- **Out of scope:** sidebar content, breadcrumb content, page rendering, editor (their own specs), drag-and-drop of folders (beta).
- **Integration:** none (this is the base the others plug into).
- **Changelog:** Added: application frame, welcome screen and black-and-white theme.

## Prompt

> Implement `specs/ui/shell/spec.md` following the rules in `specs/README.md`. Read
> docs/UI.md and open docs/ui/mockup.html for the look. Replace the current
> `frontend/src/shell.ts`, `shell.test.ts`, `style.css` and `main.ts`. The app must build
> (`npx vite build` needs `frontend/bindings`; if missing, rely on `tsc` and vitest) and
> run against the fake API. Done when the tests in the table pass and `npx tsc --noEmit` is clean.
