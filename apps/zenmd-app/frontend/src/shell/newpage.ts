// File › New Page… (⌘N): a modal that asks for a name and a layout, then
// creates the page through api.createPage. The layouts come from api.layouts()
// (built-in first, then folder, then user; blocked and invalid ones are not
// offered); choosing one fills the new page with that layout's first help
// snippet (frontmatter and body), or `layout: <id>` and a title.
import type { Api, LayoutInfo } from "../api";
import { button, el } from "@zenmd/kit";

/** What a card needs: the plain page is the article layout. */
type Choice = { id: string; label: string; hint: string };
const ARTICLE = "article";
const PLAIN: Choice = { id: ARTICLE, label: "Page", hint: "Plain markdown" };
const SOURCE_ORDER: Record<LayoutInfo["source"], number> = { builtin: 0, folder: 1, user: 2 };

/** The page text a layout starts with: its first help snippet, else a minimal page. */
export function startingText(l: Pick<LayoutInfo, "id" | "help"> | undefined, id: string): string {
  const first = l?.help.snippets.find((s) => s.markdown.trim() !== "");
  return first ? first.markdown : `---\nlayout: ${id}\n---\n# Title`;
}

export type NewPageOptions = {
  api: Api;
  doc: Document;
  /** Folder of the page being read ("guides/"), where the new page goes by default. */
  folder: string;
  onCreated(path: string): void;
  onClose(): void;
};

export type NewPageDialog = { el: HTMLElement; close(): void };

let seq = 0;

export function openNewPage(opts: NewPageOptions): NewPageDialog {
  const { api, doc } = opts;
  const id = ++seq;
  const backdrop = el("div", "zen-newpage-backdrop");
  const form = el("form", "zen-newpage");
  form.setAttribute("role", "dialog");
  form.setAttribute("aria-modal", "true");
  form.noValidate = true;
  const title = el("h2", "zen-newpage-title", "New page");
  title.id = `zen-newpage-title-${id}`;
  form.setAttribute("aria-labelledby", title.id);

  // Layout: a radio group of cards.
  const layoutLabel = el("p", "zen-newpage-label", "Layout");
  layoutLabel.id = `zen-newpage-layout-${id}`;
  const group = el("div", "zen-newpage-layouts");
  group.setAttribute("role", "radiogroup");
  group.setAttribute("aria-labelledby", layoutLabel.id);
  let layout = ARTICLE;
  let choices: Choice[] = [PLAIN];
  let known: LayoutInfo[] = [];
  let cards: HTMLButtonElement[] = [];
  function drawCards(): void {
    cards = choices.map((l) => {
      const b = button(l.label, () => choose(l.id, false), { className: "zen-newpage-layout" });
      b.replaceChildren(el("span", "zen-newpage-layout-name", l.label), el("span", "zen-newpage-layout-hint", l.hint));
      b.setAttribute("role", "radio");
      b.dataset.layout = l.id;
      return b;
    });
    group.replaceChildren(...cards);
    choose(layout, false);
  }
  group.addEventListener("keydown", (e) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const i = Math.max(0, choices.findIndex((l) => l.id === layout));
    choose(choices[(i + step + choices.length) % choices.length].id, true);
  });
  function choose(next: string, focus: boolean): void {
    layout = choices.some((l) => l.id === next) ? next : ARTICLE;
    for (const c of cards) {
      const on = c.dataset.layout === layout;
      c.setAttribute("aria-checked", String(on));
      c.tabIndex = on ? 0 : -1;
      if (on && focus) c.focus();
    }
  }
  drawCards();
  // The plain page is there at once; the layouts follow when the list arrives (and stay out if it fails).
  api.layouts().then((all) => {
    if (closed) return;
    known = all;
    const usable = all.filter((l) => l.id !== ARTICLE && l.status !== "blocked" && l.status !== "invalid");
    usable.sort((a, b) => SOURCE_ORDER[a.source] - SOURCE_ORDER[b.source] || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
    choices = [PLAIN, ...usable.map((l) => ({ id: l.id, label: l.id, hint: l.description }))];
    drawCards();
  }, () => { /* only the plain page is offered */ });

  const nameLabel = el("label", "zen-newpage-label", "Name");
  const name = el("input", "zen-newpage-name");
  name.type = "text";
  name.id = `zen-newpage-name-${id}`;
  name.autocomplete = "off";
  name.spellcheck = false;
  name.value = `${opts.folder}untitled`;
  nameLabel.htmlFor = name.id;
  const hint = el("p", "zen-newpage-hint", "Inside the open folder; .md is added. Use / for a subfolder.");
  const error = el("p", "zen-newpage-error");
  error.setAttribute("role", "alert");

  const actions = el("div", "zen-newpage-actions");
  const cancel = button("Cancel", () => close());
  const create = el("button", "primary", "Create");
  create.type = "submit";
  actions.append(cancel, create);

  form.append(title, layoutLabel, group, nameLabel, name, hint, error, actions);
  backdrop.append(form);
  doc.body.append(backdrop);

  const before = doc.activeElement as HTMLElement | null;
  name.focus();
  name.setSelectionRange(opts.folder.length, name.value.length);

  let busy = false;
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (busy) return;
    busy = true;
    create.disabled = cancel.disabled = true;
    error.textContent = "";
    try {
      // The page is made plain, then filled with the layout's starting text.
      const path = await api.createPage(name.value, "");
      if (layout !== ARTICLE) {
        try {
          await api.save(path, startingText(known.find((l) => l.id === layout), layout));
        } catch (err) {
          console.warn(`zen: could not fill ${path} for layout ${layout}`, err);
        }
      }
      close();
      opts.onCreated(path);
    } catch (err) {
      error.textContent = err instanceof Error ? err.message : String(err);
      busy = false;
      create.disabled = cancel.disabled = false;
      name.focus();
    }
  });

  // Esc closes, Tab stays inside, a click outside closes.
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      if (!busy) close();
    } else if (e.key === "Tab") {
      const items = Array.from(form.querySelectorAll<HTMLElement>("input, button:not([disabled])")).filter((x) => x.tabIndex >= 0);
      const first = items[0];
      const last = items[items.length - 1];
      if (e.shiftKey && doc.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && doc.activeElement === last) { e.preventDefault(); first.focus(); }
    }
  };
  doc.addEventListener("keydown", onKey, true);
  backdrop.addEventListener("mousedown", (e) => { if (e.target === backdrop && !busy) close(); });

  let closed = false;
  function close(): void {
    if (closed) return;
    closed = true;
    doc.removeEventListener("keydown", onKey, true);
    backdrop.remove();
    if (before?.isConnected) before.focus();
    opts.onClose();
  }

  return { el: backdrop, close };
}
