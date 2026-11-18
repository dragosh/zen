import type { NavNode, Workspace } from "../host";

const p = (title: string, path: string): NavNode => ({ title, path, kind: "page", children: [] });
const g = (title: string, path: string, children: NavNode[]): NavNode => ({ title, path, kind: "group", children });

/** Two levels deep so ancestors and collapsed first-level defaults are testable. */
export function deepTree(): NavNode[] {
  return [
    p("Home", "README.md"),
    g("Guides", "guides", [p("Install", "guides/install.md"), g("Advanced", "guides/adv", [p("Tuning", "guides/adv/tuning.md")])]),
    { title: "API", path: "api.md", kind: "page", children: [p("Auth", "api/auth.md")] },
  ];
}
export function deepWorkspace(tree = deepTree()): Workspace {
  return { root: "/deep", name: "deep", home: "README.md", tree };
}
export const items = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLElement>("[role=treeitem]"));
export const item = (el: HTMLElement, path: string) =>
  items(el).find((i) => i.dataset.path === path) as HTMLElement;
export const visible = (el: HTMLElement) =>
  items(el).filter((i) => !i.parentElement?.closest("[role=treeitem][aria-expanded=false]")).map((i) => i.dataset.path);
