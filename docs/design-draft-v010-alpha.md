# Zen.app ☯ 


`Zen` is a desktop markdown reader and editor with live reload and a runtime
plugin system, built on [Wails v3](https://v3.wails.io).

## Requirements (v0.1.0-alpha)

1. `Zen.app` - single(bundle) application which bundles main CLI `zen` binary and other prebuilt binaries for support 
2. `zen` terminal CLI

## Zen.app
  Main window show a big button to open a specific folder
  After open the app builds an index tree with all the markdown files based on their internal reference and turns it into a sidebar navigation 
  If the folder contains a root `README.md` file render it by default otherwise let the use select the file from the navigation.

## zen (CLI)
  Behave in the same way as the application only that it's opening the `cwd` where the command was invoked from

### Reference CLI subcommands
```
zen -> open the current folder, if there is a README.md file renders it
zenmd edit [path/to-file] -> editor with option specific filepath
```
## Core features
  - Server side markdown render with a pluggable system 
  - Editor WYSIWYG with a nice UI view and a button for split view
  - A page should render basic markdown elements like (images, svg, etc.)
  - All internal links should open in the same view and have a breadcrumb navigation on top
  - External links should ask to open a default browser.
  - Plug-in system for built-in renderers such as (diagrams, slides, polls, etc)
  - Syntax for custom plugins to be injected 
  - Use frontmatter to enable/disable custom plugins or other page configuration
  - Use a global configuration file.
  
## Brand and UI/UX design
  - Yin and Yang inspiration for logo but 
  - Black and white design with grayscale for buttons
  - Typography, editorial font
