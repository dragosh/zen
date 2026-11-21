package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func buildOf(t *testing.T, global, folder string) (Build, []string, error) {
	t.Helper()
	dir := t.TempDir()
	g, root := "", ""
	if global != "" {
		g = write(t, filepath.Join(dir, "config.yml"), global)
	}
	if folder != "" {
		root = filepath.Join(dir, "ws")
		write(t, filepath.Join(root, WorkspaceFile), folder)
	}
	cfg := Load(g, root)
	if ws := cfg.Warnings(); len(ws) != 0 {
		t.Fatalf("loading warned about an export: section: %v", ws)
	}
	return cfg.Build()
}

func TestBuildSection(t *testing.T) {
	// Nothing configured: zero values.
	b, warns, err := buildOf(t, "", "")
	if err != nil || len(warns) != 0 || !reflect.DeepEqual(b, Build{}) {
		t.Errorf("empty: %+v %v %v", b, warns, err)
	}

	// All keys.
	b, warns, err = buildOf(t, "", `
export:
  out: _site
  id: my-notes
  editURL: https://github.com/me/notes/edit/main/{path}
  drafts: true
  portable: true
  exclude: ["private/**", "scratch.md"]
`)
	want := Build{Out: "_site", ID: "my-notes", EditURL: "https://github.com/me/notes/edit/main/{path}", Drafts: true, Portable: ptr(true), Exclude: []string{"private/**", "scratch.md"}}
	if err != nil || len(warns) != 0 || !reflect.DeepEqual(b, want) {
		t.Errorf("all keys: %+v %v %v", b, warns, err)
	}

	// The folder wins key by key over the global file; the rest comes from global.
	b, _, err = buildOf(t, "export:\n  out: global-out\n  id: g\n  drafts: true\n  exclude: [\"a/**\"]\n", "export:\n  out: _site\n  portable: true\n  exclude: [\"b/**\"]\n")
	want = Build{Out: "_site", ID: "g", Drafts: true, Portable: ptr(true), Exclude: []string{"b/**"}}
	if err != nil || !reflect.DeepEqual(b, want) {
		t.Errorf("layers: %+v %v", b, err)
	}
	// Global alone works.
	b, _, err = buildOf(t, "export:\n  id: only-global\n", "")
	if err != nil || b.ID != "only-global" {
		t.Errorf("global only: %+v %v", b, err)
	}

	// Unknown keys are warnings (sorted), the rest still applies.
	b, warns, err = buildOf(t, "", "export:\n  out: x\n  zebra: 1\n  apple: 2\n")
	if err != nil || b.Out != "x" || len(warns) != 2 || !strings.Contains(warns[0], "export.apple") || !strings.Contains(warns[1], "export.zebra") {
		t.Errorf("unknown keys: %+v %v %v", b, warns, err)
	}

	// Wrong types are errors naming the key.
	for yaml, key := range map[string]string{
		"export:\n  out: 5\n":                  "export.out",
		"export:\n  id: [a]\n":                 "export.id",
		"export:\n  editURL: true\n":           "export.editURL",
		"export:\n  drafts: \"yes\"\n":         "export.drafts",
		"export:\n  portable: 1\n":             "export.portable",
		"export:\n  exclude: \"private/**\"\n": "export.exclude",
		"export:\n  exclude: [1]\n":            "export.exclude",
		"export: nope\n":                       "export",
		"export:\n  - a\n":                     "export",
	} {
		_, _, err := buildOf(t, "", yaml)
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%q: error %v, want one naming %s", yaml, err, key)
		}
	}
	// An id that could escape its namespace.
	if _, _, err := buildOf(t, "", "export:\n  id: \"a b/c\"\n"); err == nil || !strings.Contains(err.Error(), "export.id") {
		t.Errorf("bad id: %v", err)
	}
}

func ptr(b bool) *bool { return &b }

func TestBuildPortableUnset(t *testing.T) {
	if b, _, err := buildOf(t, "", "export:\n  out: x\n"); err != nil || b.Portable != nil {
		t.Errorf("portable not set: %v %v", b.Portable, err)
	}
	if b, _, err := buildOf(t, "", "export:\n  portable: false\n"); err != nil || b.Portable == nil || *b.Portable {
		t.Errorf("portable: false: %v %v", b.Portable, err)
	}
}
