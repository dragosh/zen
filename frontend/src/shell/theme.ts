// Theme preference (settings panel): system, light or dark. Stored per
// machine in localStorage, applied as <html data-theme>, and kept in sync
// across windows through the storage event.
export type Theme = "system" | "light" | "dark";
export const THEME_KEY = "zen.theme";

export function storedTheme(): Theme {
  try {
    const v = localStorage.getItem(THEME_KEY);
    return v === "light" || v === "dark" ? v : "system";
  } catch {
    return "system";
  }
}

export function applyTheme(t: Theme, doc: Document = document): void {
  const root = doc.documentElement;
  if (t === "system") delete root.dataset.theme;
  else root.dataset.theme = t;
}

export function setTheme(t: Theme, doc: Document = document): void {
  try {
    if (t === "system") localStorage.removeItem(THEME_KEY);
    else localStorage.setItem(THEME_KEY, t);
  } catch { /* storage unavailable: this window only */ }
  applyTheme(t, doc);
}

/** Applies the stored theme now and whenever another window changes it. */
export function followTheme(doc: Document = document): () => void {
  applyTheme(storedTheme(), doc);
  const view = doc.defaultView;
  const onStorage = (e: StorageEvent) => {
    if (e.key === null || e.key === THEME_KEY) applyTheme(storedTheme(), doc);
  };
  view?.addEventListener("storage", onStorage);
  return () => view?.removeEventListener("storage", onStorage);
}
