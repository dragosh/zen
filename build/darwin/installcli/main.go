// Command installcli links the zenmd command (and its zen alias, which stops
// working in 0.4.0) into a folder on the PATH, or removes those links.
//
//	go run ./build/darwin/installcli -app /Applications/Zen.app -dir /usr/local/bin [-uninstall]
//	go run ./build/darwin/installcli -exe bin/zenmd -dir ~/.local/bin   # a development build
//
// It only replaces or removes links that point at zen.md: into a Zen.app
// bundle, at any zenmd executable (a development build), or at the old bin/zen
// of a zen checkout. A different tool called zen is left alone with a warning.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	name  = "zenmd"
	alias = "zen" // removed in 0.4.0
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("installcli", flag.ContinueOnError)
	fs.SetOutput(stderr)
	app := fs.String("app", "/Applications/Zen.app", "the installed Zen.app")
	exe := fs.String("exe", "", "link this executable instead of the one in -app (a development build)")
	dir := fs.String("dir", "/usr/local/bin", "the folder for the links")
	uninstall := fs.Bool("uninstall", false, "remove the links instead")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	target := *exe
	if target == "" {
		target = filepath.Join(*app, "Contents", "MacOS", name)
	}
	if abs, err := filepath.Abs(target); err == nil {
		target = abs
	}
	l := linker{dir: *dir, target: target, dev: *exe != "", stdout: stdout, stderr: stderr}
	if *uninstall {
		return l.uninstall()
	}
	return l.install()
}

type linker struct {
	dir, target    string
	dev            bool
	stdout, stderr io.Writer
}

func (l linker) install() int {
	if fi, err := os.Stat(l.target); err != nil || fi.IsDir() {
		where := l.target
		if !l.dev {
			where = filepath.Dir(filepath.Dir(filepath.Dir(l.target))) // the .app
		}
		fmt.Fprintf(l.stderr, "installcli: %s not found. Copy bin/Zen.app to /Applications first (or set APP_PATH).\n", where)
		return 1
	}
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		fmt.Fprintf(l.stderr, "installcli: %v\n", err)
		return 1
	}
	code := 0
	for _, n := range []string{name, alias} {
		link := filepath.Join(l.dir, n)
		switch state := l.state(link); state {
		case absent, ours:
			if state == ours {
				if err := os.Remove(link); err != nil {
					fmt.Fprintf(l.stderr, "installcli: %v\n", err)
					code = 1
					continue
				}
			}
			if err := os.Symlink(l.target, link); err != nil {
				fmt.Fprintf(l.stderr, "installcli: %v\n", err)
				code = 1
				continue
			}
			fmt.Fprintf(l.stdout, "Linked %s -> %s\n", link, l.target)
		default:
			fmt.Fprintf(l.stderr, "installcli: warning: %s is not a link into Zen.app (another tool?); left alone\n", link)
		}
	}
	return code
}

func (l linker) uninstall() int {
	code := 0
	names := []string{name, alias}
	states := []linkState{l.state(filepath.Join(l.dir, name)), l.state(filepath.Join(l.dir, alias))} // before removing: zen may point at zenmd
	for i, n := range names {
		link := filepath.Join(l.dir, n)
		switch states[i] {
		case ours:
			if err := os.Remove(link); err != nil {
				fmt.Fprintf(l.stderr, "installcli: %v\n", err)
				code = 1
				continue
			}
			fmt.Fprintf(l.stdout, "Removed %s\n", link)
		case foreign:
			fmt.Fprintf(l.stderr, "installcli: warning: %s is not a link into Zen.app; left alone\n", link)
		}
	}
	return code
}

type linkState int

const (
	absent linkState = iota
	ours
	foreign
)

// state classifies path: missing, a link into Zen.app (possibly through other
// links, such as zen -> zenmd -> the bundle, or dangling), or anything else.
func (l linker) state(path string) linkState {
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return absent
	}
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return foreign
	}
	for hops := 0; hops < 8; hops++ {
		to, err := os.Readlink(path)
		if err != nil {
			return foreign
		}
		if !filepath.IsAbs(to) {
			to = filepath.Join(filepath.Dir(path), to)
		}
		to = filepath.Clean(to)
		if l.isZen(to) {
			return ours
		}
		if fi, err := os.Lstat(to); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			return foreign
		}
		path = to
	}
	return foreign
}

// isZen reports whether p is one of zen.md's executables: inside a Zen.app
// bundle, the development executable this run links (or its zen sibling), any
// executable named zenmd (a development build of a checkout; switching between
// it and the app needs no manual rm), or the bin/zen of a zen checkout (the
// development link made before the rename).
func (l linker) isZen(p string) bool {
	if l.dev && (p == l.target || p == filepath.Join(filepath.Dir(l.target), alias)) {
		return true
	}
	if filepath.Base(p) == name {
		return true
	}
	if dir := filepath.Dir(p); filepath.Base(p) == alias && filepath.Base(dir) == "bin" {
		if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "build", "darwin", "installcli")); err == nil {
			return true
		}
	}
	parts := strings.Split(p, string(filepath.Separator))
	for i := 0; i+3 < len(parts); i++ {
		if strings.EqualFold(parts[i], "Zen.app") && parts[i+1] == "Contents" && parts[i+2] == "MacOS" {
			return true
		}
	}
	return false
}
