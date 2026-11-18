// Pure markdown editing commands: (text, selection) → (text, selection).
// The editor applies the result to the textarea as one native edit.

export type Edit = { text: string; selStart: number; selEnd: number };
export type CommandName =
  "bold" | "italic" | "strike" | "h1" | "h2" | "quote" | "bullet" | "numbered" | "task" | "code" | "link" | "table";

/** Wraps the selection in open/close, or removes them when already there. */
function wrap(e: Edit, open: string, close: string): Edit {
  const { text, selStart: s, selEnd: t } = e;
  const sel = text.slice(s, t);
  if (sel.length >= open.length + close.length && sel.startsWith(open) && sel.endsWith(close)) {
    const inner = sel.slice(open.length, sel.length - close.length);
    return { text: text.slice(0, s) + inner + text.slice(t), selStart: s, selEnd: s + inner.length };
  }
  if (text.slice(s - open.length, s) === open && text.slice(t, t + close.length) === close && s >= open.length) {
    return {
      text: text.slice(0, s - open.length) + sel + text.slice(t + close.length),
      selStart: s - open.length, selEnd: t - open.length,
    };
  }
  return { text: text.slice(0, s) + open + sel + close + text.slice(t), selStart: s + open.length, selEnd: t + open.length };
}

function code(e: Edit): Edit {
  const sel = e.text.slice(e.selStart, e.selEnd);
  const fenced = sel.includes("\n")
    || (e.text.slice(0, e.selStart).endsWith("```\n") && e.text.slice(e.selEnd).startsWith("\n```"));
  return fenced ? wrap(e, "```\n", "\n```") : wrap(e, "`", "`");
}

function link(e: Edit): Edit {
  const { text, selStart: s, selEnd: t } = e;
  const sel = text.slice(s, t);
  const whole = sel.match(/^\[([^\]\n]*)\]\([^)\n]*\)$/);
  if (whole) {
    return { text: text.slice(0, s) + whole[1] + text.slice(t), selStart: s, selEnd: s + whole[1].length };
  }
  const head = text.slice(0, s).match(/\[([^\]\n]*)\]\($/);
  if (head && text[t] === ")" && !/[)\n]/.test(sel)) {
    const from = s - head[0].length;
    const label = head[1];
    return { text: text.slice(0, from) + label + text.slice(t + 1), selStart: from, selEnd: from + label.length };
  }
  const url = "url";
  const out = `[${sel}](${url})`;
  const at = s + sel.length + 3;
  return { text: text.slice(0, s) + out + text.slice(t), selStart: at, selEnd: at + url.length };
}

type LineKind = "h1" | "h2" | "quote" | "bullet" | "numbered" | "task";
const OWN: Record<LineKind, RegExp> = {
  h1: /^# /, h2: /^## /, quote: /^> /,
  task: /^[-*+] \[[ xX]\] /, bullet: /^[-*+] (?!\[[ xX]\] )/, numbered: /^\d+[.)] /,
};
const GROUP: Record<LineKind, LineKind[]> = {
  h1: ["h1", "h2"], h2: ["h1", "h2"], quote: ["quote"],
  bullet: ["task", "bullet", "numbered"], numbered: ["task", "bullet", "numbered"], task: ["task", "bullet", "numbered"],
};

/** Start and end offsets of the lines the selection touches. */
export function lineRange(text: string, s: number, t: number): [number, number] {
  const start = text.lastIndexOf("\n", s - 1) + 1;
  const last = t > s && text[t - 1] === "\n" ? t - 1 : t;
  const nl = text.indexOf("\n", last);
  return [start, nl < 0 ? text.length : nl];
}

/** Rewrites each line of the selected range and keeps the selection on the same text. */
function mapLines(e: Edit, fn: (lines: string[]) => string[]): Edit {
  const [start, end] = lineRange(e.text, e.selStart, e.selEnd);
  const lines = e.text.slice(start, end).split("\n");
  const out = fn(lines);
  const block = out.join("\n");
  const first = out[0].length - lines[0].length;
  const total = block.length - (end - start);
  const selStart = Math.max(start, e.selStart + first);
  const selEnd = e.selStart === e.selEnd ? selStart : Math.max(selStart, e.selEnd + total);
  return { text: e.text.slice(0, start) + block + e.text.slice(end), selStart, selEnd };
}

function linePrefix(e: Edit, kind: LineKind): Edit {
  return mapLines(e, (lines) => {
    const split = lines.map((l) => { const ind = l.match(/^\s*/)![0]; return [ind, l.slice(ind.length)]; });
    let targets = lines.map((l) => l.trim() !== "");
    if (!targets.some(Boolean)) targets = lines.map(() => true);
    const all = split.every(([, rest], i) => !targets[i] || OWN[kind].test(rest));
    let n = 1;
    return split.map(([ind, rest], i) => {
      if (!targets[i]) return ind + rest;
      if (all) return ind + rest.replace(OWN[kind], "");
      for (const k of GROUP[kind]) rest = rest.replace(OWN[k], "");
      const prefix = { h1: "# ", h2: "## ", quote: "> ", bullet: "- ", task: "- [ ] ", numbered: `${n++}. ` }[kind];
      return ind + prefix + rest;
    });
  });
}

const TABLE = "| Column | Column | Column |\n| --- | --- | --- |\n|  |  |  |\n|  |  |  |\n";

function table(e: Edit): Edit {
  const { text, selStart: s, selEnd: t } = e;
  const lead = s === 0 || text[s - 1] === "\n" ? "" : "\n";
  const tail = t < text.length && text[t] !== "\n" ? "\n" : "";
  const ins = lead + TABLE + tail;
  const caret = s + lead.length + 2; // inside the first header cell
  return { text: text.slice(0, s) + ins + text.slice(t), selStart: caret, selEnd: caret + "Column".length };
}

export const commands: Record<CommandName, (e: Edit) => Edit> = {
  bold: (e) => wrap(e, "**", "**"),
  italic: (e) => wrap(e, "_", "_"),
  strike: (e) => wrap(e, "~~", "~~"),
  code,
  link,
  table,
  h1: (e) => linePrefix(e, "h1"),
  h2: (e) => linePrefix(e, "h2"),
  quote: (e) => linePrefix(e, "quote"),
  bullet: (e) => linePrefix(e, "bullet"),
  numbered: (e) => linePrefix(e, "numbered"),
  task: (e) => linePrefix(e, "task"),
};

/** Enter: continue a list / quote / task prefix, or remove an empty one. null = default newline. */
export function continueList(e: Edit): Edit | null {
  const { text, selStart: s, selEnd: t } = e;
  if (s !== t) return null;
  const [start, end] = lineRange(text, s, s);
  const line = text.slice(start, end);
  const m = line.match(/^(\s*)(> |([-*+]) \[[ xX]\] |[-*+] |(\d+)([.)]) )/);
  if (!m || s < start + m[0].length) return null;
  if (line.slice(m[0].length).trim() === "") {
    return { text: text.slice(0, start) + text.slice(end), selStart: start, selEnd: start };
  }
  const next = m[3] ? `${m[3]} [ ] ` : m[4] ? `${Number(m[4]) + 1}${m[5]} ` : m[2];
  const ins = "\n" + m[1] + next;
  return { text: text.slice(0, s) + ins + text.slice(s), selStart: s + ins.length, selEnd: s + ins.length };
}

/** Tab: two spaces at the caret, or indent every selected line. */
export function indent(e: Edit): Edit {
  if (e.selStart === e.selEnd) {
    const at = e.selStart + 2;
    return { text: e.text.slice(0, e.selStart) + "  " + e.text.slice(e.selEnd), selStart: at, selEnd: at };
  }
  return mapLines(e, (lines) => lines.map((l) => "  " + l));
}

/** Shift-Tab: remove up to two leading spaces (or a tab) from every selected line. */
export function outdent(e: Edit): Edit {
  return mapLines(e, (lines) => lines.map((l) => l.replace(/^( {1,2}|\t)/, "")));
}
