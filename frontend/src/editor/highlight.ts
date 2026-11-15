// Syntax highlighting for the editor: a small, dependency-free tokenizer for
// markdown structure and the code inside fenced blocks. It returns ranges
// with a class; the editor's mirror draws them behind the transparent
// textarea (lint/backdrop.ts). Speed over precision: it runs on every edit.

export type Token = { start: number; end: number; cls: string };

/** Code token classes (shared names with the reader's colours, see tokens.css). */
export type CodeClass = "tok-k" | "tok-s" | "tok-c" | "tok-m" | "tok-f";

const KEYWORDS = new Set((
  // C family, JS/TS, Go, Rust, Java, C#, Swift, Kotlin
  "break case catch class const continue default defer delete do else enum export extends false final finally for " +
  "func function go goto if implements import in instanceof interface let map match mod mut new nil null package " +
  "private protected pub public return select self static struct super switch this throw throws trait true try type " +
  "typeof undefined use var void where while with yield async await fn impl loop move ref unsafe chan fallthrough range " +
  // Python, Ruby, shell
  "and as assert def del elif except from global is lambda nonlocal not or pass raise None True False begin end ensure " +
  "module rescue then unless until when fi esac done local echo export readonly"
).split(" "));

/** Languages whose comments start with # rather than //. */
const HASH_COMMENTS = new Set(["py", "python", "rb", "ruby", "sh", "bash", "zsh", "shell", "console", "yaml", "yml", "toml", "r", "perl", "pl", "makefile", "make", "dockerfile", "ini", "conf"]);
/** Languages with no comments or keywords worth colouring. */
const PLAIN = new Set(["", "text", "txt", "plain", "csv", "log"]);

const ID = /^[A-Za-z_$][\w$]*/;
const NUM = /^(?:0[xX][\da-fA-F_]+|0[bB][01_]+|\d[\d_]*(?:\.\d[\d_]*)?(?:[eE][+-]?\d+)?)/;

/**
 * Tokenizes one line of code. `inBlock` is whether a /* comment is open at
 * the start; returns whether it is still open at the end.
 */
function codeLine(line: string, base: number, lang: string, inBlock: boolean, out: Token[]): boolean {
  const hash = HASH_COMMENTS.has(lang);
  const json = lang === "json";
  let i = 0;
  const n = line.length;
  if (inBlock) {
    const close = line.indexOf("*/");
    const end = close < 0 ? n : close + 2;
    if (end > 0) out.push({ start: base, end: base + end, cls: "tok-c" });
    if (close < 0) return true;
    i = end;
  }
  while (i < n) {
    const c = line[i];
    // Comments.
    if (!json && !hash && c === "/" && line[i + 1] === "/") {
      out.push({ start: base + i, end: base + n, cls: "tok-c" });
      return false;
    }
    if (hash && c === "#" && (i === 0 || /\s/.test(line[i - 1]))) {
      out.push({ start: base + i, end: base + n, cls: "tok-c" });
      return false;
    }
    if (!json && !hash && c === "/" && line[i + 1] === "*") {
      const close = line.indexOf("*/", i + 2);
      const end = close < 0 ? n : close + 2;
      out.push({ start: base + i, end: base + end, cls: "tok-c" });
      if (close < 0) return true;
      i = end;
      continue;
    }
    // Strings.
    if (c === '"' || c === "'" || c === "`") {
      let j = i + 1;
      while (j < n && line[j] !== c) j += line[j] === "\\" ? 2 : 1;
      const end = Math.min(n, j + 1);
      out.push({ start: base + i, end: base + end, cls: "tok-s" });
      i = end;
      continue;
    }
    // Numbers (not inside identifiers).
    if (/\d/.test(c) && (i === 0 || !/[\w$]/.test(line[i - 1]))) {
      const m = NUM.exec(line.slice(i));
      if (m) {
        out.push({ start: base + i, end: base + i + m[0].length, cls: "tok-m" });
        i += m[0].length;
        continue;
      }
    }
    // Identifiers: keywords, and names followed by "(" (calls/definitions).
    if (/[A-Za-z_$]/.test(c)) {
      const m = ID.exec(line.slice(i))!;
      const word = m[0];
      const end = i + word.length;
      if (!json && KEYWORDS.has(word)) out.push({ start: base + i, end: base + end, cls: "tok-k" });
      else if (json && (word === "true" || word === "false" || word === "null")) out.push({ start: base + i, end: base + end, cls: "tok-k" });
      else if (!json && line[end] === "(") out.push({ start: base + i, end: base + end, cls: "tok-f" });
      i = end;
      continue;
    }
    i++;
  }
  return false;
}

/** Inline markdown on one non-code line: code spans, links, emphasis, html tags. */
function inline(line: string, base: number, out: Token[]): void {
  const taken: [number, number][] = [];
  const free = (s: number, e: number) => taken.every(([a, b]) => e <= a || s >= b);
  const add = (s: number, e: number, cls: string) => {
    if (e > s && free(s, e)) {
      out.push({ start: base + s, end: base + e, cls });
      taken.push([s, e]);
    }
  };
  for (const m of Array.from(line.matchAll(/(`+)(.+?)\1/g))) add(m.index!, m.index! + m[0].length, "md-code");
  for (const m of Array.from(line.matchAll(/(!?\[)([^\]]*)\]\(([^)\s]*)(?:\s+"[^"]*")?\)/g))) {
    const s = m.index!;
    if (!free(s, s + m[0].length)) continue;
    const urlStart = s + m[1].length + m[2].length + 2;
    add(s, s + m[1].length, "md-marker");
    add(s + m[1].length, s + m[1].length + m[2].length, "md-link");
    add(s + m[1].length + m[2].length, urlStart, "md-marker");
    add(urlStart, s + m[0].length - 1, "md-url");
    add(s + m[0].length - 1, s + m[0].length, "md-marker");
  }
  for (const m of Array.from(line.matchAll(/<\/?[A-Za-z][\w-]*(?:\s[^<>]*)?\/?>|https?:\/\/[^\s<>()]+/g))) {
    add(m.index!, m.index! + m[0].length, m[0].startsWith("<") ? "md-html" : "md-url");
  }
  for (const m of Array.from(line.matchAll(/(\*\*|__)(?=\S)(.+?)(?<=\S)\1/g))) add(m.index!, m.index! + m[0].length, "md-strong");
  for (const m of Array.from(line.matchAll(/(?<![\w*])([*_])(?=\S)([^*_\n]+?)(?<=\S)\1(?![\w*])/g))) add(m.index!, m.index! + m[0].length, "md-em");
  for (const m of Array.from(line.matchAll(/~~(?=\S)(.+?)(?<=\S)~~/g))) add(m.index!, m.index! + m[0].length, "md-strike");
}

/**
 * Tokens for a whole markdown document, sorted and non-overlapping.
 * Unclosed fences run to the end of the document, as in CommonMark.
 */
export function highlight(text: string): Token[] {
  const out: Token[] = [];
  let pos = 0;
  let fence: { mark: string; lang: string } | null = null;
  let inBlockComment = false;
  let lineNo = 0;
  let frontmatter = text.startsWith("---\n") || text.startsWith("---\r\n");

  while (pos <= text.length) {
    let nl = text.indexOf("\n", pos);
    if (nl < 0) nl = text.length;
    const raw = text.slice(pos, nl);
    const line = raw.endsWith("\r") ? raw.slice(0, -1) : raw;
    lineNo++;

    if (frontmatter) {
      if (line) out.push({ start: pos, end: pos + line.length, cls: "md-meta" });
      if (lineNo > 1 && (line === "---" || line === "...")) frontmatter = false;
    } else if (fence) {
      const close = line.match(/^ {0,3}(`{3,}|~{3,})\s*$/);
      if (close && close[1][0] === fence.mark[0] && close[1].length >= fence.mark.length) {
        out.push({ start: pos, end: pos + line.length, cls: "md-fence" });
        fence = null;
        inBlockComment = false;
      } else if (!PLAIN.has(fence.lang)) {
        inBlockComment = codeLine(line, pos, fence.lang, inBlockComment, out);
      }
    } else {
      const open = line.match(/^ {0,3}(`{3,}|~{3,})\s*([\w+#.-]*)/);
      const heading = line.match(/^ {0,3}(#{1,6})(\s|$)/);
      if (open && !(open[1][0] === "`" && line.slice(open[0].length).includes("`"))) {
        out.push({ start: pos, end: pos + line.length, cls: "md-fence" });
        fence = { mark: open[1], lang: open[2].toLowerCase() };
      } else if (heading) {
        out.push({ start: pos, end: pos + line.length, cls: "md-heading" });
      } else if (/^ {0,3}([-*_])(\s*\1){2,}\s*$/.test(line)) {
        out.push({ start: pos, end: pos + line.length, cls: "md-marker" });
      } else {
        const lead = line.match(/^(\s*(?:>\s?)+)?(\s*(?:[-*+]|\d{1,9}[.)])\s+(?:\[[ xX]\]\s+)?)?/);
        let at = 0;
        if (lead?.[1]) {
          out.push({ start: pos, end: pos + lead[1].length, cls: "md-quote" });
          at = lead[1].length;
        }
        if (lead?.[2]) {
          out.push({ start: pos + at, end: pos + at + lead[2].length, cls: "md-marker" });
          at += lead[2].length;
        }
        inline(line.slice(at), pos + at, out);
      }
    }
    if (nl >= text.length) break;
    pos = nl + 1;
  }
  return out.sort((a, b) => a.start - b.start);
}
