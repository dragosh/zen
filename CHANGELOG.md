# Changelog

All notable changes to zen are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
follows [Semantic Versioning](https://semver.org) (see docs/VERSIONING.md).

## [Unreleased]

### Fixed

- Plugin hardening (docs/plugin-review-2026-11.md): plugins can no longer hang or exhaust zen, run code that was not approved, or reach other plugins from page scripts.
- Slides: arrow keys and Space in the sidebar no longer also move the slides.
- Page titles keep `<`, `>` and decode entities in the sidebar and breadcrumb.
- Zen.app no longer hangs when opened from Finder.

### Changed

- The `highlight` plugin is now `codeblock`: rename `highlight:` to `codeblock:` under `plugins` in your config.
- The sandbox hides the container tool's progress lines and shows a spinner until the first output (needs `container` 1.7 or later).
- The Open… button moved to the top-right corner of the sidebar.
- Code blocks in the reader use a colour theme (shared with the editor) instead of grayscale.
- Sidebar: clearer disclosure chevrons and guide lines for nested pages.
- Edit opens the page in its own editor window (editor only, no split preview); the main window follows the unsaved text live.
- File › Open Folder (⌘O) from the menu switches the window to the new folder, also from the welcome screen.
- `zen` opens the current folder instead of a single file.
- Settings is a page in the main window (cog or ⌘,; Esc or Back returns) with Appearance, Configuration files and Plugins sections, instead of a popover.

### Added

- Plugin help: Settings › Plugins shows help for each plugin next to the list (summary, what it adds, snippets with a Copy button, a preview for built-ins) and an "All plugins" cheat sheet. Plugins add it with a `help:` key in `plugin.yml` or the Tengo `plugin` map; all built-in and demo plugins have it.
- Script plugins: a single `.tengo` file is a plugin (blocks, inlines, layouts, actions), with a `zen` module for HTML, nested markdown, storage and, with permission, reading the folder. Scripts run in a helper process with time, allocation and memory limits, so a broken script can never crash zen. Samples: `callout.tengo`, `wordcount/plugin.tengo`.
- File › New Page… (⌘N) asks for a name and a layout (page, slides or course) and opens the new page in the editor.
- Editor: the Problems panel has severity tabs and can be resized from its top edge.
- Slides: full page mode (F or the Full page button) that covers the window and goes full screen; Esc leaves.
- Copy and Wrap buttons inside every code block.
- `slides` and `course` page layouts (`layout:` in frontmatter), with keyboard control, printing and saved course progress.
- `quiz` plugin: single, multiple and free-text questions in markdown; answers checked in Go, attempts and best score saved.
- `sandbox` plugin: run a code block in a container image from your own allow-list, using the macOS `container` command, with live output.
- Plugin streams (live output), user-only plugin options, sidebar badges, Print (⌘P).
- External plugins: a folder with `plugin.yml` can add a backend program, an existing tool, page scripts with saved data, or a page layout. zen asks before running a plugin for the first time; Settings lists plugins with Allow / Block.
- Demo plugins `note` (shell), `calc` (`bc`) and `progress` (page script), and a how-to in `examples/demo/plugins/writing-plugins.md`.
- `examples/demo/`: a user guide that is also a demo of every feature (plugins, frontmatter, slugs, settings, Vale with a custom vocabulary). Open it with `cd examples/demo && zen`.
- Settings files open in zen's own editor (YAML colours, line numbers; checked before saving, applied right away).
- Settings cog at the bottom of the sidebar (⌘,): theme (System / Light / Dark, all windows) and buttons that open `config.yml` and the folder's `.zen.yml`, created from a commented template if missing.
- Editor: line numbers and syntax highlighting for markdown and fenced code (colour, light and dark).
- Frontmatter `slug` and `nav` (title, order, parent, hidden) to tune the sidebar.
- Vale spelling and style marks in the reader and editor as coloured wavy squiggles, plus a problems console in the editor explaining each flag (`plugins: {lint: false}` turns them off).
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
