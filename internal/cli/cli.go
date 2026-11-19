// Package cli parses the zenmd command line and decides whether to print
// information or start the GUI. It uses the standard library only.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zenmark-org/zenmd/kit/addons"
)

// Name is the command and executable: the name of the engine.
const Name = "zenmd"

// Alias is the old command name. It keeps working until 0.4.0 and says so.
const Alias = "zen" // removed in 0.4.0

// ProgramName tells which name the program was started as, from argv[0]:
// "zen" (the alias, matched by base name and ignoring case, as the disk may
// be case-insensitive) or "zenmd" for anything else (including a temporary
// binary from `go run .`). The link name counts, not the link's target.
func ProgramName(argv0 string) string {
	base := filepath.Base(filepath.FromSlash(argv0))
	if argv0 != "" && strings.EqualFold(base, Alias) {
		return Alias
	}
	return Name
}

// Notice prints the alias line for "zen" and nothing for any other name.
func Notice(w io.Writer, invokedAs string) {
	if invokedAs == Alias {
		fmt.Fprintln(w, "zen is now zenmd; the zen name stops working in 0.4.0")
	}
}

// Version is set at build time with
// -ldflags "-X github.com/zenmark-org/zenmd/internal/cli.Version=<v>".
var Version = "dev"

// addonList is the add-ons of this build (kit/addons); tests replace it.
var addonList = addons.List

// VersionLine is "zenmd 0.6.0", followed by the add-ons of the build when it has
// some: "zenmd 0.6.0 (add-ons: vale, voice)". The About panel shows the same.
func VersionLine() string {
	line := "zenmd " + Version
	var ids []string
	for _, a := range addonList() {
		ids = append(ids, a.ID)
	}
	if len(ids) > 0 {
		line += " (add-ons: " + strings.Join(ids, ", ") + ")"
	}
	return line
}

// Launch describes what the GUI opens.
type Launch struct {
	Dir  string // absolute directory to open
	Page string // workspace-relative page, "" for default
	Mode string // "read" | "edit"
}

// Launcher starts the GUI and blocks until it exits.
type Launcher func(Launch) error

// Build describes what `zenmd build` does: no window, no questions.
type Build struct {
	Dir      string // absolute folder
	Out      string // absolute output folder; "" = from config / default
	Portable bool   // --portable: opens from disk (the default)
	Hosted   bool   // --hosted: scripts as files, for a web server
	Drafts   bool
	NoDrafts bool // leave drafts out even when build.drafts is true (the app's dialog)
	Clean    bool
	NoPrint  bool // --no-print: no print.html
}

// Builder builds a static website. It writes the summary to stdout and one
// line per warning to stderr, and returns an error when nothing usable was
// written.
type Builder func(Build, io.Writer, io.Writer) error

// Init describes `zenmd init`: start a folder from a site template, or list and
// update the templates. It never asks anything, so it works in scripts.
type Init struct {
	Dir, Template string            // absolute target folder; template id (or the id to update)
	Values        map[string]string // --title, --author and --set key=value
	List, Update  bool
	ShowDir       bool
}

// Initer does what `zenmd init` asks. It writes its result to stdout and notes
// to stderr. Return UsageError for a wrong request (exit 2) and any other
// error for a failure (exit 1).
type Initer func(Init, io.Writer, io.Writer) error

// UsageError is an error that makes zenmd print the usage and exit with 2.
func UsageError(msg string) error { return usageError(msg) }

const usage = `zenmd - the Zenmark engine: markdown folders as pages, sites and print

Usage:
  zenmd [path]          open a folder or markdown file (default: current folder)
  zenmd edit [path]     open in the editor; a new .md file is created on save
  zenmd export site [folder]
                        build a static website; options below
  zenmd export pdf [folder] [-o file.pdf]
                        write the whole folder as one PDF file
  zenmd build [folder]  same as export site
  zenmd init site [--techdocs|--learning|--planner] [folder]
                        start a folder from a site template; options below
  zenmd help            show this help
  zenmd version         show the version

Options:
  -h, --help          show this help
  -v, --version       show the version
  --                  treat the following arguments as paths

Init options:
  --template <id>     the template (--<id> works too); none: list them
  --title <text>      the site title; --author <text>; --set <key=value>
  --list              list the templates; --update <id>; --dir

Export pdf options:
  -o, --out <file>    output file (default: <folder>/<folder name>.pdf)

Build options (export site, build):
  -o, --out <dir>     output folder (default: _site in the folder)
  --hosted            output for a web server (does not open from disk)
  --portable          output that also opens from disk (the default)
  --drafts            include pages with "draft: true"
  --clean             empty the output folder first (needs a .zen-build file)
  --no-print          leave out print.html (the whole site as one page)
`

type usageError string

func (e usageError) Error() string { return string(e) }

// Run executes the command line and returns the process exit code.
func Run(args []string, cwd string, stdout, stderr io.Writer, launch Launcher, build Builder, initer Initer, pdf PDFExporter) int {
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
			fmt.Fprintln(stdout, VersionLine())
			return 0
		case flags && strings.HasPrefix(arg, "-") && arg != "-":
			return fail(stderr, usageError("unknown option "+arg))
		case flags && first && arg == "help":
			fmt.Fprint(stdout, usage)
			return 0
		case flags && first && arg == "version":
			fmt.Fprintln(stdout, VersionLine())
			return 0
		case flags && first && arg == "build":
			return runBuild(args[i+1:], cwd, stdout, stderr, build)
		case flags && first && arg == "export":
			return runExport(args[i+1:], cwd, stdout, stderr, build, pdf)
		case flags && first && arg == "init":
			return runInit(args[i+1:], cwd, stdout, stderr, initer)
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

// runBuild parses the arguments after `zenmd build` and calls the builder.
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
		case flags && arg == "--hosted":
			b.Hosted = true
		case flags && arg == "--drafts":
			b.Drafts = true
		case flags && arg == "--no-print":
			b.NoPrint = true
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
	if b.Portable && b.Hosted {
		return fail(stderr, usageError("--portable and --hosted cannot be used together"))
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
		return fail(stderr, errors.New("this zenmd cannot build sites"))
	}
	if err := build(b, stdout, stderr); err != nil {
		return fail(stderr, err)
	}
	return 0
}

var initFlagRE = regexp.MustCompile(`^--[a-z][a-z0-9-]*$`)

// runInit parses the arguments after `zenmd init` and calls the initer.
func runInit(args []string, cwd string, stdout, stderr io.Writer, initer Initer) int {
	in := Init{Values: map[string]string{}}
	var paths []string
	set := map[string]string{}
	var title, author *string
	site, flags, noTemplate := false, true, false
	needs := func(i *int, name string) (string, error) {
		if *i+1 >= len(args) || args[*i+1] == "" {
			return "", usageError(name + " needs a value")
		}
		*i++
		return args[*i], nil
	}
	var perr error
	pickTemplate := func(id string) {
		if in.Template != "" && perr == nil {
			perr = usageError("more than one template given")
		}
		in.Template = id
	}
	for i := 0; i < len(args) && perr == nil; i++ {
		arg := args[i]
		name, val, hasVal := arg, "", false
		if flags && strings.HasPrefix(arg, "--") {
			name, val, hasVal = strings.Cut(arg, "=")
		}
		value := func() string {
			if hasVal {
				if val == "" {
					perr = usageError(name + " needs a value")
				}
				return val
			}
			v, err := needs(&i, name)
			if err != nil {
				perr = err
			}
			return v
		}
		switch {
		case flags && arg == "--":
			flags = false
		case flags && (arg == "-h" || arg == "--help"):
			fmt.Fprint(stdout, usage)
			return 0
		case flags && name == "--list":
			in.List = true
		case flags && name == "--dir":
			in.ShowDir = true
		case flags && name == "--update":
			in.Update = true
			pickTemplate(value())
			noTemplate = true
		case flags && name == "--template":
			pickTemplate(value())
		case flags && name == "--title":
			v := value()
			title = &v
		case flags && name == "--author":
			v := value()
			author = &v
		case flags && name == "--set":
			kv := value()
			k, v, ok := strings.Cut(kv, "=")
			if perr == nil && (!ok || k == "") {
				perr = usageError("--set needs key=value, not " + kv)
			}
			set[k] = v
		case flags && !hasVal && initFlagRE.MatchString(arg):
			pickTemplate(arg[2:]) // --techdocs, --learning, --planner, --<id>
		case flags && strings.HasPrefix(arg, "-") && arg != "-":
			perr = usageError("unknown option " + arg)
		case !site && len(paths) == 0 && arg == "site":
			site = true
		default:
			paths = append(paths, arg)
		}
	}
	modes := 0
	for _, b := range []bool{in.List, in.ShowDir, in.Update} {
		if b {
			modes++
		}
	}
	switch {
	case perr != nil:
		return fail(stderr, perr)
	case modes > 1:
		return fail(stderr, usageError("--list, --dir and --update cannot be used together"))
	case modes == 1 && (site || len(paths) > 0 || len(set) > 0 || title != nil || author != nil || (in.Template != "" && !noTemplate)):
		return fail(stderr, usageError("--list, --dir and --update take nothing else"))
	case modes == 0 && !site:
		return fail(stderr, usageError("zenmd init what? try: zenmd init site --techdocs"))
	case len(paths) > 1:
		return fail(stderr, usageError("more than one folder given"))
	}
	for k, v := range set {
		in.Values[k] = v
	}
	if title != nil {
		in.Values["title"] = *title
	}
	if author != nil {
		in.Values["author"] = *author
	}
	in.Dir = cwd
	if len(paths) == 1 {
		in.Dir = paths[0]
		if !filepath.IsAbs(in.Dir) {
			in.Dir = filepath.Join(cwd, in.Dir)
		}
		in.Dir = filepath.Clean(in.Dir)
	}
	if initer == nil {
		return fail(stderr, errors.New("zenmd init is not available in this build"))
	}
	missing := modes == 0 && in.Template == ""
	if missing {
		in.List = true // no template: show the choices
	}
	if err := initer(in, stdout, stderr); err != nil {
		return fail(stderr, err)
	}
	if missing {
		fmt.Fprintln(stderr, "zenmd: choose a template, for example: zenmd init site --techdocs")
		return 2
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
		fmt.Fprintf(stderr, "zenmd: %v\n\n%s", err, usage)
		return 2
	}
	fmt.Fprintf(stderr, "zenmd: %v\n", err)
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
