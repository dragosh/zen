---
title: The .zenmd.yml reference
---
# The .zenmd.yml reference

The keys that do something in this version of Zenmark. The same keys work in
the global `config.yml`.

```yaml
version: 1                      # file format version

logo: images/logo.svg           # an image in the folder, shown beside the
                                # folder's title in the sidebar

plugins:
  codeblock: true               # or { lineNumbers: true, wrap: true }
  diagram: { theme: neutral }   # neutral | default | dark
  check: true

limits:                         # global config.yml only
  files: 2000                   # markdown files, up to 100000
  entries: 50000                # files and folders scanned, up to 5000000
  openTimeout: 30               # seconds opening a folder may take, up to 600
```

While a folder opens, Zenmark shows a loader with what it is doing and a
Cancel button; Cancel, or running past `openTimeout`, keeps the folder that was
open.

## Frontmatter

A page can override the folder's settings.

| Key | Type | Example |
|---|---|---|
| `title` | text | `title: Install` |
| `slug` | lower-case words and `/` | `slug: guides/install` |
| `nav.title` | text | `nav: { title: Setup }` |
| `nav.order` | number | `nav: { order: 2 }` |
| `nav.parent` | path or slug | `nav: { parent: guides/index.md }` |
| `nav.hidden` | true / false | `nav: { hidden: true }` |
| `layout` | layout name | `layout: article` |
| `plugins.<name>` | true / false / options | `plugins: { check: false }` |
