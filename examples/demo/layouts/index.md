---
title: Layouts
layout: landing
tagline: The look of a whole page
cover: ../images/zen-mark.svg
nav:
  title: Layouts
---
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
| [landing](landing.md) | the home page of a folder, with cards for its pages; this page uses it |
| [wide](wide.md) | the whole pane, for wide tables and diagrams |
| [handout](handout.md) | a one-pager on a paper sheet |
| [slides](slides.md) | a page shown as slides, one at a time |
| [course](course.md) | a page shown as steps, unlocked by checklists |
 Settings has a **Layouts** section that lists the
layouts with help and snippets to copy; see
[Settings](../configuration/settings.md).
