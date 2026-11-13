// The Api over the generated Wails bindings (app/services). Loaded by
// getApi() only inside the Wails webview.
import { Events } from "@wailsio/runtime";
import { Zen } from "../../bindings/github.com/dragosh/zen/internal/app/index.js";
import type { Api, Crumb, NavNode, Page, Startup, Workspace } from "./index";

type Raw = Record<string, any> | null | undefined;

function nodes(list: Raw[] | null | undefined): NavNode[] {
  return (list ?? []).map((n) => ({
    title: n?.title ?? "",
    path: n?.path ?? "",
    kind: n?.kind === "group" ? "group" : "page",
    children: nodes(n?.children),
  }));
}

function workspace(w: Raw): Workspace | null {
  if (!w) return null;
  return { root: w.root ?? "", name: w.name ?? "", home: w.home ?? "", tree: nodes(w.tree) };
}

function page(p: Raw): Page {
  return {
    path: p?.path ?? "",
    title: p?.title ?? "",
    html: p?.html ?? "",
    frontmatter: p?.frontmatter ?? {},
    breadcrumb: ((p?.breadcrumb ?? []) as Crumb[]).map((c) => ({ title: c.title, path: c.path })),
    assets: { css: p?.assets?.css ?? [], js: p?.assets?.js ?? [] },
    warnings: p?.warnings ?? [],
  };
}

/** Rejections from the runtime become Errors carrying the Go message. */
async function call<T>(p: PromiseLike<T>): Promise<T> {
  try {
    return await p;
  } catch (err) {
    if (err instanceof Error) throw err;
    const msg = typeof err === "string" ? err : (err as { message?: string })?.message ?? String(err);
    throw new Error(msg);
  }
}

/** Single-argument Emit data arrives as is; several arguments as an array. */
function payload(data: unknown): unknown {
  return Array.isArray(data) && data.length === 1 ? data[0] : data;
}

const api: Api = {
  async startup(): Promise<Startup> {
    const s = (await call(Zen.Startup())) as Raw;
    return { workspace: workspace(s?.workspace), page: s?.page ?? "", mode: s?.mode === "edit" ? "edit" : "read" };
  },
  async pickFolder() {
    return workspace((await call(Zen.PickFolder())) as Raw);
  },
  async page(path) {
    return page((await call(Zen.Page(path))) as Raw);
  },
  async source(path) {
    return (await call(Zen.Source(path))) ?? "";
  },
  async preview(path, content) {
    return page((await call(Zen.Preview(path, content))) as Raw);
  },
  async save(path, content) {
    await call(Zen.Save(path, content));
  },
  async openExternal(url) {
    return Boolean(await call(Zen.OpenExternal(url)));
  },
  on(event: "workspace:changed" | "page:changed" | "workspace:opened", fn: (p: any) => void): () => void {
    return Events.On(event, (e: { data: unknown }) => {
      const d = payload(e.data);
      if (event === "workspace:opened") {
        const s = d as { workspace?: Raw; page?: string; mode?: string } | null;
        const w = s?.workspace ? workspace(s.workspace) : null;
        if (w) fn({ workspace: w, page: s?.page ?? "", mode: s?.mode === "edit" ? "edit" : "read" });
      } else if (event === "workspace:changed") {
        const w = workspace(d as Raw);
        if (w) fn(w);
      } else {
        fn({ path: (d as { path?: string } | null)?.path ?? "" });
      }
    });
  },
};

/**
 * Window close handshake (app/services behaviour 11). Go emits
 * app:before-close; a cancelable "zen:before-close" DOM event goes out (ui/editor
 * prevents it while dirty and calls detail.close() after Save/Discard). Nobody
 * objecting closes at once; otherwise Go is told the user is being asked.
 */
Events.On("app:before-close", () => {
  let answered = false;
  const close = () => {
    if (answered) return;
    answered = true;
    void Events.Emit("app:close", null);
  };
  const ev = new CustomEvent("zen:before-close", { cancelable: true, detail: { close } });
  if (window.dispatchEvent(ev)) close();
  else void Events.Emit("app:close-pending", null);
});

export default api;
