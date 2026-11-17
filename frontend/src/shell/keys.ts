// Shortcuts that used to come from native menu items and now go through the
// windows (app/menu). Plain predicates so they are testable without a window.

const plain = (e: KeyboardEvent, key: string): boolean =>
  (e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === key;

/** ⌘W, no other modifiers: close the window that has focus. */
export function closeWindowKey(e: KeyboardEvent): boolean {
  return plain(e, "w");
}

/** ⌘R, no other modifiers: reload. Dev builds have it in the Develop menu; production swallows it. */
export function reloadKey(e: KeyboardEvent): boolean {
  return plain(e, "r");
}
