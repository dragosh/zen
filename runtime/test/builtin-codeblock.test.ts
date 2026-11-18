// The built-in codeblock plugin's frontend (builtins/plugins/codeblock), loaded
// as the app loads it: an ES module with a default export, imported by the runtime.
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi } from "../../apps/zenmark/frontend/src/api/fake";
import { installRuntime } from "../src/plugins";

let rt: ReturnType<typeof installRuntime>;
let root: HTMLElement;
let codeText: (pre: Element) => string;
let mod: unknown;

beforeEach(async () => {
  document.body.replaceChildren();
  rt = installRuntime({ importModule: async () => mod, api: createFakeApi(), navigate: () => {}, page: () => ({ path: "a.md", title: "A" }) });
  root = document.body.appendChild(document.createElement("article"));
  vi.resetModules();
  // @ts-ignore plain browser JavaScript outside src, no types
  mod = await import("../../builtins/plugins/codeblock/codeblock.js");
  ({ codeText } = mod as { codeText: (pre: Element) => string });
  await rt.load({ js: ["/_plugins/codeblock/codeblock.js?v=test"], css: [] });
});
afterEach(() => {
  rt.destroy();
  vi.restoreAllMocks();
});

const settle = async () => { for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0)); };

/** What the Go hook renders for one block. */
function render(inner: string, wrap = false): HTMLElement {
  root.innerHTML = `<div data-zen-plugin="codeblock" class="zen-codeblock${wrap ? " zen-codeblock-wrap" : ""}">${inner}</div>`;
  rt.mountAll(root);
  return root.firstElementChild as HTMLElement;
}

test("adds Wrap and Copy buttons inside the block", () => {
  const w = render("<pre><code>x = 1</code></pre>");
  const btns = Array.from(w.querySelectorAll(".zen-codeblock-tools button")).map((b) => b.textContent);
  expect(btns).toEqual(["Wrap", "Copy"]);
});

test("Wrap toggles the wrap class and aria-pressed", () => {
  const w = render("<pre><code>x</code></pre>");
  const b = w.querySelector<HTMLButtonElement>(".zen-codeblock-wrapbtn")!;
  expect(b.getAttribute("aria-pressed")).toBe("false");
  b.click();
  expect(w.classList.contains("zen-codeblock-wrap")).toBe(true);
  expect(b.getAttribute("aria-pressed")).toBe("true");
  b.click();
  expect(w.classList.contains("zen-codeblock-wrap")).toBe(false);
});

test("the wrap option starts pressed", () => {
  const w = render("<pre><code>x</code></pre>", true);
  expect(w.querySelector(".zen-codeblock-wrapbtn")!.getAttribute("aria-pressed")).toBe("true");
});

test("Copy writes the code without line numbers", async () => {
  const write = vi.fn(async (_: string) => {});
  Object.defineProperty(navigator, "clipboard", { value: { writeText: write }, configurable: true });
  const w = render('<pre class="chroma"><code><span class="line"><span class="ln">1</span><span class="cl">a</span></span>\n<span class="line"><span class="ln">2</span><span class="cl">b</span></span></code></pre>');
  const b = w.querySelector<HTMLButtonElement>(".zen-codeblock-copy")!;
  b.click();
  await settle();
  expect(write).toHaveBeenCalledWith("a\nb");
  expect(b.textContent).toBe("Copied");
});

test("codeText drops the gutter", () => {
  const pre = document.createElement("pre");
  pre.innerHTML = '<code><span class="ln">1</span>x</code>';
  expect(codeText(pre)).toBe("x");
});

test("unmount removes the buttons", () => {
  const w = render("<pre><code>x</code></pre>");
  rt.unmountAll();
  expect(w.querySelector(".zen-codeblock-tools")).toBeNull();
});
