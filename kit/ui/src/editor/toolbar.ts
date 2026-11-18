// The formatting toolbar: grayscale text glyphs, roving tabindex, arrow keys.
import { el } from "../shell/dom";
import type { CommandName } from "./commands";

/** `key` is the ⌘ shortcut handled by the editor (lower-case letter). */
export const TOOLS: { id: CommandName; label: string; glyph: string; key?: string }[] = [
  { id: "bold", label: "Bold", glyph: "B", key: "b" },
  { id: "italic", label: "Italic", glyph: "I", key: "i" },
  { id: "strike", label: "Strikethrough", glyph: "S" },
  { id: "h1", label: "Heading 1", glyph: "H1" },
  { id: "h2", label: "Heading 2", glyph: "H2" },
  { id: "quote", label: "Quote", glyph: "“ ”" },
  { id: "bullet", label: "Bullet list", glyph: "•" },
  { id: "numbered", label: "Numbered list", glyph: "1." },
  { id: "task", label: "Task", glyph: "☐" },
  { id: "code", label: "Code", glyph: "</>" },
  { id: "link", label: "Link", glyph: "🔗︎", key: "k" },
  { id: "table", label: "Table", glyph: "▦" },
];

export function createToolbar(run: (id: CommandName) => void): HTMLElement {
  const bar = el("div", "zen-editor-toolbar");
  bar.setAttribute("role", "toolbar");
  bar.setAttribute("aria-label", "Formatting");
  const buttons = TOOLS.map((t, i) => {
    const b = el("button", `ghost zen-tool zen-tool-${t.id}`, t.glyph);
    b.type = "button";
    b.dataset.command = t.id;
    b.setAttribute("aria-label", t.label);
    b.title = t.key ? `${t.label} (⌘${t.key.toUpperCase()})` : t.label;
    b.tabIndex = i === 0 ? 0 : -1;
    // Keep the textarea's selection: do not take focus on mouse down.
    b.addEventListener("mousedown", (e) => e.preventDefault());
    b.addEventListener("click", () => run(t.id));
    return b;
  });
  bar.append(...buttons);

  const focusAt = (i: number) => {
    const n = buttons.length;
    const next = buttons[((i % n) + n) % n];
    for (const b of buttons) b.tabIndex = b === next ? 0 : -1;
    next.focus();
  };
  bar.addEventListener("keydown", (e) => {
    const i = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (i < 0) return;
    const to = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: buttons.length - 1 }[e.key];
    if (to === undefined) return;
    e.preventDefault();
    focusAt(to);
  });
  return bar;
}
