---
title: All settings
---
# All settings

## config.yml and .zen.yml

```yaml
version: 1                      # file format version

plugins:
  highlight: true               # or { lineNumbers: true }
  diagram: { theme: neutral }   # neutral | default | dark
  lint: true                    # Vale flags

limits:                         # global config.yml only
  files: 2000                   # markdown files, up to 100000
  entries: 50000                # files and folders scanned, up to 5000000
```

## Frontmatter

| Key | Type | Example |
|---|---|---|
| `title` | text | `title: Install` |
| `slug` | lower-case words and `/` | `slug: guides/install` |
| `nav.title` | text | `nav: { title: Setup }` |
| `nav.order` | number | `nav: { order: 2 }` |
| `nav.parent` | path or slug | `nav: { parent: guides/index.md }` |
| `nav.hidden` | true / false | `nav: { hidden: true }` |
| `plugins.<name>` | true / false / options | `plugins: { lint: false }` |

## Vale

`.vale.ini` at the folder root; see [linting](../editing/linting.md).
