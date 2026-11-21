package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// export.theme.scheme is auto (the default), light or dark; the rest of export.theme is room for engine/export-theme.
func TestExportThemeScheme(t *testing.T) {
	for in, want := range map[string]string{
		"":                                       "",
		"export:\n  out: x\n":                    "",
		"export:\n  theme:\n    scheme: auto\n":  "auto",
		"export:\n  theme:\n    scheme: light\n": "light",
		"export:\n  theme:\n    scheme: dark\n":  "dark",
	} {
		b, warns, err := loadKeys(t, "", in).Build()
		if err != nil || len(warns) != 0 || b.Scheme != want {
			t.Errorf("%q: scheme %q, warns %v, err %v, want %q", in, b.Scheme, warns, err, want)
		}
	}
	// Other keys of export.theme (the later phases) are not an error.
	b, warns, err := loadKeys(t, "", "export:\n  theme:\n    scheme: dark\n    name: paper\n    tokens:\n      accent: red\n").Build()
	if err != nil || len(warns) != 0 || b.Scheme != "dark" {
		t.Errorf("later keys: %+v %v %v", b, warns, err)
	}
}

func TestExportThemeSchemeErrors(t *testing.T) {
	for in, want := range map[string]string{
		"export:\n  theme: dark\n":              "export.theme must be a mapping",
		"export:\n  theme:\n    scheme: blue\n": "export.theme.scheme must be auto, light or dark",
		"export:\n  theme:\n    scheme: true\n": "export.theme.scheme must be auto, light or dark",
	} {
		_, _, err := loadKeys(t, "", in).Build()
		if err == nil || err.Error() != want {
			t.Errorf("%q: %v, want %q", in, err, want)
		}
	}
}

// The folder's scheme wins over the global one, key by key.
func TestExportThemeSchemeMerge(t *testing.T) {
	b, _, err := loadKeys(t, "export:\n  theme:\n    scheme: dark\n", "export:\n  out: x\n").Build()
	if err != nil || b.Scheme != "dark" || b.Out != "x" {
		t.Errorf("global: %+v %v", b, err)
	}
	b, _, err = loadKeys(t, "export:\n  theme:\n    scheme: dark\n", "export:\n  theme:\n    scheme: light\n").Build()
	if err != nil || b.Scheme != "light" {
		t.Errorf("folder wins: %+v %v", b, err)
	}
}

// SaveExportKeys is the one editor of the folder's export: mapping, for the app's Save as folder defaults and
// `zenmd export --save-defaults`: dotted keys reach export.pdf and export.theme, comments and other keys stay,
// and without confirm nothing is written.
func TestSaveExportKeys(t *testing.T) {
	root := t.TempDir()
	yml := filepath.Join(root, WorkspaceFile)
	orig := "# mine\nversion: 1\nexport:\n  # site\n  out: _site\n  theme:\n    name: paper # keep\n"
	if err := os.WriteFile(yml, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	keys := []ExportKey{StrKey("theme.scheme", "dark"), BoolKey("pdf.cover", false), BoolKey("drafts", true)}
	prev, err := SaveExportKeys(root, keys, "nothing", false)
	if err != nil || !strings.HasPrefix(prev, "Adds to .zenmd.yml:") || !strings.Contains(prev, "export.theme.scheme: dark") ||
		!strings.Contains(prev, "export.pdf.cover: false") || !strings.Contains(prev, "export.drafts: true") {
		t.Fatalf("preview: %q, %v", prev, err)
	}
	if got, _ := os.ReadFile(yml); string(got) != orig {
		t.Fatal("the preview wrote the file")
	}
	if _, err := SaveExportKeys(root, keys, "nothing", true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(yml)
	for _, keep := range []string{"# mine", "# site", "out: _site", "name: paper # keep", "scheme: dark", "cover: false", "drafts: true"} {
		if !strings.Contains(string(got), keep) {
			t.Errorf("lacks %q:\n%s", keep, got)
		}
	}
	b, _, err := Load("", root).Build()
	if err != nil || b.Scheme != "dark" || !b.Drafts {
		t.Errorf("round trip: %+v %v", b, err)
	}
	// The same keys again: nothing to change, and no empty mapping is left behind.
	if prev, err := SaveExportKeys(root, keys, "nothing", true); err != nil || prev != "nothing" {
		t.Errorf("again: %q, %v", prev, err)
	}
	// A key that is already the value in a nested mapping that does not exist leaves no empty mapping.
	root2 := t.TempDir()
	if _, err := SaveExportKeys(root2, nil, "nothing", true); err != nil {
		t.Fatal(err)
	}
	if d, _ := os.ReadFile(filepath.Join(root2, WorkspaceFile)); strings.Contains(string(d), "theme:") || strings.Contains(string(d), "pdf:") {
		t.Errorf("empty mapping:\n%s", d)
	}
}
