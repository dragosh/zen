package sitetemplates

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SitesDir is where the templates are: <templates dir>/sites.
func SitesDir(dir string) string { return filepath.Join(dir, "sites") }

// List returns the templates in <dir>/sites, sorted by order, then name,
// then the invalid ones (by folder name). Two templates with the same id:
// the first in folder-name order wins, and a warning names the other.
// Folders starting with "." and "<id>.new" update folders are not templates.
func List(dir string) ([]Template, []string) { return ListWith(dir, nil) }

// ListWith is List, and also marks the templates installed from builtins that
// have a newer version in builtins (Template.Update).
func ListWith(dir string, builtins fs.FS) ([]Template, []string) {
	sites := SitesDir(dir)
	entries, err := os.ReadDir(sites)
	if err != nil {
		return nil, nil
	}
	rec := readRecord(sites)
	var good, bad []Template
	var warns []string
	byID := map[string]string{}
	for _, e := range entries { // sorted by name
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".new") {
			continue
		}
		full := filepath.Join(sites, name)
		if fi, err := os.Stat(full); err != nil || !fi.IsDir() {
			continue
		}
		t := load(os.DirFS(full), name)
		t.Dir = full
		if t.Err != "" {
			bad = append(bad, t)
			continue
		}
		if first, dup := byID[t.ID]; dup {
			warns = append(warns, fmt.Sprintf("template %q in %s has the same id as the one in %s and is ignored", t.ID, name, first))
			continue
		}
		byID[t.ID] = name
		if v, ok := rec.Installed[t.ID]; ok {
			t.Builtin = true
			if b := builtinVersion(builtins, t.ID); b != "" && newer(b, v) {
				t.Update = b
			}
		}
		good = append(good, t)
	}
	sort.SliceStable(good, func(i, j int) bool {
		if good[i].Order != good[j].Order {
			return good[i].Order < good[j].Order
		}
		return good[i].Name < good[j].Name
	})
	return append(good, bad...), warns
}

// Builtins lists the presets of a bundle (a folder per template), as they are
// used when the templates folder cannot be written to.
func Builtins(builtins fs.FS) ([]Template, []string) {
	entries, err := fs.ReadDir(builtins, ".")
	if err != nil {
		return nil, nil
	}
	var out []Template
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub, err := fs.Sub(builtins, e.Name())
		if err != nil {
			continue
		}
		t := load(sub, e.Name())
		t.Builtin = true
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Err == "") != (out[j].Err == "") {
			return out[i].Err == ""
		}
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// load reads the manifest of a template folder; a problem is put in Err.
func load(fsys fs.FS, folder string) Template {
	data, err := fs.ReadFile(fsys, "template.yml")
	if err != nil {
		return Template{ID: folder, Name: folder, Err: "template.yml is missing or unreadable"}
	}
	t, err := ParseManifest(data)
	if err != nil {
		return Template{ID: folder, Name: folder, Err: err.Error()}
	}
	t.fsys = fsys
	return t
}

func builtinVersion(builtins fs.FS, id string) string {
	if builtins == nil {
		return ""
	}
	data, err := fs.ReadFile(builtins, id+"/template.yml")
	if err != nil {
		return ""
	}
	t, err := ParseManifest(data)
	if err != nil {
		return ""
	}
	return t.Version
}

// newer reports whether version a is higher than b (numeric parts only).
func newer(a, b string) bool {
	pa, pb := nums(a), nums(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func nums(v string) [3]int {
	var out [3]int
	core := strings.FieldsFunc(v, func(r rune) bool { return r == '-' || r == '+' })
	if len(core) == 0 {
		return out
	}
	for i, p := range strings.SplitN(core[0], ".", 3) {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out[i] = n
	}
	return out
}
