package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

func newNavApp(t *testing.T) (*Zen, string, *sink) {
	t.Helper()
	dir, _ := filepath.EvalSymlinks(tree(t, map[string]string{
		"README.md": "# Home\n\n[A](a.md) [B](b.md)\n", "a.md": "# A\n", "b.md": "# B\n", "sub/x.md": "# X\n",
	}))
	ev := &sink{}
	z, err := newZen(zenOptions{Registry: plugins.NewRegistry(), GlobalPath: filepath.Join(t.TempDir(), "config.yml"), Dir: dir, Mode: "read", Events: ev, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	return z, dir, ev
}

func TestNavService(t *testing.T) {
	z, dir, _ := newNavApp(t)
	if err := z.ReorderNav("README.md", []string{"b.md", "a.md"}); err != nil {
		t.Fatal(err)
	}
	if got := z.Startup().Workspace.Tree[0].Children[0].Path; got != "b.md" {
		t.Errorf("first child %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.md")); string(b) != "---\nnav:\n  order: 20\n---\n# A\n" {
		t.Errorf("a.md: %q", b)
	}
	if err := z.SetNavHidden("a.md", true); err != nil {
		t.Fatal(err)
	}
	if n := len(z.Startup().Workspace.Tree[0].Children); n != 1 {
		t.Errorf("%d children after hiding", n)
	}
	if err := z.SetNavHidden("../a.md", true); err == nil {
		t.Error("hid a page outside the folder")
	}
}

func TestRevealInFinder(t *testing.T) {
	z, dir, _ := newNavApp(t)
	var shown []string
	prev := revealFile
	revealFile = func(p string) error { shown = append(shown, p); return nil }
	t.Cleanup(func() { revealFile = prev })

	for rel, want := range map[string]string{"a.md": filepath.Join(dir, "a.md"), "sub": filepath.Join(dir, "sub")} {
		shown = nil
		if err := z.RevealInFinder(rel); err != nil || len(shown) != 1 || shown[0] != want {
			t.Errorf("RevealInFinder(%q): %v %v", rel, shown, err)
		}
	}
	for _, bad := range []string{"../x", "/etc/passwd", "/", "sub/../../x", "missing.md", "a\x00.md", "-R"} {
		if err := z.RevealInFinder(bad); err == nil {
			t.Errorf("RevealInFinder(%q) did not fail", bad)
		} else if strings.Contains(err.Error(), dir) {
			t.Errorf("the error names the folder: %v", err)
		}
	}
	if len(shown) != 1 {
		t.Errorf("a refused path was revealed: %v", shown)
	}
}
