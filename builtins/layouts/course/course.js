// course.js — the "course" page layout and its :::check block. Each h2 is a
// step; one step shows at a time. Progress lives in ctx.storage under
// layoutOptions.id (else the page path), and the sidebar badge shows n/m.
// Plain browser JavaScript; the page's own nodes are moved, never re-parsed.
const isBlank = (n) => n.nodeType === 8 || (n.nodeType === 3 && n.textContent.trim() === "");

function el(doc, name, cls, text) {
  const e = doc.createElement(name);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}
function button(doc, cls, text) {
  const b = el(doc, "button", cls + " zen-btn", text);
  b.type = "button";
  return b;
}
const safeKey = (s) => String(s).replace(/[^A-Za-z0-9._-]/g, "-").slice(0, 64) || "course";
const label = (box) => (box.closest("li") || box.parentElement).textContent.trim();

/** Intro nodes (before the first h2) and the steps: { title, nodes }. */
function split(nodes) {
  const intro = [];
  const steps = [];
  for (const n of nodes) {
    if (n.nodeType === 1 && n.tagName === "H2") steps.push({ title: n.textContent.trim(), nodes: [n] });
    else if (steps.length > 0) steps[steps.length - 1].nodes.push(n);
    else if (!isBlank(n)) intro.push(n);
  }
  return { intro, steps };
}

export default {
  async mount(host, ctx) {
    // :::check blocks are enabled by the layout around them; alone they stay read-only.
    if (ctx.kind !== "layout") return;
    const doc = host.ownerDocument;
    const original = Array.from(host.childNodes);
    const { intro, steps } = split(original);
    const total = steps.length;
    const key = safeKey(ctx.attrs.id || ctx.page.path);

    // ---- state: { step, done: [index], skipped: [index], checks: { index: [ticked labels] } } ----
    let state = { step: 0, done: [], skipped: [], checks: {} };
    try {
      const saved = await ctx.storage.get(key);
      if (saved && typeof saved === "object") {
        state = {
          step: Number.isInteger(saved.step) ? saved.step : 0,
          done: Array.isArray(saved.done) ? saved.done.filter((i) => Number.isInteger(i) && i >= 0 && i < total) : [],
          skipped: Array.isArray(saved.skipped) ? saved.skipped.filter((i) => Number.isInteger(i) && i >= 0 && i < total) : [],
          checks: saved.checks && typeof saved.checks === "object" ? saved.checks : {},
        };
      }
    } catch (err) {
      console.error("zen: course progress could not be read", err);
    }
    const done = new Set(state.done);
    const skipped = new Set(state.skipped.filter((i) => !done.has(i))); // done wins

    // ---- DOM ----
    const root = el(doc, "div", "course-root");
    const header = el(doc, "header", "course-header");
    const introBox = el(doc, "div", "course-intro");
    introBox.append(...intro);
    const meta = el(doc, "div", "course-meta");
    const count = el(doc, "span", "course-count");
    count.setAttribute("role", "status");
    const bar = el(doc, "div", "course-progress");
    const fill = el(doc, "div", "course-progress-fill");
    bar.append(fill);
    const reset = button(doc, "course-reset zen-btn-quiet", "Reset");
    meta.append(count, bar, reset);
    header.append(introBox, meta);

    const body = el(doc, "div", "course-body");
    const nav = el(doc, "nav", "course-steps");
    nav.setAttribute("aria-label", "Steps");
    const list = el(doc, "ol");
    nav.append(list);
    const main = el(doc, "div", "course-main");
    const footer = el(doc, "div", "course-nav");
    const prev = button(doc, "course-prev", "Previous");
    const next = button(doc, "course-next zen-btn-primary", "Next");
    const skip = button(doc, "course-skip zen-btn-quiet", "Skip step");
    const actions = el(doc, "div", "course-actions");
    actions.append(next, skip); // Skip step follows Next in tab order
    footer.append(prev, actions);

    const sections = steps.map((s) => {
      const sec = el(doc, "section", "course-step");
      sec.append(...s.nodes);
      main.append(sec);
      return sec;
    });
    const items = steps.map((s, i) => {
      const li = el(doc, "li");
      const b = el(doc, "button", "course-step-link");
      b.type = "button";
      b.append(el(doc, "span", "course-step-title", s.title));
      b.addEventListener("click", () => open(i));
      li.append(b);
      list.append(li);
      return { li, b };
    });
    const boxesOf = (i) => Array.from(sections[i].querySelectorAll('[data-zen-name="check"] input[type="checkbox"]'));
    // The checklists are the kit's: a frame, each item a choice with its box inside.
    for (const c of main.querySelectorAll('[data-zen-name="check"]')) {
      c.classList.add("zen-block", "zen-block-body");
      for (const li of c.querySelectorAll("li")) if (li.querySelector(':scope > input[type="checkbox"]')) li.classList.add("zen-choice");
    }
    const allBoxes = sections.flatMap((_, i) => boxesOf(i));
    for (const b of allBoxes) b.disabled = false;
    for (let i = 0; i < total; i++) {
      const ticked = Array.isArray(state.checks[i]) ? state.checks[i] : null;
      if (ticked) for (const b of boxesOf(i)) b.checked = ticked.includes(label(b));
    }

    if (total > 0) {
      body.append(nav, el(doc, "div", "course-content"));
      body.lastChild.append(main, footer);
    } else {
      body.append(el(doc, "div", "course-content"));
    }
    root.append(header, body);
    host.replaceChildren(root);

    // ---- behaviour ----
    let step = 0;
    const firstUndone = () => {
      let i = 0;
      while (i < total && (done.has(i) || skipped.has(i))) i++;
      return Math.min(i, total - 1);
    };
    step = Math.max(0, Math.min(state.step, firstUndone()));
    const gated = (i) => boxesOf(i).some((b) => !b.checked);

    function snapshot() {
      const checks = {};
      for (let i = 0; i < total; i++) {
        const ticked = boxesOf(i).filter((b) => b.checked).map(label);
        if (ticked.length) checks[i] = ticked;
      }
      return { step, done: Array.from(done).sort((a, b) => a - b), skipped: Array.from(skipped).sort((a, b) => a - b), checks };
    }
    async function save() {
      try {
        await ctx.storage.set(key, snapshot());
      } catch (err) {
        count.textContent = `Could not save: ${err.message}`;
      }
    }
    function draw() {
      const reach = total > 0 ? firstUndone() : 0;
      sections.forEach((s, i) => {
        s.hidden = i !== step;
      });
      items.forEach(({ li, b }, i) => {
        const locked = !done.has(i) && !skipped.has(i) && i > reach;
        li.dataset.state = done.has(i) ? "done" : skipped.has(i) ? "skipped" : i === step ? "current" : locked ? "locked" : "open";
        b.disabled = locked;
        if (i === step) b.setAttribute("aria-current", "step");
        else b.removeAttribute("aria-current");
      });
      count.textContent = `${done.size} of ${total} steps${skipped.size ? ` \u00b7 ${skipped.size} skipped` : ""}`;
      fill.style.width = total ? `${(done.size / total) * 100}%` : "0%";
      prev.disabled = step === 0;
      const last = step === total - 1;
      next.textContent = last ? "Finish" : "Next";
      skip.hidden = total === 0 || done.has(step);
      next.disabled = total === 0 || gated(step) || (last && done.has(step));
      ctx.badge(ctx.page.path, total > 0 ? `${done.size}/${total}` : null);
    }
    function open(i) {
      if (i < 0 || i >= total || i === step) return;
      step = i;
      draw();
      save();
    }
    prev.addEventListener("click", () => open(step - 1));
    next.addEventListener("click", () => {
      if (gated(step)) return;
      done.add(step);
      skipped.delete(step);
      if (step < total - 1) step++;
      draw();
      save();
    });
    skip.addEventListener("click", () => {
      if (total === 0 || done.has(step)) return;
      skipped.add(step);
      if (step < total - 1) step++;
      draw();
      save();
    });
    const onChange = (e) => {
      if (!(e.target instanceof doc.defaultView.HTMLInputElement) || !allBoxes.includes(e.target)) return;
      if (gated(step)) done.delete(step); // unticking takes the step back
      else if (skipped.has(step)) {
        // every box of a skipped step ticked: it is done after all
        skipped.delete(step);
        done.add(step);
      }
      draw();
      save();
    };
    main.addEventListener("change", onChange);
    reset.addEventListener("click", () => {
      done.clear();
      skipped.clear();
      step = 0;
      for (const b of allBoxes) b.checked = false;
      draw();
      save();
    });

    draw();

    return () => {
      main.removeEventListener("change", onChange);
      for (const b of allBoxes) b.disabled = true;
      host.replaceChildren(...original);
    };
  },
};
