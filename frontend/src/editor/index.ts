// ui/editor: markdown source with a formatting toolbar and a live preview.
// Mounted only in an editor window (ui/editor-window): one page per window,
// Done closes the window.
import { button, el } from "../shell/dom";
import type { Feature } from "../shell/types";
import { commands, continueList, indent, outdent, type CommandName, type Edit } from "./commands";
import { createPreview } from "./preview";
import { createToolbar, TOOLS } from "./toolbar";

export { commands, continueList } from "./commands";
export type { Edit } from "./commands";

const SPLIT_KEY = "zen.editor.split";
const RATIO_KEY = "zen.editor.ratio";

function readStore(key: string): string | null {
  try { return localStorage.getItem(key); } catch { return null; }
}
function writeStore(key: string, value: string): void {
  try { localStorage.setItem(key, value); } catch { /* storage unavailable */ }
}
const clampRatio = (n: number) => Math.min(80, Math.max(20, Math.round(n)));

/**
 * Applies `next` to the textarea as one edit through execCommand("insertText"),
 * so native undo/redo keeps working; falls back to setRangeText where the
 * engine has no editing commands (tests).
 */
export function applyEdit(ta: HTMLTextAreaElement, next: Edit): void {
  const old = ta.value;
  const now = next.text;
  let p = 0;
  while (p < old.length && p < now.length && old[p] === now[p]) p++;
  let q = 0;
  while (q < old.length - p && q < now.length - p && old[old.length - 1 - q] === now[now.length - 1 - q]) q++;
  const end = old.length - q;
  const insert = now.slice(p, now.length - q);
  if (p !== end || insert) {
    ta.focus();
    ta.setSelectionRange(p, end);
    let ok = false;
    try {
      ok = insert ? document.execCommand("insertText", false, insert) : document.execCommand("delete");
    } catch { ok = false; }
    if (!ok || ta.value !== now) {
      ta.value = now;
      ta.dispatchEvent(new Event("input", { bubbles: true }));
    }
  }
  ta.setSelectionRange(next.selStart, next.selEnd);
}

type Answer = "save" | "discard" | "cancel";
let dialogSeq = 0;

/** The unsaved-changes dialog: Save / Discard / Cancel. Esc cancels. */
function askUnsaved(): { result: Promise<Answer>; close(): void } {
  const back = el("div", "zen-editor-backdrop");
  const box = el("div", "zen-editor-dialog");
  const id = `zen-editor-dialog-${++dialogSeq}`;
  box.setAttribute("role", "dialog");
  box.setAttribute("aria-modal", "true");
  box.setAttribute("aria-labelledby", `${id}-title`);
  box.setAttribute("aria-describedby", `${id}-text`);
  const title = el("h2", "", "Unsaved changes");
  title.id = `${id}-title`;
  const text = el("p", "", "Save your changes before leaving?");
  text.id = `${id}-text`;
  const row = el("div", "zen-editor-dialog-buttons");
  const previous = document.activeElement as HTMLElement | null;
  let settle!: (a: Answer) => void;
  const result = new Promise<Answer>((r) => (settle = r));
  const finish = (a: Answer) => {
    back.remove();
    previous?.focus?.();
    settle(a);
  };
  const save = button("Save", () => finish("save"), { className: "primary" });
  const discard = button("Discard", () => finish("discard"));
  const cancel = button("Cancel", () => finish("cancel"));
  row.append(cancel, discard, save);
  box.append(title, text, row);
  back.append(box);
  const order = [cancel, discard, save];
  box.addEventListener("keydown", (e) => {
    e.stopPropagation();
    if (e.key === "Escape") {
      e.preventDefault();
      finish("cancel");
    } else if (e.key === "Tab") {
      e.preventDefault();
      const i = order.indexOf(document.activeElement as HTMLButtonElement);
      order[(i + (e.shiftKey ? order.length - 1 : 1)) % order.length].focus();
    }
  });
  document.body.append(back);
  save.focus();
  return { result, close: () => finish("cancel") };
}

export const editorFeature: Feature = {
  id: "editor",
  mount(slots, ctx) {
    const { api, store } = ctx;
    let current = store.get().path;
    let saved = "";
    let dirty = false;
    let loadSeq = 0;
    let dialog: ReturnType<typeof askUnsaved> | null = null;
    let leaving: Promise<boolean> | null = null;
    let split = readStore(SPLIT_KEY) !== "0";
    let ratio = clampRatio(Number(readStore(RATIO_KEY) ?? 50) || 50);

    // Actions: unsaved dot, Split, Done.
    const dot = el("span", "zen-editor-dot", "●");
    dot.setAttribute("aria-label", "Unsaved changes");
    dot.title = "Unsaved changes";
    dot.hidden = true;
    const splitBtn = button("Split", () => toggleSplit(), { className: "ghost", title: "Split (⌘⇧P)" });
    const doneBtn = button("Done", () => void done(), { className: "primary", title: "Done (Esc)" });
    slots.actions.append(dot, splitBtn, doneBtn);

    // Main: toolbar, bar, panes.
    const root = el("div", "zen-editor");
    const toolbar = createToolbar((id) => run(id));
    const bar = el("div", "zen-editor-bar");
    bar.setAttribute("role", "alert");
    bar.hidden = true;
    const panes = el("div", "zen-editor-panes");
    const ta = el("textarea", "zen-editor-source");
    ta.spellcheck = true;
    ta.setAttribute("aria-label", "Markdown source");
    ta.wrap = "soft";
    const divider = el("div", "zen-editor-divider");
    divider.setAttribute("role", "separator");
    divider.setAttribute("aria-orientation", "vertical");
    divider.setAttribute("aria-label", "Resize preview");
    divider.setAttribute("aria-valuemin", "20");
    divider.setAttribute("aria-valuemax", "80");
    divider.tabIndex = 0;
    const preview = createPreview(api);
    panes.append(ta, divider, preview.el);
    root.append(toolbar, bar, panes);
    slots.main.append(root);

    function setDirty(v: boolean): void {
      dirty = v;
      dot.hidden = !v;
      store.set({ dirty: v });
    }
    function showBar(message: string, actions: HTMLButtonElement[] = []): void {
      bar.replaceChildren(el("span", "zen-editor-bar-text", message), ...actions);
      bar.hidden = false;
    }
    function hideBar(): void {
      bar.hidden = true;
      bar.replaceChildren();
    }
    function renderSplit(): void {
      root.classList.toggle("split", split);
      splitBtn.setAttribute("aria-pressed", String(split));
      preview.el.hidden = !split;
      divider.hidden = !split;
      ta.style.flexBasis = split ? `${ratio}%` : "";
      divider.setAttribute("aria-valuenow", String(ratio));
      if (split) preview.update(current, ta.value);
    }
    function toggleSplit(): void {
      split = !split;
      writeStore(SPLIT_KEY, split ? "1" : "0");
      renderSplit();
    }
    function setRatio(n: number): void {
      ratio = clampRatio(n);
      writeStore(RATIO_KEY, String(ratio));
      renderSplit();
    }

    const selection = (): Edit => ({ text: ta.value, selStart: ta.selectionStart, selEnd: ta.selectionEnd });
    function run(id: CommandName): void {
      applyEdit(ta, commands[id](selection()));
    }

    async function load(path: string, keepCaret = false): Promise<void> {
      current = path;
      const mine = ++loadSeq;
      try {
        const text = await api.source(path);
        if (mine !== loadSeq) return;
        if (ta.value !== text) {
          const [s, e] = [ta.selectionStart, ta.selectionEnd];
          ta.value = text;
          if (keepCaret) ta.setSelectionRange(Math.min(s, text.length), Math.min(e, text.length));
          else ta.setSelectionRange(0, 0);
        }
        saved = text;
        setDirty(false);
        hideBar();
        if (split) preview.update(path, text);
      } catch (err) {
        if (mine === loadSeq) showBar(`Could not open ${path}: ${(err as Error).message ?? err}`);
      }
    }

    async function save(): Promise<boolean> {
      const text = ta.value;
      const path = current;
      try {
        await api.save(path, text);
        if (path === current) {
          saved = text;
          setDirty(ta.value !== saved);
          hideBar();
        }
        return true;
      } catch (err) {
        showBar(`Could not save: ${(err as Error).message ?? err}`);
        return false;
      }
    }

    /**
     * Resolves true when it is fine to leave the current text. Asking twice
     * (Done, then the window's close button) shares one dialog.
     */
    function confirmLeave(): Promise<boolean> {
      if (!dirty) return Promise.resolve(true);
      if (leaving) return leaving;
      dialog = askUnsaved();
      leaving = dialog.result.then(async (answer) => {
        dialog = null;
        if (answer === "cancel") return false;
        if (answer === "save") return save();
        setDirty(false);
        return true;
      }).finally(() => { leaving = null; });
      return leaving;
    }

    /** Done (or Esc): close the editor window, asking first when dirty. */
    async function done(): Promise<void> {
      if (await confirmLeave()) {
        setDirty(false);
        window.dispatchEvent(new CustomEvent("zen:close-window"));
      }
    }

    // Typing.
    ta.addEventListener("input", () => {
      setDirty(ta.value !== saved);
      if (split) preview.update(current, ta.value);
    });
    ta.addEventListener("scroll", () => {
      const max = ta.scrollHeight - ta.clientHeight;
      if (split) preview.follow(max > 0 ? ta.scrollTop / max : 0);
    });
    ta.addEventListener("keydown", (e) => {
      const mod = e.metaKey || e.ctrlKey;
      if (mod && !e.shiftKey && !e.altKey) {
        const tool = TOOLS.find((t) => t.key === e.key.toLowerCase());
        if (tool) {
          e.preventDefault();
          run(tool.id);
        }
        return;
      }
      if (e.key === "Tab" && !mod && !e.altKey) {
        e.preventDefault();
        applyEdit(ta, (e.shiftKey ? outdent : indent)(selection()));
      } else if (e.key === "Enter" && !mod && !e.shiftKey && !e.altKey && !e.isComposing) {
        const next = continueList(selection());
        if (next) {
          e.preventDefault();
          applyEdit(ta, next);
        }
      }
    });

    // Divider: drag or arrow keys.
    divider.addEventListener("keydown", (e) => {
      const step = { ArrowLeft: -5, ArrowRight: 5 }[e.key];
      if (step !== undefined) {
        e.preventDefault();
        setRatio(ratio + step);
      } else if (e.key === "Home" || e.key === "End") {
        e.preventDefault();
        setRatio(e.key === "Home" ? 20 : 80);
      }
    });
    divider.addEventListener("pointerdown", (e) => {
      e.preventDefault();
      const rect = panes.getBoundingClientRect();
      if (rect.width <= 0) return;
      const move = (m: PointerEvent) => setRatio(((m.clientX - rect.left) / rect.width) * 100);
      const up = () => {
        window.removeEventListener("pointermove", move);
        window.removeEventListener("pointerup", up);
        root.classList.remove("is-resizing");
      };
      root.classList.add("is-resizing");
      window.addEventListener("pointermove", move);
      window.addEventListener("pointerup", up);
    });

    // Global shortcuts while the editor is mounted.
    const onKey = (e: KeyboardEvent) => {
      if (dialog || e.defaultPrevented) return;
      const mod = e.metaKey || e.ctrlKey;
      const k = e.key.toLowerCase();
      if (mod && !e.shiftKey && k === "s") {
        e.preventDefault();
        void save();
      } else if (mod && e.shiftKey && k === "p") {
        e.preventDefault();
        toggleSplit();
      } else if (e.key === "Escape" && !mod) {
        const active = document.activeElement;
        if (!active || active === document.body || toolbar.contains(active)) {
          e.preventDefault();
          void done();
        }
      }
    };
    document.addEventListener("keydown", onKey);

    // Closing the window.
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (!dirty) return;
      e.preventDefault();
      e.returnValue = "";
      void confirmLeave().then((ok) => { if (ok) window.close(); });
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    // Closing the native window (app/services): api/wails.ts dispatches this.
    const onBeforeClose = (e: Event) => {
      if (!dirty) return;
      e.preventDefault();
      const detail = (e as CustomEvent<{ close?: () => void; cancel?: () => void }>).detail;
      void confirmLeave().then((ok) => (ok ? detail?.close?.() : detail?.cancel?.()));
    };
    window.addEventListener("zen:before-close", onBeforeClose);

    // The file changed on disk.
    const offChanged = api.on("page:changed", (ev) => {
      if (ev.path !== current) return;
      if (!dirty) {
        void load(current, true);
        return;
      }
      showBar("This page changed on disk.", [
        button("Reload from disk", () => void load(current, true)),
        button("Keep mine", hideBar, { className: "ghost" }),
      ]);
    });

    renderSplit();
    void load(current).then(() => ta.focus());

    return () => {
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("beforeunload", onBeforeUnload);
      window.removeEventListener("zen:before-close", onBeforeClose);
      offChanged();
      dialog?.close();
      preview.destroy();
      loadSeq++;
      root.remove();
      dot.remove();
      splitBtn.remove();
      doneBtn.remove();
      if (dirty) store.set({ dirty: false });
    };
  },
};
