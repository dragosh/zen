// @vitest-environment happy-dom
import { expect, test } from "vitest";
import type { Page } from "../host";
import { pageStats } from "./stats";

const page = (p: Partial<Page> = {}): Page => ({
  path: "a.md", title: "A", html: "", frontmatter: {}, breadcrumb: [], assets: { css: [], js: [] }, warnings: [],
  layout: { name: "article", source: "" }, ...p,
});
const root = (html: string) => {
  const d = document.createElement("article");
  d.innerHTML = html;
  return d;
};

test("counts words, headings, links, images, plugins and front matter keys", () => {
  const s = pageStats(root(`
    <h1>Title here</h1><p>One two <a href="b.md">three</a> four <a href="https://x.org">five</a>
    <a href="#top">six</a> <a href="mailto:a@b.c">mail</a> <a href="../c.md#x">seven</a></p>
    <h2>Sub</h2><img src="a.png" alt=""><img src="b.png" alt="">
    <div data-zen-plugin="quiz">Q1</div><div data-zen-plugin="quiz">Q2</div><span data-zen-plugin="calc">1+1</span>
  `), page({ frontmatter: { title: "A", tags: ["x"], layout: "article" } }));
  expect(s.headings).toBe(2);
  expect(s.links).toEqual({ internal: 3, external: 2 });
  expect(s.images).toBe(2);
  expect(s.plugins).toEqual(["calc", "quiz"]);
  expect(s.frontmatter).toEqual(["title", "tags", "layout"]);
  expect(s.words).toBe(14);
});

test("an empty page has zeros and no minutes", () => {
  const s = pageStats(root(""), page());
  expect(s).toEqual({ words: 0, minutes: 0, headings: 0, links: { internal: 0, external: 0 }, images: 0, plugins: [], frontmatter: [] });
});

test("reading time is words / 200, rounded, at least one minute", () => {
  const words = (n: number) => `<p>${Array(n).fill("w").join(" ")}</p>`;
  expect(pageStats(root(words(1)), page()).minutes).toBe(1);
  expect(pageStats(root(words(200)), page()).minutes).toBe(1);
  expect(pageStats(root(words(500)), page()).minutes).toBe(3);
  expect(pageStats(root(words(1000)), page()).minutes).toBe(5);
});

test("missing front matter does not throw", () => {
  expect(pageStats(root("<p>x</p>"), page({ frontmatter: undefined as never })).frontmatter).toEqual([]);
});
