// Zenmark › About Zenmark: a sheet in the main window (not the standard macOS panel) with the logo, name,
// version, copyright and This build (what the app is made of). It works in a bare shell and with no folder
// open. One sheet per document: asking again brings it to the front. All text through textContent.
import "./settings.css";
import type { KitHost } from "../host";
import { buildSection } from "./addons";
import { button, el } from "./dom";

const open = new WeakMap<Document, { close(): void }>();

export function openAbout(
  doc: Document,
  o: { host: Pick<KitHost, "addons" | "build">; name: string; version: string; logoUrl: string },
): { close(): void } {
  const again = open.get(doc);
  if (again) {
    (doc.querySelector<HTMLElement>(".zen-about button") ?? undefined)?.focus();
    return again;
  }
  const backdrop = el("div", "zen-about-backdrop");
  const sheet = el("div", "zen-about");
  sheet.setAttribute("role", "dialog");
  sheet.setAttribute("aria-modal", "true");
  const title = el("h2", "zen-about-name", o.name);
  title.id = "zen-about-title";
  sheet.setAttribute("aria-labelledby", title.id);
  const logo = el("img", "zen-about-logo");
  logo.src = o.logoUrl;
  logo.alt = "";
  const head = el("div", "zen-about-head");
  head.append(
    logo, title,
    el("p", "zen-about-version", `Version ${o.version}`),
    el("p", "zen-about-copyright", `© ${new Date().getFullYear()} ${o.name}`),
  );
  const build = buildSection(o.host);
  build.show?.();
  const close = button("Close", () => handle.close(), { className: "primary" });
  const foot = el("div", "zen-about-foot");
  foot.append(close);
  sheet.append(head, build.el!, foot);
  backdrop.append(sheet);
  doc.body.append(backdrop);
  const before = doc.activeElement as HTMLElement | null;
  close.focus();

  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      handle.close();
    } else if (e.key === "Tab") {
      const items = Array.from(sheet.querySelectorAll<HTMLElement>("button")).filter((x) => !(x as HTMLButtonElement).disabled);
      if (items.length === 0) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (e.shiftKey && doc.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && doc.activeElement === last) { e.preventDefault(); first.focus(); }
      else if (!sheet.contains(doc.activeElement)) { e.preventDefault(); first.focus(); }
    }
  };
  doc.addEventListener("keydown", onKey, true);
  backdrop.addEventListener("mousedown", (e) => { if (e.target === backdrop) handle.close(); });

  let closed = false;
  const handle = {
    close() {
      if (closed) return;
      closed = true;
      doc.removeEventListener("keydown", onKey, true);
      backdrop.remove();
      open.delete(doc);
      if (before?.isConnected) before.focus();
    },
  };
  open.set(doc, handle);
  return handle;
}
