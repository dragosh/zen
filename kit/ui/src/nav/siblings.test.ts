import { expect, test } from "vitest";
import type { NavNode } from "../host";
import { copyLink, dropOrder, folderOf, movedOrder, relativePath, siblingGroup } from "./siblings";

const p = (title: string, path: string, children: NavNode[] = []): NavNode => ({ title, path, kind: "page", children });
const g = (title: string, path: string, children: NavNode[]): NavNode => ({ title, path, kind: "group", children });
const tree: NavNode[] = [
  p("Home", "README.md", [p("A", "a.md"), p("B", "b.md", [p("B1", "b/1.md")]), g("Folder", "f", [p("F1", "f/1.md")]), p("C", "c.md")]),
  g("Other pages", "", [g("notes", "notes", [p("N1", "notes/1.md"), p("N2", "notes/2.md")]), p("Loose", "loose.md")]),
];

test("siblingGroup: the page siblings and the parent the engine expects", () => {
  expect(siblingGroup(tree, "b.md")).toEqual({ parent: "README.md", pages: ["a.md", "b.md", "c.md"] });
  expect(siblingGroup(tree, "b/1.md")).toEqual({ parent: "b.md", pages: ["b/1.md"] });
  expect(siblingGroup(tree, "README.md")).toEqual({ parent: "", pages: ["README.md"] });
  expect(siblingGroup(tree, "notes/2.md")).toEqual({ parent: "notes", pages: ["notes/1.md", "notes/2.md"] });
  expect(siblingGroup(tree, "loose.md")).toEqual({ parent: "", pages: ["loose.md"] });
  expect(siblingGroup(tree, "nope.md")).toBeNull();
  expect(siblingGroup(tree, "f")).toBeNull(); // a folder is not a page
});

test("movedOrder: one step, nothing at the ends", () => {
  const pages = ["a.md", "b.md", "c.md"];
  expect(movedOrder(pages, "b.md", -1)).toEqual(["b.md", "a.md", "c.md"]);
  expect(movedOrder(pages, "b.md", 1)).toEqual(["a.md", "c.md", "b.md"]);
  expect(movedOrder(pages, "a.md", -1)).toBeNull();
  expect(movedOrder(pages, "c.md", 1)).toBeNull();
  expect(movedOrder(pages, "x.md", 1)).toBeNull();
  expect(pages).toEqual(["a.md", "b.md", "c.md"]); // not changed
});

test("dropOrder: index is the place in the list as it was (before that row)", () => {
  const pages = ["a.md", "b.md", "c.md", "d.md"];
  expect(dropOrder(pages, "a.md", 3)).toEqual(["b.md", "c.md", "a.md", "d.md"]);
  expect(dropOrder(pages, "d.md", 0)).toEqual(["d.md", "a.md", "b.md", "c.md"]);
  expect(dropOrder(pages, "b.md", 4)).toEqual(["a.md", "c.md", "d.md", "b.md"]);
  expect(dropOrder(pages, "b.md", 1)).toBeNull(); // where it is
  expect(dropOrder(pages, "b.md", 2)).toBeNull(); // right after itself: the same place
  expect(dropOrder(pages, "x.md", 0)).toBeNull();
});

test("folderOf: where New Page Here puts the page", () => {
  expect(folderOf({ path: "README.md", kind: "page" })).toBe("");
  expect(folderOf({ path: "guides/install.md", kind: "page" })).toBe("guides/");
  expect(folderOf({ path: "notes", kind: "group" })).toBe("notes/");
  expect(folderOf({ path: "a/b", kind: "group" })).toBe("a/b/");
});

test("relativePath: from the folder of a page", () => {
  expect(relativePath("", "guides/install.md")).toBe("guides/install.md");
  expect(relativePath("README.md", "guides/install.md")).toBe("guides/install.md");
  expect(relativePath("guides/usage.md", "guides/install.md")).toBe("install.md");
  expect(relativePath("guides/usage.md", "README.md")).toBe("../README.md");
  expect(relativePath("guides/deep/x.md", "guides/install.md")).toBe("../install.md");
  expect(relativePath("a/x.md", "b/y.md")).toBe("../b/y.md");
});

test("copyLink: a markdown link that survives odd titles and paths", () => {
  expect(copyLink("Install", "guides/install.md", "")).toBe("[Install](guides/install.md)");
  expect(copyLink("Install", "guides/install.md", "guides/usage.md")).toBe("[Install](install.md)");
  expect(copyLink("A [b] \\ c", "x.md", "")).toBe("[A \\[b\\] \\\\ c](x.md)");
  expect(copyLink("T", "my notes/a (1) #2.md", "")).toBe("[T](my%20notes/a%20%281%29%20%232.md)");
  expect(copyLink("Über", "über/ü.md", "")).toBe("[Über](über/ü.md)");
});
