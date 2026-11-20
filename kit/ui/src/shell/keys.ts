// Shortcuts that used to come from native menu items and now go through the
// windows (app/menu). Plain predicates so they are testable without a window.

const plain = (e: KeyboardEvent, key: string): boolean =>
  (e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === key;

/** ⌘W, no other modifiers: close the window that has focus. */
export function closeWindowKey(e: KeyboardEvent): boolean {
  return plain(e, "w");
}

/** What a window shortcut asks for. */
export type ShellAction = "open-folder" | "settings" | "new-page" | "new-site" | "toggle-sidebar" | "back" | "forward";

/**
 * The shortcut of the window frame: ⌘O, ⌘, ⌘N, ⇧⌘N, ⌘\ and ⌘[ ⌘]. A shortcut whose feature the build
 * does not have gives null (New Page is the editor's, New Site is newsite's, the sidebar is nav's);
 * Open Folder, Settings and Back/Forward are in every build.
 */
export function shellAction(e: KeyboardEvent, has: (feature: string) => boolean): ShellAction | null {
  if (!(e.metaKey || e.ctrlKey) || e.altKey) return null;
  const k = e.key.toLowerCase();
  if (e.shiftKey) return k === "n" && has("newsite") ? "new-site" : null;
  switch (k) {
    case "o": return "open-folder";
    case ",": return "settings";
    case "n": return has("editor") ? "new-page" : null;
    case "\\": return has("nav") ? "toggle-sidebar" : null;
    case "[": return "back";
    case "]": return "forward";
  }
  return null;
}

/** ⌘R, no other modifiers: reload. Dev builds have it in the Develop menu; production swallows it. */
export function reloadKey(e: KeyboardEvent): boolean {
  return plain(e, "r");
}
