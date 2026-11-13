export type Entry = { path: string; anchor: string };

export type History = {
  /** Adds an entry after the current one, dropping anything forward of it. */
  push(entry: Entry): void;
  /** Moves one step and returns the new current entry, or null at the end. */
  back(): Entry | null;
  forward(): Entry | null;
  current(): Entry | null;
  canBack(): boolean;
  canForward(): boolean;
  reset(): void;
};

/** The in-app navigation stack (the webview's own history is not used). */
export function createHistory(): History {
  let entries: Entry[] = [];
  let index = -1;

  return {
    push(entry) {
      const cur = entries[index];
      if (cur && cur.path === entry.path && cur.anchor === entry.anchor) return;
      entries = [...entries.slice(0, index + 1), { path: entry.path, anchor: entry.anchor }];
      index = entries.length - 1;
    },
    back: () => (index > 0 ? entries[--index] : null),
    forward: () => (index < entries.length - 1 ? entries[++index] : null),
    current: () => entries[index] ?? null,
    canBack: () => index > 0,
    canForward: () => index < entries.length - 1,
    reset() {
      entries = [];
      index = -1;
    },
  };
}
