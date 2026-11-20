// The reader: loads Page for store.path, mounts its HTML and wires links,
// anchors, plugin assets, live reload, warnings, errors and the Edit action.
import { hasAddon, hasFeature, type Alert, type Page } from "../host";
import { createInspector, inspectorKey } from "../inspector";
import { bindFind, createFind, domFinder } from "../find";
import { flash, markText } from "../lint";
import { installRuntime } from "@zenmd/runtime/plugins";
import { installImageZoom } from "@zenmd/runtime/zoom/zoom";
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

/** A page with a layout other than article (core/layouts) is marked so page.css hands the whole pane to it. */
function setLayout(el: HTMLElement, ref: Page["layout"] | undefined): void {
  const layout = ref?.name ?? "";
  for (const c of Array.from(el.classList)) if (c.startsWith(LAYOUT_CLASS)) el.classList.remove(c);
  if (layout !== "article" && /^[a-z][a-z0-9-]*$/.test(layout)) {
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

/** A layout's "Edit this page" links (data-zen-action="edit") are removed from view in an app without the editor. */
function hideEditLinks(root: HTMLElement): void {
  for (const a of Array.from(root.querySelectorAll<HTMLElement>('[data-zen-action="edit"]'))) {
    a.hidden = true;
    a.style.display = "none"; // a layout's own display rule must not bring it back
  }
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

    // A folder written for another markdown syntax (a MyST project): one quiet line above the page
    // offers to turn the dialect on (engine/dialects). Not in a read-only app.
    const hint = el("div", "zen-page-warnings zen-dialect-hint");
    hint.hidden = true;
    hint.setAttribute("role", "status");
    const hintText = el("div", "zen-page-warnings-text");
    let hintId = "";
    let hintName = "";
    const hideHint = () => { hint.hidden = true; hintId = ""; };
    const turnOn = button("Turn on", () => {
      const id = hintId;
      if (!id || !ctx.api.setDialect) return;
      ctx.api.setDialect(id, true).then(hideHint, (err) => { hintText.textContent = `Could not turn on ${hintName}: ${message(err)}`; });
    }, { className: "ghost", text: "Turn on" });
    const notNow = button("Not now", () => dismissHint(), { className: "ghost", text: "Not now" });
    hint.append(hintText, turnOn, notNow);
    function dismissHint(): void {
      const id = hintId;
      hideHint();
      if (id) ctx.api.dismissDialectHint?.(id).catch((err) => console.warn("zen: could not remember the dismissal", err));
    }
    async function showDialectHint(): Promise<void> {
      const api = ctx.api;
      if (!hasFeature(api, "editor") || !api.dialectHint || !api.dialects || !api.setDialect) return;
      try {
        const id = await api.dialectHint();
        if (!id || gone) return;
        const d = (await api.dialects()).find((x) => x.id === id);
        if (!d || d.on || gone) return;
        hintId = id;
        hintName = d.name;
        hintText.textContent = `This folder looks like a ${d.name} project. Turn on ${d.name} syntax?`;
        hint.hidden = false;
      } catch (err) {
        console.warn("zen: could not check for markdown dialects", err);
      }
    }
    let gone = false;

    const error = el("div", "zen-page-error");
    error.hidden = true;
    error.setAttribute("role", "alert");
    const errorText = el("p", "zen-page-error-message");
    error.append(errorText, button("Retry", () => void load(false), { className: "primary" }));

    const article = el("article", "zen-page");
    main.append(progress, warnings, hint, error, article);

    /** Edit opens the page in its own editor window (ui/editor-window). */
    const openEditor = () => {
      const { path } = ctx.store.get();
      if (!path) return;
      ctx.api.openEditor(path).catch((err) => console.error(`zen: could not edit ${path}`, err));
    };
    const edit = button("Edit", openEditor, { title: "Edit (⌘E)" });

    // Vale alerts (ui/lint): marks in the article; the list is in the Inspector (ui/inspector),
    // whose button in the sidebar footer (shell/layout.ts) carries the count; it is wired by events.
    // Only in builds with the vale add-on (kit/addons).
    const lintOn = hasAddon(ctx.api, "vale");
    let lintMarks: (HTMLElement | null)[] = [];
    const inspector = createInspector({
      doc, host: ctx.api, external: true,
      onSelectIssue(_alert: Alert, i: number) {
        const m = lintMarks[i];
        if (!m?.isConnected) return;
        m.scrollIntoView({ behavior: reducedMotion() ? "auto" : "smooth", block: "center" });
        flash(m);
      },
    });
    // Without the editor feature (a read-only app) there is no Edit button, no ⌘E and no edit links.
    const editable = hasFeature(ctx.api, "editor");
    if (editable) slots.actions.append(edit);
    // The panel sits beside the main pane, before it (inspector.css makes room for it).
    main.before(inspector.el);

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
    // Find in page (ui/find): ⌘F and Edit › Find, in the window that has the focus.
    const slotIndex = (block: HTMLElement, cls: string) =>
      Array.from(block.parentElement?.children ?? []).filter((c) => c.classList.contains(cls)).indexOf(block);
    const find = createFind({
      doc, host: doc.body,
      finder: domFinder(() => (article.hidden ? null : article), {
        skip: (e) => e.classList.contains("slides-notes") || e.classList.contains("slides-overview"),
        // A hidden slide or course step is shown by the layout itself; other hidden blocks are just unhidden.
        reveal(block) {
          if (block.classList.contains("slides-slide")) {
            ctx.navigate(rendered.path, `slide-${slotIndex(block, "slides-slide") + 1}`);
            return true;
          }
          if (block.classList.contains("course-step")) {
            const link = article.querySelectorAll<HTMLButtonElement>(".course-step-link")[slotIndex(block, "course-step")];
            if (link && !link.disabled) { link.click(); return true; }
          }
          return false;
        },
      }),
    });
    const offFind = bindFind(doc, ctx.api, find, { active: () => ctx.store.get().mode === "read" && shownPath !== null });
    function render(page: Page): void {
      plugins.unmountAll();
      rendered = { path: page.path, title: page.title };
      mountPage(article, page);
      if (!editable) hideEditLinks(article);
      void plugins.mountAll(article, page.assets);
      find.refresh();
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
      if (!lintOn) return; // no vale add-on: no calls, no marks
      try {
        const r = await ctx.api.lint(path, content);
        if (mine !== seq) return;
        lintMarks = r.available ? markText(article, r.alerts) : [];
        inspector.setIssues(r.available ? r.alerts : null);
      } catch (err) {
        if (mine !== seq) return;
        lintMarks = [];
        inspector.setIssues(null);
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
        inspector.setIssues(null);
        inspector.setPage(null);
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
      if (!keepScroll) inspector.setIssues("checking");
      inspector.setPage(path, page, article);
      void lint(path, mine);
      warningText.textContent = page.warnings.join("\n");
      warnings.hidden = page.warnings.length === 0;
      doc.title = `${page.title} — ${ctx.brand}`;
      ctx.store.set({ page });
      if (keepScroll) main.scrollTop = scroll;
      else scrollToAnchor(loadedAnchor, false);
    }

    const onState = () => {
      const { path, anchor, mode } = ctx.store.get();
      inspector.setActive(mode === "read" && path !== "");
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
    const zoom = installImageZoom(article); // a click on an image shows it large (ui/image-zoom)

    const onKey = (e: KeyboardEvent) => {
      if (editable && (e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "e") {
        e.preventDefault();
        openEditor();
      }
      if (inspectorKey(e) && inspector.available) {
        e.preventDefault();
        inspector.toggle();
      }
      if (e.key === "Escape" && !hint.hidden && !e.defaultPrevented) dismissHint();
      // ⌘P is handled by the print flow (print/page.ts, installed by the shell).
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
      inspector.setPage(path, page, article);
      void lint(path, mine, content);
      warningText.textContent = page.warnings.join("\n");
      warnings.hidden = page.warnings.length === 0;
      doc.title = `${page.title} — ${ctx.brand}`;
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
    void showDialectHint();

    return () => {
      gone = true;
      seq++;
      zoom.destroy();
      offFind();
      find.destroy();
      stopProgress();
      plugins.destroy();
      anchorListeners.clear();
      offStore();
      offChanged();
      offDraft();
      doc.removeEventListener("keydown", onKey);
      progress.remove();
      warnings.remove();
      hint.remove();
      error.remove();
      article.remove();
      edit.remove();
      inspector.destroy();
    };
  },
};
