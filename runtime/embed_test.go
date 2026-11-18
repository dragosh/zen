package runtime_test

import (
	"io/fs"
	"strings"
	"testing"

	zenruntime "github.com/zenmark-org/zenmd/runtime"
)

// TestFS: the embedded runtime has both files, and the script is a classic
// IIFE a page can load with <script src> (no module syntax).
func TestFS(t *testing.T) {
	fsys := zenruntime.FS()
	js, err := fs.ReadFile(fsys, "zen.js")
	if err != nil {
		t.Fatal(err)
	}
	css, err := fs.ReadFile(fsys, "zen.css")
	if err != nil {
		t.Fatal(err)
	}
	if len(js) == 0 || len(css) == 0 {
		t.Fatalf("empty runtime: zen.js %d bytes, zen.css %d bytes", len(js), len(css))
	}
	if strings.Contains(string(js), "import(") && strings.HasPrefix(strings.TrimSpace(string(js)), "import ") {
		t.Error("zen.js is an ES module")
	}
}
