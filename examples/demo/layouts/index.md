---
title: Layouts
nav:
  title: Layouts
---
# Layouts

A **plugin** is a part of a page: a diagram, a checklist. A **layout** is the
look and feel of the whole page: its structure, type and behaviour. A page can
choose one in its frontmatter:

```yaml
---
layout: article
---
```

Every page uses [article](article.md) unless it names another. This build
has these layouts:

| Layout | For |
|---|---|
| [article](article.md) | the reading view, the default |
| [docs](docs.md) | documentation, with a contents rail and previous and next links |
| [post](post.md) | blog posts, with a cover, date, author and tags |
 Settings has a **Layouts** section that lists the
layouts with help and snippets to copy; see
[Settings](../configuration/settings.md).
