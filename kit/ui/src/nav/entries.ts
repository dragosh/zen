// The items of the sidebar's context menu for one node. What the host cannot do is not in `env`,
// and the item is not in the menu.
import type { NavNode } from "../host";
import type { MenuEntry } from "./menu";
import { copyLink, folderOf, siblingGroup } from "./siblings";

export type EntryEnv = {
  tree: NavNode[];
  /** The page open now ("" for none): Copy Link is relative to it. */
  current: string;
  open(path: string): void;
  editor?(path: string): void;
  newPage?(folder: string): void;
  /** Move the page one place among its siblings. */
  move?(path: string, delta: -1 | 1): void;
  copy?(text: string): void;
  reveal?(path: string): void;
  /** Ask, then hide the page from the sidebar. */
  hide?(node: NavNode): void;
};

export function menuEntries(node: NavNode, env: EntryEnv): MenuEntry[] {
  if (!node.path) return []; // the "Other pages" group
  const at = node.path;
  const groups: MenuEntry[][] = [];
  const add = (...items: (MenuEntry | false | undefined)[]) => {
    const g = items.filter((x): x is MenuEntry => !!x);
    if (g.length) groups.push(g);
  };
  if (node.kind === "page") {
    const sib = siblingGroup(env.tree, at);
    const i = sib ? sib.pages.indexOf(at) : -1;
    add(
      { label: "Open", run: () => env.open(at) },
      !!env.editor && { label: "Open in Editor", run: () => env.editor!(at) },
    );
    add(
      !!env.newPage && { label: "New Page Here…", run: () => env.newPage!(folderOf(node)) },
      !!env.move && { label: "Move Up", run: () => env.move!(at, -1), disabled: !sib || i <= 0 },
      !!env.move && { label: "Move Down", run: () => env.move!(at, 1), disabled: !sib || i < 0 || i >= sib.pages.length - 1 },
    );
    add(
      !!env.copy && { label: "Copy Path", run: () => env.copy!(at) },
      !!env.copy && { label: "Copy Link", run: () => env.copy!(copyLink(node.title, at, env.current)) },
      !!env.reveal && { label: "Reveal in Finder", run: () => env.reveal!(at) },
    );
    add(!!env.hide && { label: "Hide from Sidebar", run: () => env.hide!(node) });
  } else {
    add(!!env.newPage && { label: "New Page Here…", run: () => env.newPage!(folderOf(node)) });
    add(
      !!env.copy && { label: "Copy Path", run: () => env.copy!(at) },
      !!env.reveal && { label: "Reveal in Finder", run: () => env.reveal!(at) },
    );
  }
  return groups.flatMap((g, i) => (i ? ["separator" as const, ...g] : g));
}
