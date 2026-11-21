// Frontend of the built-in sandbox plugin: code view, Run / Stop and a console
// with the live output of ctx.stream. Everything from the page or the backend is
// inserted with textContent. Chunks are JSON lines, see sandbox.go.
const MAX_OUTPUT = 1 << 20;
// CSI sequences (colours, cursor moves) and OSC strings; stray carriage returns.
const ANSI = /\u001b\[[0-9;?]*[ -/]*[@-~]|\u001b\][^\u0007\u001b]*(?:\u0007|\u001b\\)|\u001b[@-Z\\-_]|\r/g;
export const stripAnsi = (s) => String(s).replace(ANSI, "");

function el(doc, tag, cls, text) {
  const e = doc.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function seconds(ms) {
  return (ms / 1000).toFixed(1) + " s";
}

function mount(root, ctx) {
  const doc = root.ownerDocument;
  const box = root.querySelector(".zen-sandbox");
  const codeEl = root.querySelector(".zen-sandbox-code");
  if (!box || !codeEl) return undefined;
  if (box.classList.contains("zen-sandbox-setup")) return undefined; // the note explains what to configure

  const attrs = ctx.attrs || {};
  const editable = attrs.editable !== undefined && attrs.editable !== "false";
  let code = codeEl.textContent || "";

  let editor = null;
  if (editable) {
    editor = el(doc, "textarea", "zen-sandbox-editor");
    editor.value = code;
    editor.spellcheck = false;
    editor.rows = Math.min(24, Math.max(4, code.split("\n").length));
    editor.setAttribute("aria-label", "Code to run");
    editor.addEventListener("input", () => { code = editor.value; });
    codeEl.replaceWith(editor);
  }

  const bar = el(doc, "div", "zen-sandbox-bar zen-block-bar");
  const run = el(doc, "button", "zen-sandbox-run zen-btn zen-btn-primary", "Run");
  const stop = el(doc, "button", "zen-sandbox-stop zen-btn", "Stop");
  run.type = stop.type = "button";
  stop.disabled = true;
  const spin = el(doc, "span", "zen-sandbox-spin");
  spin.setAttribute("aria-hidden", "true");
  spin.hidden = true;
  const status = el(doc, "span", "zen-sandbox-status zen-block-status");
  status.setAttribute("role", "status");
  bar.append(run, stop, spin, status);
  const con = el(doc, "pre", "zen-sandbox-console");
  con.setAttribute("aria-live", "off");
  box.append(bar, con);

  let handle = null;
  let size = 0;
  let truncated = false;
  let gone = false;
  let started = false; // first output seen: the container is up

  // Until the first line arrives (image pull, VM boot) show a spinner.
  function firstOutput() {
    if (started) return;
    started = true;
    showNote(null); // output means the service is up
    spin.hidden = true;
    status.textContent = "running…";
  }

  function append(text, err) {
    if (truncated) return;
    let t = stripAnsi(text) + "\n";
    if (size + t.length > MAX_OUTPUT) {
      t = t.slice(0, Math.max(0, MAX_OUTPUT - size));
      truncated = true;
    }
    size += t.length;
    const atEnd = con.scrollHeight - con.scrollTop - con.clientHeight < 24;
    con.append(err ? el(doc, "span", "zen-sandbox-err", t) : doc.createTextNode(t));
    if (truncated) con.append(el(doc, "span", "zen-sandbox-err", "\n[output truncated at 1 MB]\n"));
    if (atEnd) con.scrollTop = con.scrollHeight;
  }

  function setRunning(on) {
    run.disabled = on;
    stop.disabled = !on;
    if (!on) spin.hidden = true;
  }

  function onChunk(raw) {
    let c;
    try { c = JSON.parse(raw); } catch { append(String(raw), false); return; }
    if (c.s === "out") { firstOutput(); append(String(c.t ?? ""), false); }
    else if (c.s === "err") { firstOutput(); append(String(c.t ?? ""), true); }
    else if (c.s === "exit") {
      spin.hidden = true;
      status.textContent = c.timedOut
        ? `timed out after ${seconds(c.ms)}`
        : `exit ${c.code} in ${seconds(c.ms)}`;
    }
  }

  function start() {
    if (handle) return;
    con.replaceChildren();
    size = 0;
    truncated = false;
    started = false;
    spin.hidden = false;
    status.textContent = "starting container…";
    setRunning(true);
    const h = ctx.stream(
      { op: "run", image: attrs.image || "", cmd: attrs.cmd || "", file: attrs.file || "", code },
      onChunk,
    );
    handle = h;
    h.done.then(
      () => {},
      (err) => {
        const msg = err && err.message ? err.message : String(err);
        if (msg === "cancelled") status.textContent = "stopped";
        else {
          append(msg, true);
          status.textContent = "failed";
          check(); // the service may have stopped since the note was last checked
        }
      },
    ).finally(() => {
      if (handle === h) handle = null;
      if (!gone) setRunning(false);
    });
  }

  run.addEventListener("click", start);
  stop.addEventListener("click", () => { if (handle) handle.cancel(); });

  // The service note follows the service: it is checked when the block mounts, when the window gets
  // the focus back (say, after `container system start` in a terminal) and on Check again.
  let note = null;
  function showNote(text) {
    if (!text) {
      if (note) note.remove();
      note = null;
      return;
    }
    if (!note) {
      note = el(doc, "div", "zen-sandbox-note");
      const again = el(doc, "button", "zen-sandbox-recheck zen-btn", "Check again");
      again.type = "button";
      again.addEventListener("click", check);
      note.append(el(doc, "span", "zen-sandbox-note-text"), " ", again);
      box.insertBefore(note, box.firstChild);
    }
    note.firstChild.textContent = text;
  }
  function check() {
    ctx.invoke({ op: "status" }).then((s) => {
      if (gone || !s) return;
      if (s.service === "missing") showNote("Apple's container tool is not installed. Install it, then run `container system start` in a terminal.");
      else if (s.service === "stopped") showNote("The container service is not running. Run `container system start` in a terminal, then press Run.");
      else showNote(null);
    }).catch(() => {});
  }
  const onFocus = () => { if (note) check(); };
  const view = doc.defaultView;
  view?.addEventListener("focus", onFocus);
  check();

  return () => {
    gone = true;
    view?.removeEventListener("focus", onFocus);
    if (handle) handle.cancel();
    showNote(null);
    bar.remove();
    con.remove();
  };
}

export default { mount };
