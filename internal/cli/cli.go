// Package cli parses the zen command line and decides whether to print
// information or start the GUI. It uses the standard library only.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Version is set at build time with
// -ldflags "-X github.com/dragosh/zen/internal/cli.Version=<v>".
var Version = "dev"

// Launch describes what the GUI opens.
type Launch struct {
	Dir  string // absolute directory to open
	Page string // workspace-relative page, "" for default
	Mode string // "read" | "edit"
}

// Launcher starts the GUI and blocks until it exits.
type Launcher func(Launch) error

// Build describes what `zen build` does: no window, no questions.
type Build struct {
	Dir      string // absolute folder
	Out      string // absolute output folder; "" = from config / default
	Portable bool
	Drafts   bool
	Clean    bool
}

// Builder builds a static website. It writes the summary to stdout and one
// line per warning to stderr, and returns an error when nothing usable was
// written.
type Builder func(Build, io.Writer, io.Writer) error

const usage = `zen - markdown reader and editor

Usage:
  zen [path]          open a folder or markdown file (default: current folder)
  zen edit [path]     open in the editor; a new .md file is created on save
  zen build [folder]  build a static website; options below
  zen help            show this help
  zen version         show the version

Options:
  -h, --help          show this help
  -v, --version       show the version
  --                  treat the following arguments as paths

Build options:
  -o, --out <dir>     output folder (default: _site in the folder)
  --portable          output that opens from disk (file://)
  --drafts            include pages with "draft: true"
  --clean             empty the output folder first (needs a .zen-build file)
`

type usageError string

func (e usageError) Error() string { return string(e) }

// Run executes the command line and returns the process exit code.
func Run(args []string, cwd string, stdout, stderr io.Writer, launch Launcher, build Builder) int {
	mode := "read"
	var paths []string
	flags, first := true, true
	for i, arg := range args {
		switch {
		case flags && strings.HasPrefix(arg, "-psn_"):
			continue
		case flags && arg == "--":
			flags = false
		case flags && (arg == "-h" || arg == "--help"):
			fmt.Fprint(stdout, usage)
			return 0
		case flags && (arg == "-v" || arg == "--version"):
			fmt.Fprintf(stdout, "zen %s\n", Version)
			return 0
		case flags && strings.HasPrefix(arg, "-") && arg != "-":
			return fail(stderr, usageError("unknown option "+arg))
		case flags && first && arg == "help":
			fmt.Fprint(stdout, usage)
			return 0
		case flags && first && arg == "version":
			fmt.Fprintf(stdout, "zen %s\n", Version)
			return 0
		case flags && first && arg == "build":
			return runBuild(args[i+1:], cwd, stdout, stderr, build)
		case flags && first && arg == "edit":
			mode = "edit"
		default:
			paths = append(paths, arg)
		}
		first = false
	}
	if len(paths) > 1 {
		return fail(stderr, usageError("more than one path given"))
	}

	l := Launch{Dir: cwd, Mode: mode}
	if len(paths) == 0 && finderLaunch(cwd) {
		l.Dir = "" // welcome screen
	}
	if len(paths) == 1 {
		var err error
		if l.Dir, l.Page, err = resolve(paths[0], cwd, mode == "edit"); err != nil {
			return fail(stderr, err)
		}
	}
	if err := launch(l); err != nil {
		return fail(stderr, err)
	}
	return 0
}

// runBuild parses the arguments after `zen build` and calls the builder.
func runBuild(args []string, cwd string, stdout, stderr io.Writer, build Builder) int {
	b := Build{}
	var paths []string
	out, hasOut, flags := "", false, true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case flags && arg == "--":
			flags = false
		case flags && (arg == "-h" || arg == "--help"):
			fmt.Fprint(stdout, usage)
			return 0
		case flags && arg == "--portable":
			b.Portable = true
		case flags && arg == "--drafts":
			b.Drafts = true
		case flags && arg == "--clean":
			b.Clean = true
		case flags && (arg == "-o" || arg == "--out"):
			if i+1 >= len(args) {
				return fail(stderr, usageError(arg+" needs a folder"))
			}
			i++
			out, hasOut = args[i], true
		case flags && (strings.HasPrefix(arg, "-o=") || strings.HasPrefix(arg, "--out=")):
			_, out, _ = strings.Cut(arg, "=")
			hasOut = true
		case flags && strings.HasPrefix(arg, "-") && arg != "-":
			return fail(stderr, usageError("unknown option "+arg))
		default:
			paths = append(paths, arg)
		}
	}
	if hasOut && out == "" {
		return fail(stderr, usageError("-o needs a folder"))
	}
	if len(paths) > 1 {
		return fail(stderr, usageError("more than one path given"))
	}
	b.Dir = cwd
	if len(paths) == 1 {
		var err error
		if b.Dir, _, err = resolve(paths[0], cwd, false); err != nil {
			return fail(stderr, err)
		}
	}
	if hasOut {
		b.Out = out
		if !filepath.IsAbs(out) {
			b.Out = filepath.Join(cwd, out)
		}
		b.Out = filepath.Clean(b.Out)
	}
	if build == nil {
		return fail(stderr, errors.New("this zen cannot build sites"))
	}
	if err := build(b, stdout, stderr); err != nil {
		return fail(stderr, err)
	}
	return 0
}

// finderLaunch reports whether zen should start without a folder: Finder,
// the Dock and launchd start apps with cwd "/". Checking for a terminal does
// not work (they attach /dev/null, which is a character device), and opening
// the whole disk is never what anyone wants, so cwd "/" alone decides.
func finderLaunch(cwd string) bool {
	return cwd == string(filepath.Separator)
}

func fail(stderr io.Writer, err error) int {
	var u usageError
	if errors.As(err, &u) {
		fmt.Fprintf(stderr, "zen: %v\n\n%s", err, usage)
		return 2
	}
	fmt.Fprintf(stderr, "zen: %v\n", err)
	return 1
}

func isMarkdown(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown":
		return true
	}
	return false
}

// resolve maps a path argument to a workspace directory and page.
func resolve(arg, cwd string, allowNew bool) (dir, page string, err error) {
	p := arg
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)

	info, statErr := os.Stat(p)
	switch {
	case statErr == nil && info.IsDir():
		return p, "", nil
	case !isMarkdown(p):
		return "", "", usageError(arg + " is neither a directory nor a markdown file")
	case statErr == nil:
	case allowNew && errors.Is(statErr, os.ErrNotExist):
		if pi, err := os.Stat(filepath.Dir(p)); err != nil || !pi.IsDir() {
			return "", "", usageError(arg + ": parent directory does not exist")
		}
	default:
		return "", "", usageError(arg + ": no such markdown file")
	}

	dir = root(filepath.Dir(p))
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return "", "", err
	}
	return dir, filepath.ToSlash(rel), nil
}

// root returns the nearest ancestor of start (inclusive) containing .zen.yml
// or .git, searching only strictly below the user's home directory. Without
// a marker, start itself is the root.
func root(start string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return start
	}
	home = filepath.Clean(home)
	for d := start; within(d, home); d = filepath.Dir(d) {
		for _, m := range []string{".zen.yml", ".git"} {
			if _, err := os.Stat(filepath.Join(d, m)); err == nil {
				return d
			}
		}
	}
	return start
}

// within reports whether d is strictly inside home.
func within(d, home string) bool {
	rel, err := filepath.Rel(home, d)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
