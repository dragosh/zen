// :::rating — a star rating built on the radio group Go rendered. A rating is kept in
// ctx.storage under "rating-<id>": {version:1, value, max, date, page, title, question}.
// Clear writes null. :::rating-results lists every stored rating.
// Everything is set with textContent; nothing is parsed as HTML.
const PREFIX = "rating-";
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

function h(tag, props, ...kids) {
  const e = document.createElement(tag);
  for (const k of Object.keys(props || {})) {
    if (k === "class") e.className = props[k];
    else if (k === "text") e.textContent = props[k];
    else e.setAttribute(k, props[k]);
  }
  for (const c of kids) if (c) e.append(c);
  return e;
}

/** "21 Nov 2026" from a stored date; "" when it is not one. */
export function formatDate(s) {
  const d = new Date(s);
  if (typeof s !== "string" || Number.isNaN(d.getTime())) return "";
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
}

/** A stored value that is not a version-1 rating within 1..max is not a rating (null). */
export function normalise(v, max) {
  if (!v || typeof v !== "object" || Array.isArray(v) || v.version !== 1) return null;
  if (!Number.isInteger(v.value) || !Number.isInteger(v.max) || v.max < 1) return null;
  if (v.value < 1 || v.value > v.max) return null;
  if (max !== undefined && v.max !== max) return null;
  return v;
}

const str = (x) => (typeof x === "string" ? x : "");

async function mountRating(el, ctx) {
  const id = ctx.attrs.id || "";
  const key = PREFIX + id;
  const fieldset = el.querySelector("fieldset.zr-rating");
  const radios = Array.from(el.querySelectorAll("input[type=radio]"));
  const stars = el.querySelector(".zr-stars");
  const statusEl = el.querySelector(".zr-status");
  if (!fieldset || radios.length === 0 || !stars || !statusEl) return;
  const max = radios.length;
  const question = (fieldset.querySelector(".zr-legend")?.textContent ?? "").trim();
  const labels = radios.map((r, i) => (r.closest("label")?.querySelector(".zr-sr")?.textContent ?? `${i + 1} of ${max}`).trim());
  const hasLabels = labels.some((l, i) => l !== `${i + 1} of ${max}`);
  const glyphs = radios.map((r) => r.closest("label")?.querySelector(".zr-glyph"));
  let hint = el.querySelector(".zr-hint");
  if (!hint) { hint = h("span", { class: "zr-hint", "aria-hidden": "true" }); stars.append(hint); }
  const note = h("p", { class: "zr-note", role: "alert" });
  note.hidden = true;
  const clear = h("button", { type: "button", class: "zr-clear", text: "Clear" });
  clear.hidden = true;
  const actions = h("div", { class: "zr-actions" }, clear);
  fieldset.append(note, actions);
  let dead = false;

  let state = null;
  try {
    state = normalise(await ctx.storage.get(PREFIX + id), max);
  } catch (err) {
    console.error("rating: could not read the stored rating", err);
  }
  if (dead) return;

  /** Draws the stars up to `upto` filled. */
  const paint = (upto) => glyphs.forEach((g, i) => { if (g) g.textContent = i < upto ? "★" : "☆"; });
  const showHint = (i) => { hint.textContent = hasLabels && i > 0 ? labels[i - 1] : ""; };
  const render = () => {
    const v = state ? state.value : 0;
    radios.forEach((r, i) => { r.checked = i + 1 === v; });
    paint(v);
    showHint(v);
    clear.hidden = !state;
    statusEl.textContent = state ? `You rated ${state.value} of ${state.max}${formatDate(state.date) ? ` · ${formatDate(state.date)}` : ""}` : "Not rated yet";
  };
  render();

  const save = async (value) => {
    const prev = state;
    state = value === null ? null : { version: 1, value, max, date: new Date().toISOString(), page: ctx.page.path, title: ctx.page.title, question };
    render();
    try {
      await ctx.storage.set(key, state);
      note.hidden = true;
    } catch (err) {
      console.warn("rating: could not save the rating");
      state = prev;
      render();
      note.textContent = `Could not save: ${err.message}`;
      note.hidden = false;
    }
  };

  const onChange = (e) => {
    const i = radios.indexOf(e.target);
    if (i >= 0) void save(i + 1);
  };
  const onKey = (e) => {
    const i = radios.indexOf(e.target);
    if (i < 0) return;
    let to = -1;
    if (e.key === "ArrowRight" || e.key === "ArrowDown") to = Math.min(max - 1, i + 1);
    else if (e.key === "ArrowLeft" || e.key === "ArrowUp") to = Math.max(0, i - 1);
    else if (e.key === "Home") to = 0;
    else if (e.key === "End") to = max - 1;
    if (to < 0) return;
    e.preventDefault();
    radios[to].focus();
    if (to !== i || !state || state.value !== to + 1) void save(to + 1);
  };
  const onOver = (e) => {
    const i = radios.findIndex((r) => r.closest("label") === e.target.closest("label"));
    if (i < 0) return;
    paint(i + 1);
    showHint(i + 1);
  };
  const onLeave = () => { paint(state ? state.value : 0); showHint(state ? state.value : 0); };
  const onClear = () => { void save(null); };

  stars.addEventListener("change", onChange);
  stars.addEventListener("keydown", onKey);
  stars.addEventListener("mouseover", onOver);
  stars.addEventListener("mouseleave", onLeave);
  clear.addEventListener("click", onClear);

  return () => {
    dead = true;
    stars.removeEventListener("change", onChange);
    stars.removeEventListener("keydown", onKey);
    stars.removeEventListener("mouseover", onOver);
    stars.removeEventListener("mouseleave", onLeave);
    clear.removeEventListener("click", onClear);
    // The markup Go rendered goes back as it was: nothing checked, not rated.
    actions.remove();
    note.remove();
    glyphs.forEach((g) => { if (g) g.textContent = "☆"; });
    radios.forEach((r) => { r.checked = false; });
    statusEl.textContent = "Not rated yet";
    hint.textContent = "";
  };
}

async function mountResults(el, ctx) {
  const original = Array.from(el.childNodes);
  const box = h("div", { class: "zr-results-list" });
  el.replaceChildren(box);
  let keys = [];
  try {
    keys = await ctx.storage.keys(PREFIX);
  } catch (err) {
    console.warn("rating: could not read the ratings");
    box.append(h("p", { class: "zr-note", text: `Could not read the ratings: ${err.message}` }));
  }
  const items = [];
  for (const k of keys) {
    try {
      const v = normalise(await ctx.storage.get(k));
      if (v) items.push(v);
    } catch { /* skip an unreadable entry */ }
  }
  items.sort((a, b) => str(b.date).localeCompare(str(a.date)));
  if (items.length === 0) {
    box.append(h("p", { class: "zr-empty", text: "No ratings yet." }));
  } else {
    const ul = h("ul", { class: "zr-results-items" });
    for (const v of items) {
      const page = str(v.page);
      const name = str(v.title) || page || "A page";
      let head;
      if (page) {
        head = h("a", { href: page, text: name });
        head.addEventListener("click", (e) => { e.preventDefault(); ctx.navigate(page); });
      } else {
        head = h("strong", { text: name });
      }
      const line = "★".repeat(v.value) + "☆".repeat(v.max - v.value);
      const when = formatDate(v.date);
      ul.append(h("li", {}, head,
        h("span", { class: "zr-summary", text: ` ${str(v.question)}` }),
        h("span", { class: "zr-stars-line", text: ` ${line}` }),
        h("span", { class: "zr-summary", text: when ? ` ${when}` : "" })));
    }
    box.append(ul);
  }
  return () => el.replaceChildren(...original);
}

export default {
  mount(el, ctx) {
    return ctx.name === "rating-results" ? mountResults(el, ctx) : mountRating(el, ctx);
  },
};
