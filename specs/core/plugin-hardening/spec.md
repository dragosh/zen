# core/plugin-hardening

| | |
|---|---|
| Status | Part A (Go) implemented on `feature/plugin-hardening-go`; Part B (page) pending |
| Target version | 0.2.0-alpha.2 |
| Branches | `feature/plugin-hardening-go`, `feature/plugin-hardening-ui` |
| Depends on | waves 6–8 |
| Source | docs/plugin-review-2026-11.md (findings H1–H4, M1–M7, L1–L10) |

Fixes every finding in the review. Two parts with disjoint ownership; the
**shared contract** below is fixed and both parts code against it.

## Shared contract (both parts)

1. **Plugin frontend scripts are ES modules with a default export**:
   ```js
   export default { mount(el, ctx) { …; return () => {} } };
   ```
   zen loads each plugin's JS with dynamic `import(url)` and binds the default
   export to that plugin's id. `zen.register` is **removed**; `window.zen` is
   defined with `Object.defineProperty(window, "zen", { value: Object.freeze({ version: 2 }), writable: false, configurable: false })`.
   A module without a default `mount` shows a `.zen-plugin-error` naming the plugin.
2. **Versioned asset URLs.** `Page.assets.js/css` URLs carry `?v=<first 12 hex of the plugin's approved hash>` for external plugins and `?v=<app version>` for built-ins. `/_plugins/<id>/<file>` ignores the query for lookup. A changed plugin therefore loads under a new URL.
3. **Page-aware calls.** Service signatures change to `PluginInvoke(id, page, json)` and `PluginStream(id, page, json)`; `page` is the workspace path of the page the block is on (`ctx.page.path`), and Go resolves the enabled set, options and `ZEN_PAGE` for that page's frontmatter (M4).
4. **`plugins:changed` event** (no payload) is emitted by Go after any change to the external plugin set or a trust decision (and after a folder open). The page side then unmounts every plugin wrapper, forgets all bound modules, removes plugin `<link>` stylesheets, and re-mounts from the current page's assets (new versioned URLs).
5. **Allowing needs the user.** `PluginTrust(id, true)` shows the native Allow / Block dialog in Go and only records Allow when the user confirms there; `PluginTrust(id, false)` needs no dialog.

## Part A — Go (`feature/plugin-hardening-go`)

Owns `internal/core/**`, `internal/app/**` (not plugin `assets/*.js`), `docs/PLUGINS.md`, `specs/core/{extplugins,plugin-extras}/spec.md`, `docs/plugin-review-2026-11.md` (status column only).

| Finding | Required behaviour |
|---|---|
| H1 | Plugin folder walk uses `Lstat`. `plugin.yml`, every frontend file, `command[0]` when it is a path, and every **argument** that names an existing file inside or outside the folder must resolve (symlinks followed) inside the plugin folder, else the manifest is rejected. The hash covers the resolved content of every regular file in the folder; nothing is skipped (`.DS_Store` included). Symlinks inside the folder are hashed by target content. |
| H3 | Manifest read: `Lstat`, regular file only, read via `io.LimitReader(max+1)`. Hash/walk: special files (FIFO, socket, device) → manifest rejected with a clear problem; never opened. |
| H2, L2, L3 | One helper `spawnWait(ctx, cmd, limits)` used by `run` and `Stream`: a single deadline for process + both pipes; on deadline kill the group, close the pipes, return a timeout error (never truncated success); `run` takes a context cancelled on folder change and quit. |
| M1, M2, L5, L6 | The hash is computed once when a plugin is loaded and stored on it. Approval records exactly that hash (refuse if the folder now hashes differently: re-prompt). Before every spawn and every asset serve, a cheap fingerprint (paths + size + mtime of all files) is compared with the one taken at load; a mismatch marks the plugin undecided, emits `plugins:changed`, and the call fails with "plugin changed; allow it again". Folder plugins' trust key includes the workspace root. Errors from hashing/trust writes are shown, not dropped. |
| M3 | Rendering does not hold `state.mu` while backends run: take a snapshot (renderer, registry, config) under the lock, render outside it. |
| M4 | Contract 3. |
| M6 | Streams record their plugin id; blocking/switching off a plugin or changing trust cancels its streams. |
| M7 | Plugin `allow` rules cannot allow `animate`, `set`, `use`, `foreignObject`, or attributes `values`, `from`, `to`, `by`, `srcset`, `xlink:href`, `href` on non-`a` elements, `attributename`, `style`, `on*`, nor redefine `data-zen-*`, `class`, `rel`, `id`. Rules are applied only inside that plugin's `data-zen-plugin` subtree (sanitise per wrapper, then compose). |
| L1 | Stream output is batched: at most one event per 50 ms per stream carrying the lines so far (joined with `\n`); total bytes counted once, newlines included. |
| L4 | Reloads of the external plugin set are serialised. |
| L7 | Duplicate env keys: last one wins deterministically and only one entry is emitted; document the mapping. |
| L8 | Atomic write helper (temp + fsync + rename) for trust and storage; a corrupt file is renamed to `*.corrupt-<time>` and reported once, not silently overwritten. |
| L9 | `/_ws/` serves `.js`, `.mjs`, `.html`, `.svg`? — scripts and HTML as `text/plain`; SVG keeps `Content-Security-Policy: sandbox`. |
| L10 | Finder `PATH` adds `~/.local/bin`, `~/.cargo/bin`, `/opt/local/bin`; documented in PLUGINS.md with how to use absolute paths. |
| Docs | PLUGINS.md and both specs updated to match (trust re-check, external backends run as the user with network access, storage per plugin per folder, stream event batching, the new frontend module form and that built-in plugins use it too). |

Tests: turn each review probe into a regression test (probes are at `/private/tmp/claude-501/zen-review/zen/internal/core/{extplugins,render}/probe*_test.go`), plus tests for every row above.

## Part B — page side (`feature/plugin-hardening-ui`)

Owns `frontend/src/**`, `internal/plugins/*/assets/*.js` (JS only), `examples/demo/.zen/plugins/progress/progress.js`, `examples/demo/plugins/writing-plugins.md`.

| Finding | Required behaviour |
|---|---|
| H4 | Contract 1 and 5 on the page side: runtime loads modules with `import()` keyed by plugin id from the page's assets; a ctx is created only for that id; no API to register under another id; `window.zen` non-writable. Stream events are delivered only to the ctx whose call id matches (already) and the runtime ignores events for unknown calls. |
| M5 | Contract 4: on `plugins:changed`, `workspace:opened`, and folder change: unmount all, drop bound modules, remove plugin stylesheets, remount with the new asset URLs. |
| M4 | `ctx.invoke`/`ctx.stream` pass `ctx.page.path` (contract 3). |
| Scripts | Convert `slides.js`, `course.js`, `quiz.js`, `sandbox.js`, `progress.js` to `export default { mount }` and update their vitest tests; update `writing-plugins.md` (frontend section, trust wording "allowing a plugin also lets its page script use zen's functions"). `diagram.js` keeps its own loader but must not use `zen.register`. |

## Done when

Both parts merged: `task build`, `go vet ./... && go test ./...`, `cd frontend && npx tsc --noEmit && npx vitest run` all green; every review probe, re-run, shows the fixed behaviour; Vale on `examples/demo` flags only the two deliberate typos.
