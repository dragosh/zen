# Site templates

A site template is a folder that `zen init site` (or File › New Site from
Template…, ⇧⌘N) copies into a new or existing folder. Spec:
[specs/core/init](../specs/core/init/spec.md).

```sh
zen init site --techdocs docs       # Technical documentation (Diátaxis)
zen init site --learning course     # A learning path
zen init site --planner plans       # A planner
zen init site --template my-id .    # any template in your templates folder
zen init --list                     # what is there
zen init --dir                      # where the templates folder is
zen init --update techdocs          # a newer built-in, written next to yours as techdocs.new
```

## Where templates live

`<zen config dir>/templates/sites/<id>/`, next to `config.yml`. Move it in the
global `config.yml`:

```yaml
templates:
  dir: ~/Documents/zen templates
```

The three built-ins are copied there the first time templates are listed, so
you can edit them. zen never overwrites your copy, and a built-in you delete
stays deleted.

## A template

```
my-template/
  template.yml
  README.md
  guides/{{date}}-first.md
  .zen.yml
```

```yaml
id: my-template
name: My template
description: One line shown on the card
version: 1.0.0
order: 50
ask:
  title: { label: Site title, default: "{{folder}}" }
  author: { label: Author, default: the team }
ignore: [".DS_Store"]
```

In `.md`, `.yml`, `.yaml`, `.txt` and `.css` files and in file names,
`{{title}}`, `{{author}}`, `{{date}}`, `{{folder}}` and every `ask:` key are
replaced with their values. This is plain text replacement: nothing in a
template runs. Other files are copied as they are.

Creating a site never overwrites anything. If one file would clash, nothing is
written and the clashes are listed. Templates are limited to 2,000 files and 50 MB.
