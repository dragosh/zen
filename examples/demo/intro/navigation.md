# Sidebar, links and breadcrumbs

## The sidebar is built from your links

Zenmark reads every markdown file and follows the links between them, starting
at `README.md`. A page appears under the **first** page that links to it, so
the sidebar mirrors how your documents point to each other. You never keep a
separate table of contents. The order of the links on a page is the order in
the sidebar: this guide's own sidebar follows the links of its
[README](../README.md).

Pages that nothing links to are listed under **Other pages**, grouped by
folder. This folder has one, `notes/scratch.md`. Another page,
[a hidden note](../notes/hidden.md), is reachable only through this link: it
asks to stay out of the sidebar.

To change a page's place or label, use
[frontmatter](frontmatter.md#navigation).

## The sidebar menu

Right-click a page in the sidebar (or Control-click it, or press the menu key
or <kbd>⇧F10</kbd> on the selected row) for a menu:

| Item | What it does |
|---|---|
| Open | the page |
| Copy Path, Copy Link | `guides/install.md`, or `[Install](install.md)` ready to paste |
| Reveal in Finder | the file, selected in Finder |

The arrow keys move in the sidebar: <kbd>↑</kbd> and <kbd>↓</kbd> between
rows, <kbd>→</kbd> and <kbd>←</kbd> to open and close a branch.
<kbd>⌘\</kbd> shows or hides the whole sidebar.

## Links

| Link | What happens |
|---|---|
| [A page in this folder](markdown.md) | opens in place |
| [A heading on a page](frontmatter.md#slugs) | opens and scrolls there |
| [A slug](open) | opens the page with `slug: open` |
| [An external site](https://commonmark.org) | asks before opening your browser |
| [A missing page](does-not-exist.md) | shown muted, does nothing |

The last link is broken on purpose. The Inspector lists it under Problems, see
[the Inspector](inspector.md).

## Breadcrumb, back and forward

The bar above the page shows where you are in the sidebar tree; each part is
a link. **‹** and **›**, or <kbd>⌘[</kbd> and <kbd>⌘]</kbd>, go back and forward.

## Live reload

Change a file in any editor and the open page updates by itself, keeping your
scroll position. New, renamed and deleted pages update the sidebar.
