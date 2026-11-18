package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dragosh/zenmd/engine/config"
	"github.com/dragosh/zenmd/engine/plugins"
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
	ws := filepath.Join(z.s.Workspace().Root, config.WorkspaceFile)
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

func TestConfigInEditorWindow(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# Home\n"})
	global := filepath.Join(t.TempDir(), "zen", "config.yml")
	ev := &sink{}
	z, err := newZen(zenOptions{Registry: plugins.NewRegistry(), GlobalPath: global, Dir: dir, Events: ev, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	rig := newWindowRig(t)
	z.windows = rig.m

	// Opens zen's editor window on the config "page", titled with the file name.
	if err := z.OpenConfig("global"); err != nil {
		t.Fatal(err)
	}
	if len(rig.specs) != 1 || rig.specs[0].Name != editorName("zen:config/global") ||
		!strings.Contains(rig.specs[0].URL, "path=zen%3Aconfig%2Fglobal") || !strings.HasPrefix(rig.specs[0].Title, "config.yml") {
		t.Fatalf("specs = %+v", rig.specs)
	}

	// Source and Save go to the file; lint is off.
	src, err := z.Source("zen:config/global")
	if err != nil || !strings.Contains(src, "version: 1") {
		t.Fatalf("source: %q %v", src, err)
	}
	if r, _ := z.Lint("zen:config/global", src); r.Available {
		t.Fatal("config files must not be linted")
	}
	if err := z.Save("zen:config/global", "limits:\n  files: 1\n"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(global); string(b) != "limits:\n  files: 1\n" {
		t.Fatalf("saved: %q", b)
	}
	if fi, _ := os.Stat(global); fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}

	// Invalid YAML or a non-mapping is refused and the file is kept.
	for _, bad := range []string{"limits: [\n", "- a\n- b\n"} {
		if err := z.Save("zen:config/global", bad); err == nil || !strings.Contains(err.Error(), "not saved") {
			t.Fatalf("bad %q: %v", bad, err)
		}
	}
	if b, _ := os.ReadFile(global); string(b) != "limits:\n  files: 1\n" {
		t.Fatalf("overwritten by bad YAML: %q", b)
	}

	// Saving the folder's .zen.yml re-renders the open page.
	if _, err := z.Page("README.md"); err != nil {
		t.Fatal(err)
	}
	before := len(ev.all())
	if err := z.OpenConfig("workspace"); err != nil {
		t.Fatal(err)
	}
	if err := z.Save("zen:config/workspace", "plugins:\n  lint: false\n"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range ev.all()[before:] {
		if e.Name == EventPageChanged {
			found = true
		}
	}
	if !found {
		t.Fatal("no page:changed after saving .zen.yml")
	}

	// The Edit path still refuses these pseudo paths.
	if err := z.OpenEditor("zen:config/global"); err == nil {
		t.Fatal("OpenEditor accepted a config path")
	}
}
