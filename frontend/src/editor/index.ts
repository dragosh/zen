// ui/editor: markdown source with a formatting toolbar and a problems console.
// Mounted only in an editor window (ui/editor-window): one page per window,
// Done closes the window. There is no preview here: the main window follows
// the unsaved text live (api.draft -> page:draft).
import type { Alert } from "../api";
import { backdrop, renderConsole } from "../lint";
import { button, el } from "../shell/dom";
import type { Feature } from "../shell/types";
import { commands, continueList, indent, outdent, type CommandName, type Edit } from "./commands";
import { createToolbar, TOOLS } from "./toolbar";

export { commands, continueList } from "./commands";
export type { Edit } from "./commands";

/** Lint the unsaved text this long after typing stops (ui/lint). */
export const LINT_DELAY = 600;
/** Share the unsaved text with the main window this long after typing stops. */
export const DRAFT_DELAY = 150;

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
    // Actions: unsaved dot, Done.
    const dot = el("span", "zen-editor-dot", "●");
    dot.setAttribute("aria-label", "Unsaved changes");
    dot.title = "Unsaved changes";
    dot.hidden = true;
    const doneBtn = button("Done", () => void done(), { className: "primary", title: "Done (Esc)" });
    slots.actions.append(dot, doneBtn);

    // Main: toolbar, bar, source, problems console.
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
    // ui/lint: squiggles on a mirror behind the textarea, problems console below.
    const lintView = backdrop(ta);
    const problems = renderConsole({
      onSelect: (_a: Alert, i: number) => lintView.select(i),
      onLink: (url) => void api.openExternal(url).catch(() => undefined),
    });
    panes.append(lintView.el);
    root.append(toolbar, bar, panes, problems.el);
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
    // The main window follows the unsaved text (ui/editor-window).
    let draftTimer: ReturnType<typeof setTimeout> | undefined;
    let draftShared = false;
    function shareDraft(): void {
      clearTimeout(draftTimer);
      draftTimer = setTimeout(() => {
        if (ta.value === saved) dropDraft();
        else {
          draftShared = true;
          api.draft(current, ta.value);
        }
      }, DRAFT_DELAY);
    }
    /** The main window goes back to the file on disk. */
    function dropDraft(): void {
      clearTimeout(draftTimer);
      if (!draftShared) return;
      draftShared = false;
      api.draft(current, null);
    }

    let lintSeq = 0;
    let lintTimer: ReturnType<typeof setTimeout> | undefined;
    function showLint(alerts: Alert[] | null, forText: string): void {
      lintView.update(alerts ?? [], forText);
      problems.update(alerts);
    }
    /** Lints the current text now; only the latest request's answer is drawn. */
    async function lintNow(): Promise<void> {
      clearTimeout(lintTimer);
      const mine = ++lintSeq;
      const path = current;
      const text = ta.value;
      if (!text) {
        showLint(null, text);
        return;
      }
      try {
        const r = await api.lint(path, text);
        if (mine !== lintSeq || path !== current) return;
        showLint(r.available ? r.alerts : null, text);
      } catch (err) {
        if (mine !== lintSeq) return;
        showLint(null, ta.value);
        console.warn(`zen: could not lint ${path}`, err);
      }
    }
    function scheduleLint(): void {
      clearTimeout(lintTimer);
      lintTimer = setTimeout(() => void lintNow(), LINT_DELAY);
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
        dropDraft();
        lintView.refresh();
        void lintNow();
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
          if (ta.value === saved) dropDraft();
          void lintNow();
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
        dropDraft();
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
      shareDraft();
      scheduleLint();
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

    // Global shortcuts while the editor is mounted.
    const onKey = (e: KeyboardEvent) => {
      if (dialog || e.defaultPrevented) return;
      const mod = e.metaKey || e.ctrlKey;
      const k = e.key.toLowerCase();
      if (mod && !e.shiftKey && k === "s") {
        e.preventDefault();
        void save();
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

    void load(current).then(() => ta.focus());

    return () => {
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("beforeunload", onBeforeUnload);
      window.removeEventListener("zen:before-close", onBeforeClose);
      offChanged();
      dialog?.close();
      dropDraft();
      loadSeq++;
      lintSeq++;
      clearTimeout(lintTimer);
      problems.destroy();
      lintView.destroy();
      root.remove();
      dot.remove();
      doneBtn.remove();
      if (dirty) store.set({ dirty: false });
    };
  },
};
