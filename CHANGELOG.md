# Changelog

All notable changes to zen are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
follows [Semantic Versioning](https://semver.org) (see docs/VERSIONING.md).

## [Unreleased]

### Fixed

- Page titles keep `<`, `>` and decode entities in the sidebar and breadcrumb.
- Zen.app no longer hangs when opened from Finder.

### Changed

- Edit opens the page in its own editor window, connected to the main window.
- File › Open Folder (⌘O) from the menu switches the window to the new folder, also from the welcome screen.
- `zen` opens the current folder instead of a single file.

### Added

- Frontmatter `slug` and `nav` (title, order, parent, hidden) to tune the sidebar.
- Vale spelling and style marks in the reader and editor (`plugins: {lint: false}` turns them off).
- Vale prose linter (3.25.1) bundled in Zen.app, fetched and checksum-verified at build time.
- `zen <file>` renders a markdown file in the window, rendered and sanitised in Go.
- `task install` links the `zen` binary into `~/.local/bin`.
- Open a folder as a workspace with safe file access (core).
- Global `config.yml`, workspace `.zen.yml` and page frontmatter settings (core).
- Plugin API v1 with fenced, container and inline directive syntax (core).
- `Zen.app` bundle with the `zen` command inside, yin-yang icon, ad-hoc signed, not sandboxed; `task package`, `task install:cli`.
- Folders with more than 2,000 markdown files or 50,000 files and folders are refused with a clear message (e.g. a home folder); `limits.files` / `limits.entries` in the global `config.yml` change them.
- Open a folder or markdown file with Zen from Finder (Open With, `open -a Zen <path>`).
- Folder workspaces in the app: folder picker, saving, live reload, confirm before opening external links, confirm before closing with unsaved edits.
- Syntax highlighting plugin (grayscale).
- Mermaid diagram plugin (mermaid 11.4.1 bundled).
- Go-side markdown rendering with plugin pipeline and sanitising.
- Navigation tree built from links between pages, sidebar and breadcrumb.
- Live reload when files change on disk (core).
- Page view with in-place link navigation and external link confirmation.
- Markdown editor with toolbar, live preview and split view.
- `zen edit [path]`.
- Application frame, welcome screen and black-and-white theme.
- SemVer versioning from a single `VERSION` file.
- Project plan, UI design, plugin system and feature specs for 0.1.0-alpha.1.
