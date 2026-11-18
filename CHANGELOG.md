# Changelog

All notable changes to zen are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
follows [Semantic Versioning](https://semver.org) (see docs/VERSIONING.md).

## [Unreleased]

### Fixed

- Diagrams render again (the diagram script now exports `mount`, as the plugin runtime expects).
- Reload keeps the page you were reading.

- Plugin hardening (docs/plugin-review-2026-11.md): plugins can no longer hang or exhaust zen, run code that was not approved, or reach other plugins from page scripts.
- Slides: arrow keys and Space in the sidebar no longer also move the slides.
- Page titles keep `<`, `>` and decode entities in the sidebar and breadcrumb.
- Zen.app no longer hangs when opened from Finder.

### Changed

- Engine: the browser runtime (plugin and layout mounting, built sites, print, page CSS) is `runtime/` (`@zenmd/runtime`), embedded in Go as package `runtime`; the app and `zenmd build` use one build of it. Built sites are byte for byte the same. CI runs on actions v7 and Node 24.

- Engine: the Go module is `github.com/dragosh/zenmd` (the repository was renamed); the engine packages are public under `engine/` (documented, with an "embed zenmd" example) and the default plugins, layouts and site templates under `builtins/`. CI checks the API with apidiff. No visible change.

- The command is now `zenmd`, the zen.md engine. `zen` still works until 0.4.0 and says so on stderr. Run `task install` again for both links (`task install:cli` for /usr/local/bin needs sudo).
- Engine: the folder session (opening, rendering, plugins, layouts, build, print, templates) is a headless Go package, `engine/session`; the app is a thin layer over it. No visible change.

- zen.md-branded DMG art (generated from the icon) and file icon; no Wails template art or comments left. Info.plist gains the product name, category, copyright and export-compliance keys, and the bundle ships a privacy manifest.

- The app is now called **zen.md** (window titles, menus, About, welcome screen, README). The command line tool is still `zen`, and the settings folder and plugin APIs keep their names.

- Opening a big folder is faster: the folder is walked once, and live reload starts watching in the background (~/Developer, 945 pages: 3 s → 0.6 s).

- A slimmer menu bar: File has New Page and Open Folder; developer items (Reload, DevTools) are gone from release builds. ⌘W closes the focused window; ⌘, opens Settings from any window.

- Plugins can no longer declare layouts (`layouts:` in plugin.yml or a .tengo file is an error pointing to `.zen/layouts/`).
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

- Two distribution channels: `task package` (Developer ID: signed, notarised `.dmg`, everything included) and `task package:appstore` (sandboxed Mac App Store `.pkg`, without program, tool and container plugins or the `zen` command). See docs/DISTRIBUTION.md; the App Store build still needs an Apple account and owner decisions.

- Site templates: `zen init site --techdocs|--learning|--planner` and File › New Site from Template… (⇧⌘N, also on the welcome screen) start a folder from editable presets kept in your templates folder (technical docs after Diátaxis, a learning path, a planner). See docs/TEMPLATES.md.
- Print Page (⌘P) and Print Site (⇧⌘P): the whole folder as one document with a cover and contents, through the print panel (Save as PDF); `zen build` writes `print.html` (`--no-print` to skip).
- File › Build Website… (⇧⌘B) and Settings › Website: build the open folder as a static website from the app, with progress, warnings by page, and Open in Browser.

- A loader while a folder opens: the folder's name, what zen is doing (reading the folder, indexing pages) with counts, and Cancel. Opening stops after `limits.openTimeout` seconds (default 30) and keeps the folder that was open. The folder given at launch now opens after the window shows.

- `zen build`: export a folder as a static website that opens from disk or any web server (`--hosted` for a web-server-only build), rendered with the same plugins and layouts as the app.
- Layouts are their own concept, separate from plugins: `.zen/layouts/<id>/layout.yml` with an HTML template (Go `html/template`), CSS and an optional script; folder defaults and path patterns in `.zen.yml`; Settings › Layouts and the New Page picker list them. New built-in layouts `docs`, `post`, `landing`, `wide`, `handout` (slides and course unchanged). Demo: a `memo` folder layout and a `blog/` folder using `layouts: "blog/**": post`. See docs/LAYOUTS.md.
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
