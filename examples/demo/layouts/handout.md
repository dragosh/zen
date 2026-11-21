---
title: The handout layout
layout: handout
date: 2026-11-17
author: The Zenmark team
---
# The handout layout

For one-pagers. The page sits on a white sheet, the size of a sheet of paper,
with a header and a footer.

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

## Paper

The sheet is A4. For US Letter use `layout: { name: handout, paper: letter }`.

> The header shows the author and the date, and the footer shows the title and
> the date.

Printing the sheet comes with printing in a later version of Zenmark.
