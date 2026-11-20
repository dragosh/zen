package app

import (
	"os"
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
	// Outside the folder or missing; folders and other files inside it may be revealed (ui/sidebar-menu).
	for _, p := range []string{"../x.md", "/etc/hosts", "missing.md"} {
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

// Settings › Configuration shows the two settings files in Finder by name; every other path stays confined to the folder.
func TestRevealInFinderConfig(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# Home\n", ".zen.yml": "a: 1\n"})
	var shown []string
	old := revealFile
	revealFile = func(p string) error { shown = append(shown, p); return nil }
	t.Cleanup(func() { revealFile = old })
	z, _, _, _ := newTestZen(t, dir, "", "read")

	if err := z.RevealInFinder("zen:config/workspace"); err != nil {
		t.Fatal(err)
	}
	if len(shown) != 1 || filepath.Base(shown[0]) != ".zen.yml" {
		t.Fatalf("shown %v", shown)
	}
	// The global file: shown when it exists; "does not exist yet" otherwise, without a path in the message.
	global, _, err := z.s.ConfigFile("global")
	if err != nil {
		t.Skip("no global settings file on this system")
	}
	_ = os.Remove(global)
	if err := z.RevealInFinder("zen:config/global"); err == nil || strings.Contains(err.Error(), filepath.Dir(global)) {
		t.Errorf("missing global file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(global), 0o755); err == nil && os.WriteFile(global, []byte("a: 1\n"), 0o644) == nil {
		if err := z.RevealInFinder("zen:config/global"); err != nil {
			t.Errorf("global file: %v", err)
		}
	}
	// Anything else under the prefix, or a path that only looks like it, is refused.
	n := len(shown)
	for _, p := range []string{"zen:config/other", "zen:config/../../etc/hosts", "zen:config/", "x/zen:config/global"} {
		if err := z.RevealInFinder(p); err == nil {
			t.Errorf("RevealInFinder(%q) succeeded", p)
		}
	}
	if len(shown) != n {
		t.Fatalf("rejected paths were revealed: %v", shown[n:])
	}
}
