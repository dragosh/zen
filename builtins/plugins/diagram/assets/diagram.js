// Renders .zen-diagram placeholders with the vendored mermaid bundle, which is
// loaded lazily as a classic script (it is an IIFE defining globalThis.mermaid).
export const MERMAID_URL = "/_plugins/diagram/mermaid.min.js";
const THEMES = ["neutral", "dark", "default"];
let loading = null;
let seq = 0;

/** Loads mermaid once; resolves with globalThis.mermaid. */
export function loadMermaid(doc = document, g = globalThis, url = MERMAID_URL) {
  if (g.mermaid) return Promise.resolve(g.mermaid);
  if (loading && loading.doc === doc) return loading.p;
  doc.head.querySelectorAll("script").forEach((s) => s.dataset && s.dataset.zenDiagram !== undefined && s.remove());
  const p = new Promise((resolve, reject) => {
    const s = doc.createElement("script");
    s.src = url;
    s.dataset.zenDiagram = "";
    s.addEventListener("load", () => (g.mermaid ? resolve(g.mermaid) : reject(new Error("mermaid did not load"))));
    s.addEventListener("error", () => reject(new Error("could not load mermaid")));
    doc.head.append(s);
  });
  loading = { doc, p };
  p.catch(() => { if (loading && loading.p === p) loading = null; });
  return p;
}

function appIsDark(doc) {
  const t = doc.documentElement && doc.documentElement.dataset.theme;
  if (t === "dark") return true;
  if (t === "light") return false;
  const mm = doc.defaultView && doc.defaultView.matchMedia;
  return !!(mm && mm.call(doc.defaultView, "(prefers-color-scheme: dark)").matches);
}

/** The mermaid theme for el: its data-theme, forced to dark/neutral by the app theme. */
export function mermaidTheme(el, doc = document) {
  if (appIsDark(doc)) return "dark";
  const t = THEMES.includes(el.dataset.theme) ? el.dataset.theme : "neutral";
  return t === "dark" ? "neutral" : t;
}

/** Renders one diagram wrapper (once) into an svg, or its error under the source. */
export async function renderOne(el, mermaid) {
  if (el.dataset.rendered !== undefined) return;
  el.dataset.rendered = "";
  const doc = el.ownerDocument || document;
  const pre = el.querySelector("pre");
  if (!pre) return;
  const id = "zen-diagram-" + ++seq;
  mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: mermaidTheme(el, doc) });
  try {
    const { svg } = await mermaid.render(id, pre.textContent);
    const box = doc.createElement("div");
    box.className = "zen-diagram-svg";
    box.innerHTML = svg; // mermaid output, sanitised by mermaid in strict mode
    pre.remove();
    el.append(box);
  } catch (err) {
    const stray = doc.getElementById && doc.getElementById("d" + id);
    if (stray) stray.remove();
    showError(el, err);
  }
}

function showError(el, err) {
  const box = (el.ownerDocument || document).createElement("div");
  box.className = "zen-plugin-error";
  const msg = err && err.message ? err.message : String(err);
  box.textContent = "diagram: " + msg;
  el.append(box);
  // The Console shows the page's console, so a broken diagram leaves a line there too.
  const first = ("diagram: " + msg).split("\n")[0];
  console.warn(first.length > 200 ? first.slice(0, 199) + "…" : first);
}

/** Renders every not-yet-rendered diagram under root. */
export async function renderAll(root, mermaid) {
  for (const el of root.querySelectorAll(".zen-diagram:not([data-rendered])")) await renderOne(el, mermaid);
}

// The page runtime mounts this on every [data-zen-plugin="diagram"] wrapper.
export default {
  async mount(wrapper, ctx) {
    // The renderer's [data-zen-plugin] wrapper holds one .zen-diagram (with its theme).
    const el = (String(wrapper.className || "").split(/\s+/).includes("zen-diagram") ? wrapper : wrapper.querySelector(".zen-diagram")) || wrapper;
    // ctx.assetUrl finds the bundle in the app and in a site made by `zenmd build`.
    const url = ctx && typeof ctx.assetUrl === "function" ? ctx.assetUrl("mermaid.min.js") : MERMAID_URL;
    let mermaid;
    try {
      mermaid = await loadMermaid(el.ownerDocument || document, globalThis, url);
    } catch (err) {
      showError(el, err);
      return;
    }
    await renderOne(el, mermaid);
  },
};
