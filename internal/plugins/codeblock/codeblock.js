// Frontend of the built-in codeblock plugin: a Copy and a Wrap button inside
// every code block. The Go hook wraps each <pre> in
// <div data-zen-plugin="codeblock" class="zen-codeblock">; zen mounts this on
// each wrapper after every render.
const COPIED_MS = 1500;

function button(doc, cls, text, label) {
  const b = doc.createElement("button");
  b.type = "button";
  b.className = cls;
  b.textContent = text;
  b.setAttribute("aria-label", label);
  b.title = label;
  return b;
}

/** The block's text without the line-number gutter. */
export function codeText(pre) {
  const c = pre.cloneNode(true);
  for (const n of Array.from(c.querySelectorAll(".ln, .lnt, .lntd:first-child"))) n.remove();
  return c.textContent ?? "";
}

async function copy(doc, text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // The webview may refuse the async clipboard: fall back to a selection.
    const ta = doc.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    doc.body.append(ta);
    ta.select();
    let ok = false;
    try { ok = doc.execCommand("copy"); } catch { ok = false; }
    ta.remove();
    return ok;
  }
}

function mount(root) {
  const doc = root.ownerDocument;
  const pre = root.querySelector("pre");
  if (!pre) return undefined;

  const tools = doc.createElement("div");
  tools.className = "zen-codeblock-tools";
  const wrap = button(doc, "zen-codeblock-wrapbtn", "Wrap", "Wrap long lines");
  const cp = button(doc, "zen-codeblock-copy", "Copy", "Copy code");
  const sync = () => wrap.setAttribute("aria-pressed", String(root.classList.contains("zen-codeblock-wrap")));
  sync();
  tools.append(wrap, cp);
  root.append(tools);

  let timer = 0;
  wrap.addEventListener("click", () => {
    root.classList.toggle("zen-codeblock-wrap");
    sync();
  });
  cp.addEventListener("click", async () => {
    const ok = await copy(doc, codeText(pre));
    cp.textContent = ok ? "Copied" : "Copy failed";
    clearTimeout(timer);
    timer = setTimeout(() => { cp.textContent = "Copy"; }, COPIED_MS);
  });

  return () => {
    clearTimeout(timer);
    tools.remove();
  };
}

export default { mount };
