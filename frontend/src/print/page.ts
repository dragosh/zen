// Print Page: ⌘P and File › Print Page… print the page that is shown, as it looks, through the print
// panel. The shell calls installPagePrint once per window; the print flow waits for the page to be
// ready (ready.ts), prepares it for paper, then opens the print panel: in the app through the service
// (printWindow; window.print() does nothing in the macOS web view), in a browser with window.print().
import { whenReady, type ReadyOptions } from "./ready";

/** The part of the service API this needs: the menu event of File › Print Page…. */
export type PagePrintApi = {
  on(event: "menu:print-page", fn: () => void): () => void;
  /** The app's native print panel for the calling window (absent in a browser). */
  printWindow?(title: string): Promise<void>;
};

export type PagePrintOptions = ReadyOptions & {
  /** What to print; default: the editor window's preview, else the article, else the body. */
  root?: HTMLElement;
  /** Opens the print panel; default: window.print(). The app passes its native print. */
  printer?(): Promise<void> | void;
};

function rootOf(doc: Document, given?: HTMLElement): HTMLElement {
  return given
    ?? doc.querySelector<HTMLElement>(".zen-editor-preview")
    ?? doc.querySelector<HTMLElement>("article.zen-page, .zen-page")
    ?? doc.body;
}

/**
 * What paper needs and CSS cannot do: details print open (put back afterwards), and an external link
 * whose text already is its URL does not print the URL a second time.
 */
export function prepareForPrint(root: HTMLElement): () => void {
  const opened: HTMLDetailsElement[] = [];
  for (const d of Array.from(root.querySelectorAll("details"))) {
    if (!d.open) {
      d.open = true;
      opened.push(d);
    }
  }
  for (const a of Array.from(root.querySelectorAll<HTMLAnchorElement>('a[data-zen-link="external"]'))) {
    if (a.textContent?.trim() === a.getAttribute("href")) a.setAttribute("data-zen-print", "url");
  }
  return () => { for (const d of opened) d.open = false; };
}

function once(fn: () => void): () => void {
  let done = false;
  return () => { if (!done) { done = true; fn(); } };
}

/** Waits for the page, prepares it and opens the print panel. Resolves with what was not ready in time. */
export async function printPage(doc: Document, opts: PagePrintOptions = {}): Promise<{ late: string[] }> {
  const root = rootOf(doc, opts.root);
  const result = await whenReady(root, opts);
  const win = doc.defaultView;
  const restore = once(prepareForPrint(root));
  win?.addEventListener("afterprint", restore, { once: true });
  if (opts.printer) {
    // The native panel is a sheet that does not report back: put the page back on the next
    // interaction after it, or after a minute at most.
    try {
      await opts.printer();
    } catch (err) {
      restore();
      throw err;
    }
    const later = () => {
      doc.removeEventListener("pointerdown", later, true);
      doc.removeEventListener("keydown", later, true);
      restore();
    };
    doc.addEventListener("pointerdown", later, true);
    doc.addEventListener("keydown", later, true);
    win?.setTimeout(later, 60_000);
  } else {
    win?.print();
  }
  return result;
}

/**
 * Wires ⌘P (a browser, and where the native menu does not take the key) and the menu event to the print
 * flow. A request while one is waiting is ignored. The menu event reaches every window; only the one
 * with the focus prints. Returns the remover.
 */
export function installPagePrint(doc: Document, api: PagePrintApi, opts: PagePrintOptions = {}): () => void {
  let busy = false;
  const run = () => {
    if (busy) return;
    busy = true;
    const printer = opts.printer ?? (api.printWindow ? () => api.printWindow!(doc.title) : undefined);
    void printPage(doc, { ...opts, printer })
      .catch((err) => console.error("zen: printing failed", err))
      .finally(() => { busy = false; });
  };
  const onKey = (e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "p") {
      e.preventDefault();
      run();
    }
  };
  doc.addEventListener("keydown", onKey);
  const off = api.on("menu:print-page", () => { if (doc.hasFocus()) run(); });
  return () => {
    doc.removeEventListener("keydown", onKey);
    off();
  };
}
