package session

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

// roZen opens dir in a read-only session with its own config folder.
func roZen(t *testing.T, dir string) (*Session, string) {
	t.Helper()
	dir, _ = filepath.EvalSymlinks(dir)
	cfg := t.TempDir()
	s := New(Options{
		GlobalPath: filepath.Join(cfg, "config.yml"), Registry: plugins.NewRegistry(),
		Events: &sink{}, Runtime: stubAssets(), ReadOnly: true,
	})
	t.Cleanup(s.Close)
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

// snapshot hashes every file under dir (path -> hash), and lists its folders.
func snapshot(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	m := map[string][32]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.IsDir() {
			m[rel+"/"] = [32]byte{}
			return nil
		}
		b, err := os.ReadFile(p)
		m[rel] = sha256.Sum256(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestReadOnlyRefusesWrites(t *testing.T) {
	dir := tree(t, docs)
	s, dir := roZen(t, dir)
	before := snapshot(t, dir)

	nav := s.Workspace()
	var paths []string
	for _, n := range nav.Tree {
		if n.Path != "" {
			paths = append(paths, n.Path)
		}
	}
	if len(paths) == 0 {
		paths = []string{"README.md"}
	}

	calls := map[string]func() error{
		"Save":         func() error { return s.Save("README.md", "# changed\n") },
		"Save config":  func() error { return s.Save(".zen.yml", "version: 1\n") },
		"CreatePage":   func() error { _, err := s.CreatePage("new", ""); return err },
		"ReorderNav":   func() error { return s.ReorderNav("", paths) },
		"SetNavHidden": func() error { return s.SetNavHidden("README.md", true) },
		"SaveBuildDefaults": func() error {
			_, err := s.SaveBuildDefaults(BuildOptions{Out: filepath.Join(dir, "_site")}, true)
			return err
		},
		"EnsureConfig ws":    func() error { _, err := s.EnsureConfig("workspace"); return err },
		"EnsureConfig glob.": func() error { _, err := s.EnsureConfig("global"); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrReadOnly) {
			t.Errorf("%s: error %v, want ErrReadOnly", name, err)
		}
	}
	if ErrReadOnly.Error() != "this app is read-only" {
		t.Errorf("message %q", ErrReadOnly)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("the folder changed:\nbefore %v\nafter  %v", before, after)
	}
	if _, err := os.Stat(s.globalPath); err == nil {
		t.Error("the global config file was created")
	}

	// A session that is not read-only still writes.
	w := New(Options{GlobalPath: filepath.Join(t.TempDir(), "config.yml"), Registry: plugins.NewRegistry(), Events: &sink{}})
	t.Cleanup(w.Close)
	d2 := tree(t, docs)
	if err := w.Open(d2); err != nil {
		t.Fatal(err)
	}
	if err := w.Save("README.md", "# ok\n"); err != nil {
		t.Errorf("writable session: %v", err)
	}
}

// The exported writers of the session: a new one must be listed here as
// guarded (and covered by TestReadOnlyRefusesWrites) or as exempt.
func TestEveryWriterGuarded(t *testing.T) {
	guarded := []string{"Save", "CreatePage", "ReorderNav", "SetNavHidden", "SaveBuildDefaults", "EnsureConfig"}
	exempt := []string{"SetOpenTimeout"} // a test and embedder knob, writes nothing to disk
	typ := reflect.TypeOf(&Session{})
	have := map[string]bool{}
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		have[name] = true
		for _, p := range []string{"Save", "Create", "Set", "Reorder", "Write", "Delete", "Rename"} {
			if strings.HasPrefix(name, p) && !slices.Contains(guarded, name) && !slices.Contains(exempt, name) {
				t.Errorf("Session.%s looks like a writer: guard it with s.writable() and list it in readonly_test.go, or list it as exempt", name)
			}
		}
	}
	for _, name := range append(slices.Clone(guarded), exempt...) {
		if !have[name] {
			t.Errorf("Session.%s is listed but does not exist", name)
		}
	}
}

func TestReadOnlyReads(t *testing.T) {
	s, dir := roZen(t, tree(t, docs))
	if pg, err := s.Page("README.md"); err != nil || !strings.Contains(pg.HTML, "Home") {
		t.Errorf("Page: %v %q", err, pg.HTML)
	}
	if src, err := s.Source("README.md"); err != nil || !strings.Contains(src, "# Home") {
		t.Errorf("Source: %v %q", err, src)
	}
	if pg, err := s.Preview("README.md", "# Draft\n"); err != nil || !strings.Contains(pg.HTML, "Draft") {
		t.Errorf("Preview: %v %q", err, pg.HTML)
	}
	if _, err := s.PageInfo("README.md"); err != nil {
		t.Errorf("PageInfo: %v", err)
	}
	if ws := s.Workspace(); ws == nil || len(ws.Tree) == 0 {
		t.Errorf("Workspace: %+v", ws)
	}
	if _, err := s.EnsureConfig("global"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("EnsureConfig creating: %v", err)
	}
	// EnsureConfig of a file that exists is a read: it only returns the path.
	p := filepath.Join(dir, ".zen.yml")
	if err := os.WriteFile(p, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := s.EnsureConfig("workspace"); err != nil || got != p {
		t.Errorf("EnsureConfig existing: %q, %v", got, err)
	}
}

func TestReadOnlyExport(t *testing.T) {
	dir := tree(t, fivePages)
	s, dir := roZen(t, dir)
	out := filepath.Join(dir, "_site")
	res, err := s.Build(context.Background(), BuildOptions{Out: out, Portable: true}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Pages == 0 {
		t.Errorf("no pages built: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Errorf("output not written: %v", err)
	}
	// The defaults are not saved.
	if _, err := s.SaveBuildDefaults(BuildOptions{Out: out}, true); !errors.Is(err, ErrReadOnly) {
		t.Errorf("SaveBuildDefaults: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".zen.yml")); err == nil {
		t.Error(".zen.yml was written")
	}
	names := []string{}
	es, _ := os.ReadDir(dir)
	for _, e := range es {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if !slices.Contains(names, "_site") {
		t.Errorf("folder: %v", names)
	}
}
