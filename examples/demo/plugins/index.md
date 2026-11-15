---
title: Plugins
nav:
---
# Plugins

Plugins add renderers to zen. They run in Go when a page is rendered, and
their output is cleaned like everything else, so a plugin can never inject
scripts into a page.

| Plugin | Does | On by default |
|---|---|---|
| [highlight](highlight.md) | colours code blocks | yes |
| [diagram](diagram.md) | draws mermaid diagrams | yes |
| `lint` | Vale flags (a switch, see [linting](../editing/linting.md)) | yes |

Turn one off or set its options in frontmatter, `.zen.yml` or `config.yml`:

```yaml
plugins:
  highlight: true
  diagram: { theme: neutral }   # a mapping turns it on with options
  lint: false                   # false turns it off
```

How plugins are written into markdown: [plugin syntax](directives.md).
