package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type result struct {
	code     int
	stdout   string
	stderr   string
	launched bool
	l        Launch
	built    bool
	b        Build
	inited   bool
	i        Init
}

// builderErr is what the fake Builder returns.
var builderErr error

// initerErr is what the fake Initer returns.
var initerErr error

func run(t *testing.T, cwd string, launchErr error, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	r := result{}
	r.code = Run(args, cwd, &out, &errOut, func(l Launch) error {
		r.launched = true
		r.l = l
		return launchErr
	}, func(b Build, stdout, stderr io.Writer) error {
		r.built = true
		r.b = b
		fmt.Fprintln(stdout, "Built 1 pages and 0 files into out (1 warnings)")
		fmt.Fprintln(stderr, "a.md: a warning")
		return builderErr
	}, func(i Init, stdout, stderr io.Writer) error {
		r.inited = true
		r.i = i
		if i.List {
			fmt.Fprintln(stdout, "techdocs  Technical documentation — Diátaxis")
		}
		return initerErr
	})
	r.stdout, r.stderr = out.String(), errOut.String()
	return r
}

// home makes a fake home directory and returns it.
func home(t *testing.T) string {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	return h
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func expect(t *testing.T, r result, want Launch) {
	t.Helper()
	if r.code != 0 || !r.launched || r.l != want {
		t.Fatalf("got %+v, want launch %+v", r, want)
	}
}

func TestNoArgsOpensCwd(t *testing.T) {
	cwd := t.TempDir()
	expect(t, run(t, cwd, nil), Launch{Dir: cwd, Page: "", Mode: "read"})

	r := run(t, cwd, errors.New("boom"))
	if r.code != 1 || !strings.Contains(r.stderr, "boom") {
		t.Fatalf("launch failure: %+v", r)
	}
}

func TestPathArguments(t *testing.T) {
	h := home(t)
	ws := filepath.Join(h, "ws")
	write(t, filepath.Join(ws, ".zen.yml"))
	write(t, filepath.Join(ws, "docs", "a.md"))

	// directory, relative to cwd
	expect(t, run(t, h, nil, "ws"), Launch{Dir: ws, Mode: "read"})
	// file under a .zen.yml workspace
	expect(t, run(t, h, nil, "ws/docs/a.md"), Launch{Dir: ws, Page: "docs/a.md", Mode: "read"})

	// .git marker
	repo := filepath.Join(h, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repo, "x", "y", "b.markdown"))
	expect(t, run(t, h, nil, filepath.Join(repo, "x", "y", "b.markdown")),
		Launch{Dir: repo, Page: "x/y/b.markdown", Mode: "read"})

	// no marker: file's own directory
	write(t, filepath.Join(h, "plain", "sub", "c.md"))
	expect(t, run(t, h, nil, "plain/sub/c.md"),
		Launch{Dir: filepath.Join(h, "plain", "sub"), Page: "c.md", Mode: "read"})

	// marker in home itself is never used as root
	write(t, filepath.Join(h, ".zen.yml"))
	write(t, filepath.Join(h, "other", "d.md"))
	expect(t, run(t, h, nil, "other/d.md"),
		Launch{Dir: filepath.Join(h, "other"), Page: "d.md", Mode: "read"})
	// file directly in home: its directory (home) is the root, no climbing
	write(t, filepath.Join(h, "e.md"))
	expect(t, run(t, h, nil, "e.md"), Launch{Dir: h, Page: "e.md", Mode: "read"})

	// outside home: no climbing to /
	out := t.TempDir()
	write(t, filepath.Join(out, "f.md"))
	expect(t, run(t, h, nil, filepath.Join(out, "f.md")), Launch{Dir: out, Page: "f.md", Mode: "read"})
}

func TestEdit(t *testing.T) {
	h := home(t)
	write(t, filepath.Join(h, "ws", ".zen.yml"))
	write(t, filepath.Join(h, "ws", "a.md"))
	ws := filepath.Join(h, "ws")

	expect(t, run(t, ws, nil, "edit"), Launch{Dir: ws, Mode: "edit"})
	expect(t, run(t, h, nil, "edit", "ws"), Launch{Dir: ws, Mode: "edit"})
	expect(t, run(t, h, nil, "edit", "ws/a.md"), Launch{Dir: ws, Page: "a.md", Mode: "edit"})
	// new file in an existing directory
	if err := os.MkdirAll(filepath.Join(ws, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	expect(t, run(t, h, nil, "edit", "ws/notes/new.md"), Launch{Dir: ws, Page: "notes/new.md", Mode: "edit"})

	// new file whose parent is missing
	if r := run(t, h, nil, "edit", "ws/missing/new.md"); r.code != 2 || r.launched {
		t.Fatalf("missing parent: %+v", r)
	}
	// new file is not allowed for read
	if r := run(t, h, nil, "ws/notes/new.md"); r.code != 2 || r.launched {
		t.Fatalf("read new file: %+v", r)
	}
}

func TestHelpAndVersion(t *testing.T) {
	cwd := t.TempDir()
	for _, a := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		r := run(t, cwd, nil, a...)
		if r.code != 0 || r.launched || !strings.Contains(r.stdout, "Usage:") || !strings.Contains(r.stdout, "edit") {
			t.Fatalf("%v: %+v", a, r)
		}
		for _, line := range strings.Split(r.stdout, "\n") {
			if len(line) > 80 {
				t.Fatalf("usage line too long: %q", line)
			}
		}
	}
	for _, a := range [][]string{{"-v"}, {"--version"}, {"version"}} {
		r := run(t, cwd, nil, a...)
		if r.code != 0 || r.launched || r.stdout != "zen "+Version+"\n" {
			t.Fatalf("%v: %+v", a, r)
		}
	}
}

func TestUsageErrors(t *testing.T) {
	h := home(t)
	write(t, filepath.Join(h, "a.md"))
	write(t, filepath.Join(h, "b.md"))
	write(t, filepath.Join(h, "c.txt"))
	cases := [][]string{
		{"--nope"},
		{"edit", "--nope"},
		{"a.md", "b.md"},
		{"edit", "a.md", "b.md"},
		{"missing"},
		{"missing.md"},
		{"c.txt"},
		{"edit", "c.txt"},
	}
	for _, a := range cases {
		r := run(t, h, nil, a...)
		if r.code != 2 || r.launched || r.stdout != "" || r.stderr == "" {
			t.Fatalf("%v: %+v", a, r)
		}
	}
}

func TestSeparators(t *testing.T) {
	h := home(t)
	write(t, filepath.Join(h, "-v.md"))
	expect(t, run(t, h, nil, "--", "-v.md"), Launch{Dir: h, Page: "-v.md", Mode: "read"})
	expect(t, run(t, h, nil, "edit", "--", "-v.md"), Launch{Dir: h, Page: "-v.md", Mode: "edit"})
	expect(t, run(t, h, nil, "-psn_0_12345"), Launch{Dir: h, Mode: "read"})
	expect(t, run(t, h, nil, "-psn_0_1", "edit"), Launch{Dir: h, Mode: "edit"})
	// "edit" after -- is a path, not a command
	if err := os.Mkdir(filepath.Join(h, "edit"), 0o755); err != nil {
		t.Fatal(err)
	}
	expect(t, run(t, h, nil, "--", "edit"), Launch{Dir: filepath.Join(h, "edit"), Mode: "read"})
}

func TestFinderLaunchShowsWelcome(t *testing.T) {
	home(t)
	// Finder / Dock / launchd start the app with cwd "/" (and /dev/null as
	// stdin, which looks like a terminal): always the welcome screen.
	r := run(t, "/", nil)
	expect(t, r, Launch{Dir: "", Mode: "read"})
	r = run(t, "/", nil, "-psn_0_12345")
	expect(t, r, Launch{Dir: "", Mode: "read"})

	// An explicit path still opens it.
	d := t.TempDir()
	r = run(t, "/", nil, d)
	expect(t, r, Launch{Dir: d, Mode: "read"})

	// Any other cwd opens the cwd.
	r = run(t, d, nil)
	expect(t, r, Launch{Dir: d, Mode: "read"})
}

func TestBuildCommand(t *testing.T) {
	h := home(t)
	ws := filepath.Join(h, "ws")
	write(t, filepath.Join(ws, ".zen.yml"))
	write(t, filepath.Join(ws, "docs", "a.md"))
	write(t, filepath.Join(h, "plain", "b.md"))

	built := func(r result, want Build) {
		t.Helper()
		if r.code != 0 || !r.built || r.launched || r.b != want {
			t.Fatalf("got %+v, want build %+v", r, want)
		}
	}
	// Defaults: the current folder.
	built(run(t, ws, nil, "build"), Build{Dir: ws})
	// A folder, relative to cwd or absolute.
	built(run(t, h, nil, "build", "ws"), Build{Dir: ws})
	built(run(t, h, nil, "build", ws), Build{Dir: ws})
	// A markdown file means its folder, as zen [path] resolves it.
	built(run(t, h, nil, "build", "ws/docs/a.md"), Build{Dir: ws})
	built(run(t, h, nil, "build", "plain/b.md"), Build{Dir: filepath.Join(h, "plain")})
	// Flags in any order; -o is relative to cwd.
	built(run(t, h, nil, "build", "-o", "out", "--portable", "--drafts", "--clean", "ws"), Build{Dir: ws, Out: filepath.Join(h, "out"), Portable: true, Drafts: true, Clean: true})
	built(run(t, h, nil, "build", "--hosted", "ws"), Build{Dir: ws, Hosted: true})
	built(run(t, h, nil, "build", "ws", "--clean", "-o=site"), Build{Dir: ws, Out: filepath.Join(h, "site"), Clean: true})
	built(run(t, h, nil, "build", "--out", "/tmp/zen-out", "--drafts"), Build{Dir: h, Out: "/tmp/zen-out", Drafts: true})
	built(run(t, h, nil, "build", "--out=rel", "--", "ws"), Build{Dir: ws, Out: filepath.Join(h, "rel")})
	// "build" is a command only first; after -- it is a path.
	if err := os.Mkdir(filepath.Join(h, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	expect(t, run(t, h, nil, "--", "build"), Launch{Dir: filepath.Join(h, "build"), Mode: "read"})
	// The usage text lists it.
	r := run(t, h, nil, "--help")
	if !strings.Contains(r.stdout, "zen build [folder]") || !strings.Contains(r.stdout, "--portable") || !strings.Contains(r.stdout, "--hosted") {
		t.Errorf("usage lacks build:\n%s", r.stdout)
	}
}

func TestBuildExitCodes(t *testing.T) {
	h := home(t)
	write(t, filepath.Join(h, "ws", "a.md"))
	write(t, filepath.Join(h, "c.txt"))
	ws := filepath.Join(h, "ws")

	// 0: built; the summary is on stdout and the warnings on stderr, untouched by the CLI.
	r := run(t, ws, nil, "build")
	if r.code != 0 || r.stdout != "Built 1 pages and 0 files into out (1 warnings)\n" || r.stderr != "a.md: a warning\n" {
		t.Fatalf("success: %+v", r)
	}
	// 1: the build failed.
	builderErr = errors.New("output folder is not empty")
	defer func() { builderErr = nil }()
	r = run(t, ws, nil, "build")
	if r.code != 1 || !strings.Contains(r.stderr, "zen: output folder is not empty") || !strings.Contains(r.stderr, "a.md: a warning") {
		t.Fatalf("failure: %+v", r)
	}
	builderErr = nil
	// 2: usage errors, and nothing was built.
	for _, a := range [][]string{
		{"build", "-o"}, {"build", "--out"}, {"build", "--nope"}, {"build", "ws", "ws"}, {"build", "missing"}, {"build", "c.txt"}, {"build", "-o", ""},
		{"build", "--portable", "--hosted"},
	} {
		r := run(t, h, nil, a...)
		if r.code != 2 || r.built || r.launched || r.stdout != "" || !strings.Contains(r.stderr, "Usage:") {
			t.Fatalf("%v: %+v", a, r)
		}
	}
	// No Builder wired: a plain error, not a crash.
	var out, errOut bytes.Buffer
	if code := Run([]string{"build"}, ws, &out, &errOut, func(Launch) error { return nil }, nil, nil); code != 1 || !strings.Contains(errOut.String(), "build") {
		t.Errorf("nil builder: %d %q", code, errOut.String())
	}
}

func TestInitCommand(t *testing.T) {
	h := home(t)
	initd := func(r result, want Init) {
		t.Helper()
		if r.code != 0 || !r.inited || r.launched || r.built || !reflect.DeepEqual(r.i, want) {
			t.Fatalf("got %+v (code %d, stderr %q), want %+v", r.i, r.code, r.stderr, want)
		}
	}
	vals := func(kv ...string) map[string]string {
		m := map[string]string{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	// The folder defaults to the current folder; relative folders are made absolute.
	initd(run(t, h, nil, "init", "site", "--techdocs"), Init{Dir: h, Template: "techdocs", Values: vals()})
	initd(run(t, h, nil, "init", "site", "--learning", "my docs"), Init{Dir: filepath.Join(h, "my docs"), Template: "learning", Values: vals()})
	initd(run(t, h, nil, "init", "site", "docs", "--planner"), Init{Dir: filepath.Join(h, "docs"), Template: "planner", Values: vals()})
	initd(run(t, h, nil, "init", "site", "/abs/path", "--template", "mine"), Init{Dir: "/abs/path", Template: "mine", Values: vals()})
	initd(run(t, h, nil, "init", "site", "--template=mine", "x"), Init{Dir: filepath.Join(h, "x"), Template: "mine", Values: vals()})
	// --<id> works for any template name, including ones zen does not ship.
	initd(run(t, h, nil, "init", "site", "--my-own-2", "x"), Init{Dir: filepath.Join(h, "x"), Template: "my-own-2", Values: vals()})
	// Values: --title and --author, --set key=value; --title wins over --set title= in any order.
	initd(run(t, h, nil, "init", "site", "--techdocs", "--title", "My docs", "--author=Ana", "--set", "team=Docs", "--set", "tagline=a=b"),
		Init{Dir: h, Template: "techdocs", Values: vals("title", "My docs", "author", "Ana", "team", "Docs", "tagline", "a=b")})
	initd(run(t, h, nil, "init", "site", "--techdocs", "--title", "A", "--set", "title=B"), Init{Dir: h, Template: "techdocs", Values: vals("title", "A")})
	initd(run(t, h, nil, "init", "site", "--techdocs", "--set", "title=B", "--title=A"), Init{Dir: h, Template: "techdocs", Values: vals("title", "A")})
	initd(run(t, h, nil, "init", "site", "--techdocs", "--set", "author=x", "--set", "author=y"), Init{Dir: h, Template: "techdocs", Values: vals("author", "y")})
	// --list, --dir and --update need no "site".
	initd(run(t, h, nil, "init", "--list"), Init{Dir: h, List: true, Values: vals()})
	initd(run(t, h, nil, "init", "--dir"), Init{Dir: h, ShowDir: true, Values: vals()})
	initd(run(t, h, nil, "init", "--update", "techdocs"), Init{Dir: h, Update: true, Template: "techdocs", Values: vals()})
	initd(run(t, h, nil, "init", "--update=planner"), Init{Dir: h, Update: true, Template: "planner", Values: vals()})
	// Without a template: the list is printed, exit 2.
	r := run(t, h, nil, "init", "site")
	if r.code != 2 || !r.i.List || !strings.Contains(r.stdout, "techdocs  Technical documentation") || !strings.Contains(r.stderr, "--techdocs") || r.launched {
		t.Errorf("no template: %+v", r)
	}
	// "init" is a command only first; after -- it is a path.
	if err := os.Mkdir(filepath.Join(h, "init"), 0o755); err != nil {
		t.Fatal(err)
	}
	expect(t, run(t, h, nil, "--", "init"), Launch{Dir: filepath.Join(h, "init"), Mode: "read"})
	// The usage text lists it.
	r = run(t, h, nil, "--help")
	if !strings.Contains(r.stdout, "zen init site") || !strings.Contains(r.stdout, "--techdocs") {
		t.Errorf("usage lacks init:\n%s", r.stdout)
	}
	for _, line := range strings.Split(r.stdout, "\n") {
		if len(line) > 80 {
			t.Errorf("usage line too long: %q", line)
		}
	}
}

func TestInitExitCodes(t *testing.T) {
	h := home(t)
	// 0 and the Initer's own output.
	r := run(t, h, nil, "init", "--list")
	if r.code != 0 || !strings.Contains(r.stdout, "techdocs") {
		t.Fatalf("list: %+v", r)
	}
	// 1: the Initer failed (clash, invalid template, unwritable folder).
	initerErr = errors.New("files already exist: README.md")
	defer func() { initerErr = nil }()
	r = run(t, h, nil, "init", "site", "--techdocs")
	if r.code != 1 || r.stderr != "zen: files already exist: README.md\n" || r.launched {
		t.Fatalf("failure: %+v", r)
	}
	// 2: usage errors from the Initer, with the usage text.
	initerErr = UsageError("no template \"nosuch\"")
	r = run(t, h, nil, "init", "site", "--nosuch")
	if r.code != 2 || !strings.Contains(r.stderr, "no template") || !strings.Contains(r.stderr, "Usage:") {
		t.Fatalf("usage from initer: %+v", r)
	}
	initerErr = nil
	// 2: usage errors found by the parser; the Initer is not called.
	for _, a := range [][]string{
		{"init"}, {"init", "page"}, {"init", "site", "--techdocs", "--learning"}, {"init", "site", "--template"}, {"init", "site", "--title"},
		{"init", "site", "--set", "novalue"}, {"init", "site", "--set", "=x"}, {"init", "site", "--set"}, {"init", "site", "a", "b", "--techdocs"},
		{"init", "site", "--update", "x"}, {"init", "--update"}, {"init", "--list", "x"}, {"init", "--dir", "--list"}, {"init", "site", "--Bad"},
		{"init", "site", "--techdocs", "--template", "x"}, {"init", "site", "--techdocs", "-x"}, {"init", "--list", "--techdocs"},
		{"init", "site", "--template", ""}, {"init", "--update", ""},
	} {
		r := run(t, h, nil, a...)
		if r.code != 2 || r.inited || r.launched || r.stdout != "" || !strings.Contains(r.stderr, "Usage:") {
			t.Fatalf("%v: %+v", a, r)
		}
	}
	// -h anywhere after init prints the usage and exits 0.
	if r := run(t, h, nil, "init", "site", "--help"); r.code != 0 || r.inited || !strings.Contains(r.stdout, "zen init site") {
		t.Errorf("help: %+v", r)
	}
	// No Initer wired: a plain error, not a crash.
	var out, errOut bytes.Buffer
	if code := Run([]string{"init", "site", "--techdocs"}, h, &out, &errOut, func(Launch) error { return nil }, nil, nil); code != 1 || !strings.Contains(errOut.String(), "init") {
		t.Errorf("nil initer: %d %q", code, errOut.String())
	}
}
