# The editor

Press **Edit** or <kbd>⌘E</kbd> on a page. The page opens in its own editor
window; the main window keeps reading and **follows what you type**, live,
before you save.

## What the editor shows

- **Line numbers** down the left.
- **Syntax colours** for markdown (headings, emphasis, links, code) and for
  the code inside fenced blocks.
- **Spelling and style flags** from Vale as wavy underlines, with the
  **Problems** list underneath. See [linting](linting.md).

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
- **Done** or <kbd>Esc</kbd> closes the window. With unsaved changes zen asks
  **Save**, **Discard** or **Cancel**, also when you close the main window or
  quit.
- If the file changes on disk while you edit, zen offers **Reload from disk**
  or **Keep mine**.
