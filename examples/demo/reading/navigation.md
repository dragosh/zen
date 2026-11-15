# Sidebar, links and breadcrumbs

## The sidebar is built from your links

zen reads every markdown file and follows the links between them, starting at
`README.md`. A page appears under the **first** page that links to it, so the
sidebar mirrors how your documents point to each other. You never maintain a
separate table of contents.

Pages that nothing links to are listed under **Other pages**, grouped by
folder. This folder has one, `notes/scratch.md`. Another page,
[a hidden note](../notes/hidden.md), is reachable only through this link: it
asks to stay out of the sidebar.

To change a page's place, label or order, use
[frontmatter](frontmatter.md#navigation).

## Links

| Link | What happens |
|---|---|
| [A page in this folder](../reference/markdown.md) | opens in place |
| [A heading on a page](frontmatter.md#slugs) | opens and scrolls there |
| [A slug](install) | opens the page with `slug: install` |
| [An external site](https://github.com/errata-ai/vale) | asks before opening your browser |
| [A missing page](does-not-exist.md) | shown muted, does nothing |

## Breadcrumb, back and forward

The bar above the page shows where you are in the sidebar tree; each part is
a link. **‹** and **›**, or <kbd>⌘[</kbd> and <kbd>⌘]</kbd>, go back and forward.

## Live reload

Change a file in any editor and the open page updates by itself, keeping your
scroll position. New, renamed and deleted pages update the sidebar.
