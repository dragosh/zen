---
title: Checklists
---
# Checklists

A `check` block holds a checklist: a plain list of boxes to tick.

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
