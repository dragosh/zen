// check.js — makes a :::check block a real to-do list outside a course. The boxes are enabled,
// ticks are kept in ctx.storage by label, and a Reset button shows once they differ from the
// markdown. In a course (layout: course) the layout owns the boxes, so this does nothing.
// Labels are read with textContent and compared as text; nothing is parsed as HTML.
const COURSE = '[data-zen-layout="course"]';
const label = (box) => (box.closest("li") || box.parentElement).textContent.replace(/\s+/g, " ").trim();

/** FNV-1a, 8 hex digits: keeps long or unusual page paths apart in a short key. */
function hash(s) {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) h = Math.imul(h ^ s.charCodeAt(i), 0x01000193) >>> 0;
  return h.toString(16).padStart(8, "0");
}
/** check-<page path>#<n> as the storage allows it: letters, digits . _ - and 64 characters at most. */
function storageKey(path, n) {
  const safe = path.replace(/[^A-Za-z0-9._-]/g, "-");
  const plain = `check-${safe}.${n}`;
  if (safe === path && plain.length <= 64) return plain;
  return `check-${safe.slice(0, 30)}-${hash(path)}.${n}`;
}

/** The saved ticks, or null when the stored value is not { version: 1, ticked: [...] }. */
function ticksOf(saved) {
  if (!saved || typeof saved !== "object" || saved.version !== 1 || !Array.isArray(saved.ticked)) return null;
  return new Set(saved.ticked.filter((t) => typeof t === "string"));
}

export default {
  async mount(el, ctx) {
    if (el.closest(COURSE)) return;
    const doc = el.ownerDocument;
    const boxes = Array.from(el.querySelectorAll('input[type="checkbox"]'));
    if (boxes.length === 0) return;
    const original = boxes.map((b) => b.checked);
    const index = Array.from(doc.querySelectorAll('[data-zen-plugin="check"]')).indexOf(el);
    const key = storageKey(ctx.page.path, Math.max(index, 0));
    let dead = false;
    let reset = null;

    for (const b of boxes) b.disabled = false;
    let saved = null;
    try {
      saved = ticksOf(await ctx.storage.get(key));
    } catch (err) {
      console.error("zen: checklist could not be read", err);
    }
    if (dead) return;
    if (saved) boxes.forEach((b) => { b.checked = saved.has(label(b)); });

    const differs = () => boxes.some((b, i) => b.checked !== original[i]);
    function sync() {
      if (differs() && !reset) {
        reset = doc.createElement("button");
        reset.type = "button";
        reset.className = "zen-check-reset";
        reset.textContent = "Reset";
        reset.setAttribute("aria-label", "Reset checklist");
        reset.addEventListener("click", onReset);
        const list = el.querySelector("ul, ol");
        if (list && list.parentNode) list.parentNode.insertBefore(reset, list.nextSibling);
        else el.append(reset);
      } else if (!differs() && reset) {
        reset.remove();
        reset = null;
      }
    }
    async function persist(value) {
      try {
        await ctx.storage.set(key, value);
      } catch (err) {
        console.error("zen: checklist could not be saved", err);
      }
    }
    function onChange() {
      sync();
      persist(differs() ? { version: 1, ticked: boxes.filter((b) => b.checked).map(label) } : null);
    }
    function onReset() {
      boxes.forEach((b, i) => { b.checked = original[i]; });
      sync();
      persist(null);
    }
    // Clicking the text of an item ticks its box, like a label would.
    function onClick(e) {
      const t = e.target;
      if (!t || typeof t.closest !== "function" || t.closest("input, a, button")) return;
      const li = t.closest("li");
      const box = li && boxes.find((b) => b.closest("li") === li);
      if (box && !box.disabled) box.click();
    }
    el.addEventListener("change", onChange);
    el.addEventListener("click", onClick);
    sync();

    return () => {
      dead = true;
      el.removeEventListener("change", onChange);
      el.removeEventListener("click", onClick);
      if (reset) reset.remove();
      for (const b of boxes) b.disabled = true;
    };
  },
};
