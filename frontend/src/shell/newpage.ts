// File › New Page… (⌘N): a modal that asks for a name and a layout (plain
// page, slides or course), then creates the page through api.createPage.
import type { Api, PageLayout } from "../api";
import { button, el } from "./dom";

export const LAYOUTS: { id: PageLayout; label: string; hint: string }[] = [
  { id: "", label: "Page", hint: "Plain markdown" },
  { id: "slides", label: "Slides", hint: "A presentation: --- starts the next slide" },
  { id: "course", label: "Course", hint: "Steps with checklists and saved progress" },
];

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
  let layout: PageLayout = "";
  const cards = LAYOUTS.map((l) => {
    const b = button(l.label, () => choose(l.id, false), { className: "zen-newpage-layout" });
    b.replaceChildren(el("span", "zen-newpage-layout-name", l.label), el("span", "zen-newpage-layout-hint", l.hint));
    b.setAttribute("role", "radio");
    b.dataset.layout = l.id;
    return b;
  });
  group.append(...cards);
  group.addEventListener("keydown", (e) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const i = LAYOUTS.findIndex((l) => l.id === layout);
    choose(LAYOUTS[(i + step + LAYOUTS.length) % LAYOUTS.length].id, true);
  });
  function choose(next: PageLayout, focus: boolean): void {
    layout = next;
    for (const c of cards) {
      const on = c.dataset.layout === next;
      c.setAttribute("aria-checked", String(on));
      c.tabIndex = on ? 0 : -1;
      if (on && focus) c.focus();
    }
  }
  choose("", false);

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
      const path = await api.createPage(name.value, layout);
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
