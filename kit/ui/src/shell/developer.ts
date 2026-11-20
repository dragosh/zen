// The Console setting (ui/developer-tab, ui/inspector): Settings › Developer › "Show Console in the Inspector".
// The Console is a tab of the Inspector; this file keeps the switch, the key that opens it and nothing
// else. The setting is per machine (localStorage, like the theme) and every window follows it. The
// storage key stays `zen.developer`, so users keep their choice.

export const DEV_KEY = "zen.developer";
const EVENT = "zen:developer";

export function developerEnabled(): boolean {
  try {
    return localStorage.getItem(DEV_KEY) === "1";
  } catch {
    return false;
  }
}

/** Remembers the setting and tells this window (other windows hear the storage event). */
export function setDeveloperEnabled(on: boolean): void {
  try {
    if (on) localStorage.setItem(DEV_KEY, "1");
    else localStorage.removeItem(DEV_KEY);
  } catch { /* storage unavailable: this window only */ }
  window.dispatchEvent(new CustomEvent(EVENT));
}

/** Calls fn when the setting changes, here or in another window. */
export function followDeveloper(fn: () => void, win: Window = window): () => void {
  const onStorage = (e: StorageEvent) => { if (e.key === null || e.key === DEV_KEY) fn(); };
  win.addEventListener(EVENT, fn);
  win.addEventListener("storage", onStorage);
  return () => {
    win.removeEventListener(EVENT, fn);
    win.removeEventListener("storage", onStorage);
  };
}

/** ⌥⌘J (or ⌥⌃J): the key is "∆" with Option held, so the physical key is what counts. */
export function developerKey(e: KeyboardEvent): boolean {
  return e.altKey && (e.metaKey || e.ctrlKey) && !e.shiftKey && e.code === "KeyJ";
}
