# plugins/sandbox (plan)

| | |
|---|---|
| Status | done |
| Target version | 0.3.0 |
| Depends on | core/extplugins, ui/plugins, **new: streaming backend calls** (below) |
| Kind | external plugin: backend (Go binary or shell) + frontend |

## Goal

Run the code on a page inside a predefined container image, from the page:

````markdown
```sandbox {image=python:3.12-slim cmd="python main.py" file=main.py}
print("hello from a container")
```
````

The block shows the code, a **Run** button, and a console with the live output
and exit code. It uses macOS's `container` command (Apple's container
platform, `/opt/homebrew/bin/container` here, version 1.6), not Docker.

## How it works

1. **Frontend** (`sandbox.js`): renders the code (editable `<textarea>` optional via `{editable}`), Run / Stop, and an output pane. Run calls `ctx.invoke({op: "run", code, image, cmd, file})` and appends streamed chunks; Stop calls `{op: "stop", id}`.
   The service note (stopped or missing) has a **Check again** button and is checked again when the window gets the focus back, after a failed run, and cleared on the first output of a run (#12).
2. **Backend** (`action` hook): checks `container system status` (offers the exact `container system start` command when the service is not running — it is not running on this Mac today), writes the code to a temp dir, then runs
   `container run --rm --name zen-<id> --read-only --progress none --no-dns --network none -c 1 -m 512M -v <tmp>:/work -w /work <image> sh -c <cmd>`
   with a wall-clock limit (default 30 s) and returns stdout, stderr, exit code. Stop = `container kill zen-<id>`.
3. **Predefined images only.** The image must be in the plugin's allow-list, configured by the user, never by the page:
   ```yaml
   plugins:
     sandbox:
       images: { python: "python:3.12-slim", node: "node:22-alpine", go: "golang:1.23-alpine" }
       timeout: 30
       network: false
   ```
   A block names a key (`image=python`); anything else is refused with a message naming the setting. Pulling an image happens only from Settings (`container image pull`), never implicitly on Run.

## Needs from the core (touches core rendering/host)

| Need | Why | Change |
|---|---|---|
| **Streaming actions** | output must appear while the program runs; `action` today returns once | `backend.hooks: [stream]`: `ctx.stream(payload, onChunk)` → Go keeps the process, forwards stdout lines as `plugin:stream` events tagged with a call id; `ctx.cancel(id)` kills the process group. Timeout up to 10 min for streams. |
| **Longer-lived calls** | 60 s cap is too low for builds | per-hook `timeout` for `stream` |
| **Per-plugin user settings only** | a page must not widen the image list | option flag `scope: user` in `options` → value read from global `config.yml` only (like `limits`) |
| **Trust wording** | this plugin starts containers | `Runs()` line: "starts containers with `container run`" |

## Steps

1. Core: `stream` hook, `ctx.stream`/`ctx.cancel`, `scope: user` options (spec `core/plugin-streams`).
2. Plugin backend as a small Go program in `plugins-src/sandbox` built into `Contents/Helpers/zen-sandbox` and listed in `resources.yml`; the plugin folder ships in `<user config dir>/zen/plugins/sandbox` on first use.
3. Frontend console (ANSI colours → classes, 1 MB output cap, auto-scroll, copy).
4. Settings section for images: list, Pull, remove; status of the container service.
5. Demo page `examples/demo/plugins/sandbox.md`.

## Security

- Containers are VM-isolated by Apple's platform; still: no network by default, read-only root, 1 CPU / 512 MB, temp dir mounted alone (never the workspace), `--rm`, name prefix `zen-` so leftovers can be cleaned on quit.
- Image allow-list lives in user config; pages choose keys only. Commands come from the block but run **inside** the container.
- Trust prompt names the capability; blocked by default like every external plugin.

## Open questions

- Should Run require a click every time (yes for alpha) or allow `{autorun}` for trusted folders?
- Keep one warm container per page for speed (`container exec`) or a fresh one per run (simpler, slower)? Start with fresh.
