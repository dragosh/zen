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
