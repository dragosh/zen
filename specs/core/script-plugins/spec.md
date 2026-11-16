# core/script-plugins (plan)

| | |
|---|---|
| Status | planned (after the plugin hardening fixes) |
| Target version | 0.2.0 |
| Depends on | core/extplugins, core/plugin-extras |
| Owns | `internal/core/scriptplugins/` (new), discovery hook in `internal/core/extplugins`, docs/PLUGINS.md, `examples/demo/.zen/plugins/*.tengo` |

## Why

Today a backend plugin is a separate program: a folder, a `plugin.yml`, an
executable, environment variables, a process per call, and a trust prompt
that has to warn "runs programs". That is the right tool for wrapping
existing tools (`bc`, `container`, `git`), but heavy for the common case:
"turn this block into some HTML".

A **script plugin** is one file, run inside zen by an embedded language,
with no access to anything zen does not hand it.

## Language: Tengo (`github.com/d5/tengo/v2` v2.17.0, MIT)

Prototype results (scratchpad, 2026-11-16), same "note" plugin in each:

| | Tengo | JavaScript (goja) |
|---|---|---|
| per call (compile once, clone, run) | ~25 µs | ~5 µs |
| endless loop | stopped at the timeout (context) | stopped at the timeout (interrupt) |
| runaway memory | **stopped by `SetMaxAllocs` after 10 ms** | no allocation limit: grew until the 2 s timeout |
| file / network access | none unless a module is granted (`os` → compile error) | none by default |
| dependencies | 3 packages | 25 packages |
| syntax | Go-like, small | JavaScript |

Tengo wins on the property that matters most for running code from a
downloaded folder inside the app process: a hard allocation budget. JavaScript
remains the language for page scripts (frontend), where the browser already
isolates it. Alternatives considered: Starlark (Python-like, step limit but no
allocation limit, larger API), Lua (no allocation limit), Yaegi (full Go, no
sandbox), WASM via wazero (strong sandbox but needs a compiler toolchain from
authors — revisit for "bring any language").

Note: Tengo's `v3.0.0` tag is not a valid Go module (its go.mod says `/v2`);
use v2.17.0.

## The authoring model

One file anywhere a plugin folder can be: `.zen/plugins/note.tengo` (or
`.zen/plugins/note/plugin.tengo` with assets beside it).

```go
// .zen/plugins/note.tengo
plugin := {
    version: "0.1.0",
    description: "Callout boxes",
    blocks: ["note", "warning"],
    css: "div.note { border-left: 3px solid; padding: .5em 1em }"   // optional inline CSS
}

if hook == "block" {
    title := attrs.title || (name == "warning" ? "Warning" : "Note")
    out = zen.html("div", {class: name}, zen.html("strong", {}, title), zen.markdown(body))
}
```

- **Inputs** (globals): `hook` ("block", "inline", "source", "html", "action"), `name`, `body`, `attrs`, `config`, `page` {path, title}, `payload` (action).
- **Output**: assign `out` (string HTML, or for `action` any value → JSON).
- **`zen` module** (the only API; nothing else is importable except pure stdlib modules `text`, `math`, `times`, `fmt`, `json`, `enum`, `base64`, `hex`, `rand`):
  - `zen.escape(s)`, `zen.html(tag, attrs, ...children)` (escapes text children),
  - `zen.markdown(s)` renders markdown through zen's pipeline (nested plugins included, depth limit),
  - `zen.storage.get(key)` / `zen.storage.set(key, value)` / `zen.storage.keys(prefix)` (same store as `ctx.storage`),
  - `zen.read(path)` reads a file **in the folder** (UTF-8, ≤ 1 MB) — only when `plugin.permissions` contains `"read"`,
  - `zen.pages()` lists the folder's pages {path, title} — `"read"` permission.
- **Limits** per call: 200 ms wall time (`plugin.timeout` up to 2 s), 500 000 allocations, output 2 MB, script file 256 KB.
- **Frontend**: a script plugin can still ship `js`/`css` files (folder form) and use `zen.register`; `ctx.invoke` runs the script with `hook == "action"`.

## Trust

Script plugins without permissions cannot touch files, processes or the
network, so the prompt says "renders blocks only" and could be skipped for
them (decision for the owner). With `"read"` it says "reads files in this
folder". Hash/approval rules otherwise as today.

## Steps

1. `internal/core/scriptplugins`: compile once per file hash, clone per call, globals, `zen` module, limits; implements `plugins.Plugin` + hooks + `AssetProvider` (inline CSS served as `/_plugins/<id>/inline.css`).
2. Discovery: `*.tengo` and `*/plugin.tengo` next to `plugin.yml` folders; same id rules, same registry.
3. Port the `note` demo plugin to Tengo (keep the shell version as the "program" example) and add a `.tengo` section to the how-to.
4. Tests: inputs/outputs per hook, every limit, permission gating, malformed scripts as page warnings, concurrent calls, determinism of `zen.html` escaping.
