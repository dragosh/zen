package layouts

import (
	"os"
	"path/filepath"
	"testing"
)

// write creates the files (path -> content) under a new <tmp>/<id> folder and
// returns it.
func write(t *testing.T, id string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), id)
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func mustLoad(t *testing.T, id string, files map[string]string) *Layout {
	t.Helper()
	l, err := Load(write(t, id, files), SourceFolder)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return l
}
