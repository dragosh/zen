// The runtime of a built site (`zenmd build`). It reads <script type="application/json" id="zen-page">
// ({ path, siteId, root, assets: { css, js }, portable }) and mounts the plugin and layout scripts of the
// page with the same contract as the app (docs/PLUGINS.md, docs/LAYOUTS.md): it reuses
// plugins.ts with a static `api`. No Wails bindings are imported; the app's backend does not
// exist here, so invoke and stream reject with "runs in the zen app only".
import { icon } from "../icons";
import { installImageZoom } from "../zoom/zoom";
import { installRuntime } from "../plugins";
import type { RuntimeApi } from "../types";
import "./static.css";

const NOT_HERE = "runs in the zen app only";
const KEY_RE = /^[A-Za-z0-9._-]{1,64}$/;

type Kind = "plugin" | "layout";

/** The part of the service API a built site can answer, as the runtime sees it. */
export type StaticApi = RuntimeApi;

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

/**
 * The fixed top bar (ui/site-topbar). The shell writes its buttons hidden, so a page without scripts shows only the
 * breadcrumb; here they are shown and wired: the sidebar button (⌘\, remembered per site), Back and Forward (⌘[ and
 * ⌘]) over the browser's history. Returns what undoes it.
 */
function installTopBar(doc: Document, win: Window, siteId: string): () => void {
  const q = <T extends HTMLElement>(sel: string) => doc.querySelector<T>(sel);
  const frame = q(".zen-site-frame");
  const side = q(".zen-site-side");
  const toggle = q<HTMLButtonElement>(".zen-site-sidebar-toggle");
  const back = q<HTMLButtonElement>(".zen-site-back");
  const forward = q<HTMLButtonElement>(".zen-site-forward");
  if (!frame && !back && !forward) return () => {};
  const undo: Array<() => void> = [];
  let flipSidebar: (() => void) | null = null;
  for (const b of [toggle, back, forward]) if (b) b.hidden = false;
  frame?.setAttribute("data-js", "");
  const store = (): Storage | null => {
    try {
      return win.localStorage ?? null;
    } catch {
      return null;
    }
  };
  const session = (): Storage | null => {
    try {
      return win.sessionStorage ?? null;
    } catch {
      return null;
    }
  };

  // The sidebar. Wide windows use the choice kept for this site; narrow ones start with it closed and open it over the page.
  if (frame && side && toggle) {
    const key = `zen.site.${siteId}.sidebar`;
    const narrow = win.matchMedia?.("(max-width: 719px)");
    const menu = side.querySelector<HTMLDetailsElement>(".zen-site-menu");
    if (menu) menu.open = true; // the button is the disclosure now, not the Contents summary
    toggle.prepend(icon(doc, "sidebar"));
    let open = true;
    const show = (on: boolean) => {
      open = on;
      frame.dataset.sidebar = on ? "open" : "closed";
      side.hidden = !on;
      toggle.setAttribute("aria-expanded", String(on));
      const text = on ? "Hide sidebar" : "Show sidebar";
      toggle.setAttribute("aria-label", text);
      toggle.title = `${text} (⌘\\)`;
    };
    const initial = () => {
      if (narrow?.matches) return false;
      try {
        return store()?.getItem(key) !== "closed";
      } catch {
        return true;
      }
    };
    show(initial());
    const flip = () => {
      show(!open);
      if (narrow?.matches) return; // narrow windows open it for a moment; the wide choice is not changed
      try {
        store()?.setItem(key, open ? "open" : "closed");
      } catch { /* not remembered; it still applies to this page */ }
    };
    toggle.addEventListener("click", flip);
    const onWidth = () => show(initial());
    narrow?.addEventListener?.("change", onWidth);
    const onEsc = (e: KeyboardEvent) => {
      if (e.key === "Escape" && narrow?.matches && open) show(false);
    };
    doc.addEventListener("keydown", onEsc);
    undo.push(() => {
      toggle.removeEventListener("click", flip);
      narrow?.removeEventListener?.("change", onWidth);
      doc.removeEventListener("keydown", onEsc);
    });
    flipSidebar = flip;
  }

  // Back and Forward. A page does not know its place in the history, so each entry is numbered in history.state
  // (zenIdx) as it is first shown, and the last number and the furthest one are kept for the tab (sessionStorage).
  const place = { idx: 0, max: 0 };
  const sync = () => {
    let st = null as { zenIdx?: unknown } | null;
    try {
      st = win.history.state as { zenIdx?: unknown } | null;
    } catch { /* none */ }
    let kept: { idx?: number; max?: number } = {};
    try {
      kept = JSON.parse(session()?.getItem(`zen.site.${siteId}.visit`) ?? "{}") ?? {};
    } catch { /* a visit that does not parse starts over */ }
    if (st && typeof st === "object" && typeof st.zenIdx === "number") {
      place.idx = st.zenIdx; // an entry seen before: back, forward or a reload
      place.max = Math.max(kept.max ?? 0, place.idx);
    } else {
      place.idx = typeof kept.idx === "number" ? kept.idx + 1 : 0; // a new entry: what was ahead is gone
      place.max = place.idx;
      try {
        win.history.replaceState({ ...(st && typeof st === "object" ? st : {}), zenIdx: place.idx }, "");
      } catch { /* the entry stays unnumbered */ }
    }
    try {
      session()?.setItem(`zen.site.${siteId}.visit`, JSON.stringify(place));
    } catch { /* not kept */ }
    if (back) back.disabled = place.idx <= 0;
    if (forward) forward.disabled = place.idx >= place.max;
  };
  sync();
  const goBack = () => { if (back && !back.disabled) win.history.back(); };
  const goForward = () => { if (forward && !forward.disabled) win.history.forward(); };
  back?.addEventListener("click", goBack);
  forward?.addEventListener("click", goForward);
  win.addEventListener("popstate", sync);
  win.addEventListener("hashchange", sync);

  const onKey = (e: KeyboardEvent) => {
    if (!(e.metaKey || e.ctrlKey) || e.altKey || e.shiftKey) return;
    const act = e.key === "\\" ? flipSidebar : e.key === "[" ? goBack : e.key === "]" ? goForward : undefined;
    if (!act) return;
    e.preventDefault();
    act();
  };
  doc.addEventListener("keydown", onKey);
  return () => {
    undo.forEach((u) => u());
    back?.removeEventListener("click", goBack);
    forward?.removeEventListener("click", goForward);
    win.removeEventListener("popstate", sync);
    win.removeEventListener("hashchange", sync);
    doc.removeEventListener("keydown", onKey);
  };
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
    api: staticApi(page.siteId),
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
  const root = doc.querySelector<HTMLElement>(".zen-page") ?? doc.body;
  const zoom = installImageZoom(root);
  const stopBar = installTopBar(doc, win, page.siteId);
  stopPrevious = () => {
    stopBar();
    doc.removeEventListener("click", onClick);
    win.removeEventListener("hashchange", onHash);
    zoom.destroy();
    rt.destroy();
  };

  void rt.mountAll(root, { js: page.assets.js, css: page.assets.css });
}

if (typeof document !== "undefined") {
  const run = () => start(document);
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", run);
  else run();
}
