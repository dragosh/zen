// The page side of plugins (docs/PLUGINS.md "Frontend: mount on your blocks"):
// each plugin's script is an ES module loaded with import() from the page's
// assets; its default export ({ mount }) is bound to that plugin's id, and
// mounted and cleaned up on the [data-zen-plugin] wrappers of the page.
// Layouts (core/layouts) work the same way with their own scripts from
// /_layouts/<id>/, mounted on the [data-zen-layout] wrapper with a ctx that
// has no invoke and no stream (layouts have no programs) and whose storage
// goes to the layout services.
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
/** What a layout script gets: a plugin's ctx without the backend (no invoke, no stream). */
export type LayoutCtx = Omit<PluginCtx, "invoke" | "stream" | "kind"> & { kind: "layout" };
export type CleanUp = () => void;
export type LayoutHandler = { mount(el: HTMLElement, ctx: LayoutCtx): void | CleanUp | Promise<void | CleanUp> };
export type PluginHandler = { mount(el: HTMLElement, ctx: PluginCtx): void | CleanUp | Promise<void | CleanUp> };
export type Runtime = {
  /**
   * Mounts the page's wrappers. With assets, the scripts of the page are imported first (once per
   * runtime generation); wrappers whose script is still loading mount when it arrives. Resolves
   * when the imports are settled and their wrappers mounted.
   */
  mountAll(root: HTMLElement, assets?: PluginAssets): Promise<void>;
  /** Imports the scripts named by assets without mounting anything yet. */
  load(assets: PluginAssets): Promise<void>;
  unmountAll(): void;
  /** Unmounts everything, forgets every bound module and removes plugin stylesheets. */
  reset(): void;
  destroy(): void;
};
type Options = {
  api: Api; navigate(path: string, anchor?: string): void; page(): { path: string; title: string };
  /** The current page's anchor (default: none). */
  anchor?(): string;
  /** Subscribes to anchor changes that do not reload the page; returns the remover. */
  onAnchor?(fn: (anchor: string) => void): () => void;
  /** Sets or clears (null) the sidebar badge of a page. */
  badge?(path: string, text: string | null): void;
  /** Loads a plugin script as an ES module (default: dynamic import()). Tests inject it. */
  importModule?(url: string): Promise<unknown>;
  /**
   * Called after the runtime dropped everything because the plugin set changed (plugins:changed,
   * workspace:opened, workspace:changed); the host re-renders the current page so its new asset
   * URLs are loaded and mounted again.
   */
  refresh?(): void;
};
/** The plugin assets of a page (Page.assets); only the scripts are used here. */
export type PluginAssets = { js: string[]; css?: string[] };

const BADGE_MAX = 12;
const noop = () => {};

/** One mounted wrapper. `dead` is set when it was unmounted before an async mount finished. */
type Mounted = {
  el: HTMLElement; id: string; cleanUp?: unknown; dead: boolean;
  /** Cancel functions of the streams it started that have not ended; anchor removers. */
  streams: Set<() => void>; offs: Set<() => void>;
};

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

/** window.zen: only a version; non-writable and non-configurable, so no script can replace it. */
function ensureGlobal(): void {
  const w = window as unknown as { zen?: unknown };
  const d = Object.getOwnPropertyDescriptor(w, "zen");
  if (d && d.configurable === false) return;
  try {
    Object.defineProperty(w, "zen", { value: Object.freeze({ version: 2 }), writable: false, configurable: false, enumerable: true });
  } catch (err) {
    console.error("zen: could not define window.zen", err);
  }
}

const PLUGIN_URL = /^\/_plugins\/([^/?#]+)\//;
const LAYOUT_URL = /^\/_layouts\/([^/?#]+)\//;
/** The plugin id a script URL belongs to ("" when it is not a /_plugins/ URL). */
export function pluginOfUrl(url: string): string {
  const m = PLUGIN_URL.exec(url);
  return m ? decodeURIComponent(m[1]) : "";
}
/** The layout id a script URL belongs to ("" when it is not a /_layouts/ URL). */
export function layoutOfUrl(url: string): string {
  const m = LAYOUT_URL.exec(url);
  return m ? decodeURIComponent(m[1]) : "";
}
/** Modules are bound per kind: a plugin and a layout may share an id. */
const keyOf = (layout: boolean, id: string) => `${layout ? "layout" : "plugin"}:${id}`;

/** What loading a plugin's scripts came to: its handler, or why there is none. */
type Bound = { handler?: PluginHandler; error?: string; ready: Promise<void>; settled: boolean };

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

  function startStream(m: Mounted, id: string, pagePath: string, payload: unknown, onChunk: (chunk: string) => void): { done: Promise<void>; cancel(): void } {
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
      // Go batches output (one event per 50 ms, lines joined with "\n");
      // plugins still get one call per line.
      for (const line of e.chunk.split("\n")) {
        try {
          onChunk(line);
        } catch (err) {
          console.error(`zen: plugin "${id}" stream handler failed`, err);
        }
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
    opts.api.pluginStream(id, pagePath, text).then((c) => {
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

  /** Storage over the plugin services or, for a layout, the layout services; always bound to the wrapper's id. */
  function storageFor(id: string, layout: boolean): PluginCtx["storage"] {
    const get = (key: string) => (layout ? opts.api.layoutStorageGet(id, key) : opts.api.pluginStorageGet(id, key));
    const put = (key: string, json: string) => (layout ? opts.api.layoutStorageSet(id, key, json) : opts.api.pluginStorageSet(id, key, json));
    const list = (prefix: string) => (layout ? opts.api.layoutStorageKeys(id, prefix) : opts.api.pluginStorageKeys(id, prefix));
    return Object.freeze({
      async get(key: string): Promise<unknown> {
        const text = await get(String(key));
        return text ? (JSON.parse(text) as unknown) ?? null : null;
      },
      async set(key: string, value: unknown): Promise<void> {
        const text = JSON.stringify(value);
        if (typeof text !== "string") throw new TypeError("zen: storage values must be JSON (not undefined or a function)");
        await put(String(key), text);
      },
      keys: (prefix?: string) => list(prefix === undefined ? "" : String(prefix)),
    });
  }

  function context(el: HTMLElement, id: string, m: Mounted): PluginCtx | LayoutCtx {
    const layout = el.hasAttribute("data-zen-layout");
    const { path, title } = opts.page();
    // Bound to this wrapper's id: nothing here takes an id from the script.
    const common = {
      id,
      name: el.getAttribute("data-zen-name") || el.getAttribute("data-zen-layout") || "",
      attrs: Object.freeze(parseAttrs(el.getAttribute("data-zen-attrs"))),
      page: Object.freeze({ path, title }),
      storage: storageFor(id, layout),
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
    };
    // Layouts have no programs: no invoke, no stream.
    if (layout) return Object.freeze({ ...common, kind: "layout" as const }) as LayoutCtx;
    return Object.freeze({
      ...common,
      kind: el.tagName === "SPAN" ? "inline" : "block",
      async invoke(payload: unknown): Promise<unknown> {
        const text = await opts.api.pluginInvoke(id, path, JSON.stringify(payload ?? null) ?? "null");
        return text ? (JSON.parse(text) as unknown) : null;
      },
      stream: (payload: unknown, onChunk: (chunk: string) => void) => startStream(m, id, path, payload, onChunk),
    }) as PluginCtx;
  }

  function mountOne(el: HTMLElement, id: string, handler: PluginHandler | LayoutHandler): void {
    const m: Mounted = { el, id, dead: false, streams: new Set(), offs: new Set() };
    mounted.push(m);
    const done = (result: unknown) => {
      m.cleanUp = result;
      if (m.dead) cleanUp(m);
    };
    try {
      const result = (handler as PluginHandler).mount(el, context(el, id, m) as PluginCtx);
      if (result && typeof (result as PromiseLike<unknown>).then === "function") {
        (result as PromiseLike<unknown>).then(done, (err) => { if (!m.dead) fail(m, "mount", err); });
      } else {
        done(result);
      }
    } catch (err) {
      fail(m, "mount", err);
    }
  }

  // ---- modules: one import() per script, bound to the plugin id its URL names ----
  const importModule = opts.importModule ?? ((url: string) => import(/* @vite-ignore */ url) as Promise<unknown>);
  /** Bumped by every mountAll, unmountAll, reset and destroy: late imports of an older one do nothing. */
  let epoch = 0;
  let bound = new Map<string, Bound>();

  function bind(scripts: string[]): Bound {
    const b: Bound = { ready: Promise.resolve(), settled: false };
    b.ready = Promise.all(scripts.map((url) => importModule(url).then((mod) => ({ mod }), (err: unknown) => ({ err }))))
      .then((results) => {
        for (const r of results) {
          const def = "mod" in r ? (r.mod as { default?: unknown } | null | undefined)?.default : undefined;
          if (def && typeof (def as PluginHandler).mount === "function") {
            b.handler = def as PluginHandler;
            return;
          }
        }
        const failed = results.find((r) => "err" in r) as { err: unknown } | undefined;
        b.error = failed
          ? `its script could not be loaded: ${message(failed.err)}`
          : "its script has no default export with a mount(el, ctx) function";
      })
      .then(() => { b.settled = true; });
    return b;
  }

  /** Starts importing the scripts of assets that no module is bound for yet; returns what is still loading. */
  function loadScripts(assets: PluginAssets | undefined): Promise<unknown>[] {
    const byKey = new Map<string, string[]>();
    for (const url of assets?.js ?? []) {
      const plugin = pluginOfUrl(url);
      const layout = plugin === "" ? layoutOfUrl(url) : "";
      if ((plugin === "" && layout === "") || url.split(/[/?#]/).includes("..")) continue;
      const key = plugin !== "" ? keyOf(false, plugin) : keyOf(true, layout);
      if (!byKey.has(key)) byKey.set(key, []);
      byKey.get(key)!.push(url);
    }
    const waits: Promise<unknown>[] = [];
    byKey.forEach((scripts, key) => {
      let b = bound.get(key);
      if (!b) {
        b = bind(scripts);
        bound.set(key, b);
      }
      if (!b.settled) waits.push(b.ready);
    });
    return waits;
  }

  /**
   * Mounts at once when every script is here; otherwise once all of them have settled, so wrappers
   * still mount in document order (a block inside a layout before the layout).
   */
  function mountWhenReady(assets: PluginAssets | undefined): Promise<void> {
    const gen = epoch;
    const waits = loadScripts(assets);
    if (waits.length === 0) {
      mountIn();
      return Promise.resolve();
    }
    return Promise.all(waits).then(() => { if (gen === epoch) mountIn(); });
  }

  /** Mounts the wrappers of every plugin and layout with a bound script that are on the page and not mounted yet. */
  function mountIn(): void {
    if (!root) return;
    const here = root;
    for (const el of Array.from(here.querySelectorAll<HTMLElement>("[data-zen-plugin], [data-zen-layout]"))) {
      // A layout wrapper is bound to the layout's module, whatever else it says.
      const layout = el.hasAttribute("data-zen-layout");
      const id = (layout ? el.getAttribute("data-zen-layout") : el.getAttribute("data-zen-plugin")) ?? "";
      const b = bound.get(keyOf(layout, id));
      if (!b) continue;
      // An earlier mount (a layout, say) may have thrown this wrapper away; some are mounted already.
      if (!here.contains(el) || mounted.some((m) => m.el === el)) continue;
      if (b.handler) {
        mountOne(el, id, b.handler);
      } else if (b.error) {
        const m: Mounted = { el, id, dead: false, streams: new Set(), offs: new Set() };
        mounted.push(m);
        fail(m, "load", new Error(b.error));
      }
    }
  }

  /** Last mounted first, so a block inside a layout goes before the layout. */
  function unmount(): void {
    const go = mounted;
    mounted = [];
    for (const m of go.slice().reverse()) {
      m.dead = true;
      cleanUp(m);
    }
  }

  function removeStylesheets(): void {
    const doc = root?.ownerDocument ?? document;
    for (const link of Array.from(doc.head.querySelectorAll<HTMLElement>("link[data-zen-asset]"))) {
      const href = link.dataset.zenAsset ?? "";
      if (href.startsWith("/_plugins/") || href.startsWith("/_layouts/")) link.remove();
    }
  }

  function reset(): void {
    epoch++;
    unmount();
    bound = new Map();
    removeStylesheets();
  }

  // The plugin set or the folder changed: nothing loaded so far may keep running.
  const offs = (["plugins:changed", "layouts:changed", "workspace:opened", "workspace:changed"] as const).map((ev) =>
    opts.api.on(ev as "plugins:changed", () => {
      reset();
      opts.refresh?.();
    }));

  return {
    mountAll(next, assets) {
      epoch++;
      unmount();
      root = next;
      return mountWhenReady(assets);
    },
    load(assets) {
      return mountWhenReady(assets);
    },
    unmountAll() {
      epoch++;
      unmount();
      root = null;
    },
    reset,
    destroy() {
      epoch++;
      unmount();
      root = null;
      bound = new Map();
      for (const off of offs) off();
    },
  };
}
