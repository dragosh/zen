// Settings › Add-ons (kit/addons): what this build includes, read-only (add-ons are
// chosen when the app is built). Names and summaries come from the Go side and reach
// the page as text only.
import "./addons.css";
import type { KitHost } from "../host";
import { el } from "./dom";
import type { SettingsSection } from "./settings";

/** The section lists `host.addons` each time Settings is shown, or says that the build has none. */
export function addonsSection(host: Pick<KitHost, "addons">): SettingsSection {
  const section = el("div", "zen-addons");
  function render(): void {
    const list = host.addons ?? [];
    if (list.length === 0) {
      section.replaceChildren(el("p", "zen-addons-empty", "This build has no add-ons."));
      return;
    }
    section.replaceChildren(...list.map((a) => {
      const card = el("div", "zen-addon");
      card.dataset.addon = a.id;
      card.append(el("h3", "zen-addon-name", a.name), el("p", "zen-addon-summary", a.summary));
      return card;
    }));
  }
  render();
  return { id: "addons", label: "Add-ons", el: section, show: render };
}
