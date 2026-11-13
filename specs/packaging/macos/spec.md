# packaging/macos

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/packaging-macos` |
| Wave | 4 |
| Depends on | everything else |
| Owns | `build/darwin/` (except the version stamping lines owned by packaging/version), `build/appicon.*`, `resources.yml` |

## Purpose

`task package` produces `bin/Zen.app`: one bundle containing the app, the
`zen` command-line binary and any helper binaries.

## Behaviour

1. `task package` builds `bin/Zen.app` with `Contents/MacOS/zen` (the single binary: GUI and CLI), `Info.plist` (name `Zen`, id `io.github.dragosh.zen`, version per docs/VERSIONING.md, document type markdown and folders, minimum macOS 12), icon, and `Contents/Helpers/` holding every binary listed in `resources.yml` for the target arch.
2. `Zen.app/Contents/MacOS/zen --version` prints the stamped version; double-clicking the app opens the welcome screen (no cwd workspace when launched by Finder: detected by the absence of a TTY and cwd `/`).
3. Opening a folder or markdown file with the app (Finder "Open With", `open -a Zen docs/`) opens that workspace.
4. `task install:cli` links `/usr/local/bin/zen` (or `INSTALL_DIR`) to the binary inside the installed app; `task install` (dev link) keeps working.
5. Entitlements file declares app sandbox with `files.user-selected.read-write` and `files.bookmarks.app-scope`; the alpha is ad-hoc signed (`codesign -s -`), with a documented switch for a Developer ID.
6. `resources.yml` schema: list of `{name, path: {darwin-arm64, darwin-amd64}, sha256}`; a missing file or checksum mismatch fails the build. Empty list is valid.
7. `Info.plist` versions are generated from `VERSION` at build time (`task version:bump` does not touch the plists).
8. App icon: yin-yang mark from docs/UI.md, black and white, generated from `build/appicon.svg`.

## Architecture

Taskfile + plist templates in `build/darwin`; a small Go test package
`build/darwin/package_test.go` inspecting the produced bundle (skipped with
`-short` or off macOS).

## Interfaces

Tasks: `package`, `install:cli`, `darwin:sign`.

## Tests

| # | Behaviour | Test (`build/darwin/package_test.go`) |
|---|---|---|
| 1 | bundle layout, plist keys, version | `TestBundleLayout` |
| 2 | bundled binary reports version | `TestBundledCLI` |
| 5 | entitlements present, signature verifies (`codesign --verify`) | `TestSignature` |
| 6 | resources manifest validation | `TestResourcesManifest` |

Manual check list (reported, not automated): double-click launch, Open With, dark/light icon.

## UI

App icon and name only.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Shipping a tampered helper binary | sha256 in `resources.yml` verified at build | `TestResourcesManifest` |
| Over-broad file access | sandbox entitlements limited to user-selected files | `TestSignature` (entitlements dump) |

## Notes

- **Out of scope:** Developer ID signing, notarisation, DMG, App Store submission, auto-update (beta specs).
- **Integration:** none.
- **Changelog:** Added: `Zen.app` bundle with the `zen` command-line tool inside.

## Prompt

> Implement `specs/packaging/macos/spec.md` following the rules in `specs/README.md`. The
> existing Wails Taskfiles in `build/darwin` already package an app; adapt rather than
> rewrite. Done when `task package` produces a bundle that passes the tests on this Mac,
> and you have listed the manual checks you could not perform.
