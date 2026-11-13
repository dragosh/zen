// The breadcrumb: links for every crumb but the last, middle crumbs folded
// into a "…" menu when the row is wider than its slot.
import type { Crumb } from "../api";
import { button, el } from "../shell/dom";

export type BreadcrumbOptions = { onOpen(path: string): void; dirty: boolean };

function draw(host: HTMLElement, crumbs: Crumb[], opts: BreadcrumbOptions, hidden: number): void {
  const parts: HTMLElement[] = [];
  const sep = () => {
    const s = el("span", "zen-crumb-sep", "/");
    s.setAttribute("aria-hidden", "true");
    return s;
  };
  const link = (c: Crumb) => button(c.title, () => opts.onOpen(c.path), { className: "zen-crumb-link", title: c.title });
  crumbs.forEach((c, i) => {
    if (i > 0 && !(i > 1 && i <= hidden)) parts.push(sep());
    if (hidden > 0 && i >= 1 && i <= hidden) {
      if (i === 1) parts.push(more(crumbs.slice(1, hidden + 1), opts));
      return;
    }
    if (i === crumbs.length - 1) {
      const last = el("span", "zen-crumb-current", c.title);
      last.title = c.title;
      last.setAttribute("aria-current", "page");
      parts.push(last);
    } else parts.push(link(c));
  });
  if (opts.dirty) {
    const dot = el("span", "zen-crumb-dirty", "●");
    dot.title = "Unsaved changes";
    parts.push(dot);
  }
  host.replaceChildren(...parts);
}

function more(items: Crumb[], opts: BreadcrumbOptions): HTMLElement {
  const wrap = el("span", "zen-crumb-menu-wrap");
  const menu = el("div", "zen-crumb-menu");
  menu.setAttribute("role", "menu");
  menu.hidden = true;
  const toggle = button("Show hidden crumbs", () => setOpen(menu.hidden), { className: "zen-crumb-more", text: "…" });
  toggle.setAttribute("aria-haspopup", "menu");
  toggle.setAttribute("aria-expanded", "false");
  const onDoc = (e: Event) => { if (!wrap.contains(e.target as Node)) setOpen(false); };
  function setOpen(open: boolean): void {
    menu.hidden = !open;
    toggle.setAttribute("aria-expanded", String(open));
    if (open) {
      document.addEventListener("pointerdown", onDoc);
      menu.querySelector<HTMLElement>("[role=menuitem]")?.focus();
    } else document.removeEventListener("pointerdown", onDoc);
  }
  for (const c of items) {
    const b = button(c.title, () => { setOpen(false); opts.onOpen(c.path); }, { className: "zen-crumb-item" });
    b.setAttribute("role", "menuitem");
    menu.append(b);
  }
  menu.addEventListener("keydown", (e) => {
    const all = Array.from(menu.querySelectorAll<HTMLElement>("[role=menuitem]"));
    const i = all.indexOf(document.activeElement as HTMLElement);
    if (e.key === "Escape") { setOpen(false); toggle.focus(); }
    else if (e.key === "ArrowDown") all[(i + 1) % all.length]?.focus();
    else if (e.key === "ArrowUp") all[(i - 1 + all.length) % all.length]?.focus();
    else return;
    e.preventDefault();
  });
  wrap.append(toggle, menu);
  return wrap;
}

export function renderBreadcrumb(host: HTMLElement, crumbs: Crumb[], opts: BreadcrumbOptions): void {
  host.classList.add("zen-crumbs");
  let hidden = 0;
  draw(host, crumbs, opts, hidden);
  while (host.scrollWidth > host.clientWidth && hidden < crumbs.length - 2) {
    hidden++;
    draw(host, crumbs, opts, hidden);
  }
}
