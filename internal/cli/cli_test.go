package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type result struct {
	code     int
	stdout   string
	stderr   string
	launched bool
	l        Launch
}

func run(t *testing.T, cwd string, launchErr error, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	r := result{}
	r.code = Run(args, cwd, &out, &errOut, func(l Launch) error {
		r.launched = true
		r.l = l
		return launchErr
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
