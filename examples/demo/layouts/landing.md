---
title: The landing layout
layout: docs
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

## Live example

The [Layouts page](index.md) uses it: its cards are the layout pages that sit
under it in the sidebar.
