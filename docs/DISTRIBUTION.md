# Distribution

zen.md ships two ways from one source tree (spec: `specs/packaging/appstore/spec.md`).
The channel is `CHANNEL=devid` (default) or `CHANNEL=appstore`. Nothing secret is
in the repository: identities, the Team ID and the provisioning profile come from
the environment.

| | Developer ID | Mac App Store |
|---|---|---|
| Package | `bin/Zen.app`, `.dmg` | `bin/Zen.pkg` (Transporter) |
| Sandbox | no | yes (`entitlements.sandbox.plist`) |
| Go build tag | none | `appstore` |
| `zen` command, program/tool/sandbox plugins | yes | off ("Not available in the App Store version of zen") |
| Tengo and JS plugins, layouts, Vale | yes | yes |
| Identity | `Developer ID Application` | `Apple Distribution` + `3rd Party Mac Developer Installer` |

The product name, bundle id, copyright and category are in `build/config.yml`
(`info:`) and are written into `Info.plist` by `build/darwin/plist`. The name shown
in the DMG art comes from `info.productName`; regenerate the art with `task darwin:art`
(also run by `task icon`).

## Environment

| Variable | Used for |
|---|---|
| `SIGN_IDENTITY` | the signing identity (default `-`, ad hoc) |
| `TEAM_ID` | Apple Team ID (App Store application identifier) |
| `PROFILE` | path of the Mac App Store provisioning profile, embedded as `Contents/embedded.provisionprofile` |
| `INSTALLER_IDENTITY` | `productbuild --sign` |
| `KEYCHAIN_PROFILE` | `notarytool` keychain profile (`xcrun notarytool store-credentials`) |
| `ADHOC=1` | local dry run of the App Store build: ad-hoc signature, no profile, no `.pkg` |

## Developer ID

```sh
task darwin:package:universal SIGN_IDENTITY="Developer ID Application: ..."   # arm64 + x86_64, helpers too
task darwin:sign:notarize     SIGN_IDENTITY="..." KEYCHAIN_PROFILE=zen-notary # dmg, notarytool, stapler, spctl
task darwin:package:test ARCH=universal                                       # checks the bundle
```

## Mac App Store

```sh
task darwin:package:appstore SIGN_IDENTITY="Apple Distribution: ..." TEAM_ID=... \
     PROFILE=zen.provisionprofile INSTALLER_IDENTITY="3rd Party Mac Developer Installer: ..."
xcrun altool --validate-app -f bin/Zen.pkg -t macos   # or open the .pkg in Transporter
```

The build fails early without `PROFILE`, and fails without `actool` (full Xcode 26
or later), because the store needs the compiled icon catalog. A Developer ID build
without `actool` only warns and keeps `icons.icns`.

## Decisions still open (owner)

1. Apple Developer account and Team ID.
2. Bundle id (`io.github.dragosh.zen` is a placeholder; a domain you own is better).
3. Store name ("zen.md" may be taken; keep a display name that differs from the listing if needed).
4. App Store feature set: Tengo and JS plugins stay in the proposal (App Review 2.5.2 is the main risk; fallback: built-ins only).
5. Copyright holder (`© 2026 zen.md authors` is a placeholder).

## App Store Connect checklist (not built here)

- App record with the bundle id; category Productivity; age rating; price.
- Screenshots, description, keywords, support URL, privacy policy URL.
- Privacy "nutrition label": data not collected (see `PrivacyInfo.xcprivacy`).
- Export compliance: `ITSAppUsesNonExemptEncryption = false` is set.
- Upload with Transporter, test with TestFlight, then submit for review.
