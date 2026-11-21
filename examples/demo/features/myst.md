---
title: MyST
---
# MyST

Markdown written for another tool can render in Zenmark without being
rewritten. A **dialect** is another way to write the blocks that plugins already
draw. **MyST** is the syntax of MyST, Jupyter Book and Sphinx. CommonMark and GFM
stay on; see [Markdown](markdown.md).

## Turn it on

In Settings › Reader, pick MyST and tick "Use MyST in this folder". The choice is
saved in the folder's `.zenmd.yml`, so the folder renders the same everywhere:

```yaml
markdown:
  dialects: [myst]
```

Zenmark also offers to turn it on when it finds a `myst.yml`, a Jupyter Book
`_config.yml` or a Sphinx `conf.py` in the folder. This guide has it on. Without
it, a fence like ```` ```{check} ```` stays a plain code block.

## A fence with a name

A fence whose first word is a name in braces is the block that owns that name.
The text after the braces is the title. This is a checklist, written the MyST
way:

````markdown
```{check}
- [ ] I turned MyST on
- [ ] I can read this page
```
````

```{check}
- [ ] I turned MyST on
- [ ] I can read this page
```

It renders exactly like `:::check`. The colon form of MyST works too:

````markdown
:::{check}
- [x] A colon fence in braces
- [ ] Another box
:::
````

:::{check}
- [x] A colon fence in braces
- [ ] Another box
:::

A block can take options. The ones this guide shows are in the
[ratings](../plugins/rating.md) page; in MyST you write them as `:key: value`
lines at the top of the body:

````markdown
```{rating}
:id: myst-rating
:title: How was MyST?
```
````

```{rating}
:id: myst-rating
:title: How was MyST?
```

A name that no plugin owns stays code, and nothing is reported.

## Code

`{code-block} lang`, `{code} lang` and `{code-cell} lang` are plain fenced code
in `lang`, so the [code colouring](../plugins/codeblock.md) applies. A code cell
is never run.

````markdown
```{code-block} python
def greet(name):
    return f"Hello, {name}"
```
````

```{code-block} python
def greet(name):
    return f"Hello, {name}"
```

Roles such as `` {ref}`x` ``, cross-references and `{eval-rst}` are not read.
