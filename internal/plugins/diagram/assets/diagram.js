// Renders .zen-diagram placeholders with the vendored mermaid bundle, which is
// loaded lazily as a classic script (it is an IIFE defining globalThis.mermaid).
export const MERMAID_URL = "/_plugins/diagram/mermaid.min.js";
const THEMES = ["neutral", "dark", "default"];
let loading = null;
let seq = 0;

/** Loads mermaid once; resolves with globalThis.mermaid. */
export function loadMermaid(doc = document, g = globalThis) {
  if (g.mermaid) return Promise.resolve(g.mermaid);
  if (loading && loading.doc === doc) return loading.p;
  doc.head.querySelectorAll("script").forEach((s) => s.dataset && s.dataset.zenDiagram !== undefined && s.remove());
  const p = new Promise((resolve, reject) => {
    const s = doc.createElement("script");
    s.src = MERMAID_URL;
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

/** Renders every not-yet-rendered diagram under root. */
export async function renderAll(root, mermaid) {
  const doc = root.ownerDocument || root;
  for (const el of root.querySelectorAll(".zen-diagram:not([data-rendered])")) {
    el.dataset.rendered = "";
    const pre = el.querySelector("pre");
    if (!pre) continue;
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
      const box = doc.createElement("div");
      box.className = "zen-plugin-error";
      box.textContent = "diagram: " + (err && err.message ? err.message : String(err));
      el.append(box);
    }
  }
}

async function run(root) {
  if (!root.querySelector(".zen-diagram:not([data-rendered])")) return;
  try {
    await renderAll(root, await loadMermaid(root.ownerDocument || document));
  } catch (err) {
    console.error("diagram:", err);
  }
}

if (typeof document !== "undefined") {
  document.addEventListener("zen:page-rendered", (e) => run((e.detail && e.detail.root) || document.body));
  // The first page-rendered event fires before this module has loaded.
  run(document.body);
}
