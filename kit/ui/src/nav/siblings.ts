// Pure helpers for the sidebar's menu and reordering: who the siblings of a page are, the order
// after a move or a drop, and the text Copy Link puts on the clipboard.
import type { NavNode } from "../host";

export type SiblingGroup = {
  /** What the engine's ReorderNav calls the parent: the path of the node that holds the list, "" at the top and in "Other pages". */
  parent: string;
  /** The page siblings (folders are not numbered), in the order shown. */
  pages: string[];
};

/** The page siblings of the page at `path`; null when `path` is not a page of the tree. */
export function siblingGroup(tree: NavNode[], path: string, parent = ""): SiblingGroup | null {
  if (tree.some((n) => n.kind === "page" && n.path === path)) {
    return { parent, pages: tree.filter((n) => n.kind === "page").map((n) => n.path) };
  }
  for (const n of tree) {
    const below = siblingGroup(n.children, path, n.path);
    if (below) return below;
  }
  return null;
}

/** The node at `path`, anywhere in the tree. */
export function findNode(tree: NavNode[], path: string): NavNode | null {
  for (const n of tree) {
    if (n.path === path) return n;
    const below = findNode(n.children, path);
    if (below) return below;
  }
  return null;
}

/** `pages` with `path` one place earlier (delta -1) or later (1); null at the ends or when it is not there. */
export function movedOrder(pages: string[], path: string, delta: -1 | 1): string[] | null {
  const i = pages.indexOf(path);
  const j = i + delta;
  if (i < 0 || j < 0 || j >= pages.length) return null;
  const out = pages.slice();
  [out[i], out[j]] = [out[j], out[i]];
  return out;
}

/**
 * `pages` with `path` put before the row that is at `index` in the list as it is now
 * (`pages.length` is the end); null when that is where it already is.
 */
export function dropOrder(pages: string[], path: string, index: number): string[] | null {
  const from = pages.indexOf(path);
  if (from < 0 || index === from || index === from + 1) return null;
  const out = pages.filter((p) => p !== path);
  out.splice(index > from ? index - 1 : index, 0, path);
  return out;
}

/** The folder New Page Here uses: a page's own folder, or the folder group itself; "" or ends in "/". */
export function folderOf(node: { path: string; kind: "page" | "group" }): string {
  if (node.kind === "group") return node.path ? `${node.path}/` : "";
  const i = node.path.lastIndexOf("/");
  return i < 0 ? "" : node.path.slice(0, i + 1);
}

/** `to` (a workspace path) as a link written in the page at `from` ("" or a folder-less path: from the folder root). */
export function relativePath(from: string, to: string): string {
  const base = from.split("/").slice(0, -1);
  const target = to.split("/");
  let i = 0;
  while (i < base.length && i < target.length - 1 && base[i] === target[i]) i++;
  return [...Array(base.length - i).fill(".."), ...target.slice(i)].join("/");
}

/** `[Title](path)` for a markdown page, with the characters that would break it escaped. */
export function copyLink(title: string, target: string, from: string): string {
  const text = title.replace(/[\\[\]]/g, (c) => `\\${c}`);
  const href = relativePath(from, target).replace(/[ ()<>%#?\\]/g, (c) => `%${c.charCodeAt(0).toString(16).toUpperCase().padStart(2, "0")}`);
  return `[${text}](${href})`;
}
