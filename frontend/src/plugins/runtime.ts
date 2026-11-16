// The page side of external plugins (docs/PLUGINS.md "Frontend: mount on your
// blocks"): the `zen` object plugin scripts register with, and mounting and
// cleaning up their handlers on the [data-zen-plugin] wrappers of a page.
import type { Api } from "../api";

export type PluginCtx = {
  id: string; kind: "block" | "inline" | "layout"; name: string; attrs: Record<string, string>;
  page: { path: string; title: string };
  storage: { get(key: string): Promise<unknown>; set(key: string, value: unknown): Promise<void> };
  invoke(payload: unknown): Promise<unknown>;
  navigate(path: string, anchor?: string): void;
};
export type CleanUp = () => void;
export type PluginHandler = { mount(el: HTMLElement, ctx: PluginCtx): void | CleanUp | Promise<void | CleanUp> };
export type Runtime = { mountAll(root: HTMLElement): void; unmountAll(): void; destroy(): void };
type Options = { api: Api; navigate(path: string, anchor?: string): void; page(): { path: string; title: string } };

/** One mounted wrapper. `dead` is set when it was unmounted before an async mount finished. */
type Mounted = { el: HTMLElement; id: string; cleanUp?: unknown; dead: boolean };

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
    if (typeof fn !== "function") return;
    try {
      fn();
    } catch (err) {
      fail(m, "clean-up", err);
    }
  }

  function context(el: HTMLElement, id: string): PluginCtx {
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
      }),
      async invoke(payload: unknown): Promise<unknown> {
        const text = await opts.api.pluginInvoke(id, JSON.stringify(payload ?? null) ?? "null");
        return text ? (JSON.parse(text) as unknown) : null;
      },
      navigate: (path: string, anchor?: string) => opts.navigate(path, anchor),
    }) as PluginCtx;
  }

  function mountOne(el: HTMLElement, id: string, handler: PluginHandler): void {
    const m: Mounted = { el, id, dead: false };
    mounted.push(m);
    const done = (result: unknown) => {
      m.cleanUp = result;
      if (m.dead) cleanUp(m);
    };
    try {
      const result = handler.mount(el, context(el, id));
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
