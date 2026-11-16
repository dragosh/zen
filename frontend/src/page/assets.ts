// Plugin stylesheets named by Page.assets: linked once per URL, only from /_plugins/.
// Plugin scripts are not injected: the runtime import()s them (plugins/runtime.ts).
import type { Page } from "../api";

const PREFIX = "/_plugins/";

/** True for a same-origin path under /_plugins/ with no `..` segment. */
export function isPluginUrl(url: string): boolean {
  return url.startsWith(PREFIX) && !url.split(/[/?#]/).includes("..") && !url.includes("\\");
}

export function loadAssets(doc: Document, assets: Page["assets"]): void {
  for (const href of assets.css) {
    if (!isPluginUrl(href) || doc.head.querySelector(`link[data-zen-asset="${CSS.escape(href)}"]`)) continue;
    const link = doc.createElement("link");
    link.rel = "stylesheet";
    link.href = href;
    link.dataset.zenAsset = href;
    doc.head.append(link);
  }
}
