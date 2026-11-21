// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  MAX_MATCHES, bindFind, createFind, domFinder, findInText, textareaFinder,
  type Find, type FindEvents, type HighlightSupport,
} from "./index";

// ---- pure search ----
describe("findInText", () => {
  it("is case-insensitive by default and exact with match case", () => {
    expect(findInText("Foo foo FOO", "foo", false)).toEqual([0, 4, 8]);
    expect(findInText("Foo foo FOO", "foo", true)).toEqual([4]);
  });
  it("treats the query literally, never as a regex", () => {
    expect(findInText("x a.b( y axb( a.b(", "a.b(", false)).toEqual([2, 14]);
    expect(findInText("[a] (b) * + ? ^ $ \\ |", "[a]", false)).toEqual([0]);
    expect(findInText("aaa", "(", false)).toEqual([]);
  });
  it("finds nothing for an empty query and does not loop on overlaps", () => {
    expect(findInText("abc", "", false)).toEqual([]);
    expect(findInText("aaaa", "aa", false)).toEqual([0, 2]);
  });
});

describe("match cap", () => {
  it("findInText stops at MAX_MATCHES", () => {
    expect(findInText("a".repeat(MAX_MATCHES + 1000), "a", false)).toHaveLength(MAX_MATCHES);
  });
  it("the DOM finder stops at MAX_MATCHES and the bar says '5000+'", () => {
    const a = document.createElement("article");
    a.innerHTML = `<p>${"a".repeat(MAX_MATCHES + 1000)}</p>`;
    document.body.append(a);
    const host = document.createElement("div");
    document.body.append(host);
    const f = domFinder(() => a, { highlights: { registry: new Map(), create: (ranges) => ({ ranges }) } });
    expect(f.find("a", false)).toBe(MAX_MATCHES);
    const find = createFind({ doc: document, host, finder: f });
    find.open("a");
    expect(host.querySelector("[aria-live]")!.textContent).toBe(`1 of ${MAX_MATCHES}+`);
    find.destroy();
    document.body.replaceChildren();
  });
});

// ---- the page's DOM ----
function root(html: string): HTMLElement {
  const r = document.createElement("article");
  r.innerHTML = html;
  document.body.append(r);
  return r;
}

describe("domFinder", () => {
  let registry: Map<string, { ranges: Range[] }>;
  let highlights: HighlightSupport;
  beforeEach(() => {
    registry = new Map();
    highlights = { registry, create: (ranges) => ({ ranges }) };
  });
  afterEach(() => document.body.replaceChildren());

  it("counts matches and highlights them all, the current one apart", () => {
    const r = root("<p>The cat. the Dog</p><p>THE end</p>");
    const f = domFinder(() => r, { highlights });
    expect(f.find("the", false)).toBe(3);
    f.show(1);
    expect(registry.get("zen-find")!.ranges).toHaveLength(3);
    expect(registry.get("zen-find-current")!.ranges).toHaveLength(1);
    expect(registry.get("zen-find-current")!.ranges[0].toString()).toBe("the");
    expect(f.find("the", true)).toBe(1);
  });

  it("matches across inline markup inside one block", () => {
    const r = root("<p>foo <strong>bar</strong> baz</p>");
    expect(domFinder(() => r, { highlights }).find("foo bar", false)).toBe(1);
  });

  it("does not match across blocks, scripts or skipped elements", () => {
    const r = root("<p>foo</p><p>bar</p><script>foobar</script><div class=skip>foobar</div>");
    const f = domFinder(() => r, { highlights, skip: (e) => e.classList.contains("skip") });
    expect(f.find("foobar", false)).toBe(0);
    expect(f.find("bar", false)).toBe(1);
  });

  it("clear removes every highlight", () => {
    const r = root("<p>abc abc</p>");
    const f = domFinder(() => r, { highlights });
    f.find("abc", false);
    f.show(0);
    f.clear();
    expect(registry.size).toBe(0);
  });

  it("never reads the query as HTML", () => {
    const r = root("<p>a &lt;b&gt; c</p>");
    const f = domFinder(() => r, { highlights });
    expect(f.find("<b>", false)).toBe(1);
    expect(r.querySelector("b")).toBeNull();
  });

  it("opens a closed <details> that holds the match", () => {
    const r = root("<details><summary>More</summary><p>hidden needle</p></details><details open><summary>x</summary></details>");
    const d = r.querySelector("details")!;
    const f = domFinder(() => r, { highlights });
    expect(f.find("needle", false)).toBe(1);
    expect(d.open).toBe(false);
    f.show(0);
    expect(d.open).toBe(true);
  });

  it("asks the reveal hook to show a hidden block, and falls back to unhiding it", () => {
    const r = root('<section class="slides-slide" hidden><p>deep needle</p></section><section hidden><p>other needle</p></section>');
    const slide = r.querySelector<HTMLElement>(".slides-slide")!;
    const reveal = vi.fn((block: HTMLElement) => block === slide);
    const f = domFinder(() => r, { highlights, reveal });
    expect(f.find("needle", false)).toBe(2);
    f.show(0);
    expect(reveal).toHaveBeenCalledWith(slide);
    f.show(1);
    expect(r.querySelectorAll("section")[1].hidden).toBe(false);
  });

  it("selects the current match when highlights are not supported", () => {
    const r = root("<p>one two one</p>");
    const f = domFinder(() => r, { highlights: null });
    f.find("one", false);
    f.show(1);
    expect(window.getSelection()!.toString()).toBe("one");
  });

  it("returns 0 without a root", () => {
    expect(domFinder(() => null, { highlights }).find("a", false)).toBe(0);
  });
});

// ---- a textarea ----
describe("textareaFinder", () => {
  afterEach(() => document.body.replaceChildren());
  it("selects the match in the source", () => {
    const ta = document.createElement("textarea");
    ta.value = "# Title\n\nSome text, some Text.\n";
    document.body.append(ta);
    const f = textareaFinder(ta);
    expect(f.find("text", false)).toBe(2);
    f.show(1);
    expect([ta.selectionStart, ta.selectionEnd]).toEqual([25, 29]);
    expect(f.find("text", true)).toBe(1);
    f.show(0);
    expect(ta.value.slice(ta.selectionStart, ta.selectionEnd)).toBe("text");
  });
  it("offers the selected text as the starting query, and keeps focus where it was", () => {
    const ta = document.createElement("textarea");
    const other = document.createElement("input");
    ta.value = "alpha beta";
    document.body.append(ta, other);
    ta.setSelectionRange(6, 10);
    expect(textareaFinder(ta).selected()).toBe("beta");
    other.focus();
    textareaFinder(ta).find("alpha", false);
    textareaFinder(ta).show(0);
    expect(document.activeElement).toBe(other);
  });
});

// ---- the bar ----
const key = (target: EventTarget, k: string, init: KeyboardEventInit = {}) => {
  const e = new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...init });
  target.dispatchEvent(e);
  return e;
};
const type = (input: HTMLInputElement, value: string) => {
  input.value = value;
  input.dispatchEvent(new Event("input", { bubbles: true }));
};

describe("find bar", () => {
  let registry: Map<string, { ranges: Range[] }>;
  let host: HTMLElement;
  let article: HTMLElement;
  let find: Find;
  const bar = () => host.querySelector<HTMLElement>("[role=search]")!;
  const field = () => bar().querySelector<HTMLInputElement>("input[type=search], input")!;
  const count = () => bar().querySelector("[aria-live]")!.textContent;
  const btn = (label: string) => bar().querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!;
  const mark = (name: string) => registry.get(name)?.ranges ?? [];

  beforeEach(() => {
    registry = new Map();
    host = document.createElement("div");
    article = document.createElement("article");
    article.innerHTML = "<p>One two. one TWO one.</p><details><summary>S</summary><p>one in details</p></details>";
    document.body.append(host, article);
    find = createFind({
      doc: document, host,
      finder: domFinder(() => article, { highlights: { registry, create: (ranges) => ({ ranges }) } }),
    });
  });
  afterEach(() => { find.destroy(); document.body.replaceChildren(); });

  it("is not in the page until opened; open shows an accessible bar with the field focused", () => {
    expect(host.querySelector("[role=search]")).toBeNull();
    find.open();
    expect(bar().getAttribute("aria-label")).toBeTruthy();
    expect(field().getAttribute("aria-label")).toBeTruthy();
    expect(bar().querySelector("[aria-live=polite]")).not.toBeNull();
    expect(document.activeElement).toBe(field());
    expect(find.isOpen).toBe(true);
  });

  it("fills in the selection it is given", () => {
    find.open("two");
    expect(field().value).toBe("two");
    expect(count()).toBe("1 of 2");
  });

  it("counts 'n of m' while typing, highlights all and the current one", () => {
    find.open();
    type(field(), "one");
    expect(count()).toBe("1 of 4");
    expect(mark("zen-find")).toHaveLength(4);
    expect(mark("zen-find-current")).toHaveLength(1);
    type(field(), "zzz");
    expect(count()).toBe("No results");
    expect(mark("zen-find")).toHaveLength(0);
    type(field(), "");
    expect(count()).toBe("");
  });

  it("goes next and previous with Enter and Shift+Enter, wrapping", () => {
    find.open();
    type(field(), "one");
    key(field(), "Enter");
    expect(count()).toBe("2 of 4");
    key(field(), "Enter"); key(field(), "Enter");
    expect(count()).toBe("4 of 4");
    key(field(), "Enter");
    expect(count()).toBe("1 of 4");
    key(field(), "Enter", { shiftKey: true });
    expect(count()).toBe("4 of 4");
    btn("Previous match").click();
    expect(count()).toBe("3 of 4");
    btn("Next match").click();
    expect(count()).toBe("4 of 4");
  });

  it("next() and prev() work while the bar is closed by opening it", () => {
    find.open("one");
    find.close();
    find.next();
    expect(find.isOpen).toBe(true);
  });

  it("is case-insensitive until Aa is switched on", () => {
    find.open();
    type(field(), "TWO");
    expect(count()).toBe("1 of 2");
    const aa = btn("Match case");
    expect(aa.getAttribute("aria-pressed")).toBe("false");
    aa.click();
    expect(aa.getAttribute("aria-pressed")).toBe("true");
    expect(count()).toBe("1 of 1");
  });

  it("treats regex characters in the query literally", () => {
    article.innerHTML = "<p>call a.b( now, axb( not</p>";
    find.open();
    type(field(), "a.b(");
    expect(count()).toBe("1 of 1");
  });

  it("opens the closed <details> holding the match when jumped to", () => {
    const d = article.querySelector("details")!;
    find.open();
    type(field(), "details");
    expect(d.open).toBe(true);
  });

  it("Esc closes the bar, removes the highlights and returns the focus", () => {
    const before = document.createElement("button");
    document.body.append(before);
    before.focus();
    find.open("one");
    expect(mark("zen-find").length).toBeGreaterThan(0);
    const e = key(field(), "Escape");
    expect(e.defaultPrevented).toBe(true);
    expect(find.isOpen).toBe(false);
    expect(host.querySelector("[role=search]")).toBeNull();
    expect(registry.size).toBe(0);
    expect(document.activeElement).toBe(before);
  });

  it("opening again selects the field's text", () => {
    find.open("one");
    field().setSelectionRange(1, 1);
    find.open();
    expect(field().selectionStart).toBe(0);
    expect(field().selectionEnd).toBe(3);
    expect(host.querySelectorAll("[role=search]")).toHaveLength(1);
  });

  it("remembers the last query when reopened empty", () => {
    find.open("one");
    find.close();
    find.open();
    expect(field().value).toBe("one");
  });

  it("the close button closes", () => {
    find.open("one");
    btn("Close find").click();
    expect(find.isOpen).toBe(false);
  });

  it("refresh searches again after the page changed", () => {
    find.open("one");
    article.innerHTML = "<p>one</p>";
    find.refresh();
    expect(count()).toBe("1 of 1");
  });
});

// ---- ⌘F, ⌘G and the menu ----
describe("bindFind", () => {
  type Handler = () => void;
  let handlers: Record<string, Handler[]>;
  let find: { open: ReturnType<typeof vi.fn>; next: ReturnType<typeof vi.fn>; prev: ReturnType<typeof vi.fn>; isOpen: boolean };
  let focused: boolean;
  let off: () => void;
  const api = {
    on(event: keyof FindEvents, fn: Handler) {
      (handlers[event] ??= []).push(fn);
      return () => { handlers[event] = handlers[event].filter((h) => h !== fn); };
    },
  } as unknown as Parameters<typeof bindFind>[1];
  const emit = (event: string) => (handlers[event] ?? []).forEach((h) => h());

  beforeEach(() => {
    handlers = {};
    focused = true;
    find = { open: vi.fn(), next: vi.fn(), prev: vi.fn(), isOpen: false };
    off = bindFind(document, api, find as unknown as Find, { hasFocus: () => focused, seed: () => "sel" });
  });
  afterEach(() => off());

  it("⌘F and Ctrl+F open the bar with the selection", () => {
    expect(key(document, "f", { metaKey: true }).defaultPrevented).toBe(true);
    key(document, "F", { ctrlKey: true });
    expect(find.open).toHaveBeenCalledTimes(2);
    expect(find.open).toHaveBeenCalledWith("sel");
  });

  it("⌘G is next, ⇧⌘G is previous", () => {
    key(document, "g", { metaKey: true });
    key(document, "G", { metaKey: true, shiftKey: true });
    expect(find.next).toHaveBeenCalledTimes(1);
    expect(find.prev).toHaveBeenCalledTimes(1);
  });

  it("ignores other modifiers", () => {
    key(document, "f", { metaKey: true, altKey: true });
    key(document, "f", { metaKey: true, shiftKey: true });
    key(document, "f");
    expect(find.open).not.toHaveBeenCalled();
  });

  it("the menu events do the same, in the window that has the focus only", () => {
    emit("menu:find");
    emit("menu:find-next");
    emit("menu:find-previous");
    expect(find.open).toHaveBeenCalledTimes(1);
    expect(find.next).toHaveBeenCalledTimes(1);
    expect(find.prev).toHaveBeenCalledTimes(1);
    focused = false;
    emit("menu:find"); emit("menu:find-next"); emit("menu:find-previous");
    expect(find.open).toHaveBeenCalledTimes(1);
    expect(find.next).toHaveBeenCalledTimes(1);
  });

  it("one key press that reaches both the page and the menu steps only once", () => {
    key(document, "g", { metaKey: true });
    emit("menu:find-next");
    expect(find.next).toHaveBeenCalledTimes(1);
  });

  it("stops listening when removed", () => {
    off();
    key(document, "f", { metaKey: true });
    emit("menu:find");
    expect(find.open).not.toHaveBeenCalled();
  });
});

// ---- ui/top-bar-actions: the Find button's toggle and state signal
describe("toggle and onChange", () => {
  const mk = () => {
    const host = document.createElement("div");
    document.body.append(host);
    return createFind({ doc: document, host, finder: textareaFinder(document.createElement("textarea")) });
  };
  it("toggle opens the bar, and closes it when open", () => {
    const f = mk();
    f.toggle();
    expect(f.isOpen).toBe(true);
    f.toggle();
    expect(f.isOpen).toBe(false);
    f.destroy();
  });
  it("onChange tells every open and close, and only real changes, until it is removed", () => {
    const f = mk();
    const seen: boolean[] = [];
    const off = f.onChange((open) => seen.push(open));
    f.open();
    f.open(); // already open
    f.close();
    f.close(); // already closed
    expect(seen).toEqual([true, false]);
    off();
    f.open();
    expect(seen).toEqual([true, false]);
    f.destroy();
  });
});
