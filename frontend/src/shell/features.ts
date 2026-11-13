// The only place the shell learns about features. The lead replaces each
// placeholder with the real feature (and imports its CSS) as ui/navigation,
// ui/page and ui/editor merge.
import type { NavNode } from "../api";
import { button, el, logo } from "./dom";
import type { Feature } from "./types";

const navPlaceholder: Feature = {
  id: "nav",
  mount(slots, ctx) {
    const head = el("div", "zen-ph-ws");
    const name = el("span");
    head.append(logo("zen-ph-logo", 2), name);
    const list = el("div", "zen-ph-tree");
    const crumb = el("span");
    slots.nav.append(head, list);
    slots.breadcrumb.append(crumb);

    const rows = (nodes: NavNode[], depth: number): HTMLElement[] =>
      nodes.flatMap((n) => {
        const row = n.kind === "page"
          ? button(n.title, () => ctx.navigate(n.path), { className: "ghost zen-ph-row" })
          : el("div", "zen-ph-row zen-ph-group", n.title);
        row.style.paddingLeft = `${8 + depth * 18}px`;
        if (n.kind === "page" && n.path === ctx.store.get().path) row.setAttribute("aria-current", "page");
        return [row, ...rows(n.children, depth + 1)];
      });
    const render = () => {
      const { workspace, path } = ctx.store.get();
      name.textContent = workspace?.name ?? "";
      list.replaceChildren(...rows(workspace?.tree ?? [], 0));
      crumb.textContent = path;
    };
    render();
    const off = ctx.store.subscribe(render);
    return () => {
      off();
      head.remove();
      list.remove();
      crumb.remove();
    };
  },
};

/** A text-only stand-in for the page view and the editor. */
function mainPlaceholder(id: "page" | "editor", label: string, action: string, next: "read" | "edit"): Feature {
  return {
    id,
    mount(slots, ctx) {
      const text = el("p", "zen-empty");
      const switchMode = button(action, () => ctx.store.set({ mode: next }));
      slots.main.append(text);
      slots.actions.append(switchMode);
      const render = () => {
        text.textContent = `${label}: ${ctx.store.get().path}`;
      };
      render();
      const off = ctx.store.subscribe(render);
      return () => {
        off();
        text.remove();
        switchMode.remove();
      };
    },
  };
}

export const features: Feature[] = [
  navPlaceholder,
  mainPlaceholder("page", "Page", "Edit", "edit"),
  mainPlaceholder("editor", "Editor", "Done", "read"),
];
