---
title: The zen blog
layout: landing
tagline: Notes on writing in plain files
---
Short posts that show the `post` layout. They carry no `layout:` line: the
folder's `.zenmd.yml` gives every page under `blog/` that layout by path.

```yaml
layouts:
  "blog/**": post
```

This page picks the `landing` layout for itself, which beats the pattern.
