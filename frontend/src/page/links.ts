// Every click on an anchor in the page is intercepted: the webview never navigates.
import type { Ctx } from "../shell/types";

const SCHEME = /^[a-z][a-z0-9+.-]*:/i;
const OPENABLE = /^(https?|mailto):/i;

export function handleLinkClick(e: MouseEvent, ctx: Ctx): void {
  const a = (e.target as Element | null)?.closest?.("a");
  if (!a) return;
  e.preventDefault();
  // Layout chrome (e.g. the docs layout's "Edit this page") asks for an action.
  if (a.getAttribute("data-zen-action") === "edit") {
    const { path } = ctx.store.get();
    if (path) void ctx.api.openEditor(path).catch((err) => console.error(`zen: could not edit ${path}`, err));
    return;
  }
  const kind = a.getAttribute("data-zen-link");
  const href = a.getAttribute("href") ?? "";
  if (kind === "broken") return;
  if (kind === "internal") {
    ctx.navigate(a.getAttribute("data-path") ?? "", a.getAttribute("data-anchor") ?? "");
    return;
  }
  if (kind === "external" || SCHEME.test(href)) {
    if (OPENABLE.test(href)) void ctx.api.openExternal(href).catch((err) => console.error("zen: openExternal failed", err));
    return;
  }
  if (href.startsWith("#") && href.length > 1) ctx.navigate(ctx.store.get().path, decodeURIComponent(href.slice(1)));
}
