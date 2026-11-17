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

## Title

`title:` names the page in the window title. Without it zen uses the first
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

`slug: install` gives a page a short extra name, so `[setup](install)` or
`[setup](/install#from-the-source)` finds it from anywhere in the folder. A
real file with that name always wins over a slug.

## Plugins

Switch plugins for one page, or set their options:

```yaml
plugins:
  codeblock: false        # no code colours or buttons on this page
  diagram: { theme: dark }
  lint: false             # no Vale on this page
```

## Layout

`layout: post` gives the page the look of a [layout](../layouts/index.md), and
`layoutOptions:` sets its options. Fields such as `date` or `tagline` are read
by the layout. A folder can choose layouts by path in `.zen.yml`, so a page
needs no frontmatter at all; see [the blog](../blog/index.md).

Page settings override the folder's `.zen.yml`, which overrides the global
`config.yml`. See [settings](../configuration/settings.md).
