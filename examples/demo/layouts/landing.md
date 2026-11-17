---
title: The landing layout
layout: docs
nav:
  parent: layouts/index.md
  order: 3
  title: Landing
---
# The landing layout

For the home page of a folder. It shows a hero with the title, a tagline and
an optional cover image, then the text of the page, then a card for each page
under it in the sidebar, with its title and first paragraph.

```yaml
---
title: My guide
layout: landing
tagline: Everything you need, in one place
cover: images/cover.png
---
A short welcome.
```

## Fields and options

| Field | Type | Does |
|---|---|---|
| `tagline` | text | one line under the title |
| `cover` | image of the folder | shown faded behind the title |

`cards: false` hides the cards.

## Live examples

The [layouts page](index.md) and [the blog](../blog/index.md) both use it. Their
cards are the pages that sit under them in the sidebar.
