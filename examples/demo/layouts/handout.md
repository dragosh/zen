---
title: The handout layout
layout: handout
date: 2026-11-17
author: The Zenmark team
---
# The handout layout

For one-pagers you print. On screen the page sits on a white sheet with a
header and a footer. Print it (**File › Print Page…**, <kbd>⌘P</kbd>) and only
the sheet comes out, at the right paper size.

```yaml
---
title: Workshop handout
layout: handout
date: 2026-11-17
author: Ada
---
```

## Steps

1. Open the folder in Zenmark.
2. Pick the page you want to hand out.
3. Give it `layout: handout`, a `date` and an `author`.
4. Choose **File › Print Page…**.

## Paper

The sheet is A4. For US Letter use `layout: { name: handout, paper: letter }`.

> The header shows the author and the date, and the footer shows the title and
> the date.
