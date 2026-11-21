---
title: Frontmatter
slug: frontmatter
nav:
  title: Frontmatter
plugins:
  diagram: { theme: neutral }
---
# Page settings with frontmatter

Frontmatter is a YAML block at the very top of a page, between two `---`
lines. It is not shown on the page. This page starts with:

```yaml
---
title: Frontmatter
slug: frontmatter
nav:
  title: Frontmatter
plugins:
  diagram: { theme: neutral }
---
```

The Inspector's Page Info tab lists the keys a page has, see
[the Inspector](inspector.md).

## Title

`title:` names the page in the window title. Without it Zenmark uses the first
`# heading`, then the file name.

## Navigation

```yaml
nav:
  title: Setup            # label in the sidebar and breadcrumb
  order: 1                # position among pages under the same parent (lower first)
  parent: guides/index.md # show it under this page (a path or a slug)
  hidden: true            # leave it out of the sidebar
```

Pages without `nav:` are placed by their links. Bad values are ignored and
shown as a warning above the page, never breaking the sidebar.

## Slugs

`slug: open` gives a page a short extra name, so `[open a folder](open)` finds
it from anywhere in the folder. A real file with that name always wins over a
slug.

## Plugins

Switch a plugin off for one page, or set its options:

```yaml
plugins:
  codeblock: false        # no code colours or buttons on this page
  diagram: { theme: dark }
```

See [plugins](../plugins/index.md).

## Which setting wins

Page settings override the folder's `.zenmd.yml`, which overrides the global
`config.yml`. See [Settings](../configuration/settings.md).
