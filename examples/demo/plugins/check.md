---
title: Checklists
---
# Checklists

A `check` block holds a checklist. On an ordinary page the boxes show what the
markdown says, ticked (`[x]`) or not (`[ ]`), and clicking them does nothing.
In a page with `layout: course` you tick them, and ticking every box of a step
unlocks the next one: try the [course](../layouts/course.md).

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
- [x] I opened this guide
- [ ] I opened the Inspector
- [ ] I read this page
:::

## Options

None. Turn the block off like any plugin:

```yaml
plugins:
  check: false
```
