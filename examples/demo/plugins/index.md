---
title: Plugins
nav:
---
# Plugins

Plugins add renderers to zen. The built-in ones run in Go when a page is
rendered. Their output is cleaned like everything else, so the content of a
page can never inject scripts into it.

| Plugin | Does | On by default |
|---|---|---|
| [codeblock](codeblock.md) | colours code blocks, adds Copy and Wrap buttons | yes |
| [diagram](diagram.md) | draws mermaid diagrams | yes |
| [quiz](quiz.md) | questions with saved answers and scores | yes |
| [sandbox](sandbox.md) | runs a code block in a container | yes, after setup |
| [slides](../layouts/slides.md) | shows a page as a slide deck | yes |
| [course](../layouts/course.md) | shows a page as steps with saved progress | yes |
| `lint` | Vale flags (a switch, see [linting](../editing/linting.md)) | yes |

Turn one off or set its options in frontmatter, `.zen.yml` or `config.yml`:

```yaml
plugins:
  codeblock: true
  diagram: { theme: neutral }   # a mapping turns it on with options
  lint: false                   # false turns it off
```

How plugins are written into markdown: [plugin syntax](directives.md).

## Your own plugins

A plugin is a folder with a `plugin.yml`: a program zen runs, scripts and
styles for the page, or both. This folder carries three in `.zen/plugins/`:

| Plugin | Does | Made of |
|---|---|---|
| `note` | callout boxes | a shell script |
| `calc` | sums, worked out by `bc` | a `plugin.yml` only |
| `progress` | checklists that remember what you ticked | JavaScript and CSS |

zen asks before it uses them for the first time. The cog lists every plugin
with an Allow or Block button. To make one, read
[Writing your own plugin](writing-plugins.md).
