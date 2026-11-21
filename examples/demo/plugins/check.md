---
title: Checklists
---
# Checklists

A `check` block holds a checklist you can tick. Your ticks are kept for the
page, so the list works as a to-do list: close the page and come back, and the
boxes are as you left them. In a page with `layout: course`, ticking every box
in a step unlocks the next one: see the [course layout](../layouts/course.md).

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

## Reset

Once a box differs from what the markdown says, a small "Reset" button shows
under the list. It puts the boxes back as written and forgets your ticks.
Try it on the list above: tick a box, then press Reset.

## Boxes already ticked

Write `[x]` for a box that starts ticked:

:::check
- [x] I installed zen
- [ ] I opened the demo folder
- [ ] I read this page
:::

A plain task list outside a `check` block stays read-only, as on GitHub:

- [x] I can't be changed
- [ ] Neither can I

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
