// Waits until a page is ready to print: its plugins and layouts are mounted, its images and fonts are
// loaded, and it has stopped changing. Print Page and Print Site both call it, then window.print().
// After the timeout it gives up and says what was not ready, so a block that never finishes cannot
// keep the print panel from opening.
//
// The runtime does not mark what it has mounted, so "ready" is made of what can be observed:
//  - `mounted`: the promise of the runtime's mountAll, when the caller has it;
//  - a block that says it is still working: aria-busy="true" or data-zen-pending (a plugin that draws
//    asynchronously may set one while it works);
//  - images that have not loaded, document fonts that are loading;
//  - quiet: no change inside `root` for `quiet` ms, which lets a diagram finish drawing after mount.

export type ReadyOptions = {
  /** Give up after this many ms (default 5000). */
  timeout?: number;
  /** The page must not change for this many ms (default 100). */
  quiet?: number;
  /** The mount of the page's plugins; a rejection counts as done (the block shows its own error). */
  mounted?: PromiseLike<unknown>;
};

export const READY_TIMEOUT = 5000;
const POLL = 25;
const BLOCKS = "[data-zen-plugin], [data-zen-layout]";
const BUSY = '[aria-busy="true"], [data-zen-pending]';

const blockName = (b: Element) => b.getAttribute("data-zen-plugin") || b.getAttribute("data-zen-layout") || "block";

export function whenReady(root: HTMLElement, opts: ReadyOptions = {}): Promise<{ late: string[] }> {
  const timeout = opts.timeout ?? READY_TIMEOUT;
  const quiet = opts.quiet ?? 100;
  const doc = root.ownerDocument;
  const win = doc.defaultView;

  let lastChange = Date.now();
  let mounted = opts.mounted === undefined;
  if (opts.mounted) {
    // Mounting just ended: what it started (a diagram drawing) gets a quiet period to finish.
    const done = () => { mounted = true; lastChange = Date.now(); };
    opts.mounted.then(done, done);
  }
  const failed = new WeakSet<Element>(); // images the browser gave up on
  const onError = (e: Event) => { if (e.target instanceof Element) failed.add(e.target); };
  root.addEventListener("error", onError, true);

  const MO = win?.MutationObserver ?? (typeof MutationObserver !== "undefined" ? MutationObserver : undefined);
  const observer = MO ? new MO(() => { lastChange = Date.now(); }) : null;
  observer?.observe(root, { subtree: true, childList: true, attributes: true, characterData: true });

  /** What is not ready, by name. */
  const pending = (): string[] => {
    const out: string[] = [];
    if (!mounted) {
      for (const b of Array.from(root.querySelectorAll(BLOCKS))) out.push(blockName(b));
      if (out.length === 0) out.push("plugins");
      return out;
    }
    for (const b of Array.from(root.querySelectorAll(BUSY))) out.push(blockName(b));
    for (const img of Array.from(root.querySelectorAll("img"))) {
      if (!img.complete && !failed.has(img)) out.push(`image ${img.getAttribute("src") ?? ""}`);
    }
    const fonts = (doc as Document & { fonts?: { status: string } }).fonts;
    if (fonts && fonts.status === "loading") out.push("fonts");
    return out;
  };

  return new Promise((resolve) => {
    const start = Date.now();
    let timer: ReturnType<typeof setTimeout>;
    const finish = (late: string[]) => {
      clearTimeout(timer);
      observer?.disconnect();
      root.removeEventListener("error", onError, true);
      if (late.length > 0) console.warn(`zen: printing before ready: ${late.join(", ")}`);
      resolve({ late });
    };
    const tick = () => {
      const now = Date.now();
      const left = pending();
      if (left.length === 0 && now - lastChange >= quiet) return finish([]);
      if (now - start >= timeout) return finish(left);
      timer = setTimeout(tick, Math.min(POLL, Math.max(1, timeout - (now - start))));
    };
    timer = setTimeout(tick, 0);
  });
}
