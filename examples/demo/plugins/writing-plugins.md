---
title: Writing your own plugin
nav:
  title: Writing plugins
  parent: plugins/index.md
---
# Writing your own plugin

A plugin is a **folder with a `plugin.yml`**. There is nothing to build and
nothing to install: put the folder in place, reopen the folder of documents,
and allow the plugin when zen asks.

zen looks in two places:

| Where | For |
|---|---|
| `<folder>/.zen/plugins/<id>/` | plugins that travel with a folder of documents |
| `~/Library/Application Support/zen/plugins/<id>/` | your own plugins, for every folder |

This folder ships three small plugins in `.zen/plugins/`. Each one shows a
different part, and all three are used on this page.

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

The script registers a handler under the plugin's id. zen calls `mount` for
every `progress` block on the page, and again each time the page is rendered:

```js
// .zen/plugins/progress/progress.js — :::progress{id=...} checklists that remember.
// No backend: zen renders the container's task list, this script makes it clickable.
zen.register("progress", {
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
});
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

A script calls `zen.register(id, { mount(el, ctx) { … } })`. `el` is the
wrapper. `mount` may return a function; zen calls it before the page is
rendered again, so use it to stop timers and remove listeners.

| `ctx` | |
|---|---|
| `ctx.id`, `ctx.kind`, `ctx.name` | plugin id; `"block"`, `"inline"` or `"layout"`; the block name |
| `ctx.attrs` | the `{key=value}` attributes |
| `ctx.page` | `{ path, title }` |
| `ctx.storage.get(key)` / `.set(key, value)` | JSON values kept for each plugin and each folder, on this Mac (1 MB for a plugin) |
| `ctx.invoke(payload)` | runs the backend with `ZEN_HOOK=action`; resolves to the JSON it prints |
| `ctx.navigate(path, anchor?)` | open another page |

Storage keys are up to 64 letters, digits, `.`, `_` and `-`. `get` gives
`null` for a key that was never set. A script reaches its own plugin's storage
and backend only.

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
first time and names what the plugin runs. The answer is remembered for that
exact set of files. Change one byte of a plugin and zen asks again.

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
- **The script does nothing.** The first argument of `zen.register` must be
  the plugin's `id`, and the file must be listed under `frontend.js`.
- **`storage.set` fails.** The value must be JSON, the key must use the
  characters above, and a plugin keeps at most 1 MB.
- **A change to the plugin has no effect.** Reopen the folder. zen reads
  plugins when a folder opens, and asks again because the files changed.
