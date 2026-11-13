// The reader: loads Page for store.path, mounts its HTML and wires links,
// anchors, plugin assets, live reload, warnings, errors and the Edit action.
import type { Page } from "../api";
import { button, el } from "../shell/dom";
import type { Ctx, Feature } from "../shell/types";
import { loadAssets } from "./assets";
import { handleLinkClick } from "./links";

export { handleLinkClick };

const PROGRESS_DELAY = 150;

/** The single innerHTML in the app: Page.html is sanitised in Go. */
export function mountPage(el: HTMLElement, page: Page): void {
  el.innerHTML = page.html;
  loadAssets(el.ownerDocument, page.assets);
  el.ownerDocument.dispatchEvent(new CustomEvent("zen:page-rendered", { detail: { root: el } }));
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

    const edit = button("Edit", () => ctx.store.set({ mode: "edit" }), { title: "Edit (⌘E)" });
    slots.actions.append(edit);

    let seq = 0;
    let shownPath: string | null = null;
    let loadedAnchor = "";
    let timer: ReturnType<typeof setTimeout> | undefined;

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
      mountPage(article, page);
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
      if (path !== shownPath) void load(false);
      else if (anchor !== loadedAnchor) {
        loadedAnchor = anchor;
        scrollToAnchor(anchor, true);
      }
    };

    const onClick = (e: MouseEvent) => handleLinkClick(e, ctx);
    const onAux = (e: MouseEvent) => { if (e.button === 1) handleLinkClick(e, ctx); };
    article.addEventListener("click", onClick);
    article.addEventListener("auxclick", onAux);

    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "e") {
        e.preventDefault();
        ctx.store.set({ mode: "edit" });
      }
    };
    doc.addEventListener("keydown", onKey);

    const offStore = ctx.store.subscribe(onState);
    const offChanged = ctx.api.on("page:changed", ({ path }) => {
      if (path === shownPath && path === ctx.store.get().path) void load(true);
    });
    onState();

    return () => {
      seq++;
      stopProgress();
      offStore();
      offChanged();
      doc.removeEventListener("keydown", onKey);
      progress.remove();
      warnings.remove();
      error.remove();
      article.remove();
      edit.remove();
    };
  },
};
