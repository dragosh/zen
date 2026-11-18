// The Settings page frame, shown in the main pane (the sidebar stays): a
// section list on the left and the chosen section on the right. The frame
// owns the list, its keyboard arrows and the Appearance section (theme); the
// app gives the other sections (configuration files, plugins, website, …).
// All text through textContent.
import { button, el } from "./dom";
import { developerEnabled, setDeveloperEnabled } from "./developer";
import { setTheme, storedTheme, type Theme } from "./theme";

export type SettingsView = {
  el: HTMLElement;
  /** Called each time the page is shown: every section re-reads what it shows, then the list takes the focus. */
  show(): void;
  destroy(): void;
};

/** One section of the page. Its heading (the label) is added by the frame. */
export type SettingsSection = {
  id: string;
  label: string;
  el: HTMLElement;
  /** Called each time the page is shown. */
  show?(): void;
  destroy?(): void;
};

const THEMES: { id: Theme; label: string }[] = [
  { id: "system", label: "System" },
  { id: "light", label: "Light" },
  { id: "dark", label: "Dark" },
];

/** Appearance: the theme as a radio group shown as a segmented control. It applies to this machine and all windows. */
export function appearanceSection(doc: () => Document): SettingsSection {
  const section = el("div");
  const themeLabel = el("p", "zen-settings-label", "Theme");
  themeLabel.id = "zen-settings-theme-label";
  const seg = el("div", "zen-settings-segment");
  seg.setAttribute("role", "radiogroup");
  seg.setAttribute("aria-labelledby", themeLabel.id);
  const radios = THEMES.map(({ id, label }) => {
    const b = button(label, () => pickTheme(id), { className: "zen-settings-option" });
    b.setAttribute("role", "radio");
    b.dataset.theme = id;
    return b;
  });
  seg.append(...radios);
  seg.addEventListener("keydown", (e) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (step === undefined) return;
    e.preventDefault();
    e.stopPropagation(); // not the section list's arrows
    const i = THEMES.findIndex((t) => t.id === storedTheme());
    const next = THEMES[(i + step + THEMES.length) % THEMES.length].id;
    pickTheme(next);
    radios.find((r) => r.dataset.theme === next)!.focus();
  });
  function renderTheme(): void {
    const now = storedTheme();
    for (const r of radios) {
      const on = r.dataset.theme === now;
      r.setAttribute("aria-checked", String(on));
      r.tabIndex = on ? 0 : -1;
    }
  }
  function pickTheme(t: Theme): void {
    setTheme(t, doc());
    renderTheme();
  }
  section.append(themeLabel, seg);
  return { id: "appearance", label: "Appearance", el: section, show: renderTheme };
}

/** Developer: the "Show developer tab" switch (ui/developer-tab). It applies to this machine and all windows. */
export function developerSection(): SettingsSection {
  const section = el("div");
  const row = el("label", "zen-settings-check");
  const box = el("input");
  box.type = "checkbox";
  box.id = "zen-settings-developer";
  box.addEventListener("change", () => setDeveloperEnabled(box.checked));
  row.append(box, el("span", "", "Show developer tab"));
  const hint = el("p", "zen-settings-hint", "Adds a Developer button next to the settings cog (⌥⌘J): a panel with the app's log, the plugins' errors and the console.");
  section.append(row, hint);
  return { id: "developer", label: "Developer", el: section, show: () => { box.checked = developerEnabled(); } };
}

/** The page: sections in the order given; the first one shows first. */
export function createSettingsFrame(sections: SettingsSection[]): SettingsView {
  const page = el("section", "zen-settings-page");
  page.setAttribute("aria-label", "Settings");

  // ---- the section list

  const nav = el("nav", "zen-settings-sections");
  nav.setAttribute("aria-label", "Settings sections");
  nav.append(el("h1", "zen-settings-title", "Settings"));
  const tabs = sections.map(({ id, label }) => {
    const b = button(label, () => choose(id), { className: "zen-settings-section" });
    b.dataset.section = id;
    return b;
  });
  const items = el("div", "zen-settings-sections-list");
  items.append(...tabs);
  nav.addEventListener("keydown", (e) => {
    const step = ({ ArrowDown: 1, ArrowUp: -1 } as Record<string, number>)[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const i = sections.findIndex((s) => s.id === current);
    const next = (i + step + sections.length) % sections.length;
    choose(sections[next].id);
    tabs[next].focus();
  });
  nav.append(items);

  const body = el("div", "zen-settings-body");
  const panels = new Map<string, HTMLElement>();
  for (const { id, label, el: content } of sections) {
    const p = el("div", "zen-settings-panel");
    p.dataset.section = id;
    p.append(el("h2", "zen-settings-heading", label), content);
    p.hidden = true;
    panels.set(id, p);
    body.append(p);
  }
  page.append(nav, body);

  let current = sections[0]?.id ?? "";
  function choose(id: string): void {
    current = id;
    for (const t of tabs) {
      const on = t.dataset.section === id;
      if (on) t.setAttribute("aria-current", "true");
      else t.removeAttribute("aria-current");
      t.tabIndex = on ? 0 : -1;
    }
    panels.forEach((p, pid) => { p.hidden = pid !== id; });
  }

  choose(current);
  return {
    el: page,
    show() {
      for (const s of sections) s.show?.();
      (tabs.find((t) => t.getAttribute("aria-current") === "true") ?? tabs[0]).focus();
    },
    destroy() {
      for (const s of sections) s.destroy?.();
      page.remove();
    },
  };
}
