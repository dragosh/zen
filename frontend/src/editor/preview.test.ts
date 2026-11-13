import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { createFakeApi } from "../api/fake";
import { createPreview } from "./preview";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());
import { vi } from "vitest";

describe("preview", () => {
  it("uses api.preview only, debounced 150 ms", async () => {
    const api = createFakeApi();
    const p = createPreview(api);
    p.update("README.md", "# One");
    p.update("README.md", "# Two");
    await vi.advanceTimersByTimeAsync(149);
    expect(api.calls).toHaveLength(0);
    await vi.advanceTimersByTimeAsync(1);
    expect(api.calls.map((c) => c.method)).toEqual(["preview"]);
    expect(api.callsTo("preview")[0]).toEqual(["README.md", "# Two"]);
    const article = p.el.querySelector("article.zen-page")!;
    expect(article.querySelector("h1")!.textContent).toBe("Two");
    p.destroy();
  });

  it("the latest response wins", async () => {
    const api = createFakeApi();
    let release!: () => void;
    const slow = new Promise<void>((r) => (release = r));
    const real = api.preview;
    let n = 0;
    api.preview = async (path, text) => {
      const page = await real(path, text);
      if (n++ === 0) await slow;
      return page;
    };
    const p = createPreview(api);
    p.update("README.md", "# Old");
    await vi.advanceTimersByTimeAsync(150);
    p.update("README.md", "# New");
    await vi.advanceTimersByTimeAsync(150);
    release();
    await vi.advanceTimersByTimeAsync(0);
    expect(p.el.querySelector("h1")!.textContent).toBe("New");
    p.destroy();
  });

  it("cancels link clicks inside the preview", async () => {
    const api = createFakeApi();
    const p = createPreview(api);
    p.update("README.md", "[Install](guides/install.md) and [web](https://example.com)");
    await vi.advanceTimersByTimeAsync(150);
    const links = p.el.querySelectorAll("a");
    expect(links.length).toBe(2);
    for (const a of Array.from(links)) {
      const ev = new MouseEvent("click", { bubbles: true, cancelable: true });
      a.dispatchEvent(ev);
      expect(ev.defaultPrevented).toBe(true);
    }
    expect(api.callsTo("openExternal")).toHaveLength(0);
    p.destroy();
  });

  it("does not update after destroy", async () => {
    const api = createFakeApi();
    const p = createPreview(api);
    p.update("README.md", "# X");
    p.destroy();
    await vi.advanceTimersByTimeAsync(200);
    expect(api.calls).toHaveLength(0);
  });
});
