// The problems console (ui/lint): a panel docked under the editor listing every
// Vale alert with its position, message and rule, and — for the selected one —
// why it was flagged and where to read more. Tabs filter by severity; the top
// edge drags (or arrow-keys) the panel taller or shorter. Text from Vale only
// ever reaches the DOM through textContent.
import type { Alert } from "../api";
import { el } from "../shell/dom";

export type LintConsole = {
  el: HTMLElement;
  /** null: linting unavailable or off (the console hides). */
  update(alerts: Alert[] | null): void;
  destroy(): void;
};

export type ConsoleOptions = {
  /** Jump to the alert in the text. */
  onSelect(alert: Alert, index: number): void;
  /** Open a documentation link (https only; the caller confirms). */
  onLink(url: string): void;
};

const ICON: Record<Alert["severity"], string> = { error: "●", warning: "▲", suggestion: "◆" };
const LABEL: Record<Alert["severity"], string> = { error: "Error", warning: "Warning", suggestion: "Suggestion" };
const OPEN_KEY = "zen.lint.console.open";
const HEIGHT_KEY = "zen.lint.console.height";
const TAB_KEY = "zen.lint.console.tab";
export const HEIGHT_MIN = 72;
export const HEIGHT_MAX = 640;
const HEIGHT_DEFAULT = 180;
const HEIGHT_STEP = 24;

type Tab = "all" | Alert["severity"];
const TABS: { id: Tab; label: string }[] = [
  { id: "all", label: "All" },
  { id: "error", label: "Errors" },
  { id: "warning", label: "Warnings" },
  { id: "suggestion", label: "Suggestions" },
];

function readStr(key: string): string | null {
  try { return localStorage.getItem(key); } catch { return null; }
}
function writeStr(key: string, v: string): void {
  try { localStorage.setItem(key, v); } catch { /* storage unavailable */ }
}
const clampHeight = (h: number) => Math.round(Math.min(HEIGHT_MAX, Math.max(HEIGHT_MIN, h)));

function readOpen(): boolean {
  try { return localStorage.getItem(OPEN_KEY) !== "0"; } catch { return true; }
}
function writeOpen(v: boolean): void {
  try { localStorage.setItem(OPEN_KEY, v ? "1" : "0"); } catch { /* storage unavailable */ }
}

function counts(alerts: Alert[]): string {
  const n = { error: 0, warning: 0, suggestion: 0 };
  for (const a of alerts) n[a.severity]++;
  const parts: string[] = [];
  if (n.error) parts.push(`${n.error} ${n.error === 1 ? "error" : "errors"}`);
  if (n.warning) parts.push(`${n.warning} ${n.warning === 1 ? "warning" : "warnings"}`);
  if (n.suggestion) parts.push(`${n.suggestion} ${n.suggestion === 1 ? "suggestion" : "suggestions"}`);
  return parts.length ? parts.join(", ") : "No problems";
}

let consoleSeq = 0;

export function renderConsole(opts: ConsoleOptions): LintConsole {
  const root = el("section", "zen-lint-console");
  root.setAttribute("aria-label", "Problems");
  root.hidden = true;

  const head = el("div", "zen-lint-console-head");
  const toggle = el("button", "zen-lint-console-toggle ghost");
  toggle.type = "button";
  const title = el("span", "zen-lint-console-title", "Problems");
  const summary = el("span", "zen-lint-console-summary");
  toggle.append(title, summary);

  // Severity tabs.
  const tablist = el("div", "zen-lint-console-tabs");
  tablist.setAttribute("role", "tablist");
  tablist.setAttribute("aria-label", "Filter problems");
  let tab: Tab = (TABS.find((t) => t.id === readStr(TAB_KEY))?.id) ?? "all";
  const tabBtns = TABS.map((t) => {
    const b = el("button", "zen-lint-console-tab ghost");
    b.type = "button";
    b.id = `zen-lint-tab-${t.id}-${++consoleSeq}`;
    b.dataset.tab = t.id;
    b.setAttribute("role", "tab");
    b.append(el("span", "zen-lint-console-tab-label", t.label), el("span", "zen-lint-console-tab-count", "0"));
    b.addEventListener("click", () => setTab(t.id, false));
    return b;
  });
  tablist.append(...tabBtns);
  tablist.addEventListener("keydown", (e) => {
    const i = TABS.findIndex((t) => t.id === tab);
    const to = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: TABS.length - 1 }[e.key];
    if (to === undefined) return;
    e.preventDefault();
    setTab(TABS[(to + TABS.length) % TABS.length].id, true);
  });
  head.append(toggle, tablist);

  // Drag handle on the top edge.
  const grip = el("div", "zen-lint-console-resize");
  grip.setAttribute("role", "separator");
  grip.setAttribute("aria-orientation", "horizontal");
  grip.setAttribute("aria-label", "Resize problems panel");
  grip.setAttribute("aria-valuemin", String(HEIGHT_MIN));
  grip.setAttribute("aria-valuemax", String(HEIGHT_MAX));
  grip.title = "Drag to resize (double-click to reset)";
  grip.tabIndex = 0;

  const list = el("ol", "zen-lint-console-list");
  list.setAttribute("role", "listbox");
  list.setAttribute("aria-label", "Problems");
  const detail = el("div", "zen-lint-console-detail");
  detail.setAttribute("aria-live", "polite");
  const body = el("div", "zen-lint-console-body");
  body.setAttribute("role", "tabpanel");
  body.append(list, detail);
  root.append(grip, head, body);

  let height = clampHeight(Number(readStr(HEIGHT_KEY)) || HEIGHT_DEFAULT);
  function applyHeight(save: boolean): void {
    root.style.setProperty("--zen-lint-console-h", `${height}px`);
    grip.setAttribute("aria-valuenow", String(height));
    if (save) writeStr(HEIGHT_KEY, String(height));
  }
  function setHeight(h: number, save = true): void {
    height = clampHeight(h);
    applyHeight(save);
  }
  applyHeight(false);
  // The panel grows upwards: dragging up by d makes it d taller.
  let drag: { y: number; h: number } | null = null;
  const onMove = (e: PointerEvent) => { if (drag) setHeight(drag.h + (drag.y - e.clientY), false); };
  const onUp = () => {
    if (!drag) return;
    drag = null;
    root.classList.remove("is-resizing");
    writeStr(HEIGHT_KEY, String(height));
  };
  grip.addEventListener("pointerdown", (e) => {
    if (e.button !== 0) return;
    e.preventDefault();
    drag = { y: e.clientY, h: height };
    root.classList.add("is-resizing");
    if (!open) { open = true; writeOpen(true); renderOpen(); }
  });
  grip.addEventListener("dblclick", () => setHeight(HEIGHT_DEFAULT));
  grip.addEventListener("keydown", (e) => {
    const to = { ArrowUp: height + HEIGHT_STEP, ArrowDown: height - HEIGHT_STEP, Home: HEIGHT_MIN, End: HEIGHT_MAX }[e.key];
    if (to === undefined) return;
    e.preventDefault();
    setHeight(to);
  });
  window.addEventListener("pointermove", onMove);
  window.addEventListener("pointerup", onUp);

  let open = readOpen();
  let alerts: Alert[] = [];
  let selected = -1;

  function renderOpen(): void {
    body.hidden = !open;
    grip.hidden = !open;
    toggle.setAttribute("aria-expanded", String(open));
    root.classList.toggle("is-open", open);
  }
  toggle.addEventListener("click", () => {
    open = !open;
    writeOpen(open);
    renderOpen();
  });

  const shown = (a: Alert) => tab === "all" || a.severity === tab;
  /** Indexes (into alerts) of the rows the current tab shows. */
  const visible = () => alerts.flatMap((a, i) => (shown(a) ? [i] : []));

  function renderTabs(): void {
    const n = { all: alerts.length, error: 0, warning: 0, suggestion: 0 };
    for (const a of alerts) n[a.severity]++;
    tabBtns.forEach((b, i) => {
      const id = TABS[i].id;
      const on = id === tab;
      b.setAttribute("aria-selected", String(on));
      b.tabIndex = on ? 0 : -1;
      b.querySelector(".zen-lint-console-tab-count")!.textContent = String(n[id]);
      if (on) body.setAttribute("aria-labelledby", b.id);
    });
    Array.from(list.children).forEach((row, i) => { (row as HTMLElement).hidden = !alerts[i] || !shown(alerts[i]); });
  }

  function setTab(next: Tab, focus: boolean): void {
    tab = next;
    writeStr(TAB_KEY, tab);
    renderTabs();
    if (selected >= 0 && !shown(alerts[selected])) select(-1, false);
    else select(selected, false);
    if (focus) tabBtns[TABS.findIndex((t) => t.id === tab)].focus();
  }

  function renderDetail(): void {
    const a = alerts[selected];
    if (!a) {
      const empty = tab === "all" ? "" : `No ${TABS.find((t) => t.id === tab)!.label.toLowerCase()}.`;
      detail.replaceChildren(el("p", "zen-lint-console-hint", visible().length ? "Select a problem to see why it was flagged." : empty));
      return;
    }
    const heading = el("p", "zen-lint-console-detail-head");
    heading.append(
      el("span", `zen-lint-sev zen-lint-sev-${a.severity}`, `${ICON[a.severity]} ${LABEL[a.severity]}`),
      el("code", "zen-lint-console-check", a.check),
    );
    const why = el("p", "zen-lint-console-why", a.description || a.message);
    const parts: HTMLElement[] = [heading, el("p", "zen-lint-console-message", a.message)];
    if (a.description && a.description !== a.message) parts.push(why);
    parts.push(el("p", "zen-lint-console-where", `Line ${a.line}, column ${a.col}: “${a.match}”`));
    if (a.link.startsWith("https://")) {
      const more = el("button", "zen-lint-console-link ghost", "Learn more about this rule");
      more.type = "button";
      more.title = a.link;
      more.addEventListener("click", () => opts.onLink(a.link));
      parts.push(more);
    }
    detail.replaceChildren(...parts);
  }

  function select(i: number, jump: boolean): void {
    selected = i;
    const first = visible()[0];
    Array.from(list.children).forEach((row, j) => {
      row.setAttribute("aria-selected", String(j === i));
      (row as HTMLElement).tabIndex = j === i || (i < 0 && j === first) ? 0 : -1;
    });
    renderDetail();
    if (jump && alerts[i]) opts.onSelect(alerts[i], i);
  }

  list.addEventListener("keydown", (e) => {
    const vis = visible();
    if (!vis.length) return;
    const step = { ArrowDown: 1, ArrowUp: -1 }[e.key];
    if (step !== undefined) {
      e.preventDefault();
      const at = vis.indexOf(selected);
      const next = vis[at < 0 ? 0 : Math.min(vis.length - 1, Math.max(0, at + step))];
      select(next, false);
      (list.children[next] as HTMLElement).focus();
    } else if (e.key === "Enter" && selected >= 0) {
      e.preventDefault();
      select(selected, true);
    }
  });

  return {
    el: root,
    update(next) {
      if (next === null) {
        alerts = [];
        root.hidden = true;
        return;
      }
      const keep = alerts[selected];
      alerts = next;
      root.hidden = false;
      summary.textContent = counts(alerts);
      list.replaceChildren(...alerts.map((a, i) => {
        const row = el("li", `zen-lint-console-row zen-lint-console-row-${a.severity}`);
        row.setAttribute("role", "option");
        row.append(
          el("span", `zen-lint-sev zen-lint-sev-${a.severity}`, ICON[a.severity]),
          el("span", "zen-lint-console-pos", `${a.line}:${a.col}`),
          el("span", "zen-lint-console-msg", a.message),
          el("span", "zen-lint-console-rule", a.check),
        );
        row.querySelector(".zen-lint-sev")!.setAttribute("aria-label", LABEL[a.severity]);
        row.addEventListener("click", () => select(i, true));
        return row;
      }));
      // Keep the same problem selected across re-lints while it still exists.
      const again = keep ? alerts.findIndex((a) => a.check === keep.check && a.match === keep.match && a.line === keep.line) : -1;
      renderTabs();
      select(again >= 0 && shown(alerts[again]) ? again : -1, false);
      renderOpen();
    },
    destroy() {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      root.remove();
    },
  };
}
