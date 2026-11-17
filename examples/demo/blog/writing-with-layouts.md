---
title: Writing with layouts
date: 2026-11-17
author: Ada
tags: [layouts, writing]
nav:
  parent: blog/index.md
  order: 2
---
# Writing with layouts

A layout gives a page its look. This post has a date, an author and tags in its
frontmatter, and the `post` layout turns them into a meta line and a tag row.

## Choosing by path

You did not write `layout: post` in this page. The folder said so once:

```yaml
layouts:
  "blog/**": post
```

## Your own look

A layout is a folder with a template and a style sheet. See
[Writing your own layout](../layouts/writing-your-own-layout.md) to make one in
about ten minutes.
