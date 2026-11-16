// The reader: loads Page for store.path, mounts its HTML and wires links,
// anchors, plugin assets, live reload, warnings, errors and the Edit action.
import type { Alert, Page } from "../api";
import { flash, markText, renderPanel } from "../lint";
import { installRuntime } from "../plugins/runtime";
import { button, el } from "../shell/dom";
import type { Ctx, Feature } from "../shell/types";
import { loadAssets } from "./assets";
import { handleLinkClick } from "./links";

export { handleLinkClick };

/** Test seam: replaces import() for plugin scripts (happy-dom cannot fetch modules). */
export const pageHooks: { importModule?: (url: string) => Promise<unknown> } = {};

const PROGRESS_DELAY = 150;

/** The single innerHTML in the app: Page.html is sanitised in Go. */
export function mountPage(el: HTMLElement, page: Page): void {
  el.innerHTML = page.html;
  setLayout(el, page.layout);
  loadAssets(el.ownerDocument, page.assets);
  el.ownerDocument.dispatchEvent(new CustomEvent("zen:page-rendered", { detail: { root: el } }));
}

const LAYOUT_CLASS = "zen-layout-";

/** A layout page (ui/plugins) is marked so page.css hands the whole pane to the plugin. */
function setLayout(el: HTMLElement, layout: string): void {
  for (const c of Array.from(el.classList)) if (c.startsWith(LAYOUT_CLASS)) el.classList.remove(c);
  if (/^[a-z][a-z0-9-]*$/.test(layout ?? "")) {
    el.dataset.layout = layout;
    el.classList.add(LAYOUT_CLASS + layout);
  } else {
    delete el.dataset.layout;
  }
}

const message = (err: unknown) => (err instanceof Error ? err.message : String(err));

function reducedMotion(): boolean {
  try {
    return window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
  } catch {
    return false;
  }
}

function findAnchor(root: HTMLElement, id: string): HTMLElement | null {
  return Array.from(root.querySelectorAll<HTMLElement>("[id]")).find((n) => n.id === id) ?? null;
}

export const pageFeature: Feature = {
  id: "page",
  mount(slots, ctx: Ctx) {
    const doc = slots.main.ownerDocument;
    const main = slots.main;

    const progress = el("div", "zen-progress");
    progress.hidden = true;
    progress.setAttribute("role", "progressbar");
    progress.setAttribute("aria-label", "Loading");

    const warnings = el("div", "zen-page-warnings");
    warnings.hidden = true;
    warnings.setAttribute("role", "status");
    const warningText = el("div", "zen-page-warnings-text");
    warnings.append(warningText, button("Dismiss warnings", () => { warnings.hidden = true; }, { className: "ghost", text: "×" }));

    const error = el("div", "zen-page-error");
    error.hidden = true;
    error.setAttribute("role", "alert");
    const errorText = el("p", "zen-page-error-message");
    error.append(errorText, button("Retry", () => void load(false), { className: "primary" }));

    const article = el("article", "zen-page");
    main.append(progress, warnings, error, article);

    /** Edit opens the page in its own editor window (ui/editor-window). */
    const openEditor = () => {
      const { path } = ctx.store.get();
      if (!path) return;
      ctx.api.openEditor(path).catch((err) => console.error(`zen: could not edit ${path}`, err));
    };
    const edit = button("Edit", openEditor, { title: "Edit (⌘E)" });

    // Vale alerts (ui/lint): marks in the article, a count and panel in the actions.
    let lintMarks: (HTMLElement | null)[] = [];
    const lintPanel = renderPanel((_alert: Alert, i: number) => {
      const m = lintMarks[i];
      if (!m?.isConnected) return;
      m.scrollIntoView({ behavior: reducedMotion() ? "auto" : "smooth", block: "center" });
      flash(m);
    });
    slots.actions.append(lintPanel.el, edit);

    // External plugins (ui/plugins): their handlers mount on the wrappers of
    // every render and are cleaned up before the next one replaces the article.
    let rendered = { path: "", title: "" };
    const anchorListeners = new Set<(anchor: string) => void>();
    const plugins = installRuntime({
      api: ctx.api, navigate: (path, anchor) => ctx.navigate(path, anchor), page: () => rendered,
      importModule: pageHooks.importModule,
      // The plugin set or the folder changed: the runtime dropped every script and style; render again so the
      // page's new asset URLs are loaded and mounted.
      refresh() {
        if (shownPath === null || shownPath !== ctx.store.get().path) return;
        if (draft !== null) void showDraft(shownPath, draft);
        else void load(true);
      },
      anchor: () => ctx.store.get().anchor,
      onAnchor(fn) {
        anchorListeners.add(fn);
        return () => void anchorListeners.delete(fn);
      },
      badge(path, text) {
        const badges = { ...(ctx.store.get().badges ?? {}) };
        if (text === null) delete badges[path]; else badges[path] = text;
        ctx.store.set({ badges });
      },
    });
    function render(page: Page): void {
      plugins.unmountAll();
      rendered = { path: page.path, title: page.title };
      mountPage(article, page);
      void plugins.mountAll(article, page.assets);
    }

    let seq = 0;
    let shownPath: string | null = null;
    let loadedAnchor = "";
    let timer: ReturnType<typeof setTimeout> | undefined;

    /**
     * Lints what is shown (the file on disk, or an editor's unsaved draft)
     * after it rendered; results for an older render are dropped.
     */
    async function lint(path: string, mine: number, content = ""): Promise<void> {
      try {
        const r = await ctx.api.lint(path, content);
        if (mine !== seq) return;
        lintMarks = r.available ? markText(article, r.alerts) : [];
        lintPanel.update(r.available ? r.alerts : null);
      } catch (err) {
        if (mine !== seq) return;
        lintMarks = [];
        lintPanel.update(null);
        console.warn(`zen: could not lint ${path}`, err);
      }
    }

    const stopProgress = () => {
      clearTimeout(timer);
      progress.hidden = true;
    };

    function scrollToAnchor(anchor: string, smooth: boolean): void {
      const target = anchor ? findAnchor(article, anchor) : null;
      if (target) target.scrollIntoView({ behavior: smooth && !reducedMotion() ? "smooth" : "auto", block: "start" });
      else if (!smooth) main.scrollTop = 0;
    }

    /** keepScroll: live reload of the current page; otherwise scroll to the anchor. */
    async function load(keepScroll: boolean): Promise<void> {
      const { path, anchor } = ctx.store.get();
      const mine = ++seq;
      shownPath = path;
      loadedAnchor = anchor;
      clearTimeout(timer);
      timer = setTimeout(() => { if (mine === seq) progress.hidden = false; }, PROGRESS_DELAY);
      let page: Page;
      try {
        page = await ctx.api.page(path);
      } catch (err) {
        if (mine !== seq) return;
        stopProgress();
        lintPanel.update(null);
        errorText.textContent = `Could not load ${path}: ${message(err)}`;
        error.hidden = false;
        article.hidden = true;
        return;
      }
      if (mine !== seq) return;
      stopProgress();
      error.hidden = true;
      article.hidden = false;
      const scroll = main.scrollTop;
      render(page);
      if (!keepScroll) lintPanel.update(null);
      void lint(path, mine);
      warningText.textContent = page.warnings.join("\n");
      warnings.hidden = page.warnings.length === 0;
      doc.title = `${page.title} — zen`;
      ctx.store.set({ page });
      if (keepScroll) main.scrollTop = scroll;
      else scrollToAnchor(loadedAnchor, false);
    }

    const onState = () => {
      const { path, anchor, mode } = ctx.store.get();
      if (mode !== "read" || path === "") return;
      if (path !== shownPath) {
        draft = null;
        void load(false);
      }
      else if (anchor !== loadedAnchor) {
        loadedAnchor = anchor;
        scrollToAnchor(anchor, true);
        for (const fn of Array.from(anchorListeners)) fn(anchor);
      }
    };

    const onClick = (e: MouseEvent) => handleLinkClick(e, ctx);
    const onAux = (e: MouseEvent) => { if (e.button === 1) handleLinkClick(e, ctx); };
    article.addEventListener("click", onClick);
    article.addEventListener("auxclick", onAux);

    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "e") {
        e.preventDefault();
        openEditor();
      }
      // File > Print (⌘P) prints through the menu in the app; this serves a browser.
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "p" && ctx.store.get().mode === "read") {
        e.preventDefault();
        doc.defaultView?.print();
      }
    };
    doc.addEventListener("keydown", onKey);

    const offStore = ctx.store.subscribe(onState);
    /** The editor's unsaved text for the shown page, or null (show the file). */
    let draft: string | null = null;

    /** Renders an editor's unsaved text in place, keeping the scroll position. */
    async function showDraft(path: string, content: string): Promise<void> {
      const mine = ++seq;
      let page: Page;
      try {
        page = await ctx.api.preview(path, content);
      } catch (err) {
        if (mine === seq) console.warn(`zen: could not preview ${path}`, err);
        return;
      }
      if (mine !== seq || draft === null) return;
      error.hidden = true;
      article.hidden = false;
      const scroll = main.scrollTop;
      render(page);
      void lint(path, mine, content);
      warningText.textContent = page.warnings.join("\n");
      warnings.hidden = page.warnings.length === 0;
      doc.title = `${page.title} — zen`;
      ctx.store.set({ page });
      main.scrollTop = scroll;
    }

    const offChanged = ctx.api.on("page:changed", ({ path }) => {
      if (path === shownPath && path === ctx.store.get().path && draft === null) void load(true);
    });
    const offDraft = ctx.api.on("page:draft", ({ path, content }) => {
      if (path !== shownPath || path !== ctx.store.get().path) return;
      draft = content;
      if (content === null) void load(true);
      else void showDraft(path, content);
    });
    onState();

    return () => {
      seq++;
      stopProgress();
      plugins.destroy();
      anchorListeners.clear();
      offStore();
      offChanged();
      offDraft();
      doc.removeEventListener("keydown", onKey);
      progress.remove();
      warnings.remove();
      error.remove();
      article.remove();
      edit.remove();
      lintPanel.destroy();
    };
  },
};
