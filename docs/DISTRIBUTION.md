# Distribution

Zenmark ships two ways from one source tree (spec: `specs/packaging/appstore/spec.md`).
The channel is `CHANNEL=devid` (default) or `CHANNEL=appstore`. Nothing secret is
in the repository: identities, the Team ID and the provisioning profile come from
the environment.

| | Developer ID | Mac App Store |
|---|---|---|
| Package | `bin/Zenmark.app`, `.dmg` | `bin/Zenmark.pkg` (Transporter) |
| Sandbox | no | yes (`entitlements.sandbox.plist`) |
| Go build tag | none | `appstore` |
| `zenmd` command, program/tool/sandbox plugins | yes | off ("Not available in the App Store version") |
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
xcrun altool --validate-app -f bin/Zenmark.pkg -t macos   # or open the .pkg in Transporter
```

The build fails early without `PROFILE`, and fails without `actool` (full Xcode 26
or later), because the store needs the compiled icon catalog. A Developer ID build
without `actool` only warns and keeps `icons.icns`.

## Decided

- Name **Zenmark** (trademark search clear; no App Store app by that name), bundle `Zenmark.app`, bundle id `org.zenmark.app`, domain [zenmark.org](https://zenmark.org), GitHub organisation [zenmark-org](https://github.com/zenmark-org).
- Author **Zenmark Studios**: `© 2026 Zenmark Studios. All rights reserved.` in Info.plist (About Zenmark). The code is proprietary (LICENSE).
- Apple Developer account: yes (Developer ID signing is being set up).

## Decisions still open (owner)

1. Reserve the name "Zenmark" in App Store Connect (needs the App Store bundle id registered).
2. App Store feature set: Tengo and JS plugins stay in the proposal (App Review 2.5.2 is the main risk; fallback: built-ins only).

## App Store Connect checklist (not built here)

- App record with the bundle id; category Productivity; age rating; price.
- Screenshots, description, keywords, support URL, privacy policy URL.
- Privacy "nutrition label": data not collected (see `PrivacyInfo.xcprivacy`).
- Export compliance: `ITSAppUsesNonExemptEncryption = false` is set.
- Upload with Transporter, test with TestFlight, then submit for review.
