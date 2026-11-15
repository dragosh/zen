// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from "vitest";
import { clearMarks, markText } from "./mark";
import { alert } from "./test-util";

function article(html: string): HTMLElement {
  const a = document.createElement("article");
  a.innerHTML = html; // test fixture only
  document.body.append(a);
  return a;
}
const marks = (root: HTMLElement) => Array.from(root.querySelectorAll<HTMLElement>("mark.zen-lint"));

afterEach(() => document.body.replaceChildren());

describe("markText", () => {
  it("wraps the match in a mark with severity class and title", () => {
    const a = article("<h1>Home</h1>\n<p>Some teh text.</p>");
    const got = markText(a, [alert({ line: 3, col: 6, endCol: 8, match: "teh", severity: "warning", check: "Vale.Spelling", message: "Did you mean 'the'?" })]);
    const [m] = marks(a);
    expect(m.textContent).toBe("teh");
    expect(m.className).toBe("zen-lint zen-lint-warning");
    expect(m.getAttribute("title")).toBe("Vale.Spelling: Did you mean 'the'?");
    expect(got).toEqual([m]);
    expect(a.querySelector("p")!.textContent).toBe("Some teh text.");
  });

  it("never marks inside code, pre or plugin blocks, nor in attributes", () => {
    const a = article(
      '<p><code>teh</code></p><pre><code>teh</code></pre><div data-zen-plugin="diagram"><p>teh</p></div>' +
      '<div class="zen-plugin-error">teh</div><p><a href="teh.md" title="teh">link</a></p>',
    );
    const got = markText(a, [alert({ line: 1, match: "teh" })]);
    expect(got).toEqual([null]);
    expect(marks(a)).toHaveLength(0);
    expect(a.querySelector("a")!.getAttribute("href")).toBe("teh.md");
  });

  it("marks link text but leaves the link intact", () => {
    const a = article('<p>See <a href="x.md" data-zen-link="internal">teh guide</a>.</p>');
    markText(a, [alert({ match: "teh" })]);
    const [m] = marks(a);
    expect(m.parentElement?.tagName).toBe("A");
    expect(a.querySelector("a")!.getAttribute("href")).toBe("x.md");
  });

  it("picks occurrences by the order of alerts on a line and moves on for later lines", () => {
    // Vale flags every occurrence, so the k-th alert for a word on a line is its k-th occurrence.
    const a = article("<p>teh one teh two</p>\n<p>one teh four</p>");
    const got = markText(a, [
      alert({ line: 1, col: 1, match: "teh", message: "first" }),
      alert({ line: 1, col: 9, match: "teh", message: "second" }),
      alert({ line: 3, col: 5, match: "teh", message: "fourth" }),
    ]);
    const ms = marks(a);
    expect(ms).toHaveLength(3);
    expect(got.map((m) => m?.title)).toEqual(["Vale.Spelling: first", "Vale.Spelling: second", "Vale.Spelling: fourth"]);
    expect(a.querySelectorAll("p")[0].innerHTML).toBe(
      '<mark class="zen-lint zen-lint-error" title="Vale.Spelling: first">teh</mark> one <mark class="zen-lint zen-lint-error" title="Vale.Spelling: second">teh</mark> two',
    );
    expect(a.querySelectorAll("p")[1].firstElementChild?.title).toBe("Vale.Spelling: fourth");
  });

  it("respects word boundaries", () => {
    const a = article("<p>tehran and teh</p>");
    markText(a, [alert({ match: "teh" })]);
    expect(a.querySelector("p")!.innerHTML).toBe('tehran and <mark class="zen-lint zen-lint-error" title="Vale.Spelling: Did you really mean \'teh\'?">teh</mark>');
  });

  it("joins alerts on the same text into one mark", () => {
    const a = article("<p>Teh end.</p>");
    const got = markText(a, [alert({ match: "Teh", check: "A", message: "one" }), alert({ match: "Teh", check: "B", message: "two", severity: "suggestion" })]);
    expect(marks(a)).toHaveLength(1);
    expect(got[0]).toBe(got[1]);
    expect(got[0]!.title).toBe("A: one\nB: two");
    expect(got[0]!.className).toBe("zen-lint zen-lint-error"); // the most severe wins
  });

  it("marks a match that spans inline elements", () => {
    const a = article("<p>a very <em>good</em> idea</p>");
    const got = markText(a, [alert({ match: "very good", severity: "suggestion" })]);
    const ms = marks(a);
    expect(ms.map((m) => m.textContent).join("")).toBe("very good");
    expect(got[0]).toBe(ms[0]);
    expect(a.querySelector("p")!.textContent).toBe("a very good idea");
  });

  it("does not match across blocks", () => {
    const a = article("<p>very</p><p>good</p>");
    expect(markText(a, [alert({ match: "very good" })])).toEqual([null]);
  });

  it("lists unplaceable alerts as null", () => {
    const a = article("<p>Nothing here.</p>");
    expect(markText(a, [alert({ match: "zzz" }), alert({ match: "" })])).toEqual([null, null]);
    expect(marks(a)).toHaveLength(0);
  });

  it("keeps Vale text as text: no markup injection", () => {
    const a = article("<p>x &lt;b&gt;y&lt;/b&gt; z</p>");
    const evil = '<img src=x onerror="alert(1)"><script>alert(1)</script>';
    markText(a, [alert({ match: "<b>y</b>", check: evil, message: evil, severity: '"><script>' as never })]);
    const [m] = marks(a);
    expect(m.textContent).toBe("<b>y</b>");
    expect(m.title).toBe(`${evil}: ${evil}`);
    expect(m.className).toBe("zen-lint zen-lint-suggestion");
    expect(a.querySelector("script, img, b")).toBeNull();
  });

  it("clearMarks restores the text", () => {
    const a = article("<p>teh <em>very good</em> teh</p>");
    const before = a.innerHTML;
    markText(a, [alert({ match: "teh" }), alert({ match: "very good" }), alert({ line: 1, col: 20, match: "teh" })]);
    expect(marks(a).length).toBe(3);
    clearMarks(a);
    expect(a.innerHTML).toBe(before);
    expect(a.querySelector("p")!.childNodes.length).toBe(3);
  });

  it("marking twice replaces the old marks", () => {
    const a = article("<p>teh and foo</p>");
    markText(a, [alert({ match: "teh" })]);
    markText(a, [alert({ match: "foo" })]);
    expect(marks(a).map((m) => m.textContent)).toEqual(["foo"]);
  });
});
