---
slug: open
nav:
---
# Open a folder

zen always works on **one folder** per window. Everything it shows, edits and
watches stays inside that folder.

## From the app

- Click **Open a folder** on the welcome screen, or **Open…** at the top right
  of the sidebar, or press <kbd>⌘O</kbd>.
- In Finder, right-click a folder or a `.md` file and choose
  **Open With › Zen**, or drop it on the Dock icon.

## From a terminal

| Command | Opens |
|---|---|
| `zenmd` | the current folder |
| `zenmd docs/` | that folder |
| `zenmd notes/idea.md` | the nearest folder above it with a `.zenmd.yml` or `.git`, at that page |
| `zenmd edit README.md` | the folder, plus an editor window for the page |
| `zenmd --help` / `zenmd --version` | help, version |

## What zen shows first

If the folder has a `README.md` at its root, zen opens it. Otherwise pick a
page in the sidebar.

## Very large folders

zen refuses folders with more than **2,000 markdown files** or **50,000 files
and folders**, so opening your home folder by accident is harmless. Raise the
limits in the [global settings](../configuration/settings.md) if you need to.
