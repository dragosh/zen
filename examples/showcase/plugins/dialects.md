---
title: Other syntaxes
---
# Other syntaxes

Markdown written for another tool can render in zen without being rewritten.
A **dialect** is another way to write the containers that plugins already draw.
The first one is **MyST**, the syntax of MyST, Jupyter Book and Sphinx.

## Turn it on

A folder chooses its dialects in `.zenmd.yml`, so it renders the same in the app,
in `zenmd export`, on another Mac and in CI:

```yaml
markdown:
  dialects: [myst]
```

Without that, a ```` ```{note} ```` fence stays a code block, as it always was.
In the app you can do the same in Settings › Reader, and zen offers it when it
finds a `myst.yml`, a Jupyter Book `_config.yml` or a Sphinx `conf.py` in the
folder. This folder has it on.

## A fence with a name

A fence whose first word is a name in braces is the container that owns that
name. The text after the braces is the title:

````markdown
```{note} Remember
Body text, with **markdown**.
```
````

```{note} Remember
Body text, with **markdown**.
```

It renders exactly like `:::note{title="Remember"}`. The colon form of MyST
works too, and so do tildes:

````markdown
:::{important} Back up first
A longer fence holds a shorter one.
:::
````

:::{important} Back up first
A longer fence holds a shorter one.
:::

Names that no plugin owns stay code, and nothing is reported.

## Options

Lines at the top of the body that look like `:key: value` become attributes
of the container, as they do in MyST. A key on its own is a flag:

````markdown
```{note} Options
:class: wide
:open:
The box above has `class` and `open` set.
```
````

## Names

MyST's admonition names (`note`, `tip`, `warning`, `important`, `caution`,
`attention`, `danger`, `error`, `hint`, `seealso`, `admonition`) are ordinary
block names. A plugin answers to them by listing them in the `blocks` of its
manifest; no new field is needed. This folder's `note` plugin lists `note`,
`warning`, `important`, `attention` and `hint`, and the `callout` plugin lists
`tip` and `caution`:

```{tip} Same plugin, other syntax
A tip written the MyST way is drawn by the same plugin as `:::tip`.
```

## Code

`{code-block} lang`, `{code} lang` and `{code-cell} lang` are plain fenced code
in `lang`, so the codeblock plugin colours them. A code cell is never run.

```{code-block} python
def greet(name):
    return f"Hello, {name}"
```

Roles such as `` {ref}`x` ``, cross-references and `{eval-rst}` are not read.
