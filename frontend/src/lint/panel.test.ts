// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderPanel as render, type LintPanel } from "./panel";
import { alert } from "./test-util";

const key = (target: EventTarget, k: string) => {
  const ev = new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true });
  target.dispatchEvent(ev);
  return ev;
};

const made: LintPanel[] = [];
const renderPanel = (onSelect: Parameters<typeof render>[0]) => {
  const p = render(onSelect);
  made.push(p);
  return p;
};
afterEach(() => {
  for (const p of made.splice(0)) p.destroy();
  document.body.replaceChildren();
});

describe("renderPanel", () => {
  const alerts = [
    alert({ line: 3, match: "teh", severity: "error", message: "Spelling" }),
    alert({ line: 7, match: "very", severity: "warning", check: "Vale.Avoid", message: "Avoid very" }),
    alert({ line: 9, match: "utilize", severity: "suggestion", message: "Use use" }),
  ];

  it("shows the count, hidden when 0 or unavailable", () => {
    const p = renderPanel(() => {});
    document.body.append(p.el);
    expect(p.button.hidden).toBe(true);
    p.update(alerts);
    expect(p.button.hidden).toBe(false);
    expect(p.button.textContent).toBe("3 issues");
    p.update([alerts[0]]);
    expect(p.button.textContent).toBe("1 issue");
    p.update([]);
    expect(p.button.hidden).toBe(true);
    p.update(alerts);
    p.update(null);
    expect(p.button.hidden).toBe(true);
  });

  it("toggles a list of alerts: line, severity, message, match", () => {
    const p = renderPanel(() => {});
    document.body.append(p.el);
    p.update(alerts);
    expect(p.panel.hidden).toBe(true);
    p.button.click();
    expect(p.panel.hidden).toBe(false);
    expect(p.button.getAttribute("aria-expanded")).toBe("true");
    const items = Array.from(p.panel.querySelectorAll("li"));
    expect(items).toHaveLength(3);
    expect(items[1].querySelector(".zen-lint-line")!.textContent).toBe("7");
    expect(items[1].querySelector(".zen-lint-severity")!.textContent).toBe("warning");
    expect(items[1].querySelector(".zen-lint-message")!.textContent).toBe("Avoid very");
    expect(items[1].querySelector(".zen-lint-match")!.textContent).toBe("very");
    expect(items[1].querySelector("button")!.classList.contains("zen-lint-item-warning")).toBe(true);
    p.button.click();
    expect(p.panel.hidden).toBe(true);
    expect(p.button.getAttribute("aria-expanded")).toBe("false");
  });

  it("clicking an item calls onSelect with the alert and its index", () => {
    const onSelect = vi.fn();
    const p = renderPanel(onSelect);
    document.body.append(p.el);
    p.update(alerts);
    p.button.click();
    p.panel.querySelectorAll<HTMLButtonElement>("li button")[2].click();
    expect(onSelect).toHaveBeenCalledWith(alerts[2], 2);
  });

  it("Esc closes it, prevents the default and refocuses the button", () => {
    const p = renderPanel(() => {});
    document.body.append(p.el);
    p.update(alerts);
    p.button.click();
    const item = p.panel.querySelector<HTMLButtonElement>("li button")!;
    item.focus();
    const ev = key(item, "Escape");
    expect(ev.defaultPrevented).toBe(true);
    expect(p.panel.hidden).toBe(true);
    expect(document.activeElement).toBe(p.button);
    // Closed: Esc is left to others.
    expect(key(document.body, "Escape").defaultPrevented).toBe(false);
  });

  it("Esc anywhere in the document closes an open panel", () => {
    const p = renderPanel(() => {});
    document.body.append(p.el);
    p.update(alerts);
    p.button.click();
    expect(key(document.body, "Escape").defaultPrevented).toBe(true);
    expect(p.panel.hidden).toBe(true);
  });

  it("a new result with no alerts closes the panel", () => {
    const p = renderPanel(() => {});
    document.body.append(p.el);
    p.update(alerts);
    p.button.click();
    p.update([]);
    expect(p.panel.hidden).toBe(true);
    expect(p.panel.querySelectorAll("li")).toHaveLength(0);
  });

  it("keeps Vale text as text", () => {
    const p = renderPanel(() => {});
    document.body.append(p.el);
    const evil = '<img src=x onerror="alert(1)"><script>alert(1)</script>';
    p.update([alert({ match: evil, message: evil, check: evil })]);
    p.button.click();
    expect(p.panel.querySelector("script, img")).toBeNull();
    expect(p.panel.querySelector(".zen-lint-message")!.textContent).toBe(evil);
    expect(p.panel.querySelector(".zen-lint-match")!.textContent).toBe(evil);
  });

  it("destroy removes it and its listeners", () => {
    const p = renderPanel(() => {});
    document.body.append(p.el);
    p.update(alerts);
    p.button.click();
    p.destroy();
    expect(p.el.isConnected).toBe(false);
    expect(key(document.body, "Escape").defaultPrevented).toBe(false);
  });
});
