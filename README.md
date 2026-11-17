# ⊹╰(⌣ʟ⌣)╯⊹ ☯ zen.md

**zen.md** is a desktop markdown reader and editor with live reload and a runtime
plugin system, built on [Wails v3](https://v3.wails.io). macOS first. Its
command line tool is `zen`.

- Plan and architecture: [docs/PLAN.md](docs/PLAN.md)
- Feature specs, prompts and tests: [specs/](specs/README.md)

## Development

Requires Go 1.26+, Node 22+ and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
```

| Command | What it does |
|---|---|
| `wails3 dev` | Run the app with live reload |
| `wails3 build` | Build `bin/zen` |
| `wails3 package` | Build `bin/zen.app` (macOS) |
| `wails3 task darwin:package:dmg` | Build the `.dmg` (macOS) |
| `wails3 task test` | Run Go and frontend tests |

## CLI

```sh
zen [file ...]     # open files in the app
zen --version
zen --help
```

## Try it

`examples/demo` is a user guide written as a zen folder: it documents and
shows every feature. Build the app, then:

```bash
cd examples/demo && zen
```
