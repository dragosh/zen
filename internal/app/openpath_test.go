package app

import (
	"path/filepath"
	"testing"
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
