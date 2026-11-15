package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/plugins"
)

func TestOpenConfig(t *testing.T) {
	var opened []string
	old := openFile
	openFile = func(p string) error { opened = append(opened, p); return nil }
	t.Cleanup(func() { openFile = old })

	dir := tree(t, map[string]string{"README.md": "# Home\n"})
	global := filepath.Join(t.TempDir(), "zen", "config.yml")
	z, err := newZen(zenOptions{Registry: plugins.NewRegistry(), GlobalPath: global, Dir: dir, Events: &sink{}, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)

	// Global: created from the template (folder too), then opened.
	if err := z.OpenConfig("global"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(global)
	if !strings.Contains(string(b), "version: 1") || !strings.Contains(string(b), "# limits:") {
		t.Fatalf("global template: %q", b)
	}
	// The template changes nothing: no warnings, default limits.
	if c := config.Load(global, ""); len(c.Warnings()) != 0 || c.Limits() != (config.Limits{}) {
		t.Fatalf("template not neutral: %v %+v", c.Warnings(), c.Limits())
	}

	// An existing file is never overwritten.
	if err := os.WriteFile(global, []byte("limits:\n  files: 9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := z.OpenConfig("global"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(global); string(b) != "limits:\n  files: 9\n" {
		t.Fatalf("overwritten: %q", b)
	}

	// Workspace: .zen.yml in the folder.
	if err := z.OpenConfig("workspace"); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(z.state.workspace().Root, config.WorkspaceFile)
	if b, _ := os.ReadFile(ws); !strings.Contains(string(b), "version: 1") {
		t.Fatalf("workspace template: %q", b)
	}
	if len(opened) != 3 || opened[2] != ws {
		t.Fatalf("opened = %v", opened)
	}

	if err := z.OpenConfig("../etc/passwd"); err == nil {
		t.Fatal("unknown name accepted")
	}
	none, _ := newZen(zenOptions{Registry: plugins.NewRegistry(), Events: &sink{}, Dialogs: &fakeDialogs{}})
	if err := none.OpenConfig("workspace"); err == nil {
		t.Fatal("workspace config without a folder")
	}
}
