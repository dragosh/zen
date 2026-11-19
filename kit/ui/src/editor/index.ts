// ui/editor: markdown source with a formatting toolbar and a problems console.
// Mounted only in an editor window (ui/editor-window): one page per window,
// Done closes the window. The main window follows the unsaved text live
// (api.draft -> page:draft); Split adds a preview pane next to the source.
import { hasAddon, type Alert, type Page } from "../host";
import { createInspector, inspectorKey, type Inspector } from "../inspector";
import { bindFind, createFind, textareaFinder } from "../find";
import { backdrop, renderConsole } from "../lint";
import { button, el } from "../shell/dom";
import type { Feature } from "../shell/types";
import { commands, continueList, indent, outdent, type CommandName, type Edit } from "./commands";
import { highlight, highlightYaml } from "./highlight";
import { createPreview, type Preview } from "./preview";
import { createToolbar, TOOLS } from "./toolbar";

export { commands, continueList } from "./commands";
export type { Edit } from "./commands";

/** Lint the unsaved text this long after typing stops (ui/lint). */
export const LINT_DELAY = 600;
/** Count the unsaved text for the Inspector this long after typing stops (ui/inspector). */
export const STATS_DELAY = 500;
/** Share the unsaved text with the main window this long after typing stops. */
export const DRAFT_DELAY = 150;

/** Below this window width the preview goes under the source instead of beside it. */
export const STACK_BELOW = 900;
export const SPLIT_MIN = 20;
export const SPLIT_MAX = 80;
export const SPLIT_DEFAULT = 50;
const SPLIT_STEP = 5;
// Remembered per window kind (this is the editor window's).
const SPLIT_KEY = "zen.editor.split";
const SPLIT_POS_KEY = "zen.editor.split.pos";

const clampPos = (n: number) => Math.min(SPLIT_MAX, Math.max(SPLIT_MIN, Math.round(n)));
function readStore(key: string): string | null {
  try { return localStorage.getItem(key); } catch { return null; }
}
function writeStore(key: string, value: string): void {
  try { localStorage.setItem(key, value); } catch { /* not remembered */ }
}

/** The settings files open in the editor as "pages" zen:config/global and zen:config/workspace. */
export const CONFIG_PREFIX = "zen:config/";
export const isConfigPath = (p: string) => p.startsWith(CONFIG_PREFIX);

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
    const config = isConfigPath(current);
    const splitBtn = button("Split", () => setSplit(!split), { title: "Split view (⌘⇧P)" });
    splitBtn.setAttribute("aria-pressed", "false");
    // ui/inspector: issues, file and page statistics of the unsaved text. Not for settings files.
    let inspector: Inspector | null = null;
    if (!config) {
      inspector = createInspector({
        doc: document, host: api,
        onSelectIssue: (_a: Alert, i: number) => lintView.select(i),
        onOpenChange: (open) => { if (open) refreshStats(true); else statsSeq++; },
      });
    }
    slots.actions.append(dot, ...(inspector ? [inspector.button, splitBtn] : []), doneBtn); // no preview or Inspector for settings files

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
    // ui/lint: squiggles on a mirror behind the textarea, problems console below; both only in
    // builds with the vale add-on (kit/addons). Settings files are YAML: no markdown toolbar, no Vale, no preview.
    const lintOn = hasAddon(api, "vale");
    if (config) {
      toolbar.hidden = true;
      ta.spellcheck = false;
      ta.setAttribute("aria-label", "Settings (YAML)");
      root.classList.add("is-config");
    }
    // The mirror also draws syntax highlighting and line numbers.
    const lintView = backdrop(ta, { highlight: config ? highlightYaml : highlight, lineNumbers: true });
    const problems = renderConsole({
      onSelect: (_a: Alert, i: number) => lintView.select(i),
      onLink: (url) => void api.openExternal(url).catch(() => undefined),
    });
    panes.append(lintView.el);
    // Inspector statistics. The page it knows until the first render is an empty one, so the button shows at once.
    let statsSeq = 0;
    let statsTimer: ReturnType<typeof setTimeout> | undefined;
    let statsPage: { page: Page; root: ParentNode } | null = null;
    let lastPreview: { page: Page; article: HTMLElement } | null = null;
    const emptyPage = (path: string): Page => ({
      path, title: path.split("/").pop() ?? path, html: "", frontmatter: {}, breadcrumb: [],
      assets: { css: [], js: [] }, warnings: [], layout: { name: "article", source: "" },
    });
    function showStats(page: Page, root: ParentNode): void {
      statsPage = { page, root };
      inspector?.setPage(page.path || current, page, root);
    }
    /** The Inspector's File and Page for the current text. Nothing is rendered while it is closed. */
    function refreshStats(now = false): void {
      if (!inspector) return;
      clearTimeout(statsTimer);
      statsSeq++;
      if (!inspector.isOpen) return;
      if (split && preview) {
        // The preview renders anyway; its article is counted (onRender below).
        if (lastPreview) showStats(lastPreview.page, lastPreview.article);
        return;
      }
      const run = () => {
        const mine = ++statsSeq;
        const path = current;
        api.preview(path, ta.value).then((page) => {
          if (mine !== statsSeq || !inspector?.isOpen) return;
          // Parsed apart from the document: no scripts run, no assets load, nothing shows.
          showStats({ ...page, path }, new DOMParser().parseFromString(page.html, "text/html").body);
        }, () => { /* keep the last statistics */ });
      };
      if (now) run();
      else statsTimer = setTimeout(run, STATS_DELAY);
    }
    // Split: a preview pane beside (or, in a narrow window, below) the source, with a divider.
    let split = false;
    let pos = clampPos(Number.parseFloat(readStore(SPLIT_POS_KEY) ?? "") || SPLIT_DEFAULT);
    let preview: Preview | null = null;
    let divider: HTMLElement | null = null;
    const stacked = () => window.innerWidth < STACK_BELOW;
    function applyPos(save: boolean): void {
      panes.style.setProperty("--zen-split", `${pos}%`);
      divider?.setAttribute("aria-valuenow", String(pos));
      if (save) writeStore(SPLIT_POS_KEY, String(pos));
    }
    function applyOrientation(): void {
      const s = stacked();
      panes.classList.toggle("is-stacked", s);
      divider?.setAttribute("aria-orientation", s ? "horizontal" : "vertical");
    }
    function followSource(): void {
      const max = ta.scrollHeight - ta.clientHeight;
      preview?.follow(max > 0 ? ta.scrollTop / max : 0);
    }
    function renderPreview(): void {
      if (split && preview) preview.update(current, ta.value);
    }
    function setSplit(on: boolean): void {
      if (config || on === split) return;
      split = on;
      splitBtn.setAttribute("aria-pressed", String(on));
      writeStore(SPLIT_KEY, on ? "1" : "0");
      panes.classList.toggle("is-split", on);
      if (on) {
        preview = createPreview(api, undefined, (page, article) => {
          lastPreview = { page, article };
          if (inspector?.isOpen) showStats(page, article);
        });
        divider = makeDivider();
        panes.append(divider, preview.el);
        applyPos(false);
        applyOrientation();
        renderPreview();
        followSource();
      } else {
        preview?.destroy();
        preview = null;
        lastPreview = null;
        refreshStats(true);
        divider?.dispatchEvent(new Event("zen:dispose"));
        divider?.remove();
        divider = null;
      }
    }
    function makeDivider(): HTMLElement {
      const d = el("div", "zen-editor-divider");
      d.setAttribute("role", "separator");
      d.setAttribute("aria-label", "Resize preview");
      d.setAttribute("aria-valuemin", String(SPLIT_MIN));
      d.setAttribute("aria-valuemax", String(SPLIT_MAX));
      d.title = "Drag to resize";
      d.tabIndex = 0;
      let dragging = false;
      const onMove = (e: MouseEvent) => {
        if (!dragging) return;
        const r = panes.getBoundingClientRect();
        pos = clampPos(stacked() ? ((e.clientY - r.top) / r.height) * 100 : ((e.clientX - r.left) / r.width) * 100);
        applyPos(false);
      };
      const onUp = () => {
        if (!dragging) return;
        dragging = false;
        panes.classList.remove("is-resizing");
        applyPos(true);
      };
      d.addEventListener("mousedown", (e) => {
        if (e.button !== 0) return;
        e.preventDefault();
        dragging = true;
        panes.classList.add("is-resizing");
      });
      d.addEventListener("keydown", (e) => {
        const to = { ArrowLeft: pos - SPLIT_STEP, ArrowUp: pos - SPLIT_STEP, ArrowRight: pos + SPLIT_STEP, ArrowDown: pos + SPLIT_STEP, Home: SPLIT_MIN, End: SPLIT_MAX }[e.key];
        if (to === undefined) return;
        e.preventDefault();
        pos = clampPos(to);
        applyPos(true);
      });
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
      d.addEventListener("zen:dispose", () => {
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      });
      return d;
    }
    const onWindowResize = () => applyOrientation();
    window.addEventListener("resize", onWindowResize);
    ta.addEventListener("scroll", followSource);
    root.append(toolbar, bar, panes);
    if (lintOn) root.append(problems.el); // no Problems console without the vale add-on
    slots.main.append(root);
    // Beside the main pane, before it: inspector.css makes room for it.
    if (inspector) slots.main.before(inspector.el);
    // The textarea is in the page only now: copy its typography to the
    // highlight/lint layer (also again next frame, after layout settles).
    lintView.sync();
    requestAnimationFrame(() => lintView.sync());

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
      if (config) return;
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
    let linted = false;
    function showLint(alerts: Alert[] | null, forText: string): void {
      linted = true;
      lintView.update(alerts ?? [], forText);
      problems.update(alerts);
      inspector?.setIssues(alerts);
    }
    /** Lints the current text now; only the latest request's answer is drawn. */
    async function lintNow(): Promise<void> {
      clearTimeout(lintTimer);
      if (config || !lintOn) return;
      const mine = ++lintSeq;
      const path = current;
      const text = ta.value;
      if (!text) {
        showLint(null, text);
        return;
      }
      // Until the first answer; later the last list stays, so it does not flash while typing.
      if (!linted) inspector?.setIssues("checking");
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
        renderPreview();
        showStats(statsPage?.page.path === path ? statsPage.page : emptyPage(path), statsPage?.root ?? document.createElement("article"));
        refreshStats(true);
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
          if (statsPage) showStats(statsPage.page, statsPage.root); // the file's size and dates changed
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
      renderPreview();
      refreshStats();
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

    // Find in the markdown source (ui/find): ⌘F and Edit › Find select the match in the textarea.
    const find = createFind({ doc: document, host: document.body, finder: textareaFinder(ta), label: "Find in source" });
    const offFind = bindFind(document, api, find);
    ta.addEventListener("input", () => find.refresh());

    // Global shortcuts while the editor is mounted.
    const onKey = (e: KeyboardEvent) => {
      if (dialog || e.defaultPrevented) return;
      const mod = e.metaKey || e.ctrlKey;
      const k = e.key.toLowerCase();
      if (inspector && inspectorKey(e)) {
        e.preventDefault();
        inspector.toggle();
      } else if (mod && e.shiftKey && !e.altKey && k === "p" && !config) {
        e.preventDefault();
        setSplit(!split);
      } else if (mod && !e.shiftKey && k === "s") {
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

    if (!config && readStore(SPLIT_KEY) === "1") setSplit(true);
    void load(current).then(() => ta.focus());

    return () => {
      document.removeEventListener("keydown", onKey);
      offFind();
      find.destroy();
      window.removeEventListener("beforeunload", onBeforeUnload);
      window.removeEventListener("zen:before-close", onBeforeClose);
      window.removeEventListener("resize", onWindowResize);
      ta.removeEventListener("scroll", followSource);
      divider?.dispatchEvent(new Event("zen:dispose"));
      preview?.destroy();
      splitBtn.remove();
      statsSeq++;
      clearTimeout(statsTimer);
      inspector?.destroy();
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
