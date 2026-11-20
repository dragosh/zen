package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/session"
)

// roWindowsZen is a read-only Zen with a window manager (so a window would
// open if the guard let it).
func roWindowsZen(t *testing.T, r *windowRig, files map[string]string) *Zen {
	t.Helper()
	z, err := newZen(zenOptions{
		Registry: plugins.NewRegistry(), Dir: tree(t, files), ReadOnly: true,
		GlobalPath: filepath.Join(t.TempDir(), "config.yml"),
		Events:     &sink{}, Dialogs: &fakeDialogs{}, Open: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	z.windows = r.m
	return z
}

func TestReadOnlyNoEditor(t *testing.T) {
	r := newWindowRig(t)
	z := roWindowsZen(t, r, map[string]string{"README.md": "# Home\n"})
	if err := z.OpenEditor("README.md"); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("OpenEditor: %v", err)
	}
	for _, which := range []string{"global", "workspace"} {
		if err := z.OpenConfig(which); !errors.Is(err, session.ErrReadOnly) {
			t.Errorf("OpenConfig(%s): %v", which, err)
		}
	}
	// Without a window manager too: no text editor either.
	z.windows = nil
	old := openFile
	openFile = func(string) error { t.Error("openFile called"); return nil }
	t.Cleanup(func() { openFile = old })
	if err := z.OpenConfig("global"); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("OpenConfig without windows: %v", err)
	}
	if len(r.specs) != 0 {
		t.Errorf("windows opened: %+v", r.specs)
	}
	// Writes through the bindings are refused too.
	if err := z.Save("README.md", "x"); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("Save: %v", err)
	}
	// Reading is unchanged.
	if pg, err := z.Page("README.md"); err != nil || pg.HTML == "" {
		t.Errorf("Page: %v", err)
	}
	// Zenmark itself (features: all) is not read-only.
	if readOnlyBuild() {
		t.Error("the committed assembly has the editor")
	}
}

func TestReadOnlyStartupEdit(t *testing.T) {
	r := newWindowRig(t)
	z := roWindowsZen(t, r, map[string]string{"README.md": "# Home\n", "guide.md": "# Guide\n"})
	z.page = "guide.md"
	if err := z.openStartupEditor("edit"); err != nil {
		t.Fatalf("an edit launch falls back to the reader: %v", err)
	}
	if len(r.specs) != 0 {
		t.Errorf("editor window opened: %+v", r.specs)
	}
	if st := z.Startup(); st.Mode != "read" || st.Page != "guide.md" {
		t.Errorf("startup = %+v", st)
	}
	// An editor window from an older state file is refused.
	if err := z.OpenEditor("guide.md"); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("OpenEditor: %v", err)
	}
	if _, err := os.Stat(filepath.Join(z.s.Workspace().Root, ".zen.yml")); err == nil {
		t.Error(".zen.yml created")
	}
}

func TestStartupBuildFeatures(t *testing.T) {
	z, _, _, _ := newTestZen(t, "", "", "")
	if b := z.Startup().Build; b == nil || !slices.Contains(b.Features, "editor") {
		t.Errorf("Build.Features = %+v", b)
	}
}
