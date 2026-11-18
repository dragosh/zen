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
task darwin:sign:notarize     SIGN_IDENTITY="..." KEYCHAIN_PROFILE=zenmark-notary # dmg, notarytool, stapler, spctl
task darwin:package:test ARCH=universal                                       # checks the bundle
```

### Setting up the Developer ID certificate (once)

Developer ID needs only the **Developer ID Application** certificate and its
private key in the login keychain. It needs no registered bundle id and no
provisioning profile: those are for the App Store.

1. **Request (CSR), on the Mac that will sign.** Keychain Access → Certificate
   Assistant → *Request a Certificate From a Certificate Authority…*; your email,
   common name "Zenmark Studios", CA email empty, *Saved to disk*. This also
   creates the private key in that Mac's login keychain.
2. **Certificate.** [developer.apple.com → Certificates](https://developer.apple.com/account/resources/certificates)
   → **+** → *Developer ID Application* (G2 Sub-CA) → upload the CSR. Only the
   account holder can create it.
3. **Install.** Download `developerID_application.cer` and double-click it. If it
   shows as not trusted, also install the *Developer ID – G2* intermediate from
   [apple.com/certificateauthority](https://www.apple.com/certificateauthority/).
4. **Check.** `security find-identity -v -p codesigning` should list
   `"Developer ID Application: Zenmark Studios (TEAMID)"`. That quoted name is
   `SIGN_IDENTITY`.

#### Made on another Mac

The certificate is useless without its private key, which stays on the Mac
that made the CSR. Installing only the `.cer` elsewhere gives
"0 valid identities found". To sign on another Mac:

1. On the Mac that made the CSR: Keychain Access → *login* → *My Certificates*,
   expand "Developer ID Application: …" (the private key must be under it),
   right-click → *Export…* → `.p12`, with a password.
2. Copy the `.p12` (AirDrop, USB) and double-click it on the signing Mac; type
   the password only into the macOS dialog.
3. Check with `security find-identity -v -p codesigning`, then delete copies of
   the `.p12` you do not need. Keep one safe backup: CI signing later uses it
   (as a GitHub secret), and Apple allows only a few Developer ID certificates
   per account, so reuse it instead of making one per Mac.

Never commit the `.cer`, `.p12` or their passwords.

### Notarisation profile (once)

```sh
xcrun notarytool store-credentials zenmark-notary \
  --apple-id <apple id> --team-id <TEAMID>   # asks for an app-specific password
```

The app-specific password is made at [account.apple.com](https://account.apple.com)
→ Sign-In and Security → App-Specific Passwords. An App Store Connect API key
(`--key`, `--key-id`, `--issuer`) works too and is better for CI.

### Order

1. `task package SIGN_IDENTITY="Developer ID Application: …"`, then
   `codesign -dvv bin/Zenmark.app` and `spctl -a -vv bin/Zenmark.app`
   ("Unnotarized Developer ID" is expected at this point).
2. `task darwin:sign:notarize SIGN_IDENTITY="…" KEYCHAIN_PROFILE=zenmark-notary`
   (DMG, notarytool, stapler, spctl: "Notarized Developer ID").

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
