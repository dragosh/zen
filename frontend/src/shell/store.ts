import type { AppState, Store } from "./types";

/** The app state. `set` merges and notifies only when a field really changed. */
export function createStore(initial: Partial<AppState> = {}): Store {
  let state: AppState = { workspace: null, path: "", anchor: "", mode: "read", ...initial };
  const subscribers = new Set<(s: AppState) => void>();

  return {
    get: () => state,
    set(patch) {
      const keys = Object.keys(patch) as (keyof AppState)[];
      if (!keys.some((k) => !Object.is(state[k], patch[k]))) return;
      state = { ...state, ...patch };
      for (const fn of Array.from(subscribers)) fn(state);
    },
    subscribe(fn) {
      subscribers.add(fn);
      return () => void subscribers.delete(fn);
    },
  };
}
