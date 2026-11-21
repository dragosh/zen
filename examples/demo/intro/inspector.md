# The Inspector and the Developer panel

The **Inspector** is a panel under the page that tells you about the page you
are reading. Open and close it with the **Inspector** button at the bottom of
the sidebar, or with <kbd>⌥⌘I</kbd>. Drag its top edge to resize it; Zenmark
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

## The Developer panel

The Developer panel is two more tabs of the same Inspector, off until you turn
them on. Open **Settings** (<kbd>⌘,</kbd>), choose **Developer** and tick
**Show Console in the Inspector**. Then:

- **Console** shows the app's log, errors from plugins and what the page
  prints. Filter by source (All, App, Plugins or Page), by level or by text.
  **Clear** empties the list and **Copy** copies the lines. <kbd>⌥⌘J</kbd>
  opens the Inspector on this tab.
- **Build** shows what this Zenmark is made of: its version, features,
  plugins, layouts and theme, with a list on the left and details on the right.

Logs hold file paths and error messages, never the text of your pages.
