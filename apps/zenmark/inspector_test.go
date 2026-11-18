package app

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRevealInFinderPage(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# Home\n", "notes/a.txt": "x"})
	var shown []string
	old := revealFile
	revealFile = func(p string) error { shown = append(shown, p); return nil }
	t.Cleanup(func() { revealFile = old })
	z, _, _, _ := newTestZen(t, dir, "", "read")

	if err := z.RevealInFinder("README.md"); err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(dir, "README.md"))
	if len(shown) != 1 || shown[0] != want {
		t.Fatalf("shown %v, want [%s]", shown, want)
	}
	for _, p := range []string{"../x.md", "/etc/hosts", "notes/a.txt", "missing.md", ""} {
		if err := z.RevealInFinder(p); err == nil {
			t.Errorf("RevealInFinder(%q) succeeded", p)
		} else if strings.Contains(err.Error(), dir) {
			t.Errorf("RevealInFinder(%q) leaks the folder: %v", p, err)
		}
	}
	if len(shown) != 1 {
		t.Fatalf("rejected paths were revealed: %v", shown)
	}
}
