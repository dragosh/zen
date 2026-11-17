---
title: Writing your own layout
slug: writing-layouts
nav:
  parent: layouts/index.md
  order: 8
  title: Write your own
---
# Writing your own layout

A layout is a folder with a `layout.yml` and, as it needs, an HTML template, style
sheets and a script. You do not need any code to make one: a template and a few
lines of CSS are enough. We build the `memo` layout of this folder; see it at
work in [A memo](memo.md).

## Where it lives

| Where | For |
|---|---|
| `<folder>/.zen/layouts/<id>/` | layouts that travel with a folder of documents |
| `<user config dir>/zen/layouts/<id>/` (macOS: `~/Library/Application Support/zen/layouts/`) | your own layouts, for every folder |

zen asks before it uses a layout for the first time and again when its files
change, like for plugins.

## The definition

`.zen/layouts/memo/layout.yml`:

```yaml
id: memo
version: 0.1.0
description: A short memo with a to, from and date block
template: layout.html
css: [memo.css]
fields:
  to:   { type: string, description: Who the memo is for }
  from: { type: string, description: Who wrote it }
  date: { type: date }
```

The id must equal the folder name. A layout needs a template, CSS or a script,
or any mix. `fields` lists the frontmatter the template reads; `options` lists
settings a page can give in `layoutOptions`.

## The template

`.zen/layouts/memo/layout.html` is a Go `html/template`. Everything you print is
escaped, so a field that holds a tag shows as text:

```html
<section class="memo">
  <header>
    <h1>Memo</h1>
    <dl>
      <dt>To</dt><dd>{{.Field "to"}}</dd>
      <dt>From</dt><dd>{{.Field "from"}}</dd>
      <dt>Date</dt><dd>{{date (.Field "date")}}</dd>
    </dl>
  </header>
  {{.Content}}
</section>
```

What a template can use:

| Name | Is |
|---|---|
| `.Content` | the page text, ready to print |
| `.Title`, `.Path` | the page title and its path in the folder |
| `.Breadcrumb` | the trail of the page: each has `.Title` and `.Path` |
| `.TOC` | the headings from level two to four: `.Level`, `.ID`, `.Text` |
| `.Prev`, `.Next` | the neighbours in the sidebar, or nothing |
| `.Children` | the pages under this one in the sidebar: `.Title`, `.Path`, `.Summary` |
| `.Field "name"` | a field declared in `fields`, typed |
| `.Option "name"` | an option declared in `options`, typed |
| `.Words`, `.ReadingTime` | the number of words and the minutes to read |

Functions: `date`, `upper`, `lower`, `join` and `markdown` (turns a text field
into formatted text). Nothing else: a template cannot read files.

A link to another page needs the attributes zen uses for links:
`<a data-zen-link="internal" data-path="{{.Path}}">`. A link with
`data-zen-action="edit"` opens the editor on the page.

## The style

`.zen/layouts/memo/memo.css` styles the layout. Scope it to the layout, so it
cannot touch anything else, and use the colour variables of zen so the layout
follows the light and dark themes:

```css
[data-zen-layout="memo"] .memo { max-width: 60ch; margin: 0 auto; padding: 48px 24px; }
[data-zen-layout="memo"] .memo dl { display: grid; grid-template-columns: 5em 1fr; }
```

## Use it

```yaml
---
layout: memo
to: The whole team
from: Ada
date: 2026-11-17
---
```

Or choose it by path in `.zen.yml`:

```yaml
layouts:
  "memos/**": memo
```

## Behaviour with a script

A layout can also list `js: [memo.js]`. The file is a module with a default
export `{ mount(root, ctx) }`; zen calls it with the page element and a `ctx`
with `ctx.attrs`, `ctx.page`, `ctx.storage` and `ctx.navigate`. A layout script
has no backend: it cannot run programs.

If a template fails, the page falls back to the plain reading view with a
warning that says why.
