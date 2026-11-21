---
title: Plugins
nav:
---
# Plugins

Plugins add renderers to Zenmark. The built-in ones run when a page is
rendered, and their output is cleaned like everything else, so the content of a
page can never inject scripts into it.

| Plugin | Does | On by default |
|---|---|---|
| [codeblock](codeblock.md) | colours code blocks, adds Copy and Wrap buttons | yes |
| [diagram](diagram.md) | draws mermaid diagrams | yes |
| [check](check.md) | a checklist block | yes |

Settings has a **Plugins** section that lists them with help and snippets to
copy into a page; see [Settings](../configuration/settings.md).

Turn one off, or set its options, in frontmatter or `.zenmd.yml`:

```yaml
plugins:
  codeblock: true
  diagram: { theme: neutral }   # a mapping turns it on with options
  check: false                  # false turns it off
```

A plugin is a part of a page. The look of a whole page is a
[layout](../layouts/index.md).
