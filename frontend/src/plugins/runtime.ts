// The page side of external plugins (docs/PLUGINS.md "Frontend: mount on your
// blocks"): the `zen` object plugin scripts register with, and mounting and
// cleaning up their handlers on the [data-zen-plugin] wrappers of a page.
import type { Api, PluginStreamEvent } from "../api";

export type PluginCtx = {
  id: string; kind: "block" | "inline" | "layout"; name: string; attrs: Record<string, string>;
  page: { path: string; title: string };
  storage: { get(key: string): Promise<unknown>; set(key: string, value: unknown): Promise<void>; keys(prefix?: string): Promise<string[]> };
  invoke(payload: unknown): Promise<unknown>;
  /**
   * Starts the plugin's stream hook with a JSON payload; onChunk gets each chunk (a line, for external
   * plugins) in order. `done` resolves when it ends and rejects with the error text ("cancelled" after
   * cancel()). Streams still running when the block unmounts are cancelled.
   */
  stream(payload: unknown, onChunk: (chunk: string) => void): { done: Promise<void>; cancel(): void };
  navigate(path: string, anchor?: string): void;
  /** The anchor of the current page, "" when none. */
  anchor(): string;
  /** Calls fn when the page's anchor changes without a reload; returns the remover. Removed on unmount. */
  onAnchor(fn: (anchor: string) => void): () => void;
  /** Sets (or with null/"" clears) a short label (12 characters at most) after the page's title in the sidebar. */
  badge(path: string, text: string | null): void;
};
export type CleanUp = () => void;
export type PluginHandler = { mount(el: HTMLElement, ctx: PluginCtx): void | CleanUp | Promise<void | CleanUp> };
export type Runtime = { mountAll(root: HTMLElement): void; unmountAll(): void; destroy(): void };
type Options = {
  api: Api; navigate(path: string, anchor?: string): void; page(): { path: string; title: string };
  /** The current page's anchor (default: none). */
  anchor?(): string;
  /** Subscribes to anchor changes that do not reload the page; returns the remover. */
  onAnchor?(fn: (anchor: string) => void): () => void;
  /** Sets or clears (null) the sidebar badge of a page. */
  badge?(path: string, text: string | null): void;
};

const BADGE_MAX = 12;
const noop = () => {};

/** One mounted wrapper. `dead` is set when it was unmounted before an async mount finished. */
type Mounted = {
  el: HTMLElement; id: string; cleanUp?: unknown; dead: boolean;
  /** Cancel functions of the streams it started that have not ended; anchor removers. */
  streams: Set<() => void>; offs: Set<() => void>;
};

/**
 * Handlers by plugin id. They outlive a runtime: a plugin's script is loaded
 * once per window (page/assets.ts), so its registration has to survive the
 * reader being unmounted and mounted again.
 */
const handlers = new Map<string, PluginHandler>();
/** The runtime of the reader that is showing a page, if any. */
let active: { remount(id: string): void } | null = null;

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

function register(id: string, handler: PluginHandler): void {
  if (typeof id !== "string" || id === "" || typeof handler?.mount !== "function") {
    console.error(`zen: register(${JSON.stringify(id)}) needs a plugin id and an object with a mount(el, ctx) function`);
    return;
  }
  handlers.set(id, handler);
  active?.remount(id);
}

/** The object plugin scripts see; made once and kept, so a script may hold on to it. */
function ensureGlobal(): void {
  const w = window as unknown as { zen?: unknown };
  if (!w.zen) w.zen = Object.freeze({ register, version: 1 });
}

/** `{key=value}` attributes from data-zen-attrs: strings only, {} when missing or invalid. */
function parseAttrs(text: string | null): Record<string, string> {
  const out: Record<string, string> = {};
  if (!text) return out;
  let v: unknown;
  try {
    v = JSON.parse(text);
  } catch {
    return out;
  }
  if (typeof v !== "object" || v === null || Array.isArray(v)) return out;
  for (const k of Object.keys(v)) {
    const x = (v as Record<string, unknown>)[k];
    if (typeof x === "string") out[k] = x;
  }
  return out;
}

/** The error box built-in plugins get from Go (plugins.ErrorBox), as DOM. */
function errorBox(doc: Document, id: string, text: string): HTMLElement {
  const box = doc.createElement("div");
  box.className = "zen-plugin-error";
  const name = doc.createElement("strong");
  name.textContent = id;
  box.append(name, `: ${text}`);
  return box;
}

export function installRuntime(opts: Options): Runtime {
  ensureGlobal();
  let root: HTMLElement | null = null;
  let mounted: Mounted[] = [];

  /** Logs and shows a failure in the plugin's own wrapper; the rest of the page is untouched. */
  function fail(m: Mounted, what: string, err: unknown): void {
    console.error(`zen: plugin "${m.id}" ${what} failed`, err);
    if (m.el.isConnected) m.el.prepend(errorBox(m.el.ownerDocument, m.id, message(err)));
  }

  function cleanUp(m: Mounted): void {
    const fn = m.cleanUp;
    m.cleanUp = undefined;
    if (typeof fn === "function") {
      try {
        fn();
      } catch (err) {
        fail(m, "clean-up", err);
      }
    }
    // Whatever the block left running goes with it.
    for (const cancel of Array.from(m.streams)) cancel();
    for (const off of Array.from(m.offs)) off();
  }

  // ---- streams: one listener for the runtime, calls told apart by their random id ----
  const calls = new Map<string, (e: PluginStreamEvent) => void>();
  /** Events of calls whose id the page does not know yet (they can beat the start's answer). */
  const early = new Map<string, PluginStreamEvent[]>();
  let starting = 0;
  let stopListening: (() => void) | null = null;

  function dispatch(e: PluginStreamEvent): void {
    const handle = calls.get(e.call);
    if (handle) handle(e);
    else if (starting > 0) early.set(e.call, [...(early.get(e.call) ?? []), e]);
  }
  function listenIfNeeded(): void {
    if (!stopListening) stopListening = opts.api.on("plugin:stream", dispatch);
  }
  function settleListening(): void {
    if (starting === 0) early.clear();
    if (starting === 0 && calls.size === 0 && stopListening) {
      stopListening();
      stopListening = null;
    }
  }

  function startStream(m: Mounted, id: string, payload: unknown, onChunk: (chunk: string) => void): { done: Promise<void>; cancel(): void } {
    let resolve!: () => void;
    let reject!: (err: Error) => void;
    const done = new Promise<void>((res, rej) => { resolve = res; reject = rej; });
    done.catch(noop); // the caller may not look at it, e.g. when the block unmounts
    let call = "";
    let over = false;
    const finish = (err?: Error): void => {
      if (over) return;
      over = true;
      if (call) calls.delete(call);
      m.streams.delete(cancel);
      if (err) reject(err); else resolve();
      settleListening();
    };
    const cancel = (): void => {
      if (over) return;
      if (call) opts.api.pluginCancel(call).catch(noop);
      finish(new Error("cancelled")); // later chunks are dropped; with no id yet the start cancels once it has one
    };
    const handle = (e: PluginStreamEvent): void => {
      if (over) return;
      if (e.done) {
        finish(e.error ? new Error(e.error) : undefined);
        return;
      }
      try {
        onChunk(e.chunk);
      } catch (err) {
        console.error(`zen: plugin "${id}" stream handler failed`, err);
      }
    };
    if (m.dead) {
      over = true;
      reject(new Error("cancelled"));
      return { done, cancel: noop };
    }
    m.streams.add(cancel);
    let text: string | undefined;
    try {
      text = JSON.stringify(payload ?? null);
    } catch { /* falls to the check below */ }
    if (typeof text !== "string") {
      finish(new TypeError("zen: stream payloads must be JSON"));
      return { done, cancel };
    }
    listenIfNeeded();
    starting++;
    opts.api.pluginStream(id, text).then((c) => {
      starting--;
      call = c;
      if (over) {
        opts.api.pluginCancel(c).catch(noop);
      } else {
        calls.set(c, handle);
        for (const e of early.get(c) ?? []) handle(e);
      }
      early.delete(c);
      settleListening();
    }, (err) => {
      starting--;
      finish(err instanceof Error ? err : new Error(message(err)));
    });
    return { done, cancel };
  }

  function context(el: HTMLElement, id: string, m: Mounted): PluginCtx {
    const layout = el.getAttribute("data-zen-layout");
    const { path, title } = opts.page();
    // Bound to this wrapper's plugin id: nothing here takes an id from the script.
    return Object.freeze({
      id,
      kind: layout ? "layout" : el.tagName === "SPAN" ? "inline" : "block",
      name: el.getAttribute("data-zen-name") || layout || "",
      attrs: Object.freeze(parseAttrs(el.getAttribute("data-zen-attrs"))),
      page: Object.freeze({ path, title }),
      storage: Object.freeze({
        async get(key: string): Promise<unknown> {
          const text = await opts.api.pluginStorageGet(id, String(key));
          return text ? (JSON.parse(text) as unknown) ?? null : null;
        },
        async set(key: string, value: unknown): Promise<void> {
          const text = JSON.stringify(value);
          if (typeof text !== "string") throw new TypeError("zen: storage values must be JSON (not undefined or a function)");
          await opts.api.pluginStorageSet(id, String(key), text);
        },
        keys: (prefix?: string) => opts.api.pluginStorageKeys(id, prefix === undefined ? "" : String(prefix)),
      }),
      async invoke(payload: unknown): Promise<unknown> {
        const text = await opts.api.pluginInvoke(id, JSON.stringify(payload ?? null) ?? "null");
        return text ? (JSON.parse(text) as unknown) : null;
      },
      stream: (payload: unknown, onChunk: (chunk: string) => void) => startStream(m, id, payload, onChunk),
      navigate: (path: string, anchor?: string) => opts.navigate(path, anchor),
      anchor: () => opts.anchor?.() ?? "",
      onAnchor(fn: (anchor: string) => void): () => void {
        if (typeof fn !== "function" || !opts.onAnchor || m.dead) return noop;
        const remove = opts.onAnchor((a) => {
          try {
            fn(a);
          } catch (err) {
            console.error(`zen: plugin "${id}" anchor handler failed`, err);
          }
        });
        const off = () => {
          m.offs.delete(off);
          remove();
        };
        m.offs.add(off);
        return off;
      },
      badge(path: string, text: string | null): void {
        if (typeof path !== "string" || path === "") return;
        const label = text === null || text === undefined ? "" : Array.from(String(text).trim()).slice(0, BADGE_MAX).join("");
        opts.badge?.(path, label === "" ? null : label);
      },
    }) as PluginCtx;
  }

  function mountOne(el: HTMLElement, id: string, handler: PluginHandler): void {
    const m: Mounted = { el, id, dead: false, streams: new Set(), offs: new Set() };
    mounted.push(m);
    const done = (result: unknown) => {
      m.cleanUp = result;
      if (m.dead) cleanUp(m);
    };
    try {
      const result = handler.mount(el, context(el, id, m));
      if (result && typeof (result as PromiseLike<unknown>).then === "function") {
        (result as PromiseLike<unknown>).then(done, (err) => { if (!m.dead) fail(m, "mount", err); });
      } else {
        done(result);
      }
    } catch (err) {
      fail(m, "mount", err);
    }
  }

  /** Mounts the wrappers of one plugin (or of every registered one) that are on the page. */
  function mountIn(only?: string): void {
    if (!root) return;
    const here = root;
    for (const el of Array.from(here.querySelectorAll<HTMLElement>("[data-zen-plugin]"))) {
      const id = el.getAttribute("data-zen-plugin") ?? "";
      const handler = handlers.get(id);
      if (!handler || (only !== undefined && id !== only)) continue;
      // An earlier mount (a layout, say) may have thrown this wrapper away.
      if (!here.contains(el)) continue;
      mountOne(el, id, handler);
    }
  }

  /** Last mounted first, so a block inside a layout goes before the layout. */
  function unmount(only?: string): void {
    const go = only === undefined ? mounted : mounted.filter((m) => m.id === only);
    mounted = only === undefined ? [] : mounted.filter((m) => m.id !== only);
    for (const m of go.slice().reverse()) {
      m.dead = true;
      cleanUp(m);
    }
  }

  const self = {
    remount(id: string) {
      unmount(id);
      mountIn(id);
    },
  };
  active = self;

  return {
    mountAll(next) {
      unmount();
      root = next;
      mountIn();
    },
    unmountAll() {
      unmount();
      root = null;
    },
    destroy() {
      unmount();
      root = null;
      if (active === self) active = null;
    },
  };
}
