---
title: Sandbox
---
# Sandbox

A `sandbox` block runs its code inside a container, from the page. The block
shows the code, a Run button and a console that fills while the program runs.
It uses the `container` tool on your Mac (Apple's container platform), not Docker.

## One-time setup

1. Install Apple's container tool and check that `container --version` works in a terminal.
2. Start its service. Zenmark never starts it for you:

   ```sh
   container system start
   ```

3. Pull the image once. Zenmark never pulls an image when you press Run:

   ```sh
   container image pull python:3.12-slim
   ```

4. Name the image in your global config. Open **Settings › Configuration**,
   pick **config.yml** and press **Open in Editor** (or **Create** if it isn't
   there yet). Add:

   ```yaml
   plugins:
     sandbox:
       images:
         python: "python:3.12-slim"
       timeout: 30        # seconds, at most 600
       network: false     # no network by default
       memory: 512M
       cpus: 1
   ```

A page picks an image by its key (`image=python`). It can never name an image
itself, and a folder's `.zenmd.yml` cannot change these settings.

## A Python block

Add `editable` to get a text area you can change before pressing Run.

````markdown
```sandbox {image=python cmd="python main.py" file=main.py editable}
print("hello from a container")
```
````

```sandbox {image=python cmd="python main.py" file=main.py editable}
import platform
print("hello from a container")
print(platform.machine())
```

## A shell one-liner

The `cmd` runs with `sh -c` in the `/work` folder, next to the saved file.

```sandbox {image=python cmd="sh main.sh" file=main.sh}
echo "files here:"; ls
echo "this goes to stderr" >&2
exit 3
```

The console shows the exit code and how long the run took. Error output is
red-tinted.

## Block attributes

| Attribute | Meaning |
|---|---|
| `image` | a key of the `images` setting |
| `cmd` | the command to run, at most 2000 characters |
| `file` | the plain file name the code is saved as in `/work` |
| `editable` | show the code in a text area |

## Limits and safety

- Every run starts a fresh container that is removed when it ends.
- The root file system is read-only. Only a temporary folder is mounted, at `/work`;
  your notes folder is never mounted, and the folder is deleted afterwards.
- No network unless you set `network: true`. The default limits are one CPU and 512 MB.
- A run stops after `timeout` seconds. Stop kills the container at once.
- The code is at most 1 MB and the console keeps the first 1 MB of output.
- If the service is stopped, the block says so and tells you to run
  `container system start`. The note goes away by itself when you come back
  to the window with the service running, or press **Check again**. If an image is not set up, the block shows the code
  and the setting to add.
