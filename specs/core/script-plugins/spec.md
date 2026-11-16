# core/script-plugins (plan)

| | |
|---|---|
| Status | specified |
| Target version | 0.2.0 |
| Depends on | core/extplugins, core/plugin-extras |
| Branch | `feature/core-script-plugins` |
| Owns | `internal/core/scriptplugins/` (new), `internal/core/extplugins/` (discovery, trust, Plugins() listing for script plugins), `internal/app/extplugins.go` (wiring only), `go.mod`/`go.sum` (add `github.com/d5/tengo/v2 v2.17.0` only), docs/PLUGINS.md, `examples/demo/.zen/plugins/` (new `.tengo` samples), `examples/demo/plugins/writing-plugins.md`, `internal/app/demo_test.go` |

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

## Trust (decided)

Script plugins go through exactly the same discovery, hashing, fingerprint
re-check, per-workspace approval and `plugins:changed` flow as folder plugins
(core/plugin-hardening): the hash covers the `.tengo` file (single-file form)
or the whole folder (folder form). They are **always asked about** — no
silent allow — but the prompt describes them truthfully:
- no permissions: "renders blocks inside zen; cannot read files, run programs or use the network";
- `read`: "… and reads files in this folder".
`Plugins()` reports `kind: "script"` (folder plugins `"program"` or `"page"`) so Settings can show it.

## Steps

0. Add `github.com/d5/tengo/v2 v2.17.0` to go.mod (v3.0.0 is not a valid module).
1. `internal/core/scriptplugins`: compile once per file hash, clone per call, globals, `zen` module, limits; implements `plugins.Plugin` + hooks + `AssetProvider` (inline CSS served as `/_plugins/<id>/inline.css`).
2. Discovery: `*.tengo` and `*/plugin.tengo` next to `plugin.yml` folders; same id rules, same registry.
3. Demo: add `examples/demo/.zen/plugins/callout.tengo` (single file: `:::tip` / `:::caution` callouts with inline CSS and `zen.markdown` for the body) and `examples/demo/.zen/plugins/wordcount/plugin.tengo` (folder form, `permissions: ["read"]`, an inline `:words[]` showing the page's word count and a `:::pages` block listing every page with its word count via `zen.pages()` + `zen.read()`). Keep `note` (shell) as the program example. In `writing-plugins.md` add "The simplest plugin: one Tengo file" first, before the program section, with both samples' full source, the inputs/`zen` module table, limits, and permissions; use `:::tip` on that page so it renders. `TestDemoFolder` trusts the new samples and asserts they render.
4. Tests: inputs/outputs per hook (block, inline, source, html, action via `PluginInvoke`), every limit (timeout, allocations, output, file size), permission gating (`zen.read` without `read` fails; paths outside the folder, `..`, symlinks out, non-markdown binary files refused), unknown imports (`os`) refused at load, malformed scripts as page warnings, concurrent calls with `-race`, `zen.html` escaping (attributes and text), `zen.markdown` nesting depth, trust/hash/fingerprint re-prompt after editing the `.tengo` file.
