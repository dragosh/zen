---
slug: install
nav:
  title: Install
---
# Install zen

Zenmark is one application, `Zenmark.app`, with the `zenmd` command inside it.

## From the source

```sh
git clone https://github.com/zenmark-org/zenmd
cd zenmd
task package          # builds bin/Zenmark.app (downloads Vale once)
```

Drag `bin/Zenmark.app` into `/Applications`.

## The `zenmd` command

```sh
task install:cli      # links /usr/local/bin/zenmd (and zen) to the app
# or, while developing:
task install          # links ~/.local/bin/zenmd to bin/zenmd
```

Check it:

```sh
zenmd --version       # zenmd 0.1.0-alpha.1
```

Next: [open a folder](opening-folders.md).
