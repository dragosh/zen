# Changelog

All notable changes to zen are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
follows [Semantic Versioning](https://semver.org) (see docs/VERSIONING.md).

## [Unreleased]

### Changed

- `zen` opens the current folder instead of a single file.

### Added

- `zen <file>` renders a markdown file in the window, rendered and sanitised in Go.
- `task install` links the `zen` binary into `~/.local/bin`.
- Open a folder as a workspace with safe file access (core).
- Global `config.yml`, workspace `.zen.yml` and page frontmatter settings (core).
- Plugin API v1 with fenced, container and inline directive syntax (core).
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
