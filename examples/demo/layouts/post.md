---
title: The post layout
layout: post
date: 2026-11-17
author: The Zenmark team
tags: [layouts, writing]
cover: ../images/zen-mark.svg
---
# The post layout

For blog posts and notes. It shows an optional cover image, the title, a line
with the date, the author and the reading time, then the text, the tags and
links to the neighbouring posts.

```yaml
---
title: My first post
layout: post
date: 2026-11-17
author: Ada
tags: [notes, zenmark]
cover: images/cover.png
---
```

## Fields

| Field | Type | Does |
|---|---|---|
| `date` | date | shown in the meta line, such as 2026-11-17 |
| `author` | text | shown next to the date |
| `cover` | image of the folder | shown above the title |
| `tags` | list | shown under the text |

## Options

`reading: false` hides the reading time.

## A folder of posts

To give every page of a folder this layout without frontmatter, name it by
path in `.zenmd.yml`; the first match wins and a page's own layout beats it:

```yaml
layouts:
  "blog/**": post
```
