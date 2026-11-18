package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

func stubAssets() fstest.MapFS {
	return fstest.MapFS{
		"static/zen.js":  {Data: []byte("/*static runtime*/")},
		"static/zen.css": {Data: []byte("/*static css*/")},
	}
}

type buildApp struct {
	z      *Zen
	dir    string
	opened []string
}

// newBuildApp opens a folder of five pages with the static runtime in place.
func newBuildApp(t *testing.T) *buildApp {
	t.Helper()
	dir, _ := filepath.EvalSymlinks(tree(t, map[string]string{
		"README.md": "# Home\n\n[B](b.md)\n", "b.md": "# B\n", "c.md": "# C\n", "d.md": "# D\n", "e.md": "# E\n",
	}))
	b := &buildApp{dir: dir}
	z, err := newZen(zenOptions{
		Registry: plugins.NewRegistry(), GlobalPath: filepath.Join(t.TempDir(), "config.yml"),
		Dir: dir, Mode: "read", Events: &sink{}, Dialogs: &fakeDialogs{},
		Open: func(u string) error { b.opened = append(b.opened, u); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	b.z = z
	return b
}

func TestOpenBuiltSite(t *testing.T) {
	b := newBuildApp(t)
	var revealed []string
	prev := revealPath
	revealPath = func(p string) error { revealed = append(revealed, p); return nil }
	t.Cleanup(func() { revealPath = prev })

	out := filepath.Join(b.dir, "_site")
	if err := b.z.OpenBuiltSite(out); err == nil {
		t.Error("opened a site that was never built")
	}
	if _, err := b.z.BuildWebsite(BuildOptions{Out: out, Portable: true}); err != nil {
		t.Fatal(err)
	}
	if err := b.z.OpenBuiltSite(out); err != nil {
		t.Fatal(err)
	}
	if len(b.opened) != 1 || !strings.HasPrefix(b.opened[0], "file://") || !strings.HasSuffix(b.opened[0], "/_site/index.html") {
		t.Fatalf("opened %q", b.opened)
	}
	if err := b.z.RevealBuiltSite(out); err != nil || len(revealed) != 1 || revealed[0] != out {
		t.Errorf("reveal: %v %v", revealed, err)
	}

	// Only the last build's folder.
	for _, p := range []string{b.dir, filepath.Join(b.dir, "b.md"), filepath.Join(b.dir, "_site", ".."), "/", "/etc"} {
		if err := b.z.OpenBuiltSite(p); err == nil {
			t.Errorf("opened %s", p)
		}
		if err := b.z.RevealBuiltSite(p); err == nil {
			t.Errorf("revealed %s", p)
		}
	}
	// A marker that is gone, or an index that is a symlink: refused.
	if err := os.Remove(filepath.Join(out, ".zen-build")); err != nil {
		t.Fatal(err)
	}
	if err := b.z.OpenBuiltSite(out); err == nil {
		t.Error("opened without the marker")
	}
	if len(b.opened) != 1 {
		t.Errorf("opened %q", b.opened)
	}
}

func TestPickBuildFolder(t *testing.T) {
	b := newBuildApp(t)
	b.z.dialogs.(*fakeDialogs).folder = "/some/where"
	if got, err := b.z.PickBuildFolder(); err != nil || got != "/some/where" {
		t.Errorf("pick: %q, %v", got, err)
	}
	// Choosing does not open it as the workspace.
	if w := b.z.Startup().Workspace; w == nil || w.Root != b.dir {
		t.Errorf("workspace changed: %+v", w)
	}
}

// The dialog can switch drafts off although .zen.yml has build.drafts: true.
