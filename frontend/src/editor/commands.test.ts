import { describe, expect, it } from "vitest";
import { commands, continueList, indent, outdent, type Edit } from "./commands";

/** "a[b]c" → text "abc", selection 1..2; "a|c" → caret at 1. */
function parse(s: string): Edit {
  const caret = s.indexOf("|");
  if (caret >= 0) return { text: s.replace("|", ""), selStart: caret, selEnd: caret };
  const a = s.indexOf("[["), b = s.indexOf("]]");
  return { text: s.replace("[[", "").replace("]]", ""), selStart: a, selEnd: b - 2 };
}
function show(e: Edit): string {
  if (e.selStart === e.selEnd) return e.text.slice(0, e.selStart) + "|" + e.text.slice(e.selStart);
  return e.text.slice(0, e.selStart) + "[[" + e.text.slice(e.selStart, e.selEnd) + "]]" + e.text.slice(e.selEnd);
}
type Name = keyof typeof commands;

const wrapping: [Name, string, string][] = [
  ["bold", "a [[word]] b", "a **[[word]]** b"],
  ["bold", "a | b", "a **|** b"],
  ["italic", "a [[word]] b", "a _[[word]]_ b"],
  ["italic", "|", "_|_"],
  ["strike", "[[x]]", "~~[[x]]~~"],
  ["strike", "|", "~~|~~"],
  ["code", "a [[x]] b", "a `[[x]]` b"],
  ["code", "|", "`|`"],
  ["code", "[[one\ntwo]]", "```\n[[one\ntwo]]\n```"],
  ["link", "[[zen]]", "[zen]([[url]])"],
  ["link", "a | b", "a []([[url]]) b"],
];

describe("wrapping commands", () => {
  it.each(wrapping)("%s: %j → %j and back", (name, input, output) => {
    const once = commands[name](parse(input));
    expect(show(once)).toBe(output);
    const twice = commands[name](once);
    expect(twice.text).toBe(parse(input).text);
  });
  it("bold toggles off when the markers are inside the selection", () => {
    expect(show(commands.bold(parse("[[**x**]]")))).toBe("[[x]]");
  });
  it("link toggles off a selected link", () => {
    expect(show(commands.link(parse("[[[zen](http://z)]]")))).toBe("[[zen]]");
  });
});

const lineCases: [Name, string, string][] = [
  ["h1", "ti|tle", "# ti|tle"],
  ["h2", "ti|tle", "## ti|tle"],
  ["quote", "[[a\nb]]", "> [[a\n> b]]"],
  ["bullet", "[[a\nb]]", "- [[a\n- b]]"],
  ["numbered", "[[a\nb\nc]]", "1. [[a\n2. b\n3. c]]"],
  ["task", "[[a\nb]]", "- [ ] [[a\n- [ ] b]]"],
  ["bullet", "|", "- |"],
  ["task", "|", "- [ ] |"],
  ["quote", "x\n[[a\n\nb]]", "x\n> [[a\n\n> b]]"],
];

describe("line commands", () => {
  it.each(lineCases)("%s: %j → %j and back", (name, input, output) => {
    const once = commands[name](parse(input));
    expect(show(once)).toBe(output);
    expect(commands[name](once).text).toBe(parse(input).text);
  });
  it("task replaces a bullet prefix", () => {
    expect(commands.task(parse("- a|")).text).toBe("- [ ] a");
  });
  it("bullet replaces a numbered prefix", () => {
    expect(commands.bullet(parse("1. a|")).text).toBe("- a");
  });
  it("mixed lines get the prefix added, not toggled", () => {
    expect(commands.quote(parse("[[> a\nb]]")).text).toBe("> a\n> b");
  });
  it("h1 replaces an h2 prefix", () => {
    expect(show(commands.h1(parse("## ti|tle")))).toBe("# ti|tle");
  });
});

describe("table", () => {
  it("inserts a 3x2 skeleton on its own line", () => {
    const e = commands.table(parse("text|"));
    const lines = e.text.split("\n").filter(Boolean);
    expect(lines[0]).toBe("text");
    expect(lines.slice(1)).toHaveLength(4);
    expect(lines[1].split("|").length - 2).toBe(3);
    expect(lines[2]).toMatch(/^\|( --- \|){3}$/);
  });
});

describe("continueList", () => {
  const cases: [string, string | null][] = [
    ["- a|", "- a\n- |"],
    ["* a|", "* a\n* |"],
    ["  - a|", "  - a\n  - |"],
    ["1. a|", "1. a\n2. |"],
    ["9) a|", "9) a\n10) |"],
    ["> a|", "> a\n> |"],
    ["- [x] a|", "- [x] a\n- [ ] |"],
    ["x\n- |", "x\n|"],
    ["1. |", "|"],
    ["- [ ] |", "|"],
    ["plain|", null],
    ["", null],
  ];
  it.each(cases)("%j → %j", (input, output) => {
    const r = continueList(parse(input.includes("|") ? input : input + "|"));
    expect(r === null ? null : show(r)).toBe(output);
  });
  it("splits the line at the caret", () => {
    expect(show(continueList(parse("- ab|cd"))!)).toBe("- ab\n- |cd");
  });
  it("returns null for a selection", () => {
    expect(continueList(parse("- [[a]]"))).toBeNull();
  });
});

describe("indent / outdent", () => {
  it("tab inserts two spaces at the caret", () => {
    expect(show(indent(parse("a|b")))).toBe("a  |b");
  });
  it("indents every selected line", () => {
    expect(indent(parse("[[a\nb]]")).text).toBe("  a\n  b");
  });
  it("outdents up to two spaces per line", () => {
    expect(outdent(parse("[[   a\n b\nc]]")).text).toBe(" a\nb\nc");
    expect(show(outdent(parse("  a|")))).toBe("a|");
  });
});
