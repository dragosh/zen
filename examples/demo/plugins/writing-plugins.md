---
title: Writing your own plugin
nav:
  title: Writing plugins
  parent: plugins/index.md
---
# Writing your own plugin

A plugin is **one Tengo file**, or **a folder with a `plugin.yml`**. There is
nothing to build and nothing to install: put it in place, reopen the folder of
documents, and allow the plugin when zen asks.

zen looks in two places:

| Where | For |
|---|---|
| `<folder>/.zen/plugins/` | plugins that travel with a folder of documents |
| `~/Library/Application Support/zen/plugins/` | your own plugins, for every folder |

In each place a plugin is `<id>.tengo`, or a folder `<id>/` holding a
`plugin.tengo` (a script) or a `plugin.yml` (a program, a tool or a page
script).

This folder ships five small plugins in `.zen/plugins/`. Each one shows a
different part, and all five are used on this page.

## The simplest plugin: one Tengo file

When a block only has to turn text into HTML, write one file in
[Tengo](https://github.com/d5/tengo), a small scripting language. zen runs the
script itself, inside the app: there is no program to start and no environment
variable to read. The script reaches nothing zen does not hand it. It cannot
read files, run programs or use the network, unless it asks for the `read`
permission, and then it only reads the markdown and text files of the open
folder.

The `callout` plugin is the file `.zen/plugins/callout.tengo`. This markdown:

```markdown
:::tip{title="One file"}
A **Tengo** script gets the text of the block and returns HTML.
:::
```

renders as:

:::tip{title="One file"}
A **Tengo** script gets the text of the block and returns HTML.
:::

And the whole plugin is this file:

```go
// .zen/plugins/callout.tengo: :::tip and :::caution callouts, in one file.
plugin := {
    version: "0.1.0",
    description: "Tip and caution callouts",
    blocks: ["tip", "caution"],
    css: "div.callout { margin: 0 0 1em; padding: 0.6em 1em; border: 1px solid var(--rule); " +
         "border-left-width: 4px; border-radius: var(--radius) } " +
         "div.callout.caution { border-left-color: var(--fg) } " +
         "div.callout strong { display: block; margin-bottom: 0.25em }"
}

if hook == "block" {
    title := attrs.title || (name == "caution" ? "Caution" : "Tip")
    out = zen.html("div", {class: "callout " + name},
        zen.html("strong", {}, title),
        zen.markdown(body))
}
```

The `plugin` map at the top describes the plugin. zen reads it first, by running
the script once with no hook, so the name of the plugin is the name of the file
and the map holds the rest: the version, the blocks it owns, and optional
`css` that zen serves to the pages that use the plugin. After that the script
runs for each block. `hook` says why it runs, and whatever the script assigns
to `out` becomes the block.

`zen.markdown(body)` renders the body through zen's own pipeline, so a callout
can hold lists, code and the blocks of other plugins. `zen.html` escapes the
text it is given. A plain string is text and is escaped; the result of
`zen.html` or `zen.markdown` is HTML and is kept as it is.

The second sample, `wordcount`, uses the folder form: a folder `wordcount/`
holding `plugin.tengo`, with room for other files beside it. It owns an inline
directive, `:words[]`, and a block, `:::pages`. It asks for the `read`
permission, because it reads the files of the folder:

```go
// .zen/plugins/wordcount/plugin.tengo: word counts, from the files of the folder.
text := import("text")

plugin := {
    version: "0.1.0",
    description: "Word counts for this page and for every page",
    inlines: ["words"],
    blocks: ["pages"],
    permissions: ["read"]
}

count := func(path) {
    return len(text.fields(zen.read(path) || ""))
}

if hook == "inline" {
    out = zen.html("span", {class: "words"}, string(count(page.path)))
}

if hook == "block" {
    limit := int(attrs.limit || 0)
    items := []
    for p in zen.pages() {
        if limit > 0 && len(items) >= limit {
            break
        }
        items = append(items, zen.html("li", {}, p.title, " (", p.path, "): ", string(count(p.path)), " words"))
    }
    out = zen.html("ul", {class: "pages"}, items)
}
```

This page has :words[] words. The first five pages of this folder, with their
counts:

:::pages{limit=5}
:::

Leave out `{limit=5}` to list every page.

### What the script receives

| Global | Value |
|---|---|
| `hook` | why the script runs: `"block"`, `"inline"`, `"source"`, `"html"`, `"action"`, or `""` while zen reads the `plugin` map |
| `name` | the block or inline name (`"tip"`) |
| `body` | the text of the block or the inline, the page's markdown for `source`, the page's HTML for `html` |
| `attrs` | the `{key=value}` attributes, a map of strings |
| `config` | this plugin's options for the page, a map |
| `page` | `{path, title}` of the page being rendered |
| `payload` | for `action`: the JSON the plugin's frontend sent with `ctx.invoke`, decoded |
| `out` | what the script assigns: a string or a `zen.html` result; for `action` any value, sent back as JSON. Without `out`, a block is empty and `source` and `html` leave the text alone |

The `plugin` map takes these keys: `version` (required, SemVer), `description`,
`blocks`, `inlines`, `layouts`, `hooks` (extra hooks: `source`, `html`,
`action`), `permissions` (only `"read"`), `timeout` (a duration such as
`"500ms"`, at most `"2s"`), `css`, `options` (as in `plugin.yml`), `default`
and, in the folder form, `js` and `stylesheets` for the page. An unknown key is
an error.

### The `zen` module

The `zen` map is the only thing a script can call. Nothing else can be
imported except the pure modules `text`, `math`, `times`, `fmt`, `json`,
`enum`, `base64`, `hex` and `rand`.

| Function | Does |
|---|---|
| `zen.escape(s)` | returns `s` with `& < > " '` escaped |
| `zen.html(tag, attrs, children...)` | returns an element. Text children are escaped; lists are flattened; `true` makes a bare attribute |
| `zen.markdown(s)` | renders markdown through zen's pipeline, nested plugins included, at most 8 levels deep |
| `zen.storage.get(key)` | the value stored for `key`, or `undefined` |
| `zen.storage.set(key, value)` | stores any JSON value, in the store that `ctx.storage` reaches (1 MB for a plugin) |
| `zen.storage.keys(prefix)` | the keys that start with `prefix`, sorted |
| `zen.read(path)` | the text of a file of the folder, or `undefined`; needs `read` |
| `zen.pages()` | the folder's pages as a list of `{path, title}`; needs `read` |

`zen.read` opens markdown and text files (`.md`, `.txt`, `.csv`, `.json`,
`.yml`) of at most 1 MB, as UTF-8, and only inside the open folder: a path with
`..` or a link that leads out of it is refused.

### Limits

Every call has a budget. A script that goes over it stops, and the page shows an
error box in place of the block; the rest of the page still renders.

| Limit | Value |
|---|---|
| time | 200 ms, or `timeout` in the `plugin` map, at most 2 s |
| allocations | 500 000 objects, and a ceiling on memory growth |
| output | 2 MB |
| script file | 256 KB |

Whatever the script returns is cleaned like all other HTML, so a script can
never put a `<script>` or an `onclick` into a page.

## The three parts

A plugin can have any mix of these:

| Part | What it is | Use it for |
|---|---|---|
| **backend** | any program (a script or a binary) zen runs when a page renders | turning a block into HTML |
| **tool** | a backend whose command is a program already on your Mac | no code at all: the block's text goes in, the output is shown |
| **frontend** | JavaScript and CSS loaded on pages that use the plugin | blocks that react to clicks, layouts |

## A backend in a shell script: `note`

The `note` plugin draws callout boxes. This markdown:

```markdown
:::note{title="Remember"}
A backend gets the text of the block and prints HTML.
:::
```

renders as:

:::note{title="Remember"}
A backend gets the text of the block and prints HTML.
:::

Its `plugin.yml` names the blocks it owns and the program to run:

```yaml
id: note
version: 0.1.0
apiVersion: 1
description: Callout boxes
default: true

blocks: [note, warning]

allow:
  - { element: div, attrs: [class] }

backend:
  command: ["./note.sh"]
  hooks: [block]

frontend:
  css: [note.css]
```

And the whole plugin is this script. It serves both blocks, `note` and
`warning`, and escapes everything it prints:

```sh
#!/bin/sh
# .zen/plugins/note/note.sh — renders :::note and :::warning containers
esc() { sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g'; }
case "$ZEN_NAME" in
  warning) kind=warning title=Warning ;;
  *) kind=note title=Note ;;
esac
title="${ZEN_ATTR_TITLE:-$title}"
printf '<div class="%s"><strong>' "$kind"
printf '%s' "$title" | esc
printf '</strong>\n'
esc                                  # the body arrives on stdin
printf '</div>'
```

Its `note.css` gives the two boxes their look, using zen's own colours.

zen starts the program once for each block, in the plugin folder. The body of
the block arrives on standard input, the attributes arrive as environment
variables, and whatever the program prints becomes the block. The script must
be executable (`chmod +x note.sh`).

## A tool, no code at all: `calc`

When a program on your Mac already does the job, the plugin is only a
`plugin.yml`. This one hands `calc` blocks to `bc`:

```yaml
id: calc
version: 0.1.0
apiVersion: 1
blocks: [calc]
backend: { command: ["bc", "-l"], hooks: [block], output: text }
```

`output: text` tells zen that the program prints plain text, not HTML: zen
escapes it and shows it as it is. So this block:

````markdown
```calc
2^10
scale=4; 22/7
```
````

shows what `bc` answers:

```calc
2^10
scale=4; 22/7
```

## A frontend in plain JavaScript: `progress`

The `progress` plugin has no backend. zen renders the task list inside the
container as usual, and the plugin's script turns it into a checklist that
remembers what you ticked:

```markdown
:::progress{id=first-plugin}
- [ ] Read this page
- [ ] Allow the three demo plugins
- [ ] Copy one of them and change it
:::
```

:::progress{id=first-plugin}
- [ ] Read this page
- [ ] Allow the three demo plugins
- [ ] Copy one of them and change it
:::

Tick a box, go to another page and come back: the ticks are still there. The
`plugin.yml` lists the block and the two files:

```yaml
id: progress
version: 0.1.0
apiVersion: 1
description: Checklists that remember what you ticked
default: true

blocks: [progress]

frontend:
  js: [progress.js]
  css: [progress.css]
```

The script is a module whose default export has a `mount` function. zen loads
it for this plugin only and calls `mount` for every `progress` block on the
page, and again each time the page is rendered:

```js
// .zen/plugins/progress/progress.js — :::progress{id=...} checklists that remember.
// No backend: zen renders the container's task list, this script makes it clickable.
export default {
  async mount(el, ctx) {
    const boxes = Array.from(el.querySelectorAll('input[type="checkbox"]'));
    const count = document.createElement("p");
    count.className = "progress-count";
    count.setAttribute("role", "status");
    if (boxes.length === 0) {
      count.textContent = "No tasks in this list.";
      el.append(count);
      return () => count.remove();
    }

    // One stored value per list: the texts of the ticked items.
    const key = (ctx.attrs.id || "default").replace(/[^A-Za-z0-9._-]/g, "-").slice(0, 64);
    const label = (box) => box.parentElement.textContent.trim();
    const draw = () => {
      const done = boxes.filter((b) => b.checked).length;
      count.textContent = `${done} of ${boxes.length} done`;
    };

    const saved = await ctx.storage.get(key);
    if (Array.isArray(saved)) for (const b of boxes) b.checked = saved.includes(label(b));

    const onChange = async () => {
      draw();
      try {
        await ctx.storage.set(key, boxes.filter((b) => b.checked).map(label));
      } catch (err) {
        count.textContent = `Could not save: ${err.message}`;
      }
    };
    for (const b of boxes) b.disabled = false;
    el.addEventListener("change", onChange);
    draw();
    el.append(count);

    return () => {
      el.removeEventListener("change", onChange);
      for (const b of boxes) b.disabled = true;
      count.remove();
    };
  },
};
```

The style sheet uses zen's own colours, so it follows the light and dark themes:

```css
/* .zen/plugins/progress/progress.css — uses zen's own colour and spacing tokens. */
[data-zen-plugin="progress"] {
  margin: 0 0 1em;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--rule);
  border-radius: var(--radius);
}
[data-zen-plugin="progress"] ul { margin-bottom: var(--space-2); }
[data-zen-plugin="progress"] li:has(> input:checked) { color: var(--fg-muted); }
[data-zen-plugin="progress"] input[type="checkbox"]:not(:disabled) { cursor: pointer; }
.progress-count {
  margin: 0;
  color: var(--fg-muted);
  font: 13px/1.5 var(--font-ui);
}
```

Lists with the same `id` share their ticks, on any page of the folder. Give
each list its own `id`.

## `plugin.yml` in full

Only `id`, `version` and `apiVersion` are required. An unknown key is an error.

```yaml
id: note                      # [a-z][a-z0-9-]*, must equal the folder name
version: 0.1.0                # SemVer
apiVersion: 1
description: Callout boxes
default: true                 # on unless config switches it off

blocks: [note, warning]       # ```note fences and :::note containers
inlines: [badge]              # :badge[text]{attrs}
layouts: [slides]             # pages with frontmatter `layout: slides`

options:                      # read from config / frontmatter `plugins.note.*`
  icon: { type: bool, default: true, description: Show an icon }

allow:                        # extra HTML the sanitiser keeps for this plugin
  - { element: div, attrs: [class] }

backend:
  command: ["./note.sh"]      # relative to the plugin folder, or a program on PATH
  hooks: [block, inline]      # any of: source, block, inline, html, action
  output: html                # html (default) or text (escaped and shown in <pre>)
  timeout: 5s                 # default 5s, at most 60s

frontend:
  js: [note.js]               # ES modules, loaded only on pages that use the plugin
  css: [note.css]
```

A plugin is switched on, off and given options like the built-in ones: in
frontmatter, `.zen.yml` or `config.yml` (see [Plugins](index.md)).

## What a backend receives

The value of `ZEN_HOOK` says why the program was started. Input comes on
standard input, the result goes to standard output. Exit code 0 means success.
Anything else shows the error output in a box where the block would be; the
rest of the page still renders.

| `ZEN_HOOK` | Standard input | Standard output |
|---|---|---|
| `block` | the block's body (raw text) | HTML for the block |
| `inline` | the text in `[...]` | HTML for the span |
| `source` | the page's markdown | markdown |
| `html` | the page's HTML | HTML |
| `action` | JSON sent by the plugin's frontend (`ctx.invoke`) | JSON returned to it |

| Variable | Value |
|---|---|
| `ZEN_HOOK`, `ZEN_PLUGIN`, `ZEN_API` | hook name, plugin id, `1` |
| `ZEN_NAME` | block or inline name (`note`) |
| `ZEN_FENCED` | `1` for a fence, `0` for a `:::` container |
| `ZEN_PAGE` | page path inside the folder (`guides/install.md`) |
| `ZEN_ATTR_<KEY>` | each `{key=value}` attribute, key in capitals, `-` becomes `_` |
| `ZEN_ATTRS_JSON`, `ZEN_CONFIG_JSON` | all attributes / this plugin's options as JSON |
| `ZEN_CONFIG_<KEY>` | each option |
| `ZEN_WORKSPACE`, `ZEN_PLUGIN_DIR`, `ZEN_DATA_DIR` | folder root, plugin folder, a writable data folder for this plugin |

Nothing else is passed on, apart from `PATH`, `HOME`, `TMPDIR` and `LANG`.
Output is cut off at 2 MB, and a program that runs longer than its `timeout`
is stopped. Whatever a backend prints is cleaned like all other HTML: scripts,
styles and frames never reach the page.

## What a script receives

zen wraps every block, inline or layout a plugin owns:

```html
<div data-zen-plugin="quiz" data-zen-name="quiz" data-zen-attrs='{"id":"q1"}'> … </div>
```

Inside is the backend's HTML. For a plugin without a `block` hook it is the
content itself: the markdown of a `:::` container rendered to HTML, or the text
of a fence in `<pre class="zen-plugin-source">`.

A script is a module: `export default { mount(el, ctx) { … } }`. zen loads it
for its own plugin, so it only ever sees that plugin's blocks. `el` is the
wrapper. `mount` may return a function; zen calls it before the page is
rendered again, so use it to stop timers and remove listeners.

| `ctx` | |
|---|---|
| `ctx.id`, `ctx.kind`, `ctx.name` | plugin id; `"block"`, `"inline"` or `"layout"`; the block name |
| `ctx.attrs` | the `{key=value}` attributes |
| `ctx.page` | `{ path, title }` |
| `ctx.storage.get(key)` / `.set(key, value)` | JSON values kept for each plugin and each folder, on this Mac (1 MB for a plugin) |
| `ctx.invoke(payload)` | runs the backend with `ZEN_HOOK=action`; resolves to the JSON it prints |
| `ctx.stream(payload, onChunk)` | runs the backend's stream hook; `onChunk` gets each line, and `done` resolves at the end |
| `ctx.navigate(path, anchor?)` | open another page |

Storage keys are up to 64 letters, digits, `.`, `_` and `-`. `get` gives
`null` for a key that was never set. A script reaches its own plugin's storage
and backend only. Both calls use the settings of the page the block is on, so
a plugin switched on for one page works there without extra code.

An error in `mount` never breaks the page: it is shown in a box inside that
one block.

## Layouts

A plugin that lists `layouts: [slides]` takes over pages whose frontmatter says:

```yaml
---
layout: slides
---
```

zen renders the page as usual and wraps all of it in
`<div data-zen-plugin="…" data-zen-layout="slides">`. The reading column is
dropped, so the plugin has the whole pane. Its script is called with
`ctx.kind === "layout"` and rearranges the page, for example one slide for
each `<hr>`.

## Trust

Plugins run programs and scripts, so zen asks before it uses a plugin for the
first time and names what the plugin runs. For a Tengo plugin it says what the
plugin can do: render blocks, and read files of the folder if it asked for the
`read` permission. The answer is remembered for that
exact set of files. Change one byte of a plugin and zen asks again.

Allowing a plugin also lets its page script use zen's functions: it can read
and write that plugin's storage and call its backend. Allow only plugins you
trust. zen asks you in its own dialog, which a script cannot click for you.

A plugin that is not allowed does nothing: its blocks show as plain markdown,
with a note on the page. The settings panel (the cog, or <kbd>⌘,</kbd>) lists
every plugin with where it comes from, what it runs, and an **Allow** or
**Block** button.

## When something goes wrong

- **The block shows as plain text.** The plugin is not allowed yet (cog, then
  Plugins), it is switched off in a config file, or the name is missing from
  `blocks` in `plugin.yml`.
- **A warning about `plugin.yml` on every page.** zen rejected the file. The
  usual causes: `id` differs from the folder name, a key zen does not know, a
  file listed under `frontend` or a `./` command that does not exist.
- **An error box where the block should be.** The backend failed. The box
  shows what the program printed as an error. Try it by hand in the plugin
  folder: `echo "hello" | ZEN_HOOK=block ZEN_NAME=note ./note.sh`.
- **Permission denied.** Make the script executable: `chmod +x note.sh`.
- **The program is not found.** A backend gets a short `PATH`. Give the full
  path in `command`, or ship a script in the plugin folder.
- **The block times out.** The default is 5 seconds. Set `timeout`, up to `60s`.
- **Tags or attributes disappear.** zen cleans the output. List what you need
  under `allow`. Scripts, styles, frames and `on…` attributes are never kept.
- **The script does nothing.** The file must be listed under `frontend.js`
  and export a default object with a `mount` function. If it does not, the
  block shows an error box naming the plugin.
- **`storage.set` fails.** The value must be JSON, the key must use the
  characters above, and a plugin keeps at most 1 MB.
- **A change to the plugin has no effect.** zen asks again because the files
  changed. Allow the plugin, and the page loads the new script and styles.
