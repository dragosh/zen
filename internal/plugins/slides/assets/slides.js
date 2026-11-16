// slides.js — the "slides" page layout. The page is split on <hr> (or, without
// any, at each h1/h2) and shown one slide at a time, fitted to the pane.
// Plain browser JavaScript; the page's own nodes are moved, never re-parsed.
const BASE_W = 960; // slide width in CSS px before scaling to the pane
const NOTE = /^note:/i;
const STEP_KEYS = { ArrowRight: 1, " ": 1, PageDown: 1, ArrowLeft: -1, PageUp: -1 };
// Focus in one of these outside the deck (the sidebar tree, a menu, a toolbar
// button) owns the arrow keys and Space: the deck leaves them alone.
const WIDGETS = "nav, aside, header, [role=tree], [role=treeitem], [role=menu], [role=menubar], [role=menuitem], [role=listbox], [role=tablist], [role=toolbar], [role=dialog], button, a[href], summary";

/** "16:9" -> [16, 9]; anything else -> 16:9. */
function parseRatio(text) {
  const m = /^(\d{1,3}(?:\.\d+)?)\s*[:/]\s*(\d{1,3}(?:\.\d+)?)$/.exec(String(text || "").trim());
  if (m && Number(m[1]) > 0 && Number(m[2]) > 0) return [Number(m[1]), Number(m[2])];
  return [16, 9];
}

const isBlank = (n) => n.nodeType === 8 || (n.nodeType === 3 && n.textContent.trim() === "");
const tag = (n) => (n.nodeType === 1 ? n.tagName : "");

/** Groups the layout's nodes into slides: arrays of nodes. */
function split(nodes) {
  const nodes2 = nodes.filter((n) => !isBlank(n));
  const hasRule = nodes2.some((n) => tag(n) === "HR");
  const groups = [[]];
  for (const n of nodes2) {
    const cut = hasRule ? tag(n) === "HR" : (tag(n) === "H1" || tag(n) === "H2") && groups[groups.length - 1].length > 0;
    if (cut) {
      groups.push(hasRule ? [] : [n]);
    } else if (!(hasRule && tag(n) === "HR")) {
      groups[groups.length - 1].push(n);
    }
  }
  return groups.filter((g) => g.length > 0);
}

function el(doc, name, cls, text) {
  const e = doc.createElement(name);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

export default {
  mount(host, ctx) {
    if (ctx.kind !== "layout") return;
    const doc = host.ownerDocument;
    const original = Array.from(host.childNodes);
    const groups = split(original);
    const total = groups.length;
    if (total === 0) return;

    const [rw, rh] = parseRatio(ctx.attrs.ratio);
    const height = (BASE_W * rh) / rw;

    // ---- build the DOM ----
    const root = el(doc, "div", "slides-root");
    root.style.setProperty("--slides-ratio", `${rw} / ${rh}`);
    root.style.setProperty("--slides-w", `${BASE_W}px`);
    root.style.setProperty("--slides-h", `${height}px`);
    root.tabIndex = -1;
    if (ctx.attrs.transition === "fade") root.dataset.transition = "fade";

    const viewport = el(doc, "div", "slides-viewport");
    const stage = el(doc, "div", "slides-stage");
    viewport.append(stage);

    const slides = groups.map((nodes) => {
      const slide = el(doc, "section", "slides-slide");
      const content = el(doc, "div", "slides-content");
      const notes = el(doc, "div", "slides-notes");
      notes.hidden = true;
      for (const n of nodes) {
        (n.nodeType === 1 && tag(n) === "P" && NOTE.test(n.textContent.trim()) ? notes : content).append(n);
      }
      slide.append(content, notes);
      stage.append(slide);
      return slide;
    });

    const overview = el(doc, "div", "slides-overview");
    const bar = el(doc, "div", "slides-bar");
    const progress = el(doc, "div", "slides-progress");
    progress.setAttribute("role", "progressbar");
    progress.setAttribute("aria-valuemin", "1");
    progress.setAttribute("aria-valuemax", String(total));
    const fill = el(doc, "div", "slides-progress-fill");
    progress.append(fill);
    const count = el(doc, "span", "slides-count");
    count.setAttribute("aria-live", "polite");
    const hint = el(doc, "span", "slides-hint", "O overview · F full page");
    const full = el(doc, "button", "slides-full", "Full page");
    full.type = "button";
    full.title = "Full page (F, Esc to leave)";
    full.setAttribute("aria-pressed", "false");
    bar.append(progress, count, hint, full);
    root.append(viewport, overview, bar);
    host.replaceChildren(root);

    // ---- state ----
    let index = 0;
    const fromAnchor = (a) => {
      const m = /^slide-(\d+)$/.exec(a || "");
      return m && Number(m[1]) >= 1 ? Math.min(Number(m[1]), total) - 1 : -1;
    };

    function draw() {
      slides.forEach((s, i) => {
        s.hidden = i !== index;
      });
      count.textContent = `${index + 1} / ${total}`;
      progress.setAttribute("aria-valuenow", String(index + 1));
      fill.style.width = `${((index + 1) / total) * 100}%`;
    }
    function go(n, quiet) {
      const next = Math.max(0, Math.min(total - 1, n));
      if (next === index && quiet !== false) return;
      index = next;
      draw();
      if (!quiet) ctx.navigate(ctx.page.path, `slide-${index + 1}`);
    }

    const start = fromAnchor(ctx.anchor());
    index = start >= 0 ? start : 0;
    draw();

    // ---- fit to the pane ----
    function fit() {
      const w = viewport.clientWidth;
      const h = viewport.clientHeight;
      if (!w || !h) return;
      const scale = Math.min(w / BASE_W, h / height);
      stage.style.transform = `translate(-50%, -50%) scale(${scale})`;
    }
    let ro = null;
    if (typeof ResizeObserver === "function") {
      ro = new ResizeObserver(fit);
      ro.observe(viewport);
    }
    fit();

    // ---- overview ----
    function overviewOpen() {
      return root.dataset.overview === "true";
    }
    function setOverview(on) {
      if (!on) {
        delete root.dataset.overview;
        overview.replaceChildren();
        return;
      }
      const thumbs = slides.map((s, i) => {
        const b = el(doc, "button", "slides-thumb");
        b.type = "button";
        b.setAttribute("aria-label", `Slide ${i + 1}`);
        if (i === index) b.dataset.current = "true";
        const inner = el(doc, "div", "slides-thumb-slide");
        inner.append(s.firstElementChild.cloneNode(true));
        b.append(inner, el(doc, "span", "slides-thumb-n", String(i + 1)));
        b.addEventListener("click", () => {
          setOverview(false);
          go(i, false);
        });
        return b;
      });
      overview.replaceChildren(...thumbs);
      root.dataset.overview = "true";
    }

    // ---- full page: the deck covers the window (and goes full screen when the
    // webview allows it); F or the bar button toggles, Esc leaves ----
    let screen = false; // the deck is the fullscreen element
    function fullPage() {
      return root.dataset.fullpage === "true";
    }
    function setFullPage(on) {
      if (on === fullPage()) return;
      if (on) {
        root.dataset.fullpage = "true";
        root.focus({ preventScroll: true });
        if (typeof root.requestFullscreen === "function") {
          const r = root.requestFullscreen();
          if (r && typeof r.catch === "function") r.catch(() => {});
        }
      } else {
        delete root.dataset.fullpage;
        if (doc.fullscreenElement === root && typeof doc.exitFullscreen === "function") {
          const r = doc.exitFullscreen();
          if (r && typeof r.catch === "function") r.catch(() => {});
        }
      }
      full.setAttribute("aria-pressed", String(on));
      fit();
    }
    // Esc in real full screen is taken by the browser: leave full page with it.
    function onScreen() {
      if (doc.fullscreenElement === root) screen = true;
      else if (screen) {
        screen = false;
        setFullPage(false);
      }
    }
    doc.addEventListener("fullscreenchange", onScreen);
    full.addEventListener("click", () => setFullPage(!fullPage()));
    // A click on the deck gives it the keys (and takes them from the sidebar).
    function onPointer(e) {
      if (!(e.target && e.target.closest && e.target.closest("button"))) root.focus({ preventScroll: true });
    }
    root.addEventListener("pointerdown", onPointer);

    // ---- keys ----
    function onKey(e) {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey || !host.isConnected) return;
      const t = e.target;
      if (t && t.nodeType === 1) {
        if (/^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) || t.isContentEditable) return;
        if (!root.contains(t) && t.closest(WIDGETS)) return;
      }
      const k = e.key;
      if (k in STEP_KEYS) go(index + STEP_KEYS[k], false);
      else if (k === "Home") go(0, false);
      else if (k === "End") go(total - 1, false);
      else if (k === "o" || k === "O") setOverview(!overviewOpen());
      else if (k === "Escape" && overviewOpen()) setOverview(false);
      else if (k === "Escape" && fullPage()) setFullPage(false);
      else if (k === "f" || k === "F") setFullPage(!fullPage());
      else if (k === "p" || k === "P") {
        if (typeof window.print === "function") window.print();
      } else return;
      e.preventDefault();
    }
    doc.addEventListener("keydown", onKey);

    const offAnchor = ctx.onAnchor((a) => {
      const n = fromAnchor(a);
      if (n >= 0 && n !== index) go(n, true);
    });

    return () => {
      doc.removeEventListener("keydown", onKey);
      doc.removeEventListener("fullscreenchange", onScreen);
      root.removeEventListener("pointerdown", onPointer);
      if (fullPage()) setFullPage(false);
      offAnchor();
      if (ro) ro.disconnect();
      host.replaceChildren(...original);
    };
  },
};
