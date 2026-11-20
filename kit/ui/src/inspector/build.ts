// The Inspector's Build tab (ui/inspector-details): what the app is made of, as a list of items with a short
// note each (the version, or a count) and the selected item's details beside it. Same content as Settings ›
// This build. Names and ids come from the Go side and reach the page as text only.
import type { BuildInfo, KitHost } from "../host";
import { panelProps } from "../panel";
import { el } from "../shell/dom";
import { FEATURE_IDS } from "../shell/types";

export type BuildItem = { id: string; name: string; note: string; details(): HTMLElement };

/** The build's facts that older hosts do not send; shown only when there. */
type Extra = BuildInfo & { engineVersion?: string; channel?: string };

const NONE = "None in this build.";

function names(ids: readonly string[]): HTMLElement {
  if (ids.length === 0) return el("p", "zen-build-none", NONE);
  const list = el("ul", "zen-build-names");
  for (const id of ids) list.append(el("li", "zen-build-name", id));
  return list;
}

function features(have: readonly string[]): HTMLElement {
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

export function buildItems(host: Pick<KitHost, "addons" | "build">): BuildItem[] {
  const b = host.build as Extra | undefined;
  const version = b?.version || "dev";
  const have = b?.features ?? [...FEATURE_IDS];
  const addons = host.addons ?? [];
  const list = (ids: readonly string[] | undefined) => ids ?? [];
  const simple = (id: string, name: string, ids: readonly string[]): BuildItem => ({ id, name, note: String(ids.length), details: () => names(ids) });
  return [
    {
      id: "version", name: "Version", note: version,
      details() {
        const rows: [string, string, string][] = [["App", "app", version]];
        if (b?.engineVersion && b.engineVersion !== version) rows.push(["Engine", "engine", b.engineVersion]);
        if (b?.channel) rows.push(["Channel", "channel", b.channel]);
        return panelProps(rows);
      },
    },
    { id: "features", name: "Features", note: `${FEATURE_IDS.filter((f) => have.includes(f)).length} of ${FEATURE_IDS.length}`, details: () => features(have) },
    {
      id: "addons", name: "Add-ons", note: String(addons.length),
      details() {
        if (addons.length === 0) return el("p", "zen-build-none", NONE);
        const box = el("div", "zen-build-addons");
        for (const a of addons) {
          const card = el("div", "zen-addon");
          card.dataset.addon = a.id;
          card.append(el("h3", "zen-addon-name", a.name), el("p", "zen-addon-summary", a.summary));
          box.append(card);
        }
        return box;
      },
    },
    simple("plugins", "Plugins", list(b?.plugins)),
    simple("layouts", "Layouts", list(b?.layouts)),
    simple("templates", "Templates", list(b?.templates)),
    simple("dialects", "Dialects", list(b?.dialects)),
    simple("theme", "Theme", b?.theme ? [b.theme] : []),
  ];
}
