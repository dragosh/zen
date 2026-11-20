package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The old name of the folder settings file appears in one place of the code
// (name.go: the constant and the two notices), in the one label of Settings
// that says "old name", in the CHANGELOG and in the historical specs. Docs,
// messages, the demo and the frontend say .zenmd.yml.
func TestNoLegacyNameOutsideNotices(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	skipDir := map[string]bool{".git": true, "node_modules": true, "dist": true, "bindings": true, "specs": true, "bin": true, ".claude": true, "build": false}
	exts := map[string]bool{".go": true, ".ts": true, ".md": true, ".yml": true, ".yaml": true, ".json": true, ".html": true, ".css": true}
	allowed := func(rel, line string) bool {
		switch rel {
		case "engine/config/name.go", "CHANGELOG.md":
			return true
		case "apps/zenmark/frontend/src/shell/settings.ts":
			return strings.Contains(line, "old name")
		}
		return false
	}
	self := "engine/config/legacyname_test.go"
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDir[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !exts[filepath.Ext(p)] && d.Name() != ".zen.yml" {
			return nil
		}
		// Tests may name the legacy file to test it.
		if strings.HasSuffix(rel, "_test.go") || strings.HasSuffix(rel, ".test.ts") || rel == self {
			return nil
		}
		if d.Name() == ".zen.yml" {
			t.Errorf("%s: a shipped file with the old name", rel)
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, ".zen.yml") && !allowed(rel, line) {
				t.Errorf("%s:%d: names .zen.yml: %s", rel, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
