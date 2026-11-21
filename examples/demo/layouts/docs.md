---
title: The docs layout
layout: docs
---
# The docs layout

For documentation. The text sits in a reading column, a rail on the right lists
the headings of the page, and Previous and Next links at the end follow the
order of the sidebar. This page uses it, so look right and down.

```yaml
---
layout: docs
---
```

## What it shows

The rail lists the second and third level headings, and the **Edit this page**
link under the text opens the [editor](../intro/editor.md) on this page. Both
can be switched off:

```yaml
layout: { name: docs, toc: false, edit: false }
```

### The rail

It stays in view while you read, and it hides on a narrow window.

### Previous and next

The links come from the sidebar, so the order of your pages is the order of
your guide. Give pages `nav: { order: 1 }` to place them.

## Every page of a folder

Put the layout in `.zenmd.yml` and no page needs frontmatter:

```yaml
layout: docs
```

A page can still pick another layout for itself.
