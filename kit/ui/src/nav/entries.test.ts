import { expect, test, vi } from "vitest";
import type { NavNode } from "../host";
import { menuEntries, type EntryEnv } from "./entries";
import type { MenuEntry } from "./menu";

const p = (title: string, path: string, children: NavNode[] = []): NavNode => ({ title, path, kind: "page", children });
const g = (title: string, path: string, children: NavNode[]): NavNode => ({ title, path, kind: "group", children });
const tree: NavNode[] = [
  p("Home", "README.md", [p("Install", "guides/install.md"), p("Usage", "guides/usage.md"), p("Tips", "tips.md")]),
  g("Other pages", "", [g("notes", "notes", [p("Scratch", "notes/scratch.md")])]),
];

function env(o: Partial<EntryEnv> = {}): EntryEnv & Record<string, ReturnType<typeof vi.fn>> {
  const full = {
    tree, current: "", open: vi.fn(), editor: vi.fn(), newPage: vi.fn(), move: vi.fn(), copy: vi.fn(), reveal: vi.fn(), hide: vi.fn(),
  };
  return { ...full, ...o } as never;
}
const items = (es: MenuEntry[]) => es.filter((e): e is Exclude<MenuEntry, "separator"> => e !== "separator");
const labels = (es: MenuEntry[]) => es.map((e) => (e === "separator" ? "-" : e.label));
const find = (es: MenuEntry[], label: string) => items(es).find((e) => e.label === label)!;

test("a page has every item, in groups", () => {
  const es = menuEntries(tree[0].children[0], env());
  expect(labels(es)).toEqual([
    "Open", "Open in Editor", "-", "New Page Here…", "Move Up", "Move Down", "-", "Copy Path", "Copy Link", "Reveal in Finder", "-", "Hide from Sidebar",
  ]);
});

test("Open, Open in Editor and New Page Here use the page and its folder", () => {
  const e = env();
  const es = menuEntries(tree[0].children[0], e);
  find(es, "Open").run(); expect(e.open).toHaveBeenCalledWith("guides/install.md");
  find(es, "Open in Editor").run(); expect(e.editor).toHaveBeenCalledWith("guides/install.md");
  find(es, "New Page Here…").run(); expect(e.newPage).toHaveBeenCalledWith("guides/");
  find(es, "Reveal in Finder").run(); expect(e.reveal).toHaveBeenCalledWith("guides/install.md");
  find(es, "Hide from Sidebar").run(); expect(e.hide).toHaveBeenCalledWith(tree[0].children[0]);
});

test("Move Up is disabled for the first sibling and Move Down for the last; both run with a direction", () => {
  const e = env();
  const first = menuEntries(tree[0].children[0], e);
  expect(find(first, "Move Up").disabled).toBe(true);
  expect(find(first, "Move Down").disabled).toBeFalsy();
  find(first, "Move Down").run();
  expect(e.move).toHaveBeenCalledWith("guides/install.md", 1);
  const last = menuEntries(tree[0].children[2], e);
  expect(find(last, "Move Down").disabled).toBe(true);
  find(last, "Move Up").run();
  expect(e.move).toHaveBeenLastCalledWith("tips.md", -1);
  // An only child: both are disabled.
  const only = menuEntries(tree[0], e);
  expect(find(only, "Move Up").disabled).toBe(true);
  expect(find(only, "Move Down").disabled).toBe(true);
});

test("Copy Path is workspace-relative, Copy Link is a markdown link relative to the open page", () => {
  const e = env({ current: "guides/usage.md" });
  const es = menuEntries(tree[0].children[0], e);
  find(es, "Copy Path").run(); expect(e.copy).toHaveBeenLastCalledWith("guides/install.md");
  find(es, "Copy Link").run(); expect(e.copy).toHaveBeenLastCalledWith("[Install](install.md)");
  const none = env();
  find(menuEntries(tree[0].children[0], none), "Copy Link").run();
  expect(none.copy).toHaveBeenLastCalledWith("[Install](guides/install.md)");
});

test("items the host does not support are left out, and no separator is left at an end or doubled", () => {
  const es = menuEntries(tree[0].children[0], env({ editor: undefined, newPage: undefined, move: undefined, reveal: undefined, hide: undefined }));
  expect(labels(es)).toEqual(["Open", "-", "Copy Path", "Copy Link"]);
  const bare = menuEntries(tree[0].children[0], env({ editor: undefined, newPage: undefined, move: undefined, copy: undefined as never, reveal: undefined, hide: undefined }));
  expect(labels(bare)).toEqual(["Open"]);
  for (const drop of ["editor", "newPage", "move", "reveal", "hide"] as const) {
    const l = labels(menuEntries(tree[0].children[1], env({ [drop]: undefined })));
    expect(l[0]).not.toBe("-");
    expect(l[l.length - 1]).not.toBe("-");
    expect(l.join(",")).not.toContain("-,-");
  }
});

test("a folder group: New Page Here, Copy Path, Reveal; the Other pages group has no menu", () => {
  const notes = tree[1].children[0];
  const es = menuEntries(notes, env());
  expect(labels(es)).toEqual(["New Page Here…", "-", "Copy Path", "Reveal in Finder"]);
  const e = env();
  find(menuEntries(notes, e), "New Page Here…").run();
  expect(e.newPage).toHaveBeenCalledWith("notes/");
  expect(menuEntries(tree[1], env())).toEqual([]);
});
