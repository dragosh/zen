package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartupReadsFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	if err := os.WriteFile(p, []byte("# hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.md")

	docs := (&Documents{files: []string{p, missing}}).Startup()

	if len(docs) != 2 || docs[0].Content != "# hi" || docs[0].Name != "a.md" || docs[0].HTML != `<h1 id="hi">hi</h1>`+"\n" {
		t.Fatalf("unexpected docs: %+v", docs)
	}
	if docs[1].Error == "" {
		t.Fatal("expected error for missing file")
	}
}
