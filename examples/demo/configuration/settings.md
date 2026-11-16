# Settings and config files

## The settings panel

Click the **cog** at the bottom of the sidebar, or press <kbd>⌘,</kbd>.

- **Appearance**: System, Light or Dark, for every zen window on this Mac.
- **config.yml**: settings for all folders. **Open** edits it in zen's editor.
- **.zen.yml**: settings for the open folder.
- **Plugins**: every plugin, where it comes from and what it runs. An external
  plugin has an **Allow** or **Block** button: a blocked plugin renders as
  plain markdown until you allow it. See
  [Writing your own plugin](../plugins/writing-plugins.md).

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

This folder's own `.zen.yml` is an example (open it from the cog); see
[all settings](reference.md).
