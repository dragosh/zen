---
title: Printing
slug: printing
nav:
  title: Printing
  parent: reading/frontmatter.md
---
# Printing and PDF

zen prints through the macOS print panel, so **Save as PDF** is in it.

## Print a page

**File › Print Page…** (⌘P) prints the page you are reading, as it looks, in
black on white whatever the app theme. The sidebar, toolbar and buttons are
left out; what they show stays, such as quiz questions and the answers given
so far.

- Headings stay with the text after them, and code blocks, tables and
  diagrams are not split where they fit on a page.
- Long code lines wrap.
- Links to other sites print their address after the text, like
  [the markdown guide](https://commonmark.org/help/).
  Links to other pages print as plain text.
- Details blocks print open.

<details>
<summary>What about layouts?</summary>

A layout keeps its own print rules: [handout](../layouts/index.md) prints its
sheet, slides print one slide per page, a course prints all its steps one after
another.

</details>

## Print the whole folder

**File › Export › PDF…** (⇧⌘P) opens a preview of the whole folder as one
document: a cover, a table of contents, then every page in the order of the
sidebar, each on a new sheet.

| Option | What it does |
|---|---|
| Include drafts | Adds pages with `draft: true` (off by default). |
| Pages | **All**, or **This section**: the page you were on and the pages under it. |

Up to 500 pages fit in one document; for more, print a section.

**Export PDF…** in the preview saves the document as a PDF file straight away,
without the print panel; **Print…** is there when you want paper or the
printer's options. From the command line, `zenmd export pdf` does the same:

```sh
zenmd export pdf                  # writes <folder name>.pdf in the folder
zenmd export pdf -o ~/Desktop/guide.pdf
```

## Print from a built site

`zenmd export site` also writes `print.html`, the same single document, so a built
site can be printed from any browser. Leave it out with `--no-print`.
