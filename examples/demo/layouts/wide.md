---
title: The wide layout
layout: wide
nav:
  parent: layouts/index.md
  order: 4
  title: Wide
---
# The wide layout

Uses the whole pane instead of the reading column. It suits wide tables,
diagrams and dashboards.

```yaml
---
layout: wide
---
```

| Layout | Made of | Reads | Options | Best for | Notes |
|---|---|---|---|---|---|
| docs | template, CSS | nothing | `toc`, `edit` | documentation | rail and neighbour links |
| post | template, CSS | date, author, cover, tags | reading | blog posts | cover and meta line |
| landing | template, CSS | tagline, cover | cards | a folder home | cards for the pages below |
| wide | template, CSS | nothing | none | tables, diagrams | no reading column |
| handout | template, CSS | date, author | paper | printing | a paper sheet and print rules |
| slides | CSS, script | nothing | ratio, transition | talks | one slide at a time |
| course | CSS, script | nothing | id | lessons | steps with saved progress |

```mermaid
graph LR; Page --> Layout --> Reader; Page --> Plugins --> Layout
```
