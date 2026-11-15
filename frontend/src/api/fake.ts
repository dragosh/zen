// In-memory Api for development in a browser and for tests: a three-page
// sample workspace, emit() to simulate Go events, and a record of every call.
import type { Api, Crumb, NavNode, Page, Startup, Workspace } from "./index";

type Method = "startup" | "pickFolder" | "page" | "source" | "preview" | "save" | "openExternal" | "openEditor";
type Events = { "workspace:changed": Workspace; "page:changed": { path: string }; "workspace:opened": Startup };

export type FakeOptions = {
  /** What startup() resolves to. Default: no workspace (welcome screen). */
  startup?: Startup;
  /** What pickFolder() resolves to; null means cancelled. Default: the sample workspace. */
  pick?: Workspace | null;
  /** Markdown sources by path. Default: the sample pages. */
  sources?: Record<string, string>;
};

export type FakeApi = Api & {
  /** Every call in order. */
  calls: { method: Method; args: unknown[] }[];
  /** Arguments of every call to one method. */
  callsTo(method: Method): unknown[][];
  /** Delivers an event to the listeners registered with on(). */
  emit<E extends keyof Events>(event: E, payload: Events[E]): void;
  /** Makes the next call to `method` reject with `error`. */
  failNext(method: Method, error: Error): void;
  /** Result of the next pickFolder(); assignable between calls. */
  pick: Workspace | null;
  /** What openExternal() resolves to (the user's answer). Default true. */
  confirmExternal: boolean;
  /**
   * Events the real client sends to the other windows: a successful save()
   * broadcasts "editor:saved" (the main window receives it as page:changed).
   */
  broadcasts: { event: "editor:saved"; payload: { path: string } }[];
};

const SOURCES: Record<string, string> = {
  "README.md": "# Sample docs\n\nA small workspace served by the fake API.\n\nStart with [Install](guides/install.md), then read [Usage](guides/usage.md).\n",
  "guides/install.md": "# Install\n\nzen is a single application. Open a folder and it turns the markdown inside into a small site.\n\nSee [Usage](usage.md) for the next step.\n",
  "guides/usage.md": "# Usage\n\nPick a page in the sidebar to read it. Press Edit to change it.\n\nBack to the [README](../README.md) or the [zen site](https://example.com).\n",
};

export function sampleWorkspace(): Workspace {
  return {
    root: "/sample-docs",
    name: "sample-docs",
    home: "README.md",
    tree: [
      { title: "Sample docs", path: "README.md", kind: "page", children: [] },
      {
        title: "Guides", path: "guides", kind: "group",
        children: [
          { title: "Install", path: "guides/install.md", kind: "page", children: [] },
          { title: "Usage", path: "guides/usage.md", kind: "page", children: [] },
        ],
      },
    ],
  };
}

const escape = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

function resolve(from: string, href: string): string {
  const parts = from.split("/").slice(0, -1);
  for (const seg of href.split("/")) {
    if (seg === "..") parts.pop();
    else if (seg !== "." && seg !== "") parts.push(seg);
  }
  return parts.join("/");
}

/** Just enough markdown for the samples: `# ` headings, paragraphs and links. */
function renderFake(path: string, source: string, known: (p: string) => boolean): { title: string; html: string } {
  let title = "";
  const blocks = source.split(/\n{2,}/).map((b) => b.trim()).filter(Boolean);
  const html = blocks.map((block) => {
    const h = block.match(/^(#{1,6})\s+(.*)$/);
    if (h) {
      if (!title && h[1].length === 1) title = h[2];
      return `<h${h[1].length}>${escape(h[2])}</h${h[1].length}>`;
    }
    const text = escape(block).replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_m, label: string, href: string) => {
      if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return `<a data-zen-link="external" href="${href}">${label}</a>`;
      const [file, anchor = ""] = href.split("#");
      const target = resolve(path, file);
      if (!known(target)) return `<a data-zen-link="broken">${label}</a>`;
      return `<a data-zen-link="internal" data-path="${target}" data-anchor="${anchor}" href="#">${label}</a>`;
    });
    return `<p>${text}</p>`;
  }).join("\n");
  return { title: title || path.split("/").pop()!.replace(/\.md$/, ""), html };
}

function crumbs(tree: NavNode[], path: string, trail: Crumb[] = []): Crumb[] | null {
  for (const n of tree) {
    const here = [...trail, { title: n.title, path: n.path }];
    if (n.path === path) return here;
    const below = crumbs(n.children, path, here);
    if (below) return below;
  }
  return null;
}

export function createFakeApi(options: FakeOptions = {}): FakeApi {
  const sources: Record<string, string> = { ...(options.sources ?? SOURCES) };
  const startup: Startup = options.startup ?? { workspace: null, page: "", mode: "read" };
  const listeners: { [E in keyof Events]: Set<(p: Events[E]) => void> } = {
    "workspace:changed": new Set(),
    "page:changed": new Set(),
    "workspace:opened": new Set(),
  };
  const failures = new Map<Method, Error>();
  let tree: NavNode[] = (startup.workspace ?? sampleWorkspace()).tree;

  const render = (path: string, content: string): Page => {
    const { title, html } = renderFake(path, content, (p) => p in sources);
    return {
      path, title, html, frontmatter: {},
      breadcrumb: crumbs(tree, path) ?? [{ title, path }],
      assets: { css: [], js: [] }, warnings: [],
    };
  };
  const need = (path: string): string => {
    if (!(path in sources)) throw new Error(`page not found: ${path}`);
    return sources[path];
  };
  /** Records the call, then runs it asynchronously so rejections are promises. */
  const call = <T>(method: Method, args: unknown[], run: () => T): Promise<T> => {
    api.calls.push({ method, args });
    const failure = failures.get(method);
    if (failure) {
      failures.delete(method);
      return Promise.reject(failure);
    }
    return new Promise((res) => res(run()));
  };

  const api: FakeApi = {
    calls: [],
    pick: options.pick === undefined ? sampleWorkspace() : options.pick,
    confirmExternal: true,
    broadcasts: [],
    callsTo: (method) => api.calls.filter((c) => c.method === method).map((c) => c.args),
    emit(event, payload) {
      for (const fn of Array.from(listeners[event])) fn(payload);
    },
    failNext(method, error) {
      failures.set(method, error);
    },

    startup: () => call("startup", [], () => startup),
    pickFolder: () => call("pickFolder", [], () => {
      if (api.pick) tree = api.pick.tree;
      return api.pick;
    }),
    page: (path) => call("page", [path], () => render(path, need(path))),
    source: (path) => call("source", [path], () => need(path)),
    preview: (path, content) => call("preview", [path, content], () => render(path, content)),
    save: (path, content) => call("save", [path, content], () => {
      need(path);
      sources[path] = content;
      api.broadcasts.push({ event: "editor:saved", payload: { path } });
    }),
    openExternal: (url) => call("openExternal", [url], () => api.confirmExternal),
    openEditor: (path) => call("openEditor", [path], () => undefined),
    on(event: keyof Events, fn: (p: never) => void) {
      const set = listeners[event] as Set<(p: never) => void>;
      set.add(fn);
      return () => void set.delete(fn);
    },
  };
  return api;
}
