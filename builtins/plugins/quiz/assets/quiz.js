// :::quiz — a keyboard-first quiz form built from the questions Go rendered.
// The page carries no answers: Check sends them to the plugin's backend
// (ctx.invoke) and shows what comes back. Progress, attempts and the best score
// live in ctx.storage under "quiz-<id>": {version:1, answers, attempts:[{date,score}], best}.
// Everything is built with DOM APIs and textContent; nothing is parsed as HTML.
const PREFIX = "quiz-";
const DEFAULT_PASS = 70;
let uid = 0;

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

const fresh = () => ({ version: 1, answers: {}, attempts: [], best: 0 });

/** A stored value that is not a version-1 history is replaced by a fresh one. */
export function normalise(v) {
  if (!v || typeof v !== "object" || v.version !== 1) return fresh();
  const attempts = Array.isArray(v.attempts)
    ? v.attempts.filter((a) => a && typeof a.score === "number").map((a) => ({ date: String(a.date || ""), score: a.score }))
    : [];
  const answers = v.answers && typeof v.answers === "object" && !Array.isArray(v.answers) ? v.answers : {};
  const best = Math.max(0, ...attempts.map((a) => a.score));
  return { version: 1, answers, attempts, best };
}

/** "Best 80% · 2 attempts" (with the limit: "2 of 3 attempts"). */
export function summaryLine(s, limit) {
  const n = s.attempts.length;
  if (n === 0) return limit ? `Not attempted yet · ${limit} ${limit === 1 ? "attempt" : "attempts"} allowed` : "Not attempted yet";
  const count = limit ? `${n} of ${limit} attempts` : `${n} ${n === 1 ? "attempt" : "attempts"}`;
  return `Best ${s.best}% · ${count}`;
}

function shuffled(n) {
  const a = Array.from({ length: n }, (_, i) => i);
  for (let i = n - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

/** The questions Go rendered: [{kind, prompt, options}]. */
function readQuestions(el) {
  return Array.from(el.querySelectorAll(":scope > ol.zq-questions > li.zq-q")).map((li) => ({
    kind: li.classList.contains("zq-multi") ? "multi" : li.classList.contains("zq-text") ? "text" : "single",
    prompt: (li.querySelector(":scope > .zq-prompt")?.textContent ?? "").trim(),
    options: Array.from(li.querySelectorAll(":scope > ul.zq-options > li")).map((o) => (o.textContent ?? "").trim()),
  }));
}

async function mountQuiz(el, ctx) {
  const questions = readQuestions(el);
  const original = Array.from(el.childNodes);
  const id = ctx.attrs.id || "quiz";
  const key = PREFIX + id;
  const title = ctx.attrs.title || "";
  const pass = Number.isFinite(Number(ctx.attrs.pass)) && ctx.attrs.pass !== undefined && ctx.attrs.pass !== "" ? Number(ctx.attrs.pass) : DEFAULT_PASS;
  const each = ctx.attrs.reveal === "each";
  const shuffle = ctx.attrs.shuffle !== undefined && ctx.attrs.shuffle !== "false";
  const limit = Number.parseInt(ctx.attrs.attempts, 10) > 0 ? Number.parseInt(ctx.attrs.attempts, 10) : 0;
  const base = `zq${++uid}`;
  let dead = false;

  if (questions.length === 0) {
    el.append(h("p", { class: "zq-note", text: "This quiz has no questions." }));
    return () => el.replaceChildren(...original);
  }

  let state = fresh();
  try {
    state = normalise(await ctx.storage.get(key));
  } catch (err) {
    console.error("quiz: could not read the stored results", err);
  }
  if (dead) return;

  const save = async () => {
    try {
      await ctx.storage.set(key, state);
    } catch (err) {
      console.warn("quiz: could not save the results");
      note.textContent = `Could not save: ${err.message}`;
    }
  };

  // ---- layout ----
  const header = h("div", { class: "zq-header" });
  const titleEl = h("strong", { class: "zq-title", text: title || "Quiz" });
  const summary = h("span", { class: "zq-summary" });
  header.append(titleEl, summary);
  const form = h("form", { class: "zq-form", novalidate: "" });
  const note = h("p", { class: "zq-note", role: "alert" });
  const outcome = h("p", { class: "zq-outcome", role: "status", "aria-live": "polite", tabindex: "-1" });
  const actions = h("div", { class: "zq-actions" });
  const checkBtn = h("button", { type: "submit", class: "zq-check", text: "Check" });
  const retryBtn = h("button", { type: "button", class: "zq-retry", text: "Retry" });
  retryBtn.hidden = true;
  actions.append(checkBtn, retryBtn);

  /** Per question: {fieldset, inputs, status, explain, kind}. */
  const rows = questions.map((q, i) => {
    const fs = h("fieldset", { class: `zq-question zq-${q.kind}` });
    fs.append(h("legend", { class: "zq-legend", text: `${i + 1}. ${q.prompt}` }));
    const inputs = [];
    if (q.kind === "text") {
      const input = h("input", { type: "text", class: "zq-input", autocomplete: "off", spellcheck: "false", "aria-label": `Answer to question ${i + 1}` });
      inputs.push(input);
      fs.append(input);
    } else {
      const order = shuffle ? shuffled(q.options.length) : q.options.map((_, k) => k);
      for (const k of order) {
        const input = h("input", { type: q.kind === "multi" ? "checkbox" : "radio", name: `${base}-${i}`, value: String(k) });
        inputs.push(input);
        fs.append(h("label", { class: "zq-option" }, input, h("span", { text: q.options[k] })));
      }
    }
    const status = h("p", { class: "zq-status", "aria-live": "polite" });
    const explain = h("p", { class: "zq-explanation" });
    fs.append(status, explain);
    let button = null;
    if (each) {
      button = h("button", { type: "submit", class: "zq-check-one", text: "Check" });
      fs.append(button);
    }
    form.append(fs);
    return { fieldset: fs, inputs, status, explain, kind: q.kind, button, done: false, correct: false };
  });
  if (each) checkBtn.hidden = true;
  form.append(note, outcome, actions);
  el.replaceChildren(header, form);

  // ---- answers ----
  const read = (i) => {
    const r = rows[i];
    if (r.kind === "text") return r.inputs[0].value.trim() === "" ? undefined : r.inputs[0].value;
    const picked = r.inputs.filter((x) => x.checked).map((x) => Number(x.value));
    if (picked.length === 0) return undefined;
    return r.kind === "multi" ? picked : picked[0];
  };
  const write = (i, v) => {
    const r = rows[i];
    if (r.kind === "text") r.inputs[0].value = typeof v === "string" ? v : "";
    else {
      const set = new Set(Array.isArray(v) ? v : [v]);
      for (const x of r.inputs) x.checked = set.has(Number(x.value));
    }
  };
  const collect = (only) => {
    const out = {};
    rows.forEach((_, i) => {
      if (only !== undefined && only !== i) return;
      const v = read(i);
      if (v !== undefined) out[String(i)] = v;
    });
    return out;
  };

  const drawSummary = () => { summary.textContent = summaryLine(state, limit); };
  const exhausted = () => limit > 0 && state.attempts.length >= limit;

  const setLocked = (r, locked) => {
    for (const x of r.inputs) x.disabled = locked;
    if (r.button) r.button.disabled = locked;
  };

  const mark = (r, correct, text, explanation) => {
    r.done = true;
    r.correct = correct;
    r.fieldset.classList.toggle("zq-right", correct === true);
    r.fieldset.classList.toggle("zq-wrong", correct === false);
    r.status.textContent = text;
    r.explain.textContent = explanation || "";
    setLocked(r, true);
  };

  function finish(score, passed) {
    outcome.textContent = `Score ${score}% · ${passed ? "Passed" : `Not passed (pass mark ${pass}%)`}`;
    outcome.className = `zq-outcome ${passed ? "zq-right" : "zq-wrong"}`;
    state.attempts.push({ date: new Date().toISOString(), score });
    state.best = Math.max(state.best, score);
    state.answers = {};
    drawSummary();
    save();
    checkBtn.hidden = true;
    for (const r of rows) if (r.button) r.button.hidden = true;
    retryBtn.hidden = exhausted();
    if (exhausted()) outcome.append(" · No attempts left");
    outcome.focus();
  }

  async function check(only) {
    note.textContent = "";
    const answers = collect(only);
    if (Object.keys(answers).length === 0) {
      note.textContent = only === undefined ? "Answer at least one question first." : "Answer this question first.";
      return;
    }
    let res;
    try {
      res = await ctx.invoke({ op: "check", quiz: id, page: ctx.page.path, answers });
    } catch (err) {
      if (!dead) note.textContent = `Could not check: ${err && err.message ? err.message : err}`;
      return;
    }
    if (dead) return;
    if (!res || !Array.isArray(res.results)) {
      note.textContent = "Could not check: the answer was not understood.";
      return;
    }
    for (const x of res.results) {
      const r = rows[x.q];
      if (r && (only === undefined || only === x.q)) mark(r, !!x.correct, x.correct ? "Correct" : "Incorrect", x.explanation);
    }
    if (only === undefined) {
      rows.forEach((r) => { if (!r.done) mark(r, false, "Not answered", ""); });
      finish(Number(res.score) || 0, !!res.passed);
    } else if (rows.every((r) => r.done)) {
      const right = rows.filter((r) => r.correct).length;
      const score = Math.round((100 * right) / rows.length);
      finish(score, score >= pass);
    }
  }

  function retry() {
    for (const r of rows) {
      r.done = false;
      r.correct = false;
      r.fieldset.classList.remove("zq-right", "zq-wrong");
      r.status.textContent = "";
      r.explain.textContent = "";
      for (const x of r.inputs) { x.checked = false; if (r.kind === "text") x.value = ""; }
      setLocked(r, false);
      if (r.button) r.button.hidden = false;
    }
    outcome.textContent = "";
    outcome.className = "zq-outcome";
    note.textContent = "";
    checkBtn.hidden = each;
    retryBtn.hidden = true;
    state.answers = {};
    save();
    rows[0]?.inputs[0]?.focus();
  }

  // ---- wiring ----
  const onSubmit = (e) => {
    e.preventDefault();
    if (exhausted()) return;
    if (!each) return void check();
    const at = (e.submitter || form.ownerDocument.activeElement);
    const i = rows.findIndex((r) => r.fieldset.contains(at));
    if (i >= 0 && !rows[i].done) check(i);
  };
  const onChange = () => {
    state.answers = collect();
    save();
  };
  form.addEventListener("submit", onSubmit);
  form.addEventListener("change", onChange);
  retryBtn.addEventListener("click", retry);

  // resume what was in progress
  for (const [k, v] of Object.entries(state.answers)) {
    const i = Number(k);
    if (Number.isInteger(i) && rows[i]) write(i, v);
  }
  drawSummary();
  if (exhausted()) {
    for (const r of rows) setLocked(r, true);
    checkBtn.hidden = true;
    outcome.textContent = "No attempts left";
  }

  return () => {
    dead = true;
    form.removeEventListener("submit", onSubmit);
    form.removeEventListener("change", onChange);
    retryBtn.removeEventListener("click", retry);
    el.replaceChildren(...original);
  };
}

async function mountResults(el, ctx) {
  const original = Array.from(el.childNodes);
  const box = h("div", { class: "zq-results-list" });
  el.replaceChildren(box);
  let keys = [];
  try {
    keys = await ctx.storage.keys(PREFIX);
  } catch (err) {
    console.warn("quiz: could not read the results");
    box.append(h("p", { class: "zq-note", text: `Could not read the results: ${err.message}` }));
  }
  const items = [];
  for (const k of keys) {
    let s = null;
    try {
      const v = await ctx.storage.get(k);
      if (v && typeof v === "object" && v.version === 1) s = normalise(v);
    } catch { /* skip an unreadable entry */ }
    if (s) items.push([k.slice(PREFIX.length), s]);
  }
  if (items.length === 0) {
    box.append(h("p", { class: "zq-empty", text: "No quizzes taken yet." }));
  } else {
    const ul = h("ul", { class: "zq-results-items" });
    for (const [id, s] of items) ul.append(h("li", {}, h("strong", { text: id }), h("span", { class: "zq-summary", text: ` ${summaryLine(s, 0)}` })));
    box.append(ul);
  }
  return () => el.replaceChildren(...original);
}

export default {
  mount(el, ctx) {
    return ctx.name === "quiz-results" ? mountResults(el, ctx) : mountQuiz(el, ctx);
  },
};
