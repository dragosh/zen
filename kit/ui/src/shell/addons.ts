// Settings › This build (kit/addons, kit/assembly): what the app is made of, read-only
// (it is chosen when the app is built, in apps/<app>/app-assembly.yml). Names and ids come
// from the Go side and reach the page as text only.
import "./addons.css";
import { hasFeature, type BuildInfo, type KitHost } from "../host";
import { el } from "./dom";
import { FEATURE_IDS } from "./types";
import type { SettingsSection } from "./settings";

/** Puts the build's theme on the root element as `data-app-theme` (`default` needs no CSS); no build leaves it alone. */
export function applyAppTheme(doc: Document, build: Pick<BuildInfo, "theme"> | undefined): void {
  if (build?.theme) doc.documentElement.dataset.appTheme = build.theme;
}

function group(id: string, title: string, ...body: HTMLElement[]): HTMLElement {
  const g = el("div", "zen-build-group");
  g.dataset.group = id;
  g.append(el("h3", "zen-build-title", title), ...body);
  return g;
}

function names(ids: readonly string[]): HTMLElement {
  return el("p", "zen-build-list", ids.length ? ids.join(", ") : "none");
}

/**
 * Every feature of the catalog in its order: the ones this build has as plain names, the missing ones
 * greyed and marked "not in this build" for screen readers, so a bare build shows what it could have.
 */
function featureList(have: readonly string[]): HTMLElement {
  const list = el("ul", "zen-build-features");
  for (const id of FEATURE_IDS) {
    const li = el("li", "zen-build-feature", id);
    li.dataset.feature = id;
    if (!have.includes(id)) {
      li.classList.add("zen-build-feature-missing");
      li.append(el("span", "zen-visually-hidden", " (not in this build)"));
    }
    list.append(li);
  }
  return list;
}

/**
 * The section shows `host.addons` as cards (or says the build has none), then the plugins,
 * layouts, templates and markdown dialects the build includes and its theme as short lists, each time Settings
 * is shown. A host without a `build` shows the add-ons only.
 */
export function buildSection(host: Pick<KitHost, "addons" | "build">): SettingsSection {
  const section = el("div", "zen-addons zen-build");
  function render(): void {
    const list = host.addons ?? [];
    const cards = list.length === 0
      ? [el("p", "zen-addons-empty", "This build has no add-ons.")]
      : list.map((a) => {
        const card = el("div", "zen-addon");
        card.dataset.addon = a.id;
        card.append(el("h3", "zen-addon-name", a.name), el("p", "zen-addon-summary", a.summary));
        return card;
      });
    const groups = [group("addons", "Add-ons", ...cards)];
    const b = host.build;
    if (b) {
      // Features come first: the pieces of the app itself, before the add-ons it can take.
      groups.unshift(group("features", "Features", featureList(b.features), ...(hasFeature(host, "editor") ? [] : [el("p", "zen-build-list", "Read-only (no editor)")])));
      groups.push(
        group("plugins", "Plugins", names(b.plugins)),
        group("layouts", "Layouts", names(b.layouts)),
        group("templates", "Templates", names(b.templates)),
        group("dialects", "Dialects", names(b.dialects ?? [])),
        group("theme", "Theme", names(b.theme ? [b.theme] : [])),
      );
    }
    section.replaceChildren(...groups);
  }
  render();
  return { id: "build", label: "This build", el: section, show: render };
}
