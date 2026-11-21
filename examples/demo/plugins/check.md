---
title: Checklists
---
# Checklists

A `check` block holds a checklist. On its own it's a plain list of boxes to
tick. In a page with `layout: course`, ticking every box in a step unlocks the
next one: see the [course layout](../layouts/course.md).

## Syntax

Put a task list inside a `check` container:

```markdown
:::check
- [ ] I opened a folder
- [ ] I can see its pages in the sidebar
:::
```

It shows as:

:::check
- [ ] I opened a folder
- [ ] I can see its pages in the sidebar
:::

## Boxes already ticked

Write `[x]` for a box that starts ticked:

:::check
- [x] I installed zen
- [ ] I opened the demo folder
- [ ] I read this page
:::

## In a course

A course is a page with `layout: course` in its frontmatter. Each `check`
block ends a step, and the next step stays locked until every box above it is
ticked:

```markdown
---
layout: course
---
## Step 1

:::check
- [ ] I did step 1
:::

## Step 2

This part unlocks once step 1 is ticked.
```

## Options

None. Turn the block off like any plugin:

```yaml
plugins:
  check: false
```
