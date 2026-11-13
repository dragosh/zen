// The sidebar tree: role="tree", roving tabindex, text via textContent only.
import type { NavNode } from "../api";
import { el } from "../shell/dom";

export type TreeOptions = {
  current: string;
  onOpen(path: string): void;
  expanded: Set<string>;
  onToggle(path: string, open: boolean): void;
};

type State = { tree: NavNode[]; opts: TreeOptions; active: string; current: string };
const states = new WeakMap<HTMLElement, State>();

/** Paths of the nodes above `path`, outermost first; null when not found. */
function ancestors(nodes: NavNode[], path: string): string[] | null {
  for (const n of nodes) {
    if (n.path === path) return [];
    const below = ancestors(n.children, path);
    if (below) return [n.path, ...below];
  }
  return null;
}

const rowOf = (item: HTMLElement) => item.querySelector<HTMLElement>(":scope > .zen-nav-row")!;
const itemOf = (node: Element | null) => node?.closest<HTMLElement>("[role=treeitem]") ?? null;

function visibleItems(host: HTMLElement): HTMLElement[] {
  return Array.from(host.querySelectorAll<HTMLElement>("[role=treeitem]"))
    .filter((i) => !i.parentElement?.closest("[role=treeitem][aria-expanded=false]"));
}

function setActive(host: HTMLElement, item: HTMLElement, focus: boolean): void {
  const st = states.get(host);
  if (st) st.active = item.dataset.path ?? "";
  host.querySelectorAll<HTMLElement>('.zen-nav-row[tabindex="0"]').forEach((r) => (r.tabIndex = -1));
  const row = rowOf(item);
  row.tabIndex = 0;
  if (focus) row.focus();
}

function onKey(host: HTMLElement, e: KeyboardEvent): void {
  const st = states.get(host);
  const item = itemOf(e.target as Element);
  if (!st || !item || e.metaKey || e.ctrlKey || e.altKey) return;
  const list = visibleItems(host);
  const i = list.indexOf(item);
  const path = item.dataset.path ?? "";
  const expanded = item.getAttribute("aria-expanded");
  const move = (to: HTMLElement | undefined) => { if (to) setActive(host, to, true); };
  switch (e.key) {
    case "ArrowDown": move(list[i + 1]); break;
    case "ArrowUp": move(list[i - 1]); break;
    case "Home": move(list[0]); break;
    case "End": move(list[list.length - 1]); break;
    case "ArrowRight":
      if (expanded === "false") st.opts.onToggle(path, true);
      else if (expanded === "true") move(item.querySelector<HTMLElement>("[role=treeitem]") ?? undefined);
      break;
    case "ArrowLeft":
      if (expanded === "true") st.opts.onToggle(path, false);
      else move(itemOf(item.parentElement) ?? undefined);
      break;
    case "Enter":
    case " ":
      activate(st, item);
      break;
    default:
      return;
  }
  e.preventDefault();
}

function activate(st: State, item: HTMLElement): void {
  const path = item.dataset.path ?? "";
  if (item.dataset.kind === "page") st.opts.onOpen(path);
  else st.opts.onToggle(path, item.getAttribute("aria-expanded") !== "true");
}

function build(st: State, nodes: NavNode[], level: number, role: "tree" | "group"): HTMLUListElement {
  const ul = el("ul", role === "tree" ? "zen-nav-list" : "zen-nav-group");
  ul.setAttribute("role", role);
  for (const n of nodes) {
    const li = el("li", "zen-nav-item");
    li.setAttribute("role", "treeitem");
    li.setAttribute("aria-level", String(level));
    li.dataset.path = n.path;
    li.dataset.kind = n.kind;
    const row = el("div", "zen-nav-row");
    row.tabIndex = -1;
    row.title = n.title;
    row.style.paddingLeft = `calc(var(--space-2) + ${(level - 1) * 14}px)`;
    if (n.children.length) {
      const open = st.opts.expanded.has(n.path);
      li.setAttribute("aria-expanded", String(open));
      const twisty = el("span", "zen-nav-twisty", "▸");
      twisty.setAttribute("aria-hidden", "true");
      twisty.addEventListener("click", (e) => {
        e.stopPropagation();
        setActive(li.closest<HTMLElement>(".zen-nav-host")!, li, false);
        st.opts.onToggle(n.path, li.getAttribute("aria-expanded") !== "true");
      });
      row.append(twisty);
    } else {
      row.append(el("span", "zen-nav-spacer"));
    }
    row.append(el("span", "zen-nav-label", n.title));
    if (n.kind === "page" && n.path === st.opts.current) li.setAttribute("aria-current", "page");
    row.addEventListener("click", () => {
      const host = li.closest<HTMLElement>(".zen-nav-host");
      if (host) setActive(host, li, false);
      activate(st, li);
    });
    li.append(row);
    if (n.children.length) li.append(build(st, n.children, level + 1, "group"));
    ul.append(li);
  }
  return ul;
}

export function renderTree(host: HTMLElement, tree: NavNode[], opts: TreeOptions): void {
  let st = states.get(host);
  const hadFocus = host.contains(document.activeElement);
  if (!st) {
    st = { tree, opts, active: "", current: "" };
    states.set(host, st);
    host.addEventListener("keydown", (e) => onKey(host, e));
    host.addEventListener("focusin", (e) => {
      const item = itemOf(e.target as Element);
      if (item && rowOf(item) === e.target) setActive(host, item, false);
    });
  }
  st.tree = tree;
  st.opts = opts;
  host.classList.add("zen-nav-host");
  for (const a of ancestors(tree, opts.current) ?? []) opts.expanded.add(a);

  if (!tree.length) {
    host.replaceChildren(el("p", "zen-nav-empty", "No markdown files in this folder"));
    return;
  }
  host.replaceChildren(build(st, tree, 1, "tree"));

  const visible = visibleItems(host);
  const find = (p: string) => (p ? visible.find((i) => i.dataset.path === p) : undefined);
  const currentChanged = st.current !== opts.current;
  const target = (currentChanged ? find(opts.current) : undefined) ?? find(st.active) ?? find(opts.current) ?? visible[0];
  if (target) setActive(host, target, hadFocus);
  if (currentChanged) {
    st.current = opts.current;
    const cur = find(opts.current);
    if (cur) rowOf(cur).scrollIntoView?.({ block: "nearest" });
  }
}
