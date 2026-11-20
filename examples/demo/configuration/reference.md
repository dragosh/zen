---
title: All settings
---
# All settings

## config.yml and .zenmd.yml

```yaml
version: 1                      # file format version

layout: docs                    # layout of every page that names none
layouts:                        # layout by path, the first match wins
  "blog/**": post
  "talks/*.md": slides

plugins:
  codeblock: true               # or { lineNumbers: true, wrap: true }
  diagram: { theme: neutral }   # neutral | default | dark
  lint: true                    # Vale flags

limits:                         # global config.yml only
  files: 2000                   # markdown files, up to 100000
  entries: 50000                # files and folders scanned, up to 5000000
  openTimeout: 30               # seconds opening a folder may take, up to 600
```

While a folder opens, zen shows a loader with what it is doing and a Cancel
button; Cancel, or running past `openTimeout`, keeps the folder that was open.

## Frontmatter

| Key | Type | Example |
|---|---|---|
| `title` | text | `title: Install` |
| `slug` | lower-case words and `/` | `slug: guides/install` |
| `nav.title` | text | `nav: { title: Setup }` |
| `nav.order` | number | `nav: { order: 2 }` |
| `nav.parent` | path or slug | `nav: { parent: guides/index.md }` |
| `nav.hidden` | true / false | `nav: { hidden: true }` |
| `layout` | layout name, or a mapping with the name and options | `layout: { name: post, reading: false }` |
| `layoutOptions` | options of the layout | `layoutOptions: { ratio: "4:3" }` |
| `plugins.<name>` | true / false / options | `plugins: { lint: false }` |

## Vale

`.vale.ini` at the folder root; see [linting](../editing/linting.md).
