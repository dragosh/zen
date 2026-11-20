// Accessibility options (Settings › Accessibility): the window's zoom, the reader's text, Reduce motion and
// Increase contrast. Kept per machine in localStorage (`zen.a11y.*`), applied as attributes and variables on
// <html> as the theme is (theme.ts), and kept in step across windows through the storage event.
//
// Every stored value is parsed into a closed type and written to the page as a fixed string, so nothing from
// storage reaches a style as it is. A missing localStorage works for this window only.
export type Accessibility = {
  zoom: number; // 0.75 | 0.9 | 1 | 1.1 | 1.25 | 1.5 | 1.75 | 2
  textSize: number; // 14..28 (px)
  font: "serif" | "sans";
  lineSpacing: "normal" | "relaxed" | "loose";
  lineWidth: "narrow" | "normal" | "wide";
  underlineLinks: boolean;
  reduceMotion: "system" | "on";
  contrast: "system" | "more";
};

export const A11Y_DEFAULTS: Accessibility = {
  zoom: 1, textSize: 17, font: "serif", lineSpacing: "normal", lineWidth: "normal", underlineLinks: false, reduceMotion: "system", contrast: "system",
};
export const ZOOM_STEPS: readonly number[] = [0.75, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2];
export const TEXT_SIZE_MIN = 14;
export const TEXT_SIZE_MAX = 28;
export const A11Y_PREFIX = "zen.a11y.";

const FONTS = { serif: "var(--font-body)", sans: "var(--font-ui)" } as const;
const SPACING = { normal: "1.65", relaxed: "1.9", loose: "2.2" } as const;
const WIDTH = { narrow: "60ch", normal: "68ch", wide: "84ch" } as const;
const KEYS = Object.keys(A11Y_DEFAULTS) as (keyof Accessibility)[];

const clampSize = (n: number) => Math.min(TEXT_SIZE_MAX, Math.max(TEXT_SIZE_MIN, Math.round(n)));
const oneOf = <T extends string>(allowed: readonly T[], v: unknown, fallback: T): T => (allowed as readonly unknown[]).includes(v) ? (v as T) : fallback;

/** Reads one option from its stored string; anything invalid is the default. */
function parse<K extends keyof Accessibility>(key: K, raw: string | null): Accessibility[K] {
  const d = A11Y_DEFAULTS[key];
  if (raw === null) return d;
  switch (key) {
    case "zoom": {
      const n = /^\d(\.\d{1,2})?$/.test(raw) ? Number(raw) : NaN;
      return (ZOOM_STEPS.find((z) => z === n) ?? d) as Accessibility[K];
    }
    case "textSize": {
      const n = /^\d{2}$/.test(raw) ? Number(raw) : NaN;
      return (n >= TEXT_SIZE_MIN && n <= TEXT_SIZE_MAX ? n : d) as Accessibility[K];
    }
    case "font": return oneOf(["serif", "sans"] as const, raw, "serif") as Accessibility[K];
    case "lineSpacing": return oneOf(["normal", "relaxed", "loose"] as const, raw, "normal") as Accessibility[K];
    case "lineWidth": return oneOf(["narrow", "normal", "wide"] as const, raw, "normal") as Accessibility[K];
    case "underlineLinks": return (raw === "on") as Accessibility[K];
    case "reduceMotion": return oneOf(["system", "on"] as const, raw, "system") as Accessibility[K];
    case "contrast": return oneOf(["system", "more"] as const, raw, "system") as Accessibility[K];
  }
  return d;
}

/** The string stored for a value, "" for the default (the key is then removed). */
function serialise<K extends keyof Accessibility>(key: K, v: Accessibility[K]): string {
  if (v === A11Y_DEFAULTS[key]) return "";
  return key === "underlineLinks" ? "on" : String(v);
}

/** Cleans a value the way a read would, so a setter and storage agree. */
function clean(a: Partial<Accessibility>): Partial<Accessibility> {
  const out: Partial<Accessibility> = {};
  for (const k of KEYS) {
    if (a[k] === undefined) continue;
    const v = a[k];
    const raw = k === "textSize" ? String(clampSize(Number(v))) : k === "underlineLinks" ? (v ? "on" : "") : String(v);
    (out as Record<string, unknown>)[k] = parse(k, raw === "" ? null : raw);
  }
  return out;
}

// What this window holds when storage is unavailable (and always, as a copy of what it wrote).
let memory: Accessibility = { ...A11Y_DEFAULTS };
let persists = true;

/** False once a read or write of localStorage failed: the options then apply to this window only. */
export function a11yPersists(): boolean {
  try {
    localStorage.getItem(A11Y_PREFIX + "zoom");
    return persists;
  } catch {
    return false;
  }
}

export function storedAccessibility(): Accessibility {
  const out = { ...A11Y_DEFAULTS } as Record<string, unknown>;
  try {
    for (const k of KEYS) out[k] = parse(k, localStorage.getItem(A11Y_PREFIX + k));
    persists = true;
    return out as Accessibility;
  } catch {
    persists = false;
    return { ...memory };
  }
}

export function setAccessibility(patch: Partial<Accessibility>, doc: Document = document): Accessibility {
  const next = { ...storedAccessibility(), ...clean(patch) };
  memory = { ...next };
  try {
    for (const k of KEYS) {
      if (patch[k] === undefined) continue;
      const s = serialise(k, next[k]);
      if (s === "") localStorage.removeItem(A11Y_PREFIX + k);
      else localStorage.setItem(A11Y_PREFIX + k, s);
    }
  } catch { persists = false; /* this window only */ }
  applyAccessibility(next, doc);
  return next;
}

export function resetAccessibility(doc: Document = document): Accessibility {
  memory = { ...A11Y_DEFAULTS };
  try {
    for (const k of KEYS) localStorage.removeItem(A11Y_PREFIX + k);
  } catch { persists = false; }
  applyAccessibility(A11Y_DEFAULTS, doc);
  return { ...A11Y_DEFAULTS };
}

/** Sets the attributes and the reading variables on <html>. A default removes its variable, so the stylesheet's own value stands. */
export function applyAccessibility(a: Accessibility, doc: Document = document): void {
  const root = doc.documentElement;
  const flag = (name: "reduceMotion" | "contrast" | "linkUnderline", on: boolean, value: string) => {
    if (on) root.dataset[name] = value;
    else delete root.dataset[name];
  };
  flag("reduceMotion", a.reduceMotion === "on", "on");
  flag("contrast", a.contrast === "more", "more");
  flag("linkUnderline", a.underlineLinks === true, "on");
  const set = (name: string, value: string | null) => {
    if (value === null) root.style.removeProperty(name);
    else root.style.setProperty(name, value);
  };
  const size = clampSize(a.textSize);
  set("--zen-text-size", Number.isFinite(a.textSize) && size !== A11Y_DEFAULTS.textSize ? `${size}px` : null);
  set("--zen-text-font", a.font === "sans" ? FONTS.sans : null);
  set("--zen-line-height", a.lineSpacing === "relaxed" || a.lineSpacing === "loose" ? SPACING[a.lineSpacing] : null);
  set("--zen-measure", a.lineWidth === "narrow" || a.lineWidth === "wide" ? WIDTH[a.lineWidth] : null);
  if (!root.getAttribute("style")) root.removeAttribute("style");
}

// Where the window's zoom goes: the app gives Window.SetZoom (main.ts). Settings uses it when it is not given its own.
let zoomSink: ((z: number) => void) | undefined;
export function setZoomSink(fn: ((z: number) => void) | undefined): void { zoomSink = fn; }
export function currentZoomSink(): ((z: number) => void) | undefined { return zoomSink; }

/** Applies the stored options now and whenever another window changes them; zoom goes to `setZoom`. */
export function followAccessibility(doc: Document = document, setZoom?: (z: number) => void): () => void {
  if (setZoom) zoomSink = setZoom;
  const now = storedAccessibility();
  applyAccessibility(now, doc);
  if (setZoom && now.zoom !== A11Y_DEFAULTS.zoom) setZoom(now.zoom);
  const view = doc.defaultView;
  const onStorage = (e: StorageEvent) => {
    if (e.key !== null && !e.key.startsWith(A11Y_PREFIX)) return;
    const a = storedAccessibility();
    applyAccessibility(a, doc);
    setZoom?.(a.zoom);
  };
  view?.addEventListener("storage", onStorage);
  return () => view?.removeEventListener("storage", onStorage);
}

/** The step after `current` in `dir` (1 up, -1 down), or Actual Size for 0. The ends stay where they are. */
export function zoomStep(current: number, dir: 1 | -1 | 0): number {
  if (dir === 0) return 1;
  const eps = 1e-6;
  if (dir === 1) return ZOOM_STEPS.find((z) => z > current + eps) ?? ZOOM_STEPS[ZOOM_STEPS.length - 1];
  return [...ZOOM_STEPS].reverse().find((z) => z < current - eps) ?? ZOOM_STEPS[0];
}

/** One step of View › Zoom In / Zoom Out / Actual Size (or the keys): saved, applied and sent to the window. */
export function stepZoom(dir: 1 | -1 | 0, setZoom: ((z: number) => void) | undefined = zoomSink, doc: Document = document): number {
  const next = zoomStep(storedAccessibility().zoom, dir);
  setAccessibility({ zoom: next }, doc);
  setZoom?.(next);
  return next;
}

/** True when motion should be reduced: the option is On, or macOS asks for it. */
export function reducedMotion(doc: Document = document): boolean {
  if (doc.documentElement.dataset.reduceMotion === "on") return true;
  return doc.defaultView?.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;
}
