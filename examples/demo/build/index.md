---
title: Build a website
nav:
  title: Build a website
---
`zen build` turns this folder into a static website: one HTML page for each
markdown page, the sidebar and breadcrumb written into every page, and the
images and plugin files it uses copied next to it. Put the result on any
static host.

```sh
zen build                       # writes _site next to your pages
zen build -o public --hosted    # for a web server only
```

## What you get

- Every page the app lists becomes an `.html` file with the same path.
  `getting-started/install.md` is `getting-started/install.html`, and the
  README is also `index.html`.
- Links between pages become relative links, so the site works from any
  folder on a host.
- Plugins and layouts that render on the page are run for you. Their output is
  plain HTML.
- Pages marked `draft: true` in their frontmatter are left out. Add
  `--drafts` to include them.

## Plugins and trust

A build never asks you anything. Built-in plugins and layouts are always used.
A plugin or layout of the folder, or one of your own, is used only when you
have already allowed it in the app; otherwise it is left out and the build
prints a warning that says so.

A script that needs the app, such as a plugin that runs a program when you
click, shows its usual error box on the website: *runs in the zen app only*.

## Opening from disk

By default every page carries the scripts it needs, so the folder works when
you double-click `index.html`, even zipped and moved, and on any web server.
Scripts that import other files are left out, with a warning. Single-file
scripts work.

`--hosted` keeps the scripts as separate files instead. The pages are smaller,
but browsers refuse to run those scripts from disk: open a hosted site through
a web server.

## In the app

**File › Build Website…** (⇧⌘B) builds the open folder without the command
line. The dialog asks for the output folder, whether the site opens from disk
or only from a web server, whether to include drafts, and whether to start
clean. It shows progress as it goes, and stops after the current page if you
press Cancel.

The result lists the warnings by page; click one to open that page. **Open in
Browser** shows the site, and **Show in Finder** shows its folder. **Save as
folder defaults** writes your choices into `.zen.yml`, after showing you what
it will change. The build uses the saved files, so save pages you are editing
first. The same options are in Settings › Website.

## Settings

```yaml
build:
  out: _site
  id: my-notes
  editURL: https://github.com/me/notes/edit/main/{path}
  drafts: false
  portable: true   # false = like --hosted
  exclude: ["private/**"]
```
