// Live preview: api.preview() (sanitised in Go), debounced, latest response wins.
import type { Api, Page } from "../api";
import { el } from "../shell/dom";

export type Preview = {
  el: HTMLElement;
  update(path: string, text: string): void;
  /** Scrolls to the same proportion (0–1) as the source. */
  follow(ratio: number): void;
  destroy(): void;
};

/** Same mount as ui/page's mountPage, copied so the features stay independent. */
function mount(article: HTMLElement, page: Page): void {
  // Trusted only because it comes from Go's sanitiser via api.preview().
  article.innerHTML = page.html;
}

export function createPreview(api: Api, delay = 150): Preview {
  const root = el("div", "zen-editor-preview");
  root.setAttribute("aria-label", "Preview");
  const article = el("article", "zen-page");
  root.append(article);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  let dead = false;

  // Links in the preview never navigate.
  root.addEventListener("click", (e) => {
    if ((e.target as Element | null)?.closest?.("a")) e.preventDefault();
  });

  return {
    el: root,
    update(path, text) {
      clearTimeout(timer);
      timer = setTimeout(() => {
        const mine = ++seq;
        api.preview(path, text).then((page) => {
          if (!dead && mine === seq) mount(article, page);
        }, () => { /* keep the last good preview */ });
      }, delay);
    },
    follow(ratio) {
      const max = root.scrollHeight - root.clientHeight;
      if (max > 0) root.scrollTop = Math.round(max * ratio);
    },
    destroy() {
      dead = true;
      clearTimeout(timer);
    },
  };
}
