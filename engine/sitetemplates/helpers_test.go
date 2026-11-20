package sitetemplates

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// put writes files (slash paths) under dir.
func put(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// walk lists the files under dir, slash paths, sorted.
func walk(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const basicManifest = "id: demo\nname: Demo\ndescription: A demo\nversion: 1.0.0\n"

// tmpl writes a template folder under <dir>/sites/<folder> and returns it parsed.
func tmpl(t *testing.T, dir, folder, manifest string, files map[string]string) Template {
	t.Helper()
	root := filepath.Join(dir, "sites", folder)
	all := map[string]string{"template.yml": manifest}
	for k, v := range files {
		all[k] = v
	}
	put(t, root, all)
	got, _ := List(dir)
	for _, x := range got {
		if x.Dir == root {
			return x
		}
	}
	t.Fatalf("template %s not listed: %+v", folder, got)
	return Template{}
}

func fixedDate(t *testing.T) {
	t.Helper()
	old := now
	now = func() time.Time { return time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = old })
}

// builtinsFS is a fake bundle: techdocs 1.0.0 and planner 2.0.0.
func builtinsFS(techVersion string) fstest.MapFS {
	return fstest.MapFS{
		"techdocs/template.yml": {Data: []byte("id: techdocs\nname: Tech docs\nversion: " + techVersion + "\norder: 10\n")},
		"techdocs/README.md":    {Data: []byte("# {{title}}\n")},
		"planner/template.yml":  {Data: []byte("id: planner\nname: Planner\nversion: 2.0.0\norder: 30\n")},
		"planner/.zenmd.yml":    {Data: []byte("layout: wide\n")},
	}
}

func hasPrefixAny(s string, subs ...string) bool {
	for _, x := range subs {
		if !strings.Contains(s, x) {
			return false
		}
	}
	return true
}
