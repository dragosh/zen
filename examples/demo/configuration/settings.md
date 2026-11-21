---
title: Settings
---
# Settings

Click the **cog** at the bottom of the sidebar (Export is next to it), or press <kbd>⌘,</kbd>. The
Settings page takes the place of the page you were reading, and the sidebar
stays. Press <kbd>Esc</kbd> or <kbd>⌘[</kbd>, or choose a page, to go back.
Sections are listed on the left; the selected item shows on the right.

- **Configuration**: **config.yml** holds settings for all folders and
  **.zenmd.yml** the settings for the open folder. Each shows its text;
  **Open in Editor** edits it (**Create** when it doesn't exist yet), and
  **Reveal in Finder** shows the file in Finder. See the
  [`.zenmd.yml` reference](reference.md).
- **Appearance**: System, Light or Dark, for every Zenmark window on this Mac.
- **Accessibility**: zoom, reduced motion, increased contrast, and for the
  reader the text size, font, line spacing, line width and underlined links.
- **Reader**: the Markdown syntax Zenmark reads. CommonMark and GitHub Flavored
  Markdown are always on, and MyST can be added; see the [reader](../features/reader.md).
- **Layouts**: the [layouts](../layouts/index.md) of this build, with the help
  of the selected one.
- **Plugins**: the [plugins](../plugins/index.md) of this build with their
  state, what each adds and snippets that show how to use it in a page.
  Each snippet has a **Copy** button. **All plugins**, at the top of the list,
  shows the main snippet of every plugin on one page, as a cheat sheet.
- **Export**: the [website and PDF](../features/export.md) of this folder,
  with the same choices as File › Export.
- **Developer**: one switch, **Show Console in the Inspector**; see
  [the Developer page](../features/developer.md).

Use the arrow keys to move in a list and <kbd>Enter</kbd> to select.

## Which setting wins

From weakest to strongest:

1. Zenmark's defaults
2. the global `config.yml`
   (`~/Library/Application Support/zen/config.yml`)
3. the folder's `.zenmd.yml`
4. a page's [frontmatter](../intro/frontmatter.md)

Folder limits are the exception: only the global file can change them, so a
downloaded folder cannot lift its own limits.

This folder's own `.zenmd.yml` is an example; open it from Settings ›
Configuration, or see [the reference](reference.md).
