// Plugin assets named by Page.assets: loaded once per URL, only from /_plugins/.
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
  for (const src of assets.js) {
    if (!isPluginUrl(src) || doc.head.querySelector(`script[data-zen-asset="${CSS.escape(src)}"]`)) continue;
    const script = doc.createElement("script");
    script.type = "module";
    script.src = src;
    script.dataset.zenAsset = src;
    doc.head.append(script);
  }
}
