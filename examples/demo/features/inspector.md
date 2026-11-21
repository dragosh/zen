---
title: Inspector
---
# The Inspector

The **Inspector** is a panel under the page that tells you about the page you
are reading. Open and close it with the **Inspector** button in the top bar (a
window with a panel and a prompt), or with <kbd>⌥⌘I</kbd>. Drag its top edge to resize it; Zenmark
remembers the height and whether it was open.

Every tab has two columns: a list on the left and the selected item's
details on the right.

## Page Info

What this page is made of: its title, layout, word count, reading time,
headings, links (internal and external), images, the plugins that drew
something and the frontmatter keys. On the right, the file: its path, full
path, size and dates. **Copy path** and **Reveal in Finder** are buttons
there.

Open the panel on this page: it shows `intro/inspector.md`, a layout of
`article` and a list of headings.

## Problems

The things that are wrong on the page, one row each:

- a **broken link**, like the deliberate one on the
  [navigation page](navigation.md);
- an **image** that is missing;
- a **warning** from rendering, such as a bad frontmatter value.

Choose a row to see the message, where it is and the rule on the right, and
to scroll to it on the page, where it is marked. The Inspector button shows
how many problems the page has. Pages of this guide are clean, apart from the
one link above.

The Inspector has two more tabs, Console and Build, which are off until you
turn them on; see [Developer](developer.md).
