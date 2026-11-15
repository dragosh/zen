---
slug: install
nav:
  title: Install
---
# Install zen

zen is one application, `Zen.app`, with the `zen` command inside it.

## From the source

```sh
git clone https://github.com/dragosh/zen
cd zen
task package          # builds bin/Zen.app (downloads Vale once)
```

Drag `bin/Zen.app` into `/Applications`.

## The `zen` command

```sh
task install:cli      # links /usr/local/bin/zen to the app
# or, while developing:
task install          # links ~/.local/bin/zen to bin/zen
```

Check it:

```sh
zen --version         # zen 0.1.0-alpha.1
```

Next: [open a folder](opening-folders.md).
