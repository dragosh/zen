---
title: Export
---
# Export

Export turns the open folder into something you can share: a **website** of
HTML pages, or one **PDF** of the whole folder. Both are under
**File › Export**, in **Settings › Export**, and behind the Export button at the
bottom of the sidebar, next to the cog.

## A website

**File › Export › Website…** (<kbd>⇧⌘B</kbd>) builds the folder as a static
website: one HTML page for each markdown page, with the sidebar and breadcrumb
written into every page, and the images and plugin files it uses copied next
to it. Put the result on any static host.

The dialog asks for:

- the output folder;
- whether the site opens from disk (every page carries its scripts) or only
  from a web server (smaller pages);
- whether to include pages marked `draft: true`;
- whether to start clean.

It shows progress as it goes and stops after the current page if you press
**Cancel**. The result lists the warnings by page; click one to open that page.
**Open in Browser** shows the site and **Show in Finder** shows its folder.
**Save as folder defaults** writes your choices into `.zenmd.yml`, after
showing you what it will change.

What you get:

- Every page the app lists becomes an `.html` file with the same path, and the
  README is also `index.html`.
- Links between pages become relative links, so the site works from any folder
  on a host.
- Plugins and layouts are run for you. Their output is plain HTML.
- `print.html` is the whole site as one printable page.

## A PDF

**File › Export › PDF…** (<kbd>⇧⌘P</kbd>) opens a preview of the whole folder
as one document: a cover, a table of contents, then every page in the order of
the sidebar, each on a new sheet.

| Option | What it does |
|---|---|
| Include drafts | adds pages with `draft: true` (off by default) |
| Pages | **All**, or **This section**: the page you were on and the pages under it |

**Export PDF…** in the preview saves the PDF straight away, without the print
panel; **Print…** is there when you want paper. Up to 500 pages fit in one
document; for more, export a section.

## From a terminal

The `zenmd` command does the same without the app:

```sh
zenmd export site                       # writes _site next to your pages
zenmd export site -o public --hosted    # for a web server only
zenmd export pdf                        # writes <folder name>.pdf
zenmd export pdf -o ~/Desktop/guide.pdf
```

## Folder defaults

The same choices can live in the folder's `.zenmd.yml`, so every export of the
folder is the same:

```yaml
export:
  out: _site
  drafts: false
  portable: true   # false = for a web server only
  exclude: ["private/**"]
```
