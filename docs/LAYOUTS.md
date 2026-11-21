# zen — layouts

A **plugin** is a *part* of a page: a block, an inline, a transformation (see
[PLUGINS.md](PLUGINS.md)). A **layout** is the *look and feel of the whole
page*: its structure, typography, chrome and behaviour. Layouts have their own
folder, definition, registry, trust and settings. Implemented by
`specs/core/layouts`; Go in `internal/core/layouts`, built-ins in
`internal/layouts/<id>`.

Ideas for later layouts: [possible-layouts.md](possible-layouts.md).

## Choosing a layout

In a page's frontmatter:

```yaml
---
layout: post
layoutOptions: { reading: false }
---
```

or in one mapping (string, number and true/false values only):

```yaml
---
layout: { name: post, reading: false }
---
```

`layoutOptions` wins over the same key in the mapping. In `.zenmd.yml` and the
global `config.yml`:

```yaml
layout: docs                  # every page that chooses nothing
layouts:                      # by path; the first match wins; the folder's rules come before the global ones
  "blog/**": post
  "talks/*.md": slides
```

Precedence: page frontmatter > a `layouts:` pattern > `layout:` > the built-in
`article`. An unknown, blocked or invalid layout puts a warning on the page and
renders it with `article`.

Patterns match the whole workspace-relative path, case-sensitively: `*` is any
text inside one folder level, `**` any text across folders (`**/` at the start
also matches no folder), `?` one character.

## Built-in layouts

| id | Made of | For | Reads (fields) | Options |
|---|---|---|---|---|
| `article` | nothing | the default reading column; adds no wrapper | | |
| `docs` | template, CSS | documentation: contents rail (h2, h3), Previous / Next, Edit link | | `toc`, `edit` (bool, true) |
| `post` | template, CSS | posts and notes: cover, title, date · author · reading time, tags, Previous / Next | `date`, `author`, `cover` (image), `tags` (list) | `reading` (bool, true) |
| `landing` | template, CSS | a folder's home: hero (title, `tagline`, cover) then a card for each page under it | `tagline`, `cover` | `cards` (bool, true) |
| `wide` | template, CSS | full pane width for tables, diagrams, dashboards | | |
| `handout` | template, CSS | printable one-pager: a paper sheet on screen, print rules | `date`, `author` | `paper` (`a4` or `letter`) |
| `slides` | CSS, script | one slide at a time (`ratio`, `transition`); unchanged from the old plugin | | `ratio`, `transition` |
| `course` | CSS, script | steps with saved progress and `:::check` lists (`id`); unchanged from the old plugin | | `id` |

The `:::check` block is a small built-in plugin (`check`); a course page needs
it to unlock steps.

## What a layout is

A folder with a `layout.yml` and the files it lists. A layout may have only CSS
(restyle the article), only a template, only a script, or any mix.

| Where | Source | Trust |
|---|---|---|
| `internal/layouts/<id>` (embedded) | `builtin` | always on |
| `<folder>/.zen/layouts/<id>/` | `folder` | asked per folder |
| `<user config dir>/zen/layouts/<id>/` (macOS: `~/Library/Application Support/zen/layouts/`) | `user` | asked once, for every folder |

A folder layout shadows a user layout of the same id; a layout may not use the
id of a built-in one.

### layout.yml

```yaml
id: post                     # [a-z][a-z0-9-]*, must equal the folder name
version: 0.1.0               # SemVer
description: Blog post with cover, date and author
template: layout.html        # optional: the page structure
css: [post.css]              # optional
js: [post.js]                # optional: export default { mount(root, ctx) }
fields:                      # frontmatter the template reads
  date:   { type: date, description: Publication date }
  author: { type: string }
  cover:  { type: image, description: Workspace image shown on top }
  tags:   { type: list }
options:                     # layoutOptions, like plugin options
  reading: { type: bool, default: true, description: Show the reading time }
help:                        # shown in Settings › Layouts
  summary: A blog post.
  snippets:
    - title: A post
      markdown: "---\nlayout: post\ndate: 2026-11-17\n---\n# Title"
```

Rules, all enforced when the layout loads (an invalid layout is listed with the
problem and never used):

- an unknown key, an `id` that differs from the folder name, a version that is
  not SemVer, a layout with no template, css or js: refused;
- every file named in `template`, `css` and `js` must be a plain relative file
  of the folder (no `..`, no absolute path, no `.DS_Store`, a symlink only when
  it stays inside); the template is at most 256 KB;
- field types: `string`, `text`, `int`, `bool`, `date`, `image`, `list`; names
  start with a lower-case letter and use letters, digits, `_` and `-`;
- option types: `string`, `bool`, `int`, `map`, `list` with a default of that
  type (as in `plugin.yml`);
- `help` has the shape of plugin help (summary up to 500 characters, up to 8
  snippets of up to 4 KB). A snippet that starts with frontmatter is the
  starting text of **File › New Page** for that layout.
- the folder has at most 500 files and 20 MB, like a plugin folder, and a
  `.zen/layouts` folder is hashed the same way.

### Options

An option takes the page's value or its default. A bool accepts `true` or
`false` (also as text), an int a number (also as text). A layout that declares
options reports an undeclared one as a page warning and drops it; a layout that
declares none (like `slides` and `course`) passes every option on to its script
as it is. The options reach a layout script as `ctx.attrs`, all as strings
(`"true"`, `"3"`), in `data-zen-attrs` of the wrapper.

## Templates

A template is a Go `html/template`, run with the data below and sanitised
afterwards with zen's HTML policy. Everything it prints from data is escaped, so
a field holding `<script>` shows as text. The page text, `.Content`, is the only
trusted HTML (it goes through the same sanitiser at the end, with the page's
plugin rules). A template that does not parse makes the layout invalid; one that
fails to run puts a warning on the page and `article` is used.

| Data | Is |
|---|---|
| `.Content` | the rendered page body (HTML) |
| `.Title`, `.Path` | the page title, and its workspace-relative path |
| `.Breadcrumb` | `[]` of `{Title, Path}`, from the sidebar |
| `.TOC` | `[]` of `{Level, ID, Text}`: the h2 to h4 headings that have an id |
| `.Prev`, `.Next` | `{Title, Path}` of the neighbours in the sidebar order, or nothing |
| `.Children` | `[]` of `{Title, Path, Summary}`: the pages under this one in the sidebar; `Summary` is the first paragraph as plain text (at most 180 characters). Worked out only when the template asks |
| `.Field "name"` | the frontmatter field `name`, typed by its declaration; an undeclared name is an error |
| `.Option "name"` | the option `name`, typed, with the default; an undeclared name is an error |
| `.Words`, `.ReadingTime` | the words of the text, and the minutes to read (at 200 words a minute, at least 1) |

Field types: `string`, `text` and `image` give a string (an image is resolved to
a workspace URL, or kept when it is an `https` address; anything else is a page
warning and an empty value), `int` an int, `bool` a bool, `date` a `*time.Time`
(`nil` when the page gives none, so `{{with .Field "date"}}` works) and `list`
a list of text (a single text counts as a list of one). A missing field is the
empty value. A value of the wrong type is a page warning and the empty value.

Functions, and no others besides Go's built-in template ones: `date` (formats a
date or a date text, `{{date .}}` gives `17 November 2026`; a layout can be
given, `{{date . "2006-01-02"}}`; nothing gives an empty text), `upper`,
`lower`, `join` (`{{join .Tags ", "}}`) and `markdown` (renders a text with the
page's own markdown pipeline: `{{markdown (.Field "intro")}}`). A template
cannot read files or reach the network. Output is limited to 4 MB.

Links to other pages need the attributes zen's own links have:

```html
<a data-zen-link="internal" data-path="{{.Path}}">{{.Title}}</a>   <!-- another page -->
<a href="#{{.ID}}">{{.Text}}</a>                                   <!-- a heading of this page -->
<a data-zen-action="edit">Edit this page</a>                       <!-- opens the editor on this page -->
```

Put `data-zen-link` first in the tag so zen can finish the link. `edit` and
`print` are the only actions.

### What the sanitiser keeps

The wrapper and the template output go through the base policy plus, for a
template layout: the elements `header`, `footer`, `nav`, `article`, `aside`,
`figure`, `figcaption`, `time`, `address` and `hgroup`; the `class` attribute
(space-separated names) on those and on `p`, `h1` to `h6`, `img`,
`blockquote`, tables, `dl`, `dt` and `dd`, as the base policy already allows
it on `div`, `span`, `section`, `ul`, `ol`, `li`, `a` and a few more;
`datetime` on `time`; `aria-label`; `data-zen-action="edit|print"` on `a`; and
`data-zen-layout-source`. Style attributes, scripts, forms and frames never
get through. Because the page text goes through the same policy, those classes
are allowed in the text too.

### The wrapper

zen puts the whole page in one element, then the front end finds it:

```html
<div data-zen-layout="post" data-zen-layout-source="folder" data-zen-attrs='{"reading":"true"}'> … </div>
```

`article` adds no wrapper. The reader gives a page with a layout the whole pane
(no reading column), so a layout sets its own width and padding.

## Styles

Scope every rule to `[data-zen-layout="<id>"]` so a layout cannot restyle
anything else, and use zen's variables (`--bg`, `--fg`, `--fg-muted`, `--rule`,
`--bg-subtle`, `--font-body`, `--font-ui`, `--space-*`, `--radius`; see
[UI.md](UI.md)) so the layout follows the light and dark themes. Stay grayscale:
colour comes from the content. The page text keeps the reader's typography
(`.zen-page h1` and so on) inside the wrapper.

Add `@media print` rules for printing, and `@media (max-width: …)` for narrow
windows.

## Scripts

A layout script is an ES module with a default export `{ mount(root, ctx) }`,
loaded from `/_layouts/<id>/<file>` and bound to the layout (`ctx.kind ===
"layout"`, `ctx.id` is the layout id). It gets `ctx.attrs`, `ctx.page`,
`ctx.storage` (`get`, `set`, `keys`; kept per layout and folder), the
navigation helpers and `ctx.assetUrl(file)` (the URL of another file of the
layout, in the app and in a `zenmd build` site). A layout has no backend: `ctx.invoke` and `ctx.stream` do
not exist.

`slides` and `course` are the built-in examples (`internal/layouts/slides`,
`internal/layouts/course`).

## Assets

`Page.assets.css` and `.js` list a layout's files as
`/_layouts/<id>/<file>?v=<hash>` (the first 12 hex digits of the folder's hash;
the app version for built-in layouts). zen serves only the files the manifest
lists: not `layout.yml`, not the template, not other files. A file of a layout
that changed after it was loaded, or that resolves outside its folder, is not
served.

## Trust

Folder and user layouts go through the same discovery, hashing, fingerprint
check and per-folder answer as external plugins (see PLUGINS.md, "Trust"), and
the answer is kept in the same `trusted-plugins.yml` under the layout's folder.
The prompt says what a layout does: *changes how pages look; runs nothing* for
CSS and a template, and *changes how pages look and runs a page script* with
`js`. Settings › Layouts shows Allow and Block for the layouts that are not
built in. A layout whose files change after it was approved is not used (the
page says so) and zen asks again. Patterns in `.zenmd.yml` only apply layouts the
user has allowed.

## Service API

```go
Layouts() []LayoutInfo
LayoutTrust(id string, allow bool) error          // Allow shows the native dialog first
LayoutStorageGet(id, key string) (string, error)  // JSON text, "null" when none
LayoutStorageSet(id, key, value string) error     // JSON; 1 MB per layout and folder
LayoutStorageKeys(id, prefix string) ([]string, error)
```

```json
{ "id": "post", "version": "0.1.0", "description": "…", "source": "builtin|folder|user",
  "status": "on|blocked|undecided|invalid", "kind": "layout",
  "hasTemplate": true, "hasScript": false,
  "fields":  [{ "name": "date", "type": "date", "description": "…" }],
  "options": [{ "name": "reading", "type": "bool", "default": true, "description": "…" }],
  "help": { "summary": "…", "snippets": [{ "title": "…", "markdown": "…" }] },
  "problem": "" }
```

`Page.layout` is `{name, source}` (`article` and `builtin` when none is chosen).
The event `layouts:changed` (no payload) follows any change to the layout set,
a trust decision, a change to a layout's files and a folder open.

Storage keys are 1 to 64 letters, digits, `.`, `_`, `-`. The `course` layout
keeps using the storage file of the course plugin it replaces, so saved
progress is not lost; other layouts keep theirs in a folder of their own.

**File › New Page…** accepts any usable layout id; the first help snippet is the
starting text (its frontmatter gets the page's title).

## Plugins and layouts

A `layouts:` key in `plugin.yml` or in a Tengo `plugin` map is an error:

> layouts moved: a plugin cannot declare layouts; put the layout in
> .zen/layouts/<id>/layout.yml

## Example

`examples/showcase/.zen/layouts/memo/` is a layout of a folder (template plus CSS,
fields `to`, `from`, `date`); `examples/showcase/layouts/` has a page for each
built-in layout and the how-to "Writing your own layout"; `examples/showcase/blog/`
chooses `post` by path with `layouts:` in the folder's `.zenmd.yml`.
