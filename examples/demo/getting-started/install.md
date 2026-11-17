---
slug: install
nav:
  title: Install
---
# Install zen

zen.md is one application, `Zen.app`, with the `zenmd` command inside it.

## From the source

```sh
git clone https://github.com/dragosh/zen
cd zen
task package          # builds bin/Zen.app (downloads Vale once)
```

Drag `bin/Zen.app` into `/Applications`.

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
