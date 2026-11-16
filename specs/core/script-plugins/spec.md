# core/script-plugins (plan)

| | |
|---|---|
| Status | done |
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

## Notes (from the build)

- **zen.markdown needs the renderer.** A hook only gets a `plugins.Context`, so
  `Context.Markdown func(src string) (string, error)` was added (core/plugins)
  and set by core/render for every hook of a page (`state.ctx`), implemented as
  a fragment of the same render: nested plugins run, their assets count for the
  page, the HTML is sanitised once with the page, and the existing
  `maxNesting` (8) bounds the depth through `state.depth`. `Zen.PluginInvoke`
  sets it for actions from the renderer of the open folder. This touches
  `internal/core/{plugins,render}`, outside the Owns row.
- **Shared trust code.** `extplugins` exports `Subject` (what `Trust` decides
  on: `Dir`, `Hash`, `Changed`), `Guard` (the fingerprint re-check, now used by
  `Plugin` too), `ScanFolder`/`FolderFingerprint`/`HashFile`/`FileFingerprint`,
  `ReadRegular`/`ResolveInside`/`CheckInside`/`InsideRel` and `ParseOption`, so
  script plugins reuse the hardening and do not copy it. A single `.tengo` file
  must be a regular file (a symlink could be repointed without changing the
  fingerprint); in the folder form the rules are the folder's.
- **The Tengo allocation budget does not bound memory.** `SetMaxAllocs` counts
  objects, so `x = x + x` (arrays) or `s = s + s` (strings, 2 GB by default)
  reached gigabytes in tens of milliseconds. zen sets `tengo.MaxStringLen` and
  `MaxBytesLen` to 4 MB and watches the heap during a call (stop at +256 MB).
  The heap figure is process-wide, so a very busy app could in theory stop an
  innocent call; the limit is far above what a page render allocates.
- **Withheld from the allowed modules:** `times.sleep` (blocks without
  honouring the timeout) and `fmt.print`, `println`, `printf` (write to zen's
  stdout).
- **Known gap, not fixed: self-containing values crash the app.** `m := {};
  m.self = m` is legal Tengo; `string(m)`, `m == m2`, `copy(m)` and `"s" + m`
  then recurse until the Go stack overflows, a fatal error nothing can
  recover. Verified with a bare `tengo.Script` (no zen code involved). Tengo has
  several ways to make a cycle (index assignment, `splice`, `append` that
  shares a backing array), so refusing one syntax is not enough. Options for the
  lead: patch/vendor Tengo, run scripts in a helper process, or accept it for
  plugins the user allowed after a prompt that says they cannot run programs
  (it can stop the app, not read anything).
- **Allocation figures.** 500 000 objects is about 150 000 loop iterations of
  `n += i % 7` (every number made counts), so loops over a few thousand items
  are fine and the demo plugins use a small fraction.
- `plugin` map keys beyond the spec: `hooks` (source, html, action), `timeout`,
  `options`, `default`, and for the folder form `js` and `stylesheets`.
  `import("zen")` is not available; `zen` is a global.

## Isolation: scripts run in a helper process (decided 2026-11-16)

Verified: Tengo values are references, so `m := {}; m.a = m; x := string(m)`
(also `==`, `copy`, `"s" + v`) recurses inside Tengo until Go's stack limit:
`fatal error: stack overflow`, which no `recover` catches. In-process, one
approved script could kill the app and lose unsaved editor text. Scripts
therefore run in a **helper process**; the authoring surface does not change.

1. **Helper.** The same `zen` binary started with a hidden mode
   (`zen __script-host`, not listed in `--help`, refused when stdin is a
   terminal). zen keeps a small pool (default 2, max 4) of long-lived helpers,
   started on first use, each serving one call at a time.
2. **Protocol.** Newline-delimited JSON over the helper's stdin/stdout.
   zen → helper: `load {id, source, hash}` (compiled once per hash and
   cached in the helper), `call {seq, id, hash, hook, globals, limits}`.
   helper → zen: `result {seq, out | error}`, and **callbacks** for the
   `zen` module: `callback {seq, cb, fn, args}` → zen answers
   `reply {cb, value | error}`. Callbacks: `markdown`, `storage.get/set/keys`,
   `read`, `pages` — permission checks, path rules and the renderer stay in
   zen, never in the helper. `escape` and `html` run in the helper (pure).
3. **Limits.** Per call: zen's wall-clock deadline (200 ms default, ≤ 2 s,
   callback time included) — on expiry zen kills the helper and starts a
   fresh one. Allocation budget, 4 MB string/bytes caps and the heap watcher
   run inside the helper, so the watcher measures only that helper. Helper
   memory limit: `debug.SetMemoryLimit(256 MB)` plus a hard kill from zen
   when its RSS passes 512 MB.
4. **Crashes.** Helper exit, broken pipe or garbage on stdout → the call
   fails with "the script plugin crashed" (error box / page warning naming
   the plugin), the helper is replaced, other calls continue. Three crashes
   of the same plugin within a minute → that plugin is switched off for the
   session with a page warning (no restart loop).
5. **Lifecycle.** Helpers are killed on folder change and quit; a helper
   gets only `PATH`, `HOME`, `TMPDIR`, `LANG`; its cwd is a private temp dir;
   it never reads files itself.
6. **Tests.** The four crash one-liners each produce a clean error and the
   next call to the same and to another plugin succeeds; timeout kills and
   replaces the helper; callbacks (markdown nesting, storage, read with and
   without permission) round-trip; concurrent calls across the pool with
   `-race`; crash-loop switch-off; helpers gone after folder change and close;
   protocol fuzz (garbage lines) does not panic zen. Existing script-plugin
   tests keep passing unchanged (they now exercise the helper).
