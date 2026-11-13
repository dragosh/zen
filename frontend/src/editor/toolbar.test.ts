import { describe, expect, it, vi } from "vitest";
import { createToolbar, TOOLS } from "./toolbar";

describe("toolbar", () => {
  it("has the toolbar role and one labelled button per command", () => {
    const bar = createToolbar(() => {});
    expect(bar.getAttribute("role")).toBe("toolbar");
    expect(bar.getAttribute("aria-label")).toBeTruthy();
    const buttons = Array.from(bar.querySelectorAll("button"));
    expect(buttons.map((b) => b.dataset.command)).toEqual(
      ["bold", "italic", "strike", "h1", "h2", "quote", "bullet", "numbered", "task", "code", "link", "table"]);
    for (const b of buttons) expect(b.getAttribute("aria-label")).toBeTruthy();
  });

  it("tooltips include the shortcut", () => {
    const bar = createToolbar(() => {});
    const title = (id: string) => bar.querySelector<HTMLButtonElement>(`[data-command=${id}]`)!.title;
    expect(title("bold")).toMatch(/⌘B/);
    expect(title("italic")).toMatch(/⌘I/);
    expect(title("link")).toMatch(/⌘K/);
    expect(TOOLS.find((t) => t.id === "bold")!.key).toBe("b");
  });

  it("runs the command on click", () => {
    const run = vi.fn();
    const bar = createToolbar(run);
    bar.querySelector<HTMLButtonElement>("[data-command=quote]")!.click();
    expect(run).toHaveBeenCalledWith("quote");
  });

  it("uses a roving tabindex moved by arrow keys, Home and End", () => {
    const bar = createToolbar(() => {});
    document.body.append(bar);
    const buttons = Array.from(bar.querySelectorAll("button"));
    expect(buttons.filter((b) => b.tabIndex === 0)).toHaveLength(1);
    buttons[0].focus();
    const key = (k: string) => bar.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));
    key("ArrowRight");
    expect(document.activeElement).toBe(buttons[1]);
    expect(buttons[1].tabIndex).toBe(0);
    expect(buttons[0].tabIndex).toBe(-1);
    key("ArrowLeft");
    key("ArrowLeft");
    expect(document.activeElement).toBe(buttons[buttons.length - 1]);
    key("Home");
    expect(document.activeElement).toBe(buttons[0]);
    key("End");
    expect(document.activeElement).toBe(buttons[buttons.length - 1]);
    bar.remove();
  });
});
