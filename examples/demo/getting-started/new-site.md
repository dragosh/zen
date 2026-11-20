---
slug: new-site
nav:
  parent: getting-started/opening-folders.md
  title: Start a new site
  order: 1
---
# Start a new site from a template

A **site template** is a ready-made folder: a documentation site, a learning
path or a planner. zen copies it into a folder of yours, fills in your title,
and opens it. Nothing is ever overwritten.

## The three templates that come with zen

| Template | Id | For |
|---|---|---|
| Technical documentation | `techdocs` | tutorials, how-to guides, reference and explanation, after [Diátaxis](https://diataxis.fr) |
| Learning path | `learning` | courses with steps, quizzes and checklists, and a glossary |
| Planner | `planner` | a roadmap, weekly plans, decision records and meeting notes |

## From the app

- On the welcome screen, pick a card under **Start from a template**.
- Or choose **File › New Site from Template…** (<kbd>⇧⌘N</kbd>). It works from the welcome screen and from an open folder.

Zen asks for the values the template needs, starting with the site title. Then
press **Choose folder…**: the panel lets you pick an existing folder or make a
new one. **Create** makes the site and opens it. If any file would be
overwritten, zen writes nothing and lists the files in the way.

## From a terminal

```sh
zenmd init site --techdocs docs --title "Acme docs"
zenmd init site --learning course --author "Ana"
zenmd init --list
```

| Command | Does |
|---|---|
| `zenmd init site --<id> [folder]` | creates the site; the folder defaults to the current one |
| `zenmd init site --template <id>` | the same, for any template by id |
| `--title`, `--author`, `--set key=value` | give the values; the others take their defaults |
| `zenmd init --list` | lists the templates |
| `zenmd init --update <id>` | writes a newer built-in next to your copy, as `<id>.new` |
| `zenmd init --dir` | prints the templates folder |

The command never asks questions, so it works in scripts. It exits with 1 when
files are in the way, and with 2 for a mistake in the command.

## Templates are folders you can edit

The first time zen lists templates, it copies the built-in ones into a
**templates folder**, next to `config.yml`. Edit them, copy them, or add your
own: every folder in `sites/` with a `template.yml` is a template. Zen never
overwrites a template you changed, and does not bring back one you deleted.

To keep the templates somewhere else, set the folder in the global
`config.yml` (a folder's `.zenmd.yml` cannot):

```yaml
templates:
  dir: ~/Documents/zen templates
```

A template folder holds a `template.yml` and the files to copy:

```yaml
id: notes
name: Team notes
description: A home page and a place for meeting notes
version: 1.0.0
order: 40
ask:
  title: { label: Site title, default: "{{folder}}" }
  team: { label: Team, default: "" }
```

In files that end in `.md`, `.yml`, `.yaml`, `.txt` or `.css`, and in file
names, zen replaces `{{title}}`, `{{author}}`, `{{date}}`, `{{folder}}` and
each key you ask for. This is plain text replacement: nothing is run, an unknown
name stays as it is, and images are copied unchanged.
