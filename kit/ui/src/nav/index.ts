// The nav feature: sidebar tree in the `nav` slot, breadcrumb in `breadcrumb`.
import type { NavNode, Workspace } from "../host";
import { el, logo } from "../shell/dom";
import type { AppState, Feature } from "../shell/types";
import { renderBreadcrumb } from "./breadcrumb";
import { renderTree } from "./tree";

export { renderBreadcrumb } from "./breadcrumb";
export { renderTree } from "./tree";

const storageKey = (root: string) => `zen.nav.expanded:${root}`;

function loadExpanded(w: Workspace): Set<string> {
  try {
    const raw = localStorage.getItem(storageKey(w.root));
    if (raw) {
      const list: unknown = JSON.parse(raw);
      if (Array.isArray(list)) return new Set(list.filter((x): x is string => typeof x === "string"));
    }
  } catch { /* fall through to defaults */ }
  return new Set(w.tree.filter((n: NavNode) => n.children.length).map((n) => n.path));
}

function saveExpanded(root: string, set: Set<string>): void {
  try { localStorage.setItem(storageKey(root), JSON.stringify(Array.from(set))); } catch { /* storage unavailable */ }
}

export const navFeature: Feature = {
  id: "nav",
  mount(slots, ctx) {
    const head = el("div", "zen-nav-ws");
    const name = el("span", "zen-nav-ws-name");
    head.append(logo("zen-nav-logo", 2), name);
    const treeHost = el("div", "zen-nav-tree");
    const crumbHost = el("nav", "zen-nav-crumbs");
    crumbHost.setAttribute("aria-label", "Breadcrumb");
    slots.nav.append(head, treeHost);
    slots.breadcrumb.append(crumbHost);

    let root: string | null = null;
    let expanded = new Set<string>();
    let last: Partial<AppState> = {};

    const drawTree = () => {
      const { workspace, path } = ctx.store.get();
      if (!workspace) { treeHost.replaceChildren(); return; }
      renderTree(treeHost, workspace.tree, {
        current: path,
        expanded,
        badges: ctx.store.get().badges,
        onOpen: (p) => ctx.navigate(p),
        onToggle: (p, open) => {
          if (open) expanded.add(p); else expanded.delete(p);
          saveExpanded(workspace.root, expanded);
          drawTree();
        },
      });
    };
    const drawCrumbs = () => {
      const { page, path, mode, dirty } = ctx.store.get();
      const crumbs = page && page.path === path ? page.breadcrumb : [];
      renderBreadcrumb(crumbHost, crumbs, { onOpen: (p) => ctx.navigate(p), dirty: mode === "edit" && !!dirty });
    };

    const render = (s: AppState) => {
      if (s.workspace !== last.workspace || s.path !== last.path || s.badges !== last.badges) {
        const r = s.workspace?.root ?? null;
        if (r !== root) { root = r; expanded = s.workspace ? loadExpanded(s.workspace) : new Set(); }
        name.textContent = s.workspace?.name ?? "";
        drawTree();
      }
      if (s.page !== last.page || s.path !== last.path || s.mode !== last.mode || s.dirty !== last.dirty) drawCrumbs();
      last = { ...s };
    };
    render(ctx.store.get());
    const off = ctx.store.subscribe(render);
    const ro = typeof ResizeObserver === "function" ? new ResizeObserver(() => drawCrumbs()) : null;
    ro?.observe(slots.breadcrumb);

    return () => {
      off();
      ro?.disconnect();
      head.remove();
      treeHost.remove();
      crumbHost.remove();
    };
  },
};
