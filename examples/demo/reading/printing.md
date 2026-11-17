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

**File › Print Site…** (⇧⌘P) opens a preview of the whole folder as one
document: a cover, a table of contents, then every page in the order of the
sidebar, each on a new sheet.

| Option | What it does |
|---|---|
| Include drafts | Adds pages with `draft: true` (off by default). |
| Pages | **All**, or **This section**: the page you were on and the pages under it. |

Up to 500 pages fit in one document; for more, print a section.

## Print from a built site

`zen build` also writes `print.html`, the same single document, so a built
site can be printed from any browser. Leave it out with `--no-print`.
