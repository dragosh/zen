package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dragosh/zenmd/internal/core/plugins"
	"github.com/dragosh/zenmd/internal/core/workspace"
)

func TestOpenPath(t *testing.T) {
	dir := tree(t, docs)
	z, _, _, _ := newTestZen(t, "", "", "")

	st, err := z.openPath(dir)
	if err != nil || st.Workspace == nil || st.Page != "README.md" || st.Mode != "read" {
		t.Fatalf("folder: %+v %v", st, err)
	}

	st, err = z.openPath(filepath.Join(dir, "docs", "guide.md"))
	if err != nil || st.Workspace == nil || st.Page != "guide.md" {
		t.Fatalf("file: %+v %v", st, err)
	}

	if _, err := z.openPath(filepath.Join(dir, "img", "a.png")); err == nil {
		t.Fatal("non-markdown file should be refused")
	}
}

func TestOpenTooLargeFolder(t *testing.T) {
	f := workspace.MaxFiles
	workspace.MaxFiles = 2
	t.Cleanup(func() { workspace.MaxFiles = f })
	dir := tree(t, map[string]string{"a.md": "", "b.md": "", "c.md": ""})

	if _, err := newZen(zenOptions{Registry: plugins.NewRegistry(), Dir: dir, Events: &sink{}, Dialogs: &fakeDialogs{}}); err == nil ||
		err.Error() != "this folder has more than 2 markdown files; open a smaller folder" {
		t.Fatalf("startup: %v", err)
	}

	z, _, dl, _ := newTestZen(t, "", "", "")
	dl.folder = dir
	if ws, err := z.PickFolder(); ws != nil || err == nil || strings.Contains(err.Error(), dir) {
		t.Fatalf("pick: %v %v", ws, err)
	}
	if _, err := z.openPath(dir); err == nil {
		t.Fatal("open with: want error")
	}
	if z.Startup().Workspace != nil {
		t.Fatal("a refused folder must not become the workspace")
	}
}

func TestGlobalLimitsApply(t *testing.T) {
	dir := tree(t, map[string]string{"a.md": "", "b.md": "", "c.md": ""})
	global := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(global, []byte("limits:\n  files: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newZen(zenOptions{Registry: plugins.NewRegistry(), GlobalPath: global, Dir: dir, Events: &sink{}, Dialogs: &fakeDialogs{}}); err == nil ||
		!strings.Contains(err.Error(), "more than 2 markdown files") {
		t.Fatalf("global limit not applied: %v", err)
	}
}
