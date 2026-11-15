package app

import (
	"strings"
	"testing"
)

func TestSlugLinks(t *testing.T) {
	dir := tree(t, map[string]string{
		"README.md":     "# Home\n\n[i](install) [j](/install) [k](install#part) [r](real) [d](dup) [e](../install) [n](nested/slug)\n",
		"docs/setup.md": "---\nslug: install\n---\n# Setup\n",
		"docs/sub.md":   "# Sub\n\n[s](install)\n",
		"real.md":       "# Real file\n",
		"other.md":      "---\nslug: real\n---\n# Other\n",
		"a.md":          "---\nslug: dup\n---\n# A\n",
		"b.md":          "---\nslug: dup\n---\n# B\n",
		"bad.md":        "---\nslug: ../etc\n---\n# Bad\n",
		"deep/n.md":     "---\nslug: nested/slug\n---\n# N\n",
	})
	z, _, _, _ := newTestZen(t, dir, "", "read")
	p, err := z.Page("README.md")
	if err != nil {
		t.Fatal(err)
	}
	h := p.HTML
	if strings.Count(h, `data-path="docs/setup.md"`) != 3 || !strings.Contains(h, `data-path="docs/setup.md" data-anchor="part"`) {
		t.Fatalf("slug links: %s", h)
	}
	if !strings.Contains(h, `data-path="real.md"`) || strings.Contains(h, `data-path="other.md"`) {
		t.Fatalf("files must win: %s", h)
	}
	if !strings.Contains(h, `data-path="a.md"`) || strings.Contains(h, `data-path="b.md"`) {
		t.Fatalf("duplicate: first by path wins: %s", h)
	}
	if !strings.Contains(h, `data-path="deep/n.md"`) {
		t.Fatalf("nested slug: %s", h)
	}
	if strings.Count(h, `data-zen-link="broken"`) != 1 { // ../install
		t.Fatalf("../install must stay broken: %s", h)
	}
	if s, _ := z.Page("docs/sub.md"); !strings.Contains(s.HTML, `data-path="docs/setup.md"`) {
		t.Fatalf("slug from subfolder: %s", s.HTML)
	}

	// The index follows slug links: setup is a child of the home page.
	w := z.Startup().Workspace
	found := false
	for _, c := range w.Tree[0].Children {
		if c.Path == "docs/setup.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tree: %+v", w.Tree)
	}

	if b, _ := z.Page("b.md"); !hasWarn(b.Warnings, "dup") {
		t.Fatalf("duplicate warning: %v", b.Warnings)
	}
	if a, _ := z.Page("a.md"); len(a.Warnings) != 0 {
		t.Fatalf("first keeps it: %v", a.Warnings)
	}
	if b, _ := z.Page("bad.md"); !hasWarn(b.Warnings, "slug") {
		t.Fatalf("invalid slug warning: %v", b.Warnings)
	}
}

func hasWarn(ws []string, sub string) bool {
	for _, w := range ws {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}
