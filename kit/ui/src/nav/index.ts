// The nav feature: sidebar tree in the `nav` slot, breadcrumb in `breadcrumb`.
import { hasFeature, type NavNode, type Workspace } from "../host";
import { copyText } from "../shell/clipboard";
import { el } from "../shell/dom";
import type { AppState, Feature } from "../shell/types";
import { renderBreadcrumb } from "./breadcrumb";
import { menuEntries, type EntryEnv } from "./entries";
import { confirmDialog, openMenu, type MenuHandle } from "./menu";
import { findNode, movedOrder, siblingGroup } from "./siblings";
import { focusPath, renderTree } from "./tree";

export { renderBreadcrumb } from "./breadcrumb";
export { renderTree } from "./tree";

const storageKey = (root: string) => `zen.nav.expanded:${root}`;
const STATUS_MS = 6000;

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
    const doc = slots.nav.ownerDocument;
    const treeHost = el("div", "zen-nav-tree");
    // What a menu action did, or why it failed (an alert), under the tree.
    const status = el("div", "zen-nav-status");
    status.setAttribute("role", "status");
    status.setAttribute("aria-live", "polite");
    status.hidden = true;
    const crumbHost = el("nav", "zen-nav-crumbs");
    crumbHost.setAttribute("aria-label", "Breadcrumb");
    slots.nav.append(treeHost, status);
    slots.breadcrumb.append(crumbHost);

    let root: string | null = null;
    let expanded = new Set<string>();
    let last: Partial<AppState> = {};
    let menu: MenuHandle | null = null;
    let statusTimer: ReturnType<typeof setTimeout> | undefined;
    let focusTimer: ReturnType<typeof setTimeout> | undefined;
    /** The page that was just moved: it gets the focus when the new tree is drawn. */
    let pendingFocus = "";

    const say = (text: string, failed = false) => {
      clearTimeout(statusTimer);
      status.textContent = text;
      status.hidden = false;
      status.classList.toggle("zen-nav-status-error", failed);
      status.setAttribute("role", failed ? "alert" : "status");
      statusTimer = setTimeout(() => { status.hidden = true; status.textContent = ""; }, STATUS_MS);
    };
    const fail = (err: unknown) => say(err instanceof Error ? err.message : String(err), true);

    const api = ctx.api;
    // Without the editor feature (a read-only app) the sidebar only reads: no editing items, no reordering.
    const editable = hasFeature(api, "editor");
    const titleOf = (path: string) => findNode(ctx.store.get().workspace?.tree ?? [], path)?.title ?? path;

    const reorder = (parent: string, order: string[], moved: string) => {
      if (!editable || !api.reorderNav) return;
      pendingFocus = moved;
      clearTimeout(focusTimer);
      focusTimer = setTimeout(() => { pendingFocus = ""; }, 3000);
      api.reorderNav(parent, order).then(
        () => say(`Moved ${titleOf(moved)} to position ${order.indexOf(moved) + 1} of ${order.length}`),
        (err) => { pendingFocus = ""; fail(err); },
      );
    };
    const move = (path: string, delta: -1 | 1) => {
      const group = siblingGroup(ctx.store.get().workspace?.tree ?? [], path);
      const order = group && movedOrder(group.pages, path, delta);
      if (group && order) reorder(group.parent, order, path);
    };

    const showMenu = (path: string, at: { x: number; y: number }, row: HTMLElement) => {
      const { workspace, path: current } = ctx.store.get();
      const node = workspace && findNode(workspace.tree, path);
      if (!workspace || !node) return;
      const env: EntryEnv = {
        tree: workspace.tree,
        current,
        open: (p) => ctx.navigate(p),
        editor: editable && api.openEditor ? (p) => { api.openEditor(p).catch(fail); } : undefined,
        newPage: editable && ctx.newPage ? (folder) => ctx.newPage!(folder) : undefined,
        move: editable && api.reorderNav ? move : undefined,
        copy: (text) => { void copyText(doc, text).then((ok) => (ok ? say("Copied") : say("Could not copy", true))); },
        reveal: api.revealInFinder ? (p) => { api.revealInFinder!(p).catch(fail); } : undefined,
        hide: editable && api.setNavHidden ? (n) => hide(n) : undefined,
      };
      const entries = menuEntries(node, env);
      if (!entries.length) return;
      menu = openMenu({ doc, entries, x: at.x, y: at.y, returnFocus: row, label: node.title, onClose: () => { menu = null; } });
    };

    const hide = (node: NavNode) => {
      const below = node.children.length ? " The pages under it leave the sidebar with it." : "";
      confirmDialog({
        doc,
        title: `Hide “${node.title}” from the sidebar?`,
        message: `The page stays in the folder and links to it still work.${below} To show it again, remove hidden: true from nav: in its frontmatter.`,
        confirm: "Hide",
        onConfirm: () => { api.setNavHidden!(node.path, true).catch(fail); },
      });
    };

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
        onContextMenu: showMenu,
        onMove: editable && api.reorderNav ? move : undefined,
        onReorder: editable && api.reorderNav ? reorder : undefined,
      });
      if (pendingFocus && focusPath(treeHost, pendingFocus)) pendingFocus = "";
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
      menu?.close();
      clearTimeout(statusTimer);
      clearTimeout(focusTimer);
      treeHost.remove();
      status.remove();
      crumbHost.remove();
    };
  },
};
