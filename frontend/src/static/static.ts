// The runtime of a built site (`zenmd build`). It reads <script type="application/json" id="zen-page">
// ({ path, siteId, root, assets: { css, js }, portable }) and mounts the plugin and layout scripts of the
// page with the same contract as the app (docs/PLUGINS.md, docs/LAYOUTS.md): it reuses
// plugins/runtime.ts with a static `api`. No Wails bindings are imported; the app's backend does not
// exist here, so invoke and stream reject with "runs in the zen app only".
import type { Api } from "../api";
import { installRuntime } from "../plugins/runtime";
import "./static.css";

const NOT_HERE = "runs in the zen app only";
const KEY_RE = /^[A-Za-z0-9._-]{1,64}$/;

type Kind = "plugin" | "layout";

/** The part of the service API a built site can answer, as the runtime sees it. */
export type StaticApi = Pick<
  Api,
  | "pluginStorageGet" | "pluginStorageSet" | "pluginStorageKeys"
  | "layoutStorageGet" | "layoutStorageSet" | "layoutStorageKeys"
  | "pluginInvoke" | "pluginStream" | "pluginCancel" | "on"
>;

function validKey(key: string): void {
  if (!KEY_RE.test(key)) throw new Error("storage keys are 1 to 64 letters, digits, '.', '_' or '-'");
}

/**
 * Storage in the reader's localStorage, namespaced `zen.site.<siteId>.<plugin|layout>.<id>.<key>` so two
 * sites on one host do not meet. Values are JSON text, as in the app. When localStorage is unavailable
 * (private window, blocked, full) a value is kept for the session only.
 */
export function staticApi(siteId: string, store?: Storage): StaticApi {
  const session = new Map<string, string>();
  const prefixOf = (kind: Kind, id: string) => `zen.site.${siteId}.${kind}.${id}.`;
  const storage = (): Storage | null => {
    try {
      return store ?? window.localStorage ?? null;
    } catch {
      return null;
    }
  };
  const get = async (kind: Kind, id: string, key: string): Promise<string> => {
    validKey(key);
    const full = prefixOf(kind, id) + key;
    if (session.has(full)) return session.get(full)!;
    try {
      return storage()?.getItem(full) ?? "";
    } catch {
      return "";
    }
  };
  const set = async (kind: Kind, id: string, key: string, value: string): Promise<void> => {
    validKey(key);
    const full = prefixOf(kind, id) + key;
    try {
      const s = storage();
      if (!s) throw new Error("no storage");
      s.setItem(full, value);
      session.delete(full);
    } catch {
      session.set(full, value);
    }
  };
  const keys = async (kind: Kind, id: string, prefix: string): Promise<string[]> => {
    const base = prefixOf(kind, id);
    const found = new Set<string>();
    for (const k of Array.from(session.keys())) if (k.startsWith(base)) found.add(k.slice(base.length));
    try {
      const s = storage();
      for (let i = 0; s && i < s.length; i++) {
        const k = s.key(i);
        if (k && k.startsWith(base)) found.add(k.slice(base.length));
      }
    } catch { /* session keys only */ }
    return Array.from(found).filter((k) => k.startsWith(prefix)).sort();
  };
  const reject = () => Promise.reject(new Error(NOT_HERE));
  return {
    pluginStorageGet: (id, key) => get("plugin", id, key),
    pluginStorageSet: (id, key, value) => set("plugin", id, key, value),
    pluginStorageKeys: (id, prefix) => keys("plugin", id, prefix),
    layoutStorageGet: (id, key) => get("layout", id, key),
    layoutStorageSet: (id, key, value) => set("layout", id, key, value),
    layoutStorageKeys: (id, prefix) => keys("layout", id, prefix),
    pluginInvoke: reject,
    pluginStream: reject,
    pluginCancel: () => Promise.resolve(),
    on: () => () => {},
  };
}

/** Test seams: replace import() and the page navigation. */
export const staticHooks: { importModule?: (url: string) => Promise<unknown>; navigate?: (url: string) => void; protocol?: string } = {};

/** Shown in each block when a --hosted site is opened from disk, where browsers refuse to load its scripts. */
export const HOSTED_FROM_DISK = "this site was built with --hosted for a web server; open it through one, or build it again without --hosted to open it from disk";

type PageData = { path: string; siteId: string; root: string; assets: { css: string[]; js: string[] }; portable: boolean };

function readPage(doc: Document): PageData | null {
  const el = doc.getElementById("zen-page");
  if (!el) return null;
  try {
    const d = JSON.parse(el.textContent ?? "") as Partial<PageData>;
    if (typeof d !== "object" || d === null || typeof d.path !== "string") return null;
    return {
      path: d.path, siteId: typeof d.siteId === "string" ? d.siteId : "", root: typeof d.root === "string" ? d.root : "",
      assets: { css: d.assets?.css ?? [], js: d.assets?.js ?? [] }, portable: d.portable === true,
    };
  } catch {
    return null;
  }
}

/** Page path to the output file of the site: a/b.md is a/b.html (as zen build writes it). */
function outFile(path: string): string {
  return path.replace(/\.(md|markdown)$/i, "") + ".html";
}
const escapePath = (p: string) => p.split("/").map(encodeURIComponent).join("/");

const SCRIPT_URL = /^\/_(plugins|layouts)\/([^/?#]+)\/([^?#]+)/;

/** Portable scripts: `<script type="application/json" data-zen-module="plugin:<id>">{"file.js": "text"}</script>`. */
function readModules(doc: Document): Map<string, Record<string, string>> {
  const out = new Map<string, Record<string, string>>();
  for (const el of Array.from(doc.querySelectorAll<HTMLElement>("script[data-zen-module]"))) {
    try {
      const files = JSON.parse(el.textContent ?? "") as Record<string, string>;
      if (files && typeof files === "object") out.set(el.getAttribute("data-zen-module") ?? "", files);
    } catch { /* a module that does not parse is missing */ }
  }
  return out;
}

let stopPrevious: (() => void) | null = null;

export function start(doc: Document = document): void {
  stopPrevious?.(); // starting again replaces the runtime of the last start
  stopPrevious = null;
  const page = readPage(doc);
  if (!page) return;
  const win = doc.defaultView ?? window;
  const modules = page.portable ? readModules(doc) : new Map<string, Record<string, string>>();

  const load = (url: string): Promise<unknown> =>
    staticHooks.importModule ? staticHooks.importModule(url) : (import(/* @vite-ignore */ url) as Promise<unknown>);

  const importModule = async (url: string): Promise<unknown> => {
    const m = SCRIPT_URL.exec(url);
    if (!m) throw new Error("not a plugin or layout script");
    const kind = m[1] === "layouts" ? "layout" : "plugin";
    const id = decodeURIComponent(m[2]);
    const file = decodeURIComponent(m[3]);
    if (page.portable) {
      const text = modules.get(`${kind}:${id}`)?.[file];
      if (typeof text !== "string") throw new Error("its script is not in this page");
      // No import() of a file: from file:// that fails. The text becomes a Blob URL.
      const blobUrl = URL.createObjectURL(new Blob([text], { type: "text/javascript" }));
      try {
        return await load(blobUrl);
      } finally {
        URL.revokeObjectURL?.(blobUrl);
      }
    }
    if ((staticHooks.protocol ?? win.location.protocol) === "file:") throw new Error(HOSTED_FROM_DISK);
    const href = new URL(`${page.root}_zen/${kind}s/${escapePath(id)}/${escapePath(file)}`, doc.baseURI).href;
    return load(href);
  };

  const anchor = () => win.location.hash.replace(/^#/, "");
  const listeners = new Set<(a: string) => void>();
  const onHash = () => listeners.forEach((fn) => fn(anchor()));
  win.addEventListener("hashchange", onHash);

  const rt = installRuntime({
    api: staticApi(page.siteId) as unknown as Api,
    navigate(path, a) {
      const url = page.root + escapePath(outFile(path)) + (a ? "#" + encodeURIComponent(a) : "");
      (staticHooks.navigate ?? ((u: string) => win.location.assign(u)))(url);
    },
    page: () => ({ path: page.path, title: doc.title }),
    anchor,
    onAnchor(fn) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    badge: () => {},
    assetUrl: (kind, id, file) => new URL(`${page.root}_zen/${kind}s/${escapePath(id)}/${file}`, doc.baseURI).href,
    importModule,
  });

  // The print link of the handout layout.
  const onClick = (e: Event) => {
    const t = e.target as Element | null;
    const a = t?.closest?.('[data-zen-action="print"]');
    if (a) {
      e.preventDefault();
      win.print();
    }
  };
  doc.addEventListener("click", onClick);
  stopPrevious = () => {
    doc.removeEventListener("click", onClick);
    win.removeEventListener("hashchange", onHash);
    rt.destroy();
  };

  const root = doc.querySelector<HTMLElement>(".zen-page") ?? doc.body;
  void rt.mountAll(root, { js: page.assets.js, css: page.assets.css });
}

if (typeof document !== "undefined") {
  const run = () => start(document);
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", run);
  else run();
}
