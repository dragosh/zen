# Settings and config files

## The settings page

Click the **cog** at the bottom of the sidebar, or press <kbd>⌘,</kbd>. The
settings page takes the place of the page you were reading, and the sidebar
stays. Press <kbd>Esc</kbd> or <kbd>⌘[</kbd>, or choose a page, to go back. A
list of sections is on the left:

- **Appearance**: System, Light or Dark, for every zen window on this Mac.
- **Configuration files**: **config.yml** holds settings for all folders and
  **.zen.yml** the settings for the open folder. **Open** edits one in zen's
  editor.
- **Plugins**: every plugin, with its kind (built in, script, program or
  page), where it comes from and what it runs. An external plugin has an
  **Allow** or **Block** button: a blocked plugin renders as plain markdown
  until you allow it. See
  [Writing your own plugin](../plugins/writing-plugins.md).

### Help for each plugin

Next to the list of plugins, the page shows help for the plugin you select:
what it does, what it adds (blocks, inlines, layouts and options with their
types and defaults) and snippets that show how to use it in a page. Each
snippet has a **Copy** button, and a built-in plugin also shows how the snippet
looks. Use the arrow keys to move in the list and <kbd>Enter</kbd> to select.
**All plugins**, at the top of the list, shows the main snippet of every
plugin on one page, as a cheat sheet.

A missing file is created from a template with every setting commented out,
so opening it changes nothing. zen checks the YAML when you save: a mistake is
reported and the file is not written.

## Which setting wins

From weakest to strongest:

1. zen's defaults
2. the global `config.yml`
   (`~/Library/Application Support/zen/config.yml`)
3. the folder's `.zen.yml`
4. a page's [frontmatter](../reading/frontmatter.md)

Folder limits are the exception: only the global file can change them, so a
downloaded folder cannot lift its own limits.

This folder's own `.zen.yml` is an example (open it from the settings page); see
[all settings](reference.md).
