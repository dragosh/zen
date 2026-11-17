// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from "vitest";
import { alertRange, backdrop } from "./backdrop";
import { alert } from "./test-util";

afterEach(() => document.body.replaceChildren());

function setup(text: string) {
  const host = document.createElement("div");
  const before = document.createElement("span");
  const ta = document.createElement("textarea");
  const after = document.createElement("span");
  host.append(before, ta, after);
  document.body.append(host);
  ta.value = text;
  const bd = backdrop(ta);
  return { host, before, after, ta, bd };
}
const type = (ta: HTMLTextAreaElement, value: string) => {
  ta.value = value;
  ta.dispatchEvent(new Event("input", { bubbles: true }));
};
/** The mirror's text and marks as "plain [marked] plain". */
function shape(mirror: HTMLElement): string {
  return Array.from(mirror.childNodes).map((n) =>
    n.nodeType === Node.TEXT_NODE ? n.textContent : `[${n.textContent}]`).join("").replace(/\u200b$/, "");
}

describe("alertRange", () => {
  it("turns line and rune columns into textarea offsets", () => {
    const text = "# Title\n\nSome teh text.\n";
    expect(alertRange(text, alert({ line: 3, col: 6, endCol: 8, match: "teh" }))).toEqual([14, 17]);
    expect(alertRange(text, alert({ line: 1, col: 3, endCol: 7, match: "Title" }))).toEqual([2, 7]);
  });
  it("counts astral characters as one column", () => {
    const text = "😀 teh";
    const [s, e] = alertRange(text, alert({ line: 1, col: 3, endCol: 5, match: "teh" }))!;
    expect(text.slice(s, e)).toBe("teh");
  });
  it("handles CRLF and clamps out-of-range alerts", () => {
    const text = "a\r\nteh\r\n";
    const [s, e] = alertRange(text, alert({ line: 2, col: 1, endCol: 3, match: "teh" }))!;
    expect(text.slice(s, e)).toBe("teh");
    expect(alertRange(text, alert({ line: 9, match: "x" }))).toBeNull();
    const [s2, e2] = alertRange(text, alert({ line: 2, col: 2, endCol: 99, match: "eh" }))!;
    expect(text.slice(s2, e2)).toBe("eh");
  });
});

describe("backdrop", () => {
  it("wraps the textarea in place with an aria-hidden mirror behind it", () => {
    const { host, before, after, ta, bd } = setup("x");
    expect(Array.from(host.children)).toEqual([before, bd.el, after]);
    expect(bd.el.classList.contains("zen-lint-wrap")).toBe(true);
    expect(Array.from(bd.el.children)).toEqual([bd.mirror, ta]);
    expect(bd.mirror.getAttribute("aria-hidden")).toBe("true");
    expect(bd.mirror.classList.contains("zen-lint-mirror")).toBe(true);
  });

  it("mirrors the text and draws marks at the exact ranges", () => {
    const text = "# Title\n\nSome teh text and teh end.\n";
    const { ta, bd } = setup(text);
    bd.update([
      alert({ line: 3, col: 6, endCol: 8, match: "teh", severity: "error" }),
      alert({ line: 3, col: 19, endCol: 21, match: "teh", severity: "suggestion" }),
    ], text);
    expect(shape(bd.mirror)).toBe("# Title\n\nSome [teh] text and [teh] end.\n");
    expect(bd.mirror.textContent!.replace(/\u200b$/, "")).toBe(ta.value);
    const marks = bd.mirror.querySelectorAll("mark");
    expect(marks[0].className).toBe("zen-lint zen-lint-error");
    expect(marks[1].className).toBe("zen-lint zen-lint-suggestion");
    expect(bd.mark(1)).toBe(marks[1]);
    expect(bd.range(0)).toEqual([14, 17]);
  });

  it("ends with a placeholder so a trailing newline keeps its line", () => {
    const { bd } = setup("a\n");
    bd.update([alert({ match: "a" })], "a\n");
    expect(bd.mirror.textContent!.endsWith("\n\u200b")).toBe(true);
  });

  it("draws highlight classes and lint marks together", () => {
    document.body.replaceChildren();
    const ta = document.createElement("textarea");
    ta.value = "let teh = 1";
    document.body.append(ta);
    const bd = backdrop(ta, { highlight: () => [{ start: 0, end: 3, cls: "tok-k" }, { start: 10, end: 11, cls: "tok-m" }] });
    expect(bd.mirror.classList.contains("is-highlighted")).toBe(true);
    expect(bd.mirror.querySelector(".tok-k")!.textContent).toBe("let");
    bd.update([alert({ col: 5, endCol: 7, match: "teh" })]);
    expect(bd.mirror.querySelector("mark.zen-lint")!.textContent).toBe("teh");
    expect(bd.mirror.querySelector(".tok-m")!.textContent).toBe("1");
    expect(bd.mirror.textContent).toBe("let teh = 1\u200b");
    // Edits re-highlight.
    ta.value = "let teh = 12";
    ta.dispatchEvent(new Event("input"));
    expect(bd.mirror.textContent).toBe("let teh = 12\u200b");
    bd.destroy();
  });

  it("numbers every line, keeps empty and trailing lines, and splits marks per line", () => {
    document.body.replaceChildren();
    const ta = document.createElement("textarea");
    ta.value = "one\n\nthree words\n";
    document.body.append(ta);
    const bd = backdrop(ta, { lineNumbers: true });
    const lines = () => Array.from(bd.mirror.querySelectorAll<HTMLElement>(".zen-ed-line"));
    expect(lines().map((l) => l.dataset.n)).toEqual(["1", "2", "3", "4"]);
    expect(lines().map((l) => l.textContent)).toEqual(["one", "\u200b", "three words", "\u200b"]);
    expect(bd.mirror.textContent!.includes("\n")).toBe(false);
    bd.update([alert({ line: 3, col: 7, endCol: 11, match: "words" })]);
    expect(lines()[2].querySelector("mark")!.textContent).toBe("words");
    expect(bd.mark(0)).toBe(lines()[2].querySelector("mark"));
    bd.destroy();
  });

  it("joins overlapping alerts and keeps Vale text out of the DOM", () => {
    const text = "very good";
    const { bd } = setup(text);
    bd.update([
      alert({ col: 1, endCol: 9, match: "very good", severity: "suggestion" }),
      alert({ col: 1, endCol: 4, match: "very", severity: "error", message: "<script>x</script>" }),
    ], text);
    expect(shape(bd.mirror)).toBe("[very good]");
    expect(bd.mirror.querySelector("script")).toBeNull();
    expect(bd.mark(0)).toBe(bd.mark(1));
  });

  it("follows typing: ranges before and after an edit move, ranges inside it go", () => {
    const text = "teh cat teh dog";
    const { ta, bd } = setup(text);
    bd.update([alert({ col: 1, endCol: 3, match: "teh" }), alert({ col: 9, endCol: 11, match: "teh" })], text);
    type(ta, "A teh cat teh dog");
    expect(shape(bd.mirror)).toBe("A [teh] cat [teh] dog");
    type(ta, "A teh cat txh dog");
    expect(shape(bd.mirror)).toBe("A [teh] cat txh dog");
    expect(bd.range(1)).toBeNull();
  });

  it("a result for older text is moved onto the current text", () => {
    const { ta, bd } = setup("teh cat");
    type(ta, "My teh cat");
    bd.update([alert({ col: 1, endCol: 3, match: "teh" })], "teh cat");
    expect(shape(bd.mirror)).toBe("My [teh] cat");
  });

  it("refresh picks up values set without an input event", () => {
    const { ta, bd } = setup("teh");
    bd.update([alert({ match: "teh" })], "teh");
    ta.value = "something else";
    bd.refresh();
    expect(bd.mirror.querySelector("mark")).toBeNull();
    expect(bd.range(0)).toBeNull();
  });

  it("syncs scrolling", () => {
    const { ta, bd } = setup("x\n".repeat(200));
    bd.update([alert({ match: "x" })], ta.value);
    ta.scrollTop = 120;
    ta.scrollLeft = 3;
    ta.dispatchEvent(new Event("scroll"));
    expect(bd.mirror.scrollTop).toBe(ta.scrollTop);
    expect(bd.mirror.scrollLeft).toBe(ta.scrollLeft);
  });

  it("copies the textarea's typography", () => {
    const { ta, bd } = setup("x");
    ta.style.fontSize = "17px";
    ta.style.lineHeight = "1.9";
    ta.style.paddingLeft = "31px";
    bd.sync();
    expect(bd.mirror.style.fontSize).toBe("17px");
    expect(bd.mirror.style.lineHeight).toBe("1.9");
    expect(bd.mirror.style.paddingLeft).toBe("31px");
  });

  it("select puts the selection on the alert's range", () => {
    const text = "Some teh text";
    const { ta, bd } = setup(text);
    bd.update([alert({ col: 6, endCol: 8, match: "teh" })], text);
    bd.select(0);
    expect(document.activeElement).toBe(ta);
    expect([ta.selectionStart, ta.selectionEnd]).toEqual([5, 8]);
  });

  it("update with no alerts empties the mirror; destroy unwraps", () => {
    const { host, before, after, ta, bd } = setup("teh");
    bd.update([alert({ match: "teh" })], "teh");
    bd.update([], "teh");
    expect(bd.mirror.querySelector("mark")).toBeNull();
    bd.destroy();
    expect(Array.from(host.children)).toEqual([before, ta, after]);
    type(ta, "x"); // listeners are gone: no throw
  });
});

describe("backdrop typography sync", () => {
  it("copies the textarea's typography even when built before it is in the page", () => {
    document.body.replaceChildren();
    const host = document.createElement("div");
    const ta = document.createElement("textarea");
    ta.style.fontFamily = "Menlo";
    ta.style.fontSize = "13px";
    ta.style.lineHeight = "20.8px";
    ta.style.paddingLeft = "58px";
    ta.value = "teh line";
    host.append(ta);
    // Built detached, like the editor does before mounting its root.
    const bd = backdrop(ta, { lineNumbers: true });
    expect(bd.mirror.style.lineHeight).toBe("");
    document.body.append(host);
    // The next draw (an input, or alerts arriving) syncs first.
    bd.update([alert({ match: "teh" })]);
    expect(bd.mirror.style.lineHeight).toBe("20.8px");
    expect(bd.mirror.style.paddingLeft).toBe("58px");
    expect(bd.mirror.style.fontFamily).toContain("Menlo");
    bd.destroy();
  });

  it("re-syncs when the textarea's typography changes", () => {
    document.body.replaceChildren();
    const ta = document.createElement("textarea");
    ta.style.lineHeight = "20px";
    document.body.append(ta);
    const bd = backdrop(ta, { lineNumbers: true });
    expect(bd.mirror.style.lineHeight).toBe("20px");
    ta.style.lineHeight = "24px";
    ta.value = "x";
    ta.dispatchEvent(new Event("input"));
    expect(bd.mirror.style.lineHeight).toBe("24px");
    bd.destroy();
  });
});
