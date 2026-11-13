// Package cli parses the zen command line and decides whether to print
// information or start the GUI. It must not import Wails.
package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Version is set at build time with
// -ldflags "-X github.com/dragosh/zen/internal/cli.Version=<v>".
var Version = "dev"

// Launcher starts the GUI with the given absolute file paths and blocks
// until it exits.
type Launcher func(files []string) error

const usage = `zen - markdown reader and editor

Usage:
  zen [file ...]

Options:
  -h, --help       show this help
  -v, --version    show the version
`

// Run executes the command line and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer, launch Launcher) int {
	var files []string
	flags := true
	for _, arg := range args {
		switch {
		case flags && arg == "--":
			flags = false
		case flags && strings.HasPrefix(arg, "-psn_"):
		case flags && (arg == "-h" || arg == "--help"):
			fmt.Fprint(stdout, usage)
			return 0
		case flags && (arg == "-v" || arg == "--version"):
			fmt.Fprintf(stdout, "zen %s\n", Version)
			return 0
		case flags && strings.HasPrefix(arg, "-") && arg != "-":
			fmt.Fprintf(stderr, "zen: unknown option %s\n\n%s", arg, usage)
			return 2
		default:
			abs, err := filepath.Abs(arg)
			if err != nil {
				fmt.Fprintf(stderr, "zen: %v\n", err)
				return 1
			}
			files = append(files, abs)
		}
	}

	if err := launch(files); err != nil {
		fmt.Fprintf(stderr, "zen: %v\n", err)
		return 1
	}
	return 0
}
