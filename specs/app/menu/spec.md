# app/menu

| | |
|---|---|
| Status | specified |
| Target version | 0.2.0 |
| Branch | `feature/app-menu` |
| Wave | 11 |
| Depends on | ui/new-page, ui/editor-window |
| Owns | `internal/app/menu.go` + `internal/app/menu_test.go` (new; the menu moves out of `app.go`), `frontend/src/shell/keys.ts` + test (new), the key handler in `frontend/src/shell/shell.ts` |

## Purpose

Strip the native menu bar to what zen needs today: a **File** menu with only
**New Page** and **Open Folder**. Keep what macOS itself relies on, so the
app still quits, copies and pastes. Remove the developer and template items
that came with Wails (Reload, Force Reload, Toggle DevTools, Speech, Services, …).

## Behaviour

1. The menu bar is, in order:
   - **Zen**: About Zen · Settings… (⌘,) · ― · Hide Zen (⌘H) · Hide Others (⌥⌘H) · Show All · ― · Quit Zen (⌘Q)
   - **File**: New Page… (⌘N) · Open Folder… (⌘O)
   - **Edit**: Undo (⌘Z) · Redo (⇧⌘Z) · ― · Cut (⌘X) · Copy (⌘C) · Paste (⌘V) · Select All (⌘A)
   - **Window**: Minimise (⌘M) · Zoom
2. No View menu: Reload, Force Reload, Toggle DevTools, the zoom items and Enter Full Screen are gone. The green window button still enters full screen. No Services or Speech submenus, and no Paste and Match Style. Nothing in a production build opens the web inspector.
3. New Page… and Open Folder… behave exactly as today: same events (`menu:new-page`, `workspace:opened`), and the main window is shown and focused first.
4. **Settings…** (⌘,) shows the main window and opens Settings there (event `menu:settings`). Today ⌘, works only while the main window has focus.
5. **About Zen** shows the standard macOS About panel with the zen icon, the name `Zen`, the version from `VERSION` and the copyright from Info.plist. There are no Wails names or icons.
6. Shortcuts that used to come from removed menu items keep working through the windows:
   - ⌘W closes the window that has focus. An editor window with unsaved changes asks first, as its close button does today. The main window closes, and reopens from the Dock as today.
   - ⌘P prints the page in read mode. The page view already handles it; behaviour is unchanged.
   - ⌘R does nothing in production builds.
7. Dev builds (`DEV=true`) add a **Develop** menu with Reload (⌘R) and Toggle DevTools (⌥⌘I), so day-to-day work isn't slowed down. Production builds never show it.
8. The menu is built by one function, `buildMenu(app, actions) *application.Menu`, and its structure is checked by a test that lists every label and accelerator.

## Architecture

- `internal/app/menu.go` holds the menu. `app.go` calls `a.Menu.Set(buildMenu(a, menuActions{…}))`.
- `menuActions` holds the callbacks that `app.go` wires today (new page, open folder, settings), so `menu.go` doesn't depend on `Zen` internals.
- `menuSpec()` returns a plain tree of `{Label, Accel, Role}` for the test. `buildMenu` turns it into Wails items. The App and Edit menus are built item by item from roles (`About`, `Hide`, `HideOthers`, `UnHide`, `Quit`, `Undo`, `Redo`, `Cut`, `Copy`, `Paste`, `SelectAll`, `Minimise`, `Zoom`), not with `AddRole(AppMenu/EditMenu)`, so no extra items come along.
- The frontend's `shell/keys.ts` gains ⌘W through the existing Wails window API (`Window.Close()` on the current window). The editor's unsaved-changes guard already runs on window close, and ⌘W uses it.

## Interfaces

```go
// internal/app/menu.go
type menuActions struct {
	NewPage, OpenFolder, Settings func()
}
type menuItem struct {
	Label, Accel string   // Accel in Wails syntax, "" for none
	Role         string   // Wails role name, "" for a custom item
	Children     []menuItem
	Separator    bool
}
func menuSpec(dev bool) []menuItem
func buildMenu(a *application.App, act menuActions, dev bool) *application.Menu

const EventSettings = "menu:settings"
```

```ts
// frontend/src/shell/keys.ts
export function closeWindowKey(e: KeyboardEvent): boolean; // ⌘W, no other modifiers
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1, 2 | exact menus, labels, accelerators; no View/Services/Speech/DevTools in production | `internal/app/menu_test.go: TestMenuSpecProduction` |
| 3 | New Page and Open Folder call their actions | `internal/app/menu_test.go: TestMenuActions` |
| 4 | Settings action emits `menu:settings`; the shell opens Settings on it | `menu_test.go: TestMenuSettings`; `frontend/src/shell/shell.test.ts: "menu:settings opens Settings"` |
| 5 | About uses the bundle name, version and icon | `build/darwin/package_test.go: TestAboutPanel` (Info.plist keys, no "wails" in the bundle's strings and resources) |
| 6 | ⌘W closes the focused window; the editor guard runs; ⌘R does nothing | `frontend/src/shell/keys.test.ts`; `frontend/src/shell/window.test.ts: "⌘W asks before closing a dirty editor"` |
| 7 | Develop menu only when dev | `menu_test.go: TestMenuSpecDev` |
| 8 | app.go has no menu code left | `menu_test.go: TestAppGoBuildsMenuOnce` (source scan for `Menu.New`) |

Edge cases to cover:
- the shortcuts on the welcome screen: New Page is disabled there, as today, and Open Folder works;
- ⌘, while an editor window has focus brings up the main window's Settings.

## UI

The native menu only; it follows the macOS HIG item names and order. Check by hand in the app:
- every shortcut in behaviour 1 and 6;
- About Zen shows the zen icon.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| The web inspector is reachable in shipped builds | No DevTools item and no inspector in production | `TestMenuSpecProduction` |
| ⌘W loses unsaved edits | It goes through the same close guard as the window button | `window.test.ts` |

## Notes

- **Out of scope:**
  - a Help menu and docs links;
  - Open Recent (beta);
  - Print in the menu (⌘P still works from the page);
  - a View menu with sidebar and zoom items, for later.
- **Integration (done by the lead at merge):**
  - `app.go`: replace the menu block with `a.Menu.Set(buildMenu(a, menuActions{…}, devBuild))`;
  - add `EventSettings` to the frontend `api` event names in `frontend/src/api/index.ts` and `wails.ts`;
  - add a line in `examples/demo` keyboard docs (⌘W, ⌘, from any window).
- **App Store / sandbox:** the App Store requires Quit and an About panel without third-party branding. Both are covered here, and packaging/appstore depends on it.
- **Changelog:** `Changed: a slimmer menu bar — File has New Page and Open Folder; developer items (Reload, DevTools) are gone from release builds.`

## Prompt

> Implement `specs/app/menu/spec.md` following the rules in
> `specs/README.md`. Context: the menu is built at the end of
> `internal/app/app.go` with `AddRole(AppMenu/EditMenu/ViewMenu/WindowMenu)`.
> The Wails roles live in `$(go list -m -f '{{.Dir}}' github.com/wailsapp/wails/v3)/pkg/application/roles.go`.
> Move the menu to `internal/app/menu.go` as `menuSpec` + `buildMenu`, and
> build the App, Edit and Window menus from single roles. Add `menu:settings`
> and ⌘W (`frontend/src/shell/keys.ts`) as specified. Do not launch the app. Done
> when the tests in the table pass and `go vet ./internal/... && go test ./internal/...` and
> `cd frontend && npx tsc --noEmit && npx vitest run` are clean. Report the
> Integration lines, and list the manual checks: the shortcuts and the About panel.
