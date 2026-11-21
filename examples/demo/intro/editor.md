---
title: Edit a page
---
# Edit a page

Press the pencil in the top bar or <kbd>⌘E</kbd> on a page. The page opens in its own editor
window; the main window keeps reading and **follows what you type**, live,
before you save.

## What the editor shows

- **Line numbers** down the left.
- **Syntax colours** for markdown (headings, emphasis, links, code) and for
  the code inside fenced blocks.

## Split view

Press **Split** in the header, or <kbd>⇧⌘P</kbd>, to see the rendered page
next to the source. In a narrow window the preview sits below instead.

- The preview shows what you typed, before you save.
- Drag the thin divider to resize, or focus it and press <kbd>←</kbd> and
  <kbd>→</kbd>. Zenmark remembers the width and whether Split was on.
- The preview scrolls along with the source.
- Links in the preview do nothing; use the main window to follow them.
- Settings files have no preview, so they have no **Split** button.

## Toolbar

| Button | Does | Shortcut |
|---|---|---|
| **B** | bold | <kbd>⌘B</kbd> |
| *I* | italic | <kbd>⌘I</kbd> |
| ~~S~~ | strikethrough | |
| H1, H2 | heading | |
| “ ” | quote | |
| •, 1., ☐ | bullet, numbered, task list | |
| `</>` | inline code, or a code block for several lines | |
| 🔗 | link | <kbd>⌘K</kbd> |
| ▦ | table | |

Buttons toggle: press **B** again to remove the bold. <kbd>Tab</kbd> and
<kbd>⇧Tab</kbd> indent; <kbd>Enter</kbd> continues lists and quotes.

## Saving and closing

- <kbd>⌘S</kbd> saves. A dot ● in the header means unsaved changes.
- **Done** or <kbd>Esc</kbd> closes the window. With unsaved changes Zenmark asks
  **Save**, **Discard** or **Cancel**, also when you close the main window or
  quit.
- If the file changes on disk while you edit, Zenmark offers **Reload from disk**
  or **Keep mine**.

## New pages

<kbd>⌘N</kbd>, or **File › New Page…**, asks for a title and a folder and
opens the new page in an editor window. In the sidebar, **New Page Here…** on a
page's menu starts in that page's folder. The new page is linked from nowhere
yet, so it shows under **Other pages** until a page links to it; see
[the sidebar](navigation.md).

## Settings files

**Settings › Configuration** opens `config.yml` and `.zenmd.yml` in the editor
with **Open in Editor**. A file that doesn't exist yet has **Create** instead.
