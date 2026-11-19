package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/builtins"
)

// TestOutsideAssembly: a folder that uses a plugin or a layout the app was not
// built with behaves as an unknown name always did (kit/assembly): the block is
// a plain code block, the layout falls back to article, each with its warning,
// and nothing is lost from the file.
func TestOutsideAssembly(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		".zen.yml":  "version: 1\n",
		"flow.md":   "---\nplugins:\n  diagram: true\n---\n# Flow\n\n```mermaid\ngraph TD; A-->B\n```\n",
		"guide.md":  "---\nlayout: docs\n---\n# Guide\n\nText.\n",
		"README.md": "# Home\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := builtins.PluginsFor([]string{"codeblock"}) // no diagram
	if err != nil {
		t.Fatal(err)
	}
	lay, err := builtins.LayoutsFor("test", []string{"landing"}) // article and landing, no docs
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Registry: reg, Layouts: lay, Version: "test"})
	t.Cleanup(s.Close)
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}

	p, err := s.Page("flow.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(p.HTML, `data-zen-name="diagram"`) || !strings.Contains(p.HTML, "<pre") || !strings.Contains(p.HTML, "A--&gt;B") {
		t.Errorf("the mermaid block should stay a plain code block with its source: %s", p.HTML)
	}
	if !warned(p.Warnings, `unknown plugin "diagram"`) {
		t.Errorf("warnings %v: want the unknown plugin warning", p.Warnings)
	}

	p, err = s.Page("guide.md")
	if err != nil {
		t.Fatal(err)
	}
	if p.Layout.Name != "article" || !warned(p.Warnings, `unknown layout "docs"`) || !strings.Contains(p.HTML, "Text.") {
		t.Errorf("layout %+v, warnings %v: want article and the unknown layout warning", p.Layout, p.Warnings)
	}
}

func warned(ws []string, want string) bool {
	for _, w := range ws {
		if strings.Contains(w, want) {
			return true
		}
	}
	return false
}
