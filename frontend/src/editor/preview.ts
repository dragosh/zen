// The editor's optional preview pane: api.preview() of the unsaved text,
// debounced, latest response wins. It mounts through the reader's mountPage
// and plugin runtime, so plugin assets and scripts work as on a page.
import type { Api, Page } from "../api";
import { mountPage, pageHooks } from "../page";
import { installRuntime } from "../plugins/runtime";
import { el } from "../shell/dom";

/** Render the unsaved text this long after typing stops. */
export const PREVIEW_DELAY = 150;

export type Preview = {
  el: HTMLElement;
  /** Renders `text` for `path` after the debounce; an older answer never replaces a newer one. */
  update(path: string, text: string): void;
  /** Scrolls to the same proportion (0 to 1) as the source; kept across re-renders. */
  follow(ratio: number): void;
  destroy(): void;
};

export function createPreview(api: Api, delay = PREVIEW_DELAY): Preview {
  const root = el("div", "zen-editor-preview");
  root.setAttribute("aria-label", "Preview");
  root.setAttribute("role", "region");
  const article = el("article", "zen-page");
  root.append(article);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  let ratio = 0;
  let dead = false;
  let rendered = { path: "", title: "" };

  // Plugin scripts mount on the preview's blocks; navigation does nothing here.
  const plugins = installRuntime({
    api, navigate: () => undefined, page: () => rendered, importModule: pageHooks.importModule,
  });

  // Links in the preview never navigate or open anything.
  const inert = (e: MouseEvent) => {
    if ((e.target as Element | null)?.closest?.("a")) e.preventDefault();
  };
  root.addEventListener("click", inert);
  root.addEventListener("auxclick", inert);

  function apply(): void {
    const max = root.scrollHeight - root.clientHeight;
    root.scrollTop = max > 0 ? Math.round(max * ratio) : 0;
  }
  function render(page: Page): void {
    plugins.unmountAll();
    rendered = { path: page.path, title: page.title };
    mountPage(article, page);
    void plugins.mountAll(article, page.assets);
    apply();
  }

  return {
    el: root,
    update(path, text) {
      clearTimeout(timer);
      timer = setTimeout(() => {
        const mine = ++seq;
        api.preview(path, text).then((page) => {
          if (!dead && mine === seq) render(page);
        }, () => { /* keep the last good preview */ });
      }, delay);
    },
    follow(r) {
      ratio = Math.min(1, Math.max(0, r));
      apply();
    },
    destroy() {
      dead = true;
      seq++;
      clearTimeout(timer);
      root.removeEventListener("click", inert);
      root.removeEventListener("auxclick", inert);
      plugins.destroy();
      root.remove();
    },
  };
}
