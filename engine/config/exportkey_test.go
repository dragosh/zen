package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

const (
	wantLegacyKey  = "build: in .zenmd.yml is now export:; rename the key"
	wantBothKeys   = "build: in .zenmd.yml is ignored because export: is set"
	wantGlobalKey  = "build: in config.yml is now export:; rename the key"
	wantGlobalBoth = "build: in config.yml is ignored because export: is set"
	wantLegacyFile = "build: in .zen.yml is now export:; rename the key"
)

func loadKeys(t *testing.T, global, folder string) *Config {
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
	return Load(g, root)
}

// export: is the key, in the folder and in the global file; no notice.
func TestExportKey(t *testing.T) {
	c := loadKeys(t, "export:\n  id: g\n  drafts: true\n", "export:\n  out: _site\n")
	b, warns, err := c.Build()
	want := Build{Out: "_site", ID: "g", Drafts: true}
	if err != nil || len(warns) != 0 || !reflect.DeepEqual(b, want) {
		t.Errorf("%+v %v %v", b, warns, err)
	}
	if len(c.Warnings()) != 0 || c.Notice() != "" || len(c.Notices()) != 0 {
		t.Errorf("warnings %v notice %q", c.Warnings(), c.Notice())
	}
}

// build: alone is read as the old name, with a notice (not a warning).
func TestLegacyBuildKey(t *testing.T) {
	c := loadKeys(t, "", "build:\n  out: old\n  portable: false\n")
	b, _, err := c.Build()
	if err != nil || b.Out != "old" || b.Portable == nil || *b.Portable {
		t.Errorf("legacy key not read: %+v %v", b, err)
	}
	if !reflect.DeepEqual(c.Notices(), []string{wantLegacyKey}) {
		t.Errorf("notices %q", c.Notices())
	}
	if len(c.Warnings()) != 0 {
		t.Errorf("a legacy key is not an unknown-key warning: %v", c.Warnings())
	}
	// It reaches the page warnings, which is how Problems gets it.
	found := false
	for _, w := range c.ForPage(nil).Warnings {
		found = found || w == wantLegacyKey
	}
	if !found {
		t.Errorf("page warnings: %v", c.ForPage(nil).Warnings)
	}
	// The same in the legacy .zen.yml names that file.
	dir := t.TempDir()
	write(t, filepath.Join(dir, LegacyWorkspaceFile), "build:\n  out: old\n")
	if got := Load("", dir).Notices(); !reflect.DeepEqual(got, []string{legacyNotice, wantLegacyFile}) {
		t.Errorf("legacy file: %q", got)
	}
}

// With both keys, export: wins, build: is ignored, and the notice says so.
func TestBothExportAndBuild(t *testing.T) {
	c := loadKeys(t, "", "build:\n  out: old\n  id: old-id\nexport:\n  out: new\n")
	b, _, err := c.Build()
	if err != nil || b.Out != "new" || b.ID != "" {
		t.Errorf("export should win and build be ignored: %+v %v", b, err)
	}
	if !reflect.DeepEqual(c.Notices(), []string{wantBothKeys}) {
		t.Errorf("notices %q", c.Notices())
	}
	// A bad build: is ignored too.
	c = loadKeys(t, "", "build: 3\nexport:\n  out: new\n")
	if b, _, err := c.Build(); err != nil || b.Out != "new" {
		t.Errorf("ignored build: still failed: %+v %v", b, err)
	}
}

// The global config.yml has the same notices, naming that file.
func TestGlobalBuildKey(t *testing.T) {
	c := loadKeys(t, "build:\n  id: g\n", "")
	if b, _, err := c.Build(); err != nil || b.ID != "g" {
		t.Errorf("global legacy key: %+v %v", b, err)
	}
	if !reflect.DeepEqual(c.Notices(), []string{wantGlobalKey}) {
		t.Errorf("notices %q", c.Notices())
	}
	c = loadKeys(t, "build:\n  id: old\nexport:\n  id: new\n", "")
	if b, _, _ := c.Build(); b.ID != "new" {
		t.Errorf("global export should win: %+v", b)
	}
	if !reflect.DeepEqual(c.Notices(), []string{wantGlobalBoth}) {
		t.Errorf("notices %q", c.Notices())
	}
	// The folder's export: wins over the global build:, key by key.
	c = loadKeys(t, "build:\n  id: g\n  drafts: true\n", "export:\n  id: f\n")
	if b, _, _ := c.Build(); b.ID != "f" || !b.Drafts {
		t.Errorf("layers: %+v", b)
	}
}

// build: config examples appear only in the legacy notes: the code that reads
// the key, the CHANGELOG, the historical specs and the one sentence in
// docs/BUILD.md that says it is still read.
func TestNoLegacyBuildKeyOutsideNotices(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	skipDir := map[string]bool{".git": true, "node_modules": true, "dist": true, "bindings": true, "specs": true, "bin": true, ".claude": true}
	example := regexp.MustCompile("^\\s*build:|`build:`")
	oldPath := regexp.MustCompile(`\bbuild\.(out|id|editURL|drafts|portable|exclude)\b`)
	allowed := func(rel, line string) bool {
		switch rel {
		case "engine/config/export.go", "CHANGELOG.md":
			return true
		case "docs/BUILD.md":
			return strings.Contains(line, "still read")
		}
		return false
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDir[d.Name()] || rel == "build" {
				return fs.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(p)
		isConfig := d.Name() == ".zenmd.yml" || d.Name() == "config.yml"
		if ext != ".md" && ext != ".go" && !isConfig || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			bad := oldPath.MatchString(line)
			if ext == ".md" || isConfig {
				bad = bad || example.MatchString(line)
			}
			if bad && !allowed(rel, line) {
				t.Errorf("%s:%d: names the old build: key: %s", rel, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
