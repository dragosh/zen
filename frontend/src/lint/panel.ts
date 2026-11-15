// The issues button and panel: "3 issues" toggles a list of alerts; an item
// calls onSelect. Esc closes. Shared by the reader and the editor.
import type { Alert } from "../api";
import { el } from "../shell/dom";
import { severityOf } from "./mark";

export type LintPanel = {
  /** Wrapper holding the button and the panel; append it where the count belongs. */
  el: HTMLElement;
  button: HTMLButtonElement;
  panel: HTMLElement;
  /** New alerts; null (unavailable) or [] hides everything. */
  update(alerts: Alert[] | null): void;
  open(): void;
  close(): void;
  destroy(): void;
};

let panelSeq = 0;

export function renderPanel(onSelect: (alert: Alert, index: number) => void): LintPanel {
  const wrap = el("span", "zen-lint");
  const btn = el("button", "zen-lint-count ghost");
  btn.type = "button";
  btn.hidden = true;
  btn.setAttribute("aria-expanded", "false");
  const panel = el("div", "zen-lint-panel");
  panel.id = `zen-lint-panel-${++panelSeq}`;
  panel.hidden = true;
  panel.setAttribute("role", "region");
  panel.setAttribute("aria-label", "Spelling and style issues");
  btn.setAttribute("aria-controls", panel.id);
  const list = el("ul", "zen-lint-list");
  panel.append(list);
  wrap.append(btn, panel);

  let alerts: Alert[] = [];

  function render(): void {
    list.replaceChildren(...alerts.map((a, i) => {
      const sev = severityOf(a.severity);
      const item = el("button", `zen-lint-item zen-lint-item-${sev}`);
      item.type = "button";
      const line = el("span", "zen-lint-line", String(a.line));
      line.setAttribute("aria-label", `Line ${a.line}`);
      item.append(
        line,
        el("span", "zen-lint-severity", sev),
        el("span", "zen-lint-message", a.message),
        el("span", "zen-lint-match", a.match),
      );
      item.title = a.check;
      item.addEventListener("click", () => onSelect(a, i));
      const li = el("li");
      li.append(item);
      return li;
    }));
  }

  const onKey = (e: KeyboardEvent) => {
    if (e.key !== "Escape" || panel.hidden) return;
    e.preventDefault();
    e.stopPropagation();
    const inside = wrap.contains(wrap.ownerDocument.activeElement);
    close();
    if (inside) btn.focus();
  };

  function open(): void {
    if (!alerts.length) return;
    panel.hidden = false;
    btn.setAttribute("aria-expanded", "true");
    document.addEventListener("keydown", onKey, true);
  }
  function close(): void {
    panel.hidden = true;
    btn.setAttribute("aria-expanded", "false");
    document.removeEventListener("keydown", onKey, true);
  }

  btn.addEventListener("click", () => (panel.hidden ? open() : close()));

  return {
    el: wrap, button: btn, panel,
    update(next) {
      alerts = next ?? [];
      btn.textContent = alerts.length === 1 ? "1 issue" : `${alerts.length} issues`;
      btn.hidden = alerts.length === 0;
      if (!alerts.length) close();
      render();
    },
    open, close,
    destroy() {
      close();
      wrap.remove();
    },
  };
}
