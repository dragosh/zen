# ⊹╰(⌣ʟ⌣)╯⊹ ☯ zen.md

**zen.md** is a desktop markdown reader and editor with live reload and a runtime
plugin system, built on [Wails v3](https://v3.wails.io). macOS first. Its
command line tool is `zenmd` (`zen` still works until 0.4.0 and says so).

- Plan and architecture: [docs/PLAN.md](docs/PLAN.md)
- Feature specs, prompts and tests: [specs/](specs/README.md)

## Layout

```
engine/ builtins/ runtime/   zenmd: the engine, its built-ins and the browser runtime (@zenmd/runtime)
kit/desktop                  Go: the window shell any desktop app needs (windows, dialogs, print, Run)
kit/ui                       TypeScript: the shared UI (@zenmd/kit): frame, sidebar, page, editor, settings
apps/zenmd-app               zen.md itself: menu, the Zen service, its dialogs, and apps/zenmd-app/frontend
build/                       packaging (macOS, Wails)
main.go                      the zenmd command
```

Imports point down: apps, then kit, then engine ([docs/ENGINE.md](docs/ENGINE.md)).

## Development

Requires Go 1.26+, Node 22+ and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
```

| Command | What it does |
|---|---|
| `wails3 dev` | Run the app with live reload |
| `wails3 build` | Build `bin/zenmd` |
| `wails3 package` | Build `bin/Zen.app` (macOS) |
| `wails3 task darwin:package:dmg` | Build the `.dmg` (macOS) |
| `wails3 task test` | Run Go and frontend tests |

## CLI

```sh
zenmd [file ...]   # open files in the app
zenmd --version
zenmd --help
```

## Try it

`examples/demo` is a user guide written as a zen folder: it documents and
shows every feature. Build the app, then:

```bash
cd examples/demo && zenmd
```
