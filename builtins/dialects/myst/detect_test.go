package myst

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func detect(files map[string]string) bool {
	m := fstest.MapFS{}
	for p, c := range files {
		m[p] = &fstest.MapFile{Data: []byte(c)}
	}
	return New().Detect(m)
}

func TestDetect(t *testing.T) {
	yes := []map[string]string{
		{"myst.yml": "version: 1\n"},
		{"myst.yml": ""},
		{"_config.yml": "title: Book\nparse:\n  myst_enable_extensions:\n    - colon_fence\n"},
		{"_config.yml": "sphinx:\n  config:\n    myst_heading_anchors: 3\n"},
		{"conf.py": "extensions = [\"myst_parser\"]\n"},
		{"conf.py": "extensions = ['myst_nb']\n# myst_parser\n"},
	}
	for _, f := range yes {
		if !detect(f) {
			t.Errorf("not detected: %v", f)
		}
	}
	no := []map[string]string{
		{},
		{"README.md": "# x\n"},
		{"_config.yml": "title: Jekyll site\n"},
		{"conf.py": "extensions = ['sphinx.ext.autodoc']\n"},
		{"_config.yml": "# mentions myst only in a comment-free way: nothing\nplugins: [jekyll-feed]\n"},
		// only the root counts
		{"docs/myst.yml": "version: 1\n", "sub/conf.py": "myst_parser", "docs/_config.yml": "parse:\n"},
	}
	for _, f := range no {
		if detect(f) {
			t.Errorf("detected: %v", f)
		}
	}
}

// countFS records every file opened.
type countFS struct {
	fs.FS
	opened []string
}

func (c *countFS) Open(name string) (fs.File, error) {
	c.opened = append(c.opened, name)
	return c.FS.Open(name)
}

func TestDetectReadsOnly(t *testing.T) {
	big := "x: 1\n" + strings.Repeat("a", 1<<20) + "\nparse:\n"
	m := fstest.MapFS{
		"_config.yml": {Data: []byte(big)},
		"conf.py":     {Data: []byte(strings.Repeat("#\n", 100000) + "myst_parser")},
		"other.txt":   {Data: []byte("myst_parser")},
		"page.md":     {Data: []byte("```{note}\n```")},
	}
	c := &countFS{FS: m}
	if New().Detect(c) {
		t.Error("detected content past the 64 KB limit")
	}
	for _, name := range c.opened {
		if name != "." && name != "myst.yml" && name != "_config.yml" && name != "conf.py" {
			t.Errorf("opened %q", name)
		}
	}
}
