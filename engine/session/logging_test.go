package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log/slog"

	"github.com/zenmark-org/zenmd/engine/watch"
)

// capture makes h the default logger for the test.
func capture(t *testing.T) *recHandler {
	t.Helper()
	h := &recHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("log lacks %q:\n%s", w, got)
		}
	}
}

// app.logging: opening a folder logs info with the name and the page count; a
// render warning logs warn; a file event logs debug.
func TestAppLogging(t *testing.T) {
	h := capture(t)
	b := newBuildZen(t, map[string]string{
		"README.md": "# Home\n\n[B](b.md)\n",
		"b.md":      "---\nlayout: nonesuch\n---\n# B\n",
	})
	name := filepath.Base(b.dir)
	mustContain(t, h.all(), "INFO folder opened", "name="+name, "pages=2")

	if _, err := b.z.Page("b.md"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, h.all(), "DEBUG page rendered", "path=b.md", "WARN render warning", "path=b.md")

	b.z.handle([]watch.Event{{Path: "b.md", Kind: watch.Changed}})
	mustContain(t, h.all(), "DEBUG file event", "path=b.md", "kind=changed")

	if err := b.z.Open(""); err != nil {
		t.Fatal(err)
	}
	mustContain(t, h.all(), "INFO folder closed")
}

func TestConfigAndSaveLogging(t *testing.T) {
	h := capture(t)
	b := newBuildZen(t, map[string]string{"README.md": "# Home\n"})
	if err := b.z.Save("zen:config/workspace", "toc: true\n"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, h.all(), "INFO config saved", "file=.zenmd.yml", "INFO config reloaded")
	h.recs = nil
	b.z.handle([]watch.Event{{Path: ".zenmd.yml", Kind: watch.Changed}}) // edited on disk
	mustContain(t, h.all(), "INFO config reloaded", "file=.zenmd.yml")
}

// No entry from an open, a render, a save, a file event or an export has the
// folder's absolute path or the home directory in it.
func TestNoAbsolutePaths(t *testing.T) {
	h := capture(t)
	b := newBuildZen(t, map[string]string{"README.md": "# Home\n\n[B](b.md)\n", "b.md": "# B\n---\nlayout: x\n"})
	if _, err := b.z.Page("b.md"); err != nil {
		t.Fatal(err)
	}
	if err := b.z.Save("b.md", "# B2\n"); err != nil {
		t.Fatal(err)
	}
	b.z.handle([]watch.Event{{Path: "b.md", Kind: watch.Changed}})
	if _, err := b.z.Page("../nope.md"); err == nil {
		t.Fatal("want an error")
	}
	out := filepath.Join(b.dir, "_site")
	if _, err := b.z.Build(t.Context(), BuildOptions{Out: out, Portable: true}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.z.BuildWebsite(BuildOptions{Out: out, Portable: true}); err != nil {
		t.Fatal(err)
	}
	got := h.all()
	mustContain(t, got, "INFO export finished")
	if got == "" {
		t.Fatal("nothing logged")
	}
	home, _ := os.UserHomeDir()
	for _, bad := range []string{b.dir, home, os.TempDir()} {
		if bad != "" && bad != "/" && strings.Contains(got, bad) {
			t.Errorf("the log has %q:\n%s", bad, got)
		}
	}
}

// File contents and settings values are never logged.
func TestNoSecretsLogged(t *testing.T) {
	h := capture(t)
	b := newBuildZen(t, map[string]string{
		"README.md":  "# Home\n\nSECRET-BODY-TEXT\n",
		".zenmd.yml": "plugins:\n  calc:\n    token: SECRET-TOKEN-VALUE\n",
	})
	if _, err := b.z.Page("README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.z.Preview("README.md", "# P\n\nSECRET-PREVIEW-TEXT\n"); err != nil {
		t.Fatal(err)
	}
	if err := b.z.Save("README.md", "# S\n\nSECRET-SAVED-TEXT\n"); err != nil {
		t.Fatal(err)
	}
	if err := b.z.Save("zen:config/workspace", "plugins:\n  calc:\n    token: SECRET-NEW-TOKEN\n"); err != nil {
		t.Fatal(err)
	}
	b.z.reloadConfig()
	b.z.handle([]watch.Event{{Path: ".zenmd.yml", Kind: watch.Changed}})
	got := h.all()
	if got == "" {
		t.Fatal("nothing logged")
	}
	if strings.Contains(got, "SECRET") {
		t.Errorf("a secret is logged:\n%s", got)
	}
}
