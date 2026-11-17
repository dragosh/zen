// Print Page: ⌘P and File › Print Page… print the page that is shown, as it looks, through the print
// panel. The shell calls installPagePrint once per window; the print flow waits for the page to be
// ready (ready.ts), prepares it for paper, then calls window.print().
import { whenReady, type ReadyOptions } from "./ready";

/** The part of the service API this needs: the menu event of File › Print Page…. */
export type PagePrintApi = { on(event: "menu:print-page", fn: () => void): () => void };

export type PagePrintOptions = ReadyOptions & {
  /** What to print; default: the editor window's preview, else the article, else the body. */
  root?: HTMLElement;
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

/** Waits for the page, prepares it and opens the print panel. Resolves with what was not ready in time. */
export async function printPage(doc: Document, opts: PagePrintOptions = {}): Promise<{ late: string[] }> {
  const root = rootOf(doc, opts.root);
  const result = await whenReady(root, opts);
  const win = doc.defaultView;
  const restore = prepareForPrint(root);
  win?.addEventListener("afterprint", restore, { once: true });
  win?.print();
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
    void printPage(doc, opts).finally(() => { busy = false; });
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
