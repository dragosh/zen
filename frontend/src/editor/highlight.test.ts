import { expect, test } from "vitest";
import { highlight, type Token } from "./highlight";

const pick = (text: string, toks: Token[], cls: string) => toks.filter((t) => t.cls === cls).map((t) => text.slice(t.start, t.end));

test("markdown structure", () => {
  const t = "# Title\n\n- [ ] task with `code` and **bold** and _em_\n> quote\n[link](https://x.y) <b>tag</b>\n---\n";
  const toks = highlight(t);
  expect(pick(t, toks, "md-heading")).toEqual(["# Title"]);
  expect(pick(t, toks, "md-code")).toEqual(["`code`"]);
  expect(pick(t, toks, "md-strong")).toEqual(["**bold**"]);
  expect(pick(t, toks, "md-em")).toEqual(["_em_"]);
  expect(pick(t, toks, "md-quote")).toEqual(["> "]);
  expect(pick(t, toks, "md-link")).toEqual(["link"]);
  expect(pick(t, toks, "md-url")).toEqual(["https://x.y"]);
  expect(pick(t, toks, "md-html")).toEqual(["<b>", "</b>"]);
  expect(pick(t, toks, "md-marker")).toContain("- [ ] ");
  expect(pick(t, toks, "md-marker")).toContain("---");
});

test("frontmatter is one block", () => {
  const t = "---\ntitle: X\n---\n# H\n";
  const toks = highlight(t);
  expect(pick(t, toks, "md-meta")).toEqual(["---", "title: X", "---"]);
  expect(pick(t, toks, "md-heading")).toEqual(["# H"]);
});

test("fenced code: keywords, strings, comments, numbers, calls; markdown not applied inside", () => {
  const t = "```go\nfunc main() {\n\t// say **hi**\n\tfmt.Println(\"hi\", 42)\n}\n```\n# after\n";
  const toks = highlight(t);
  expect(pick(t, toks, "md-fence")).toEqual(["```go", "```"]);
  expect(pick(t, toks, "tok-k")).toEqual(["func"]);
  expect(pick(t, toks, "tok-c")).toEqual(["// say **hi**"]);
  expect(pick(t, toks, "tok-s")).toEqual(['"hi"']);
  expect(pick(t, toks, "tok-m")).toEqual(["42"]);
  expect(pick(t, toks, "tok-f")).toEqual(["main", "Println"]);
  expect(pick(t, toks, "md-strong")).toEqual([]);
  expect(pick(t, toks, "md-heading")).toEqual(["# after"]);
});

test("block comments span lines; hash comments for python and shell", () => {
  const js = "```js\nlet a = 1 /* one\ntwo */ + 2\n```";
  expect(pick(js, highlight(js), "tok-c")).toEqual(["/* one", "two */"]);
  const py = "```python\nx = 'a#b'  # note\n```";
  const toks = highlight(py);
  expect(pick(py, toks, "tok-c")).toEqual(["# note"]);
  expect(pick(py, toks, "tok-s")).toEqual(["'a#b'"]);
});

test("unclosed fences run to the end; tildes and plain text", () => {
  const t = "~~~\nif x\n~~~\n```\n# not a heading\n";
  const toks = highlight(t);
  expect(pick(t, toks, "tok-k")).toEqual([]); // no language: plain
  expect(pick(t, toks, "md-heading")).toEqual([]);
});

test("tokens are sorted and never overlap", () => {
  const t = "Some **bold `code` inside** and [a **b**](u) `x` <i>y</i> https://z.z\n```ts\nconst s = \"//no\" // yes\n```\n";
  const toks = highlight(t);
  for (let i = 1; i < toks.length; i++) expect(toks[i].start).toBeGreaterThanOrEqual(toks[i - 1].end);
});

test("handles CRLF and a large document quickly", () => {
  const t = "# A\r\n```js\r\nlet x = 1\r\n```\r\n";
  expect(pick(t, highlight(t), "tok-k")).toEqual(["let"]);
  const big = ("Some text with **bold** and `code`.\n```js\nconst a = \"s\" // c\n```\n").repeat(5000);
  const start = performance.now();
  highlight(big);
  expect(performance.now() - start).toBeLessThan(1000);
});
