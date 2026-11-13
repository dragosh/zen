# cli/entrypoint

Status: done (superseded by [cli/commands](../commands/spec.md))

## Purpose

The single `zen` binary decides from its arguments whether to print
information and exit, or to start the GUI with a list of files to open.

## Behaviour

1. With no arguments, the GUI starts with no files.
2. Positional arguments are file paths. Each is converted to an absolute path
   against the working directory and passed to the GUI in order. Existence is
   not checked here; opening is the `core/document` scope's job.
3. `--version` or `-v` prints `zen <version>` to stdout and exits 0 without
   starting the GUI.
4. `--help` or `-h` prints usage to stdout and exits 0 without starting the
   GUI.
5. An unknown flag prints an error and the usage to stderr and exits 2
   without starting the GUI.
6. `--` ends flag parsing; everything after it is a file path.
7. Arguments starting with `-psn_` are ignored (macOS adds one when an app is
   launched from Finder on some versions).
8. If the GUI returns an error, it is printed to stderr and the exit code
   is 1.

## Interfaces

```go
package cli

var Version = "dev" // set with -ldflags "-X .../internal/cli.Version=..."

type Launcher func(files []string) error

func Run(args []string, stdout, stderr io.Writer, launch Launcher) int
```

`main.go` calls `os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, launch))`.

## Acceptance criteria

All tests are in `internal/cli/cli_test.go`.

| # | Criterion | Test |
|---|---|---|
| 1 | No arguments launches with no files | `TestRunNoArgsLaunches` |
| 2 | Paths are made absolute, order kept | `TestRunFilesAreAbsolute` |
| 3 | Version flags print and do not launch | `TestRunVersion` |
| 4 | Help flags print usage and do not launch | `TestRunHelp` |
| 5 | Unknown flag exits 2, writes to stderr | `TestRunUnknownFlag` |
| 6 | `--` allows dash-prefixed file names | `TestRunDoubleDash` |
| 7 | `-psn_` arguments are dropped | `TestRunIgnoresProcessSerialNumber` |
| 8 | Launcher error exits 1 | `TestRunLaunchError` |

## Out of scope

Subcommands (`render`, `plugin`), forwarding files to an already running
instance, installing the command line tool.

## App Store notes

None for parsing. Hand-off to a sandboxed running app is covered in a later
`cli` feature.
