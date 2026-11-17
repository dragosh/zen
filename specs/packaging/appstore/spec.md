# packaging/appstore

| | |
|---|---|
| Status | in progress: build, signing, art, plist, privacy, universal, Go gate and folder-access code done; needs lead wiring (see report) and real identities / full Xcode to verify the signed App Store build |
| Target version | 0.2.0 (Developer ID build) · 0.3.0 (first App Store submission) |
| Branch | `feature/packaging-appstore` |
| Wave | 12 |
| Depends on | packaging/macos, app/menu (About panel, no developer items) |
| Owns | `build/darwin/**`, `build/config.yml`, `build/appicon.*`, `build/Taskfile.yml`, `resources.yml`, `internal/app/channel*.go` + tests (new), `internal/core/extplugins/channel*.go` + tests (new), `docs/DISTRIBUTION.md` (new, lead reviews) |

## Purpose

Ship zen two ways from one source tree:

1. a **Developer ID** build: a signed, notarised `.dmg` with the full feature set, the `zen` command and external plugins;
2. a **Mac App Store** build: sandboxed, signed for the store, and uploadable with Transporter.

Remove every leftover of the Wails project template (icons, DMG art,
config comments), so nothing a user sees says Wails.

## Behaviour

### A. Branding clean-up (both builds)
1. No Wails artwork or names in anything a user sees:
   - the DMG background and the DMG file icon are replaced with zen art made from `build/appicon.svg` (black and white, editorial font, the "Drag Zen to Applications" arrow);
   - `build/config.yml` loses the Wails template comments and the `wailsFileIcon` example;
   - the bundle's `Resources`, `Info.plist` and the DMG contain no `wails`/`Wails` string or image.
2. The DMG art is generated, not hand-edited: `go run ./build/darwin/dmgart` writes `dmg-background.png` (540×380 and @2x) and `dmg-file-icon.icns` from `build/appicon.svg`, so it can be regenerated whenever the icon changes. `task icon` runs it too.
3. The app icon ships as an **asset catalog** (`Assets.car`, compiled from `build/appicon.icon` with `actool`) **and** as `icons.icns` (for macOS 13–14). `CFBundleIconName` points to it. The 1024 px App Store icon is included. If `actool` is missing (no Xcode), the build fails for `CHANNEL=appstore` and warns for Developer ID builds.

### B. Info.plist and metadata (both builds)
4. Info.plist adds:
   - `LSApplicationCategoryType = public.app-category.productivity`;
   - `ITSAppUsesNonExemptEncryption = false`;
   - `NSHumanReadableCopyright = © 2026 <owner>`, taken from `build/config.yml` `info.copyright`;
   - `CFBundleDisplayName`/`CFBundleName` from `info.productName`.

   `LSMinimumSystemVersion` stays at 13.0. The bundle id comes from `info.bundleId`, the same in both builds; see Decisions.
5. A privacy manifest `PrivacyInfo.xcprivacy` is in `Contents/Resources`. It declares no tracking and no collected data, plus the "required reason" APIs zen uses: file timestamps (`C617.1`, `3B52.1`) and disk space, only if a dependency uses it (checked with `TestPrivacyManifest` against a symbol scan).
6. **Universal binary**: `zen` and every helper (Vale) for arm64 + x86_64. `resources.yml` gains the x86_64 Vale entry with its checksum.

### C. Developer ID channel (`task package CHANNEL=devid`, the default)
7. The binary and the helpers are signed with `SIGN_IDENTITY` ("Developer ID Application: …"), hardened runtime, timestamp, and `entitlements.plist` (no sandbox, as today). `task sign:notarize` submits the `.dmg` with `notarytool`, staples it, and checks it with `spctl -a -t open --context context:primary-signature`.
8. Everything works as today: the `zen` CLI, any folder from a terminal, external program backends, the container sandbox plugin.

### D. Mac App Store channel (`task package CHANNEL=appstore`)
9. **Signing:**
   - the app is signed with `entitlements.sandbox.plist`:
     - `app-sandbox`;
     - `files.user-selected.read-write`;
     - `files.bookmarks.app-scope`;
     - `network.client`, only if a decision below keeps a feature that needs it;
   - the helpers are signed with `helper.entitlements.sandbox.plist` (`app-sandbox` + `inherit`);
   - signing uses "Apple Distribution" plus the embedded `embedded.provisionprofile` (from `PROFILE=`);
   - `productbuild --sign "3rd Party Mac Developer Installer: …"` makes `bin/Zen.pkg`;
   - `xcrun altool --validate-app` (or Transporter) is documented, not run by the build.
10. **Folder access in the sandbox:**
    - Open Folder… and Finder "Open With" grant access.
    - zen stores a security-scoped bookmark for each opened folder. Reopening the last folder at launch resolves it (`startAccessingSecurityScopedResource`), and a stale bookmark asks again with the folder panel preselected.
    - Opening a single `.md` file grants that file only, so zen asks once for its folder (panel preselected, "Allow zen to show the pages next to this file") so it can build the sidebar. If the user declines, zen shows the file alone.
11. **Features in the App Store build:** a build-time channel (`go build -tags appstore`) switches off what the sandbox or App Review does not allow. Each switched-off feature shows a clear note in Settings and on the page ("Not available in the App Store version of zen"), never an error.
    - **Off:**
      - external **program backends**, which run a binary from the folder;
      - the **sandbox** plugin (`container`);
      - **tool** plugins;
      - the bundled **`zen` CLI** and "Install command line tool".
    - **Kept:**
      - built-in plugins and layouts;
      - **Tengo** script plugins, which run in zen's own helper inside the bundle;
      - frontend JS plugins and folder layouts, after the same trust prompt;
      - Vale (bundled helper).
12. Global config, trust and plugin storage live under the app container. The sandbox remaps `os.UserConfigDir()` there. On first run of the App Store build, zen offers once to import `~/Library/Application Support/zen/config.yml` through a file panel; the sandbox does not allow reading it silently.
13. No code path in the App Store build calls `exec` on anything outside `Contents/` (the bundle). A test checks this.

### E. Checks
14. `task package:test CHANNEL=…` checks the built bundle:
    - signature and entitlements per channel;
    - Info.plist keys;
    - the privacy manifest;
    - no Wails strings or images;
    - `Assets.car` contains `appicon` (`assetutil --info`);
    - every Mach-O is universal and signed;
    - helpers have the inherit entitlements in the App Store build.

## Architecture

- `build/darwin/Taskfile.yml`:
  - a `CHANNEL` variable (`devid` | `appstore`) picks the entitlements, the identity, the build tags (`appstore`), the profile embedding and the output (`.dmg` vs `.pkg`);
  - new tasks `package:appstore`, `sign:appstore`, `art`;
  - `create:dmg` uses the generated art.
- `build/darwin/dmgart/main.go` (new, stdlib + QuickLook for the SVG like `task icon`): writes the DMG art.
- `internal/app/channel.go` / `channel_appstore.go` (build tags): `const appStore bool`, plus `func bookmarks() folderAccess` (a no-op in Developer ID builds; Cocoa via the Wails darwin bridge in App Store builds).
- `internal/core/extplugins/channel*.go`: the build-tagged gate that refuses program backends and tool plugins in App Store builds (reported as `status: "unavailable"` with the reason, distinct from blocked).
- Security-scoped bookmarks:
  - a small cgo file in `internal/app` (`bookmarks_darwin_appstore.go`, `NSURL bookmarkDataWithOptions:` / `URLByResolvingBookmarkData:`);
  - stored next to the trust file in the container;
  - tested through an interface fake on CI and by hand in the signed build.

## Interfaces

```go
// internal/app
const appStore bool                               // from build tags
type folderAccess interface {
	Remember(dir string) error                    // store a security-scoped bookmark
	Resolve(dir string) (stop func(), err error)  // begin access; ErrStale → ask again
}

// internal/core/extplugins
var ErrUnavailable = errors.New("not available in the App Store version of zen")
func Available(kind string) error                 // "program" | "tool" | "sandbox" → ErrUnavailable in appstore builds
```

```yaml
# build/config.yml (fields used by the Taskfile)
info:
  productName: Zen
  bundleId: io.github.dragosh.zen
  copyright: "© 2026 …"
  category: public.app-category.productivity
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | no Wails strings or images in the bundle, plist, DMG art or config | `build/darwin/package_test.go: TestNoWailsBranding` (strings + image hashes of the Wails template files) |
| 2 | DMG art generated from the icon, deterministic | `build/darwin/dmgart/main_test.go: TestArt` |
| 3 | `Assets.car` with `appicon`, `.icns` fallback | `package_test.go: TestIconAssets` |
| 4 | Info.plist keys and values | `package_test.go: TestInfoPlist` |
| 5 | privacy manifest present and matches the APIs used | `package_test.go: TestPrivacyManifest` |
| 6 | universal Mach-Os | `package_test.go: TestUniversal` |
| 7 | Developer ID: hardened runtime, no sandbox; notarise task wiring | `package_test.go: TestSignatureDevID` (`ZEN_REQUIRE_BUNDLE=1`) |
| 9 | App Store: sandbox and inherit entitlements, profile, `.pkg` | `package_test.go: TestSignatureAppStore` |
| 10 | bookmarks remembered, resolved and re-asked when stale; single-file → folder prompt | `internal/app/channel_test.go: TestFolderAccess` (fake) |
| 11 | program/tool/sandbox plugins unavailable with the reason; Tengo and JS kept | `internal/core/extplugins/channel_test.go: TestAppStoreGate` (`-tags appstore`) |
| 12 | config import offered once | `internal/app/channel_test.go: TestConfigImportOnce` |
| 13 | no exec outside the bundle in appstore builds | `internal/app/channel_test.go: TestNoExecOutsideBundle` (source scan of `exec.Command` callers under the tag) |

Edge cases to cover:
- a folder that was moved, which leaves a stale bookmark;
- a folder on an external disk that is not mounted;
- a Developer ID build with no `actool`: a warning, and the build still succeeds;
- `CHANNEL=appstore` without `PROFILE`, which fails early with a clear message.

## UI

- The DMG window shows the zen art: icon left, Applications right, an arrow between them, white background, black type.
- In the App Store build, a feature that is switched off shows a muted note in Settings › Plugins and in the page's plugin box: "Not available in the App Store version of zen". There is no Allow button for it.
- The folder-access prompt is the native open panel, preselected, with a message line.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| The App Store build runs code from a folder outside the sandbox's intent | Program, tool and container plugins are refused at load under the `appstore` tag | `TestAppStoreGate`, `TestNoExecOutsideBundle` |
| Over-broad entitlements | Only user-selected read-write and app-scope bookmarks; no `files.all`, no temporary exceptions | `TestSignatureAppStore` (entitlement dump equals the expected set) |
| Helpers escape the sandbox | They are signed with `inherit` and run inside the parent's sandbox | `TestSignatureAppStore` |
| Notarisation secrets in the repo | Identity and keychain profile come from the environment or `wails3 setup`; nothing is committed | `TestNoSecrets` (grep the Taskfile and plist for team ids and passwords) |

## Notes

- **Decisions for the owner** (needed before the App Store submission, not before building):
  1. **Apple Developer account and Team ID.** The build reads `TEAM_ID`, `SIGN_IDENTITY`, `INSTALLER_IDENTITY` and `PROFILE` from the environment.
  2. **Bundle id.** `io.github.dragosh.zen` works but is tied to GitHub. Recommended: a domain you own, for example `app.zen.reader`.
  3. **Store name.** "Zen" is very likely taken on the App Store. Recommended: "Zen Markdown" as the store name, keeping `Zen` as the bundle display name.
  4. **App Store feature set.** This spec keeps Tengo and JS plugins; App Review guideline 2.5.2 (downloaded or executed code) is the main review risk. The fallback is built-ins only.
  5. **Copyright holder** for `NSHumanReadableCopyright`.
- **Out of scope:**
  - TestFlight and App Store Connect metadata (screenshots, description), which go in `docs/DISTRIBUTION.md` as a checklist;
  - auto-update for the Developer ID build (Sparkle, later);
  - Windows and Linux packages.
- **Integration (done by the lead at merge):**
  - `Taskfile.yml`: pass `CHANNEL` through `package`;
  - `.gitignore`: `*.provisionprofile`, `bin/*.pkg`;
  - README: an Install section for the two channels;
  - `docs/PLAN.md` milestones: the App Store at 0.3.0;
  - the Settings page shows `ErrUnavailable` statuses: the shared `status` union in `frontend/src/api/index.ts` gains `"unavailable"`.
- **App Store / sandbox:** this spec.
- **Changelog:**
  - `Changed: zen-branded DMG and icons; no Wails template art.`
  - `Added: Developer ID notarised build and a Mac App Store build (sandboxed).`

## Prompt

> Implement `specs/packaging/appstore/spec.md` following the rules in
> `specs/README.md`. Context:
> - Packaging lives in `build/darwin/Taskfile.yml` (`create:app:bundle`, `codesign`, `create:dmg`, `sign:notarize`).
> - The bundle tests are in `build/darwin/package_test.go` (`ZEN_REQUIRE_BUNDLE=1`).
> - The Wails template art is `build/darwin/dmg-background.png` and `dmg-file-icon.{png,icns}`.
> - The zen icon source is `build/appicon.svg` and `build/appicon.icon`.
>
> Do A and B first, then C, then D. Use no real signing identity: test with ad-hoc signatures, and skip with a clear message when an identity or profile is missing. Do not run notarisation or uploads, do not launch the app, do not start containers. Done when the tests in the table pass for both channels (signature tests may skip without identities) and `go test ./build/... ./internal/...` is clean. Report:
> - what needs a real identity to verify;
> - the owner decisions still open;
> - the Integration lines.
