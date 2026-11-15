// The problems console (ui/lint): a panel docked under the editor listing every
// Vale alert with its position, message and rule, and — for the selected one —
// why it was flagged and where to read more. Text from Vale only ever reaches
// the DOM through textContent.
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
  head.append(toggle);

  const list = el("ol", "zen-lint-console-list");
  list.setAttribute("role", "listbox");
  list.setAttribute("aria-label", "Problems");
  const detail = el("div", "zen-lint-console-detail");
  detail.setAttribute("aria-live", "polite");
  const body = el("div", "zen-lint-console-body");
  body.append(list, detail);
  root.append(head, body);

  let open = readOpen();
  let alerts: Alert[] = [];
  let selected = -1;

  function renderOpen(): void {
    body.hidden = !open;
    toggle.setAttribute("aria-expanded", String(open));
    root.classList.toggle("is-open", open);
  }
  toggle.addEventListener("click", () => {
    open = !open;
    writeOpen(open);
    renderOpen();
  });

  function renderDetail(): void {
    const a = alerts[selected];
    if (!a) {
      detail.replaceChildren(el("p", "zen-lint-console-hint", alerts.length ? "Select a problem to see why it was flagged." : ""));
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
    Array.from(list.children).forEach((row, j) => {
      row.setAttribute("aria-selected", String(j === i));
      (row as HTMLElement).tabIndex = j === i || (i < 0 && j === 0) ? 0 : -1;
    });
    renderDetail();
    if (jump && alerts[i]) opts.onSelect(alerts[i], i);
  }

  list.addEventListener("keydown", (e) => {
    const n = alerts.length;
    if (!n) return;
    const step = { ArrowDown: 1, ArrowUp: -1 }[e.key];
    if (step !== undefined) {
      e.preventDefault();
      const next = Math.min(n - 1, Math.max(0, (selected < 0 ? 0 : selected + step)));
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
      select(again, false);
      renderOpen();
    },
    destroy() {
      root.remove();
    },
  };
}
