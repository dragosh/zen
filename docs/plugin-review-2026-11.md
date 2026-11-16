# Plugin system review — November 2026

Read-only review of the plugin internals (`internal/core/extplugins`,
`internal/core/plugins`, `internal/core/render`, `internal/app/{extplugins,stream,assets,state}.go`,
`frontend/src/plugins/runtime.ts`, `frontend/src/page/assets.ts`) after waves 6–8.
Every **high** finding was reproduced with a probe test (kept outside the repo)
or confirmed by reading the code; none is fixed yet.

## High

| # | Finding | Evidence | Fix | Status |
|---|---|---|---|---|
| H1 | **The trust hash does not cover what runs.** A symlinked `plugin.yml` (or argument file) pointing outside the plugin folder can change after approval and run with no new prompt; `.DS_Store` is skipped by the hash even when it is the command. | probe: manifest changed outside → still allowed, block printed `EVIL`; `.DS_Store` changed → same hash | Refuse symlinks and arguments that resolve outside the plugin folder; read `plugin.yml` only from inside; stop skipping files the plugin references | fixed (Go) |
| H2 | **Timeout and cancel stop working** when a backend's child leaves the process group and keeps stderr open: one grace timer is shared by two waits. | probe: 2 s timeout returned after 3.2 s; stream cancelled at 0.3 s ran 4.1 s and kept emitting | One deadline for both waits, then close the pipes; one helper for `run` and `Stream` | fixed (Go) |
| H3 | **A malicious folder can hang or exhaust the app before any prompt:** `plugin.yml → /dev/zero` reads unbounded; a FIFO as `plugin.yml` or anywhere in the folder blocks discovery/hashing forever (inside `open()`, holding the app). | probes: memory climbs; `Scan` and `Hash` blocked > 2 s | `Lstat` + regular files only, read through a size limit, skip special files when hashing | fixed (Go) |
| H4 | **Page scripts are not isolated.** Any allowed script can call any service (incl. `PluginTrust(id, true)`, `Save`, `OpenExternal`), `zen.register` another plugin's id to take over its blocks, storage and backend, or replace `window.zen`. | read: `runtime.ts` `register`, `ensureGlobal`; `PluginTrust` has no native confirm | Native confirm for allowing a plugin; bind handlers to the script that loaded them (`import()` + default export); make `window.zen` non-writable | page side (part B) |

## Medium

| # | Finding | Fix | Status |
|---|---|---|---|
| M1 | Plugin files edited during a session (or `git pull`) run immediately without a new prompt; trust is only checked on folder open | Re-check the approved hash (or a cheap fingerprint) before each spawn and asset serve, or watch plugin folders | fixed (Go) |
| M2 | Approval is recorded for the files as they are when the user clicks, not as shown | Hash once at load and approve exactly that hash | fixed (Go) |
| M3 | Backends run while the app state lock is held, so a slow block stalls the whole app (forever with H2) | Snapshot renderer and registry under the lock, render outside it | fixed (Go) |
| M4 | `ctx.invoke` / `ctx.stream` ignore the page's frontmatter (plugin enabled only on a page reports "switched off"; options missing; `ZEN_PAGE` may be another page) | Send the page path; resolve settings for that page | Go done; page side passes the page (part B) |
| M5 | Plugin scripts and CSS are never unloaded: a blocked plugin keeps running; folder B's plugin with folder A's id never loads and A's handler drives B's backend; edits need a window reload | Hash-busted URLs; drop handlers and assets (or reload the webview) on folder or trust change | page side (part B); Go sends `plugins:changed` and versioned URLs |
| M6 | Blocking or switching off a plugin does not stop its running streams | Track the plugin id per stream and cancel on change | fixed (Go) |
| M7 | A plugin's `allow` rules widen the sanitiser for the whole page and can override zen's own rules (`animate values="javascript:…"`, `use xlink:href`, `srcset` survived) — only the CSP stops execution | Deny URL-carrying attributes and `animate`/`set`/`use`; never let `allow` redefine `data-zen-*`, `class`, `rel`; scope to the plugin's subtree | fixed (Go) |

## Low

| # | Finding | Status |
|---|---|---|
| L1 | One IPC event per output line: 4 MB of newlines = 4,000,000 events; empty lines are not counted against the limit | fixed (Go) |
| L2 | Truncated output reported as success when a grandchild holds stdout | fixed (Go) |
| L3 | Render and invoke processes are not cancelled on folder change or quit | fixed (Go) |
| L4 | Concurrent trust changes can leave a stale plugin list | fixed (Go) |
| L5 | Trust is keyed by real path, not workspace: a folder symlinking to an approved plugin runs it automatically | fixed (Go) |
| L6 | If hashing fails, Allow silently does nothing and the prompt repeats | fixed (Go) |
| L7 | `{a-b}` and `{a_b}` map to the same `ZEN_ATTR_A_B` (duplicate env entries) | fixed (Go) |
| L8 | Corrupt `storage.json` / `trusted-plugins.yml` are treated as empty and overwritten; no fsync | fixed (Go) |
| L9 | `/_ws/*.js` is served as JavaScript, so `script-src 'self'` also allows workspace scripts; the sanitiser is the only barrier | fixed (Go) |
| L10 | `PATH` from Finder lacks `~/.local/bin`, `~/.cargo/bin`, nvm, MacPorts: plugins can work from the terminal and fail from Finder | fixed (Go) |

## Docs and specs that disagree with the code

- PLUGINS.md "a change asks again" — only on folder reopen (M1).
- PLUGINS.md "no plugin receives file or network access" — true for built-ins, not for external backends, which run as the user.
- Stream output "only to the block that started it" — events reach every window and any script (H4).
- extplugins spec: "hash covers every file" (H1); built-ins refused for storage/invoke (superseded); "timeout > 60s rejected" (streams allow 10 min); storage limit is per plugin **per folder**.

## Simplifications

- One spawn / wait / kill / drain helper with a single deadline for `run` and `Stream` (fixes H2, L2).
- Hash once at load; keep it on the plugin; use it for decision, approval and pre-spawn checks (fixes M1, M2).
- Load plugin frontends with `import()` and a default export instead of `zen.register(id)` (fixes most of H4 and M5).
- One atomic-write helper for trust and storage; count stream output in one place.
- A **dev mode** for plugins in the user folder: no re-prompt on every edit, reload scripts on change.
- Script plugins in Tengo (specs/core/script-plugins) remove processes, environment variables and most trust questions for the common "block → HTML" case.

## Suggested fix order

1. **Containment** — H3, H2 (+L2, L3), M3: a bad or slow plugin can never hang or exhaust the app.
2. **Trust integrity** — H1, M1, M2, L5, L6: what runs is exactly what was approved.
3. **Page isolation** — H4, M5, M6, M7, L9: scripts only reach their own plugin; blocking really stops a plugin.
4. **Correctness** — M4, L1, L4, L7, L8, L10, docs/spec drift.
5. Then **script plugins (Tengo)** on the hardened base.
