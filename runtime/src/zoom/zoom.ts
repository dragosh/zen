// Image zoom (specs/ui/image-zoom): a click on an image in the page shows it over the window, fitted;
// click, + - 0 1 and a pinch zoom, a larger image pans, Esc / ✕ / a click outside close it.
// Plain DOM, no innerHTML: the copy takes src and alt of an image the sanitiser already allowed.

export const ZOOM_MIN = 0.1;
export const ZOOM_MAX = 8;
const STEP = 1.25;
const SMALL = 48; // icons and badges (both sides at most this) are not zoomed
const PAD = 32; // room around the fitted image

export interface ImageZoom {
  open(img: HTMLImageElement): void;
  close(): void;
  readonly isOpen: boolean;
  destroy(): void;
}

const isImg = (n: unknown): n is HTMLImageElement => !!n && (n as Element).nodeType === 1 && (n as Element).tagName === "IMG";

/** The image a click on target should zoom, or null when the click is not for zoom. */
export function zoomableImage(target: EventTarget | null, root: HTMLElement): HTMLImageElement | null {
  if (!isImg(target) || !root.contains(target)) return null;
  if (target.closest("a[href], button, [data-zen-plugin], [data-zen-nozoom]")) return null;
  const w = target.naturalWidth, h = target.naturalHeight;
  if (!target.complete || !w || !h) return null;
  if (w <= SMALL && h <= SMALL) return null;
  return target;
}

const clamp = (s: number) => Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, s));

function iconButton(doc: Document, label: string, text: string, onClick: () => void, className = ""): HTMLButtonElement {
  const b = doc.createElement("button");
  b.type = "button";
  b.className = `zen-zoom-btn ${className}`.trim();
  b.title = label;
  b.setAttribute("aria-label", label);
  b.textContent = text;
  b.addEventListener("click", (e) => { e.stopPropagation(); onClick(); });
  return b;
}

export function installImageZoom(root: HTMLElement): ImageZoom {
  const doc = root.ownerDocument;
  const win = doc.defaultView ?? window;
  let view: { close(): void } | null = null;

  function open(source: HTMLImageElement): void {
    view?.close();
    const nw = source.naturalWidth, nh = source.naturalHeight;
    const returnFocus = doc.activeElement as HTMLElement | null;

    const overlay = doc.createElement("div");
    overlay.className = "zen-zoom";
    overlay.setAttribute("role", "dialog");
    overlay.setAttribute("aria-modal", "true");
    overlay.setAttribute("aria-label", source.alt ? `Image: ${source.alt}` : "Image");

    const stage = doc.createElement("div");
    stage.className = "zen-zoom-stage";
    const img = doc.createElement("img");
    img.className = "zen-zoom-img";
    img.src = source.currentSrc || source.src;
    img.alt = source.alt;
    img.draggable = false;
    stage.append(img);

    const bar = doc.createElement("div");
    bar.className = "zen-zoom-bar";
    const label = doc.createElement("span");
    label.className = "zen-zoom-scale";
    label.setAttribute("aria-live", "polite");
    const closeBtn = iconButton(doc, "Close (Esc)", "✕", () => close());
    bar.append(
      iconButton(doc, "Zoom out (−)", "−", () => setScale(scale / STEP)),
      label,
      iconButton(doc, "Zoom in (+)", "+", () => setScale(scale * STEP)),
      iconButton(doc, "Fit (0)", "Fit", () => setScale(fit()), "zen-zoom-fit"),
      closeBtn,
    );
    overlay.append(stage, bar);

    /** The scale that fits the image in the window, never above 100%. */
    const fit = () => {
      const vw = (stage.clientWidth || win.innerWidth) - 2 * PAD;
      const vh = (stage.clientHeight || win.innerHeight) - 2 * PAD;
      return clamp(Math.min(1, vw / nw, vh / nh));
    };
    let scale = 1;

    /** Sets the scale, keeping the image point under (x, y) (window coordinates) where it was. */
    function setScale(next: number, x?: number, y?: number): void {
      next = clamp(next);
      const r = img.getBoundingClientRect();
      const s = stage.getBoundingClientRect();
      const px = x ?? s.left + s.width / 2, py = y ?? s.top + s.height / 2;
      const fx = r.width ? (px - r.left) / r.width : 0.5;
      const fy = r.height ? (py - r.top) / r.height : 0.5;
      scale = next;
      img.style.width = `${nw * scale}px`;
      img.style.height = `${nh * scale}px`;
      label.textContent = `${Math.round(scale * 100)}%`;
      overlay.classList.toggle("zen-zoom-panning", scale > fit() + 1e-6);
      const r2 = img.getBoundingClientRect();
      stage.scrollLeft += r2.left + fx * r2.width - px;
      stage.scrollTop += r2.top + fy * r2.height - py;
    }

    /** Click on the image: fit ↔ a closer look (100% for a large image, else twice the fit). */
    const toggle = (e: MouseEvent) => {
      e.stopPropagation();
      const f = fit();
      const closer = f < 1 ? 1 : clamp(f * 2);
      setScale(Math.abs(scale - f) < 1e-3 ? closer : f, e.clientX, e.clientY);
    };

    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const k = e.key;
      if (k === "Escape") close();
      else if (k === "+" || k === "=") setScale(scale * STEP);
      else if (k === "-" || k === "_") setScale(scale / STEP);
      else if (k === "0") setScale(fit());
      else if (k === "1") setScale(1);
      else return;
      e.preventDefault();
      e.stopPropagation();
    };
    // A pinch on the trackpad arrives as a wheel event with ctrlKey.
    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey) return;
      e.preventDefault();
      setScale(scale * Math.exp(-e.deltaY * 0.01), e.clientX, e.clientY);
    };
    const onResize = () => { if (Math.abs(scale - fitAtOpen) < 1e-3) setScale(fitAtOpen = fit()); };

    img.addEventListener("click", toggle);
    stage.addEventListener("click", () => close()); // outside the image
    overlay.addEventListener("wheel", onWheel, { passive: false });
    doc.addEventListener("keydown", onKey, true);
    win.addEventListener("resize", onResize);

    doc.body.append(overlay);
    let fitAtOpen = fit();
    setScale(fitAtOpen);
    closeBtn.focus();

    function close(): void {
      if (view !== handle) return;
      view = null;
      doc.removeEventListener("keydown", onKey, true);
      win.removeEventListener("resize", onResize);
      overlay.remove();
      if (returnFocus?.isConnected) returnFocus.focus?.();
    }
    const handle = { close };
    view = handle;
  }

  const onClick = (e: MouseEvent) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const img = zoomableImage(e.target, root);
    if (!img) return;
    e.preventDefault();
    open(img);
  };
  root.addEventListener("click", onClick);

  return {
    open,
    close: () => view?.close(),
    get isOpen() { return view !== null; },
    destroy() {
      root.removeEventListener("click", onClick);
      view?.close();
    },
  };
}
