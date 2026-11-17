package session

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/plugins/check"
	"github.com/dragosh/zen/internal/plugins/codeblock"
)

func TestCreatePage(t *testing.T) {
	dir := tree(t, docs)
	z, _, _, _ := newTestZen(t, dir, "", "")

	cases := []struct{ name, layout, want, has string }{
		{"ideas", "", "ideas.md", "---\ntitle: Ideas\n---\n# Ideas\n"},
		{"talks/my-first_deck", "slides", "talks/my-first_deck.md", "layout: slides"},
		{" docs/Learn Go.markdown ", "course", "docs/Learn Go.markdown", "layoutOptions: { id: learn-go }"},
		{"/notes/q: and a", "", "notes/q: and a.md", "title: \"Q: and a\""},
	}
	for _, c := range cases {
		got, err := z.CreatePage(c.name, c.layout)
		if err != nil || got != c.want {
			t.Fatalf("CreatePage(%q, %q) = %q, %v; want %q", c.name, c.layout, got, err, c.want)
		}
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(got)))
		if err != nil || !strings.Contains(string(b), c.has) {
			t.Errorf("%s: %v\n%s", got, err, b)
		}
		// The frontmatter parses and names the layout.
		pg, err := z.Page(got)
		if l, _ := pg.Frontmatter["layout"].(string); err != nil || l != c.layout {
			t.Errorf("%s renders: frontmatter %v, err %v", got, pg.Frontmatter, err)
		}
	}

	// Never overwrites.
	if _, err := z.CreatePage("README", ""); !errors.Is(err, errPageExists) {
		t.Errorf("existing page: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "README.md")); string(b) != docs["README.md"] {
		t.Error("README.md was overwritten")
	}
	for _, bad := range []struct{ name, layout string }{
		{"", ""}, {"   ", ""}, {"folder/", ""}, {"../out", ""}, {"a/../../out", ""},
		{".hidden", ""}, {"a/.git/x", ""}, {"notes.txt", ""}, {"ok", "poster"}, {"x\x00y", ""},
	} {
		if p, err := z.CreatePage(bad.name, bad.layout); err == nil {
			t.Errorf("CreatePage(%q, %q) = %q, want an error", bad.name, bad.layout, p)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "out.md")); err == nil {
		t.Error("wrote outside the folder")
	}
}

func TestCreatePageNoWorkspace(t *testing.T) {
	z, _, _, _ := newTestZen(t, "", "", "")
	if _, err := z.CreatePage("a", ""); err == nil {
		t.Error("created a page with no folder open")
	}
}

func TestTitleOf(t *testing.T) {
	for in, want := range map[string]string{"a/my-deck.md": "My deck", "x_y.md": "X y", "-.md": "Untitled", "über.md": "Über"} {
		if got := titleOf(in); got != want {
			t.Errorf("titleOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// The templates render with the real layout plugins and no warnings.
func TestCreatePageTemplatesRender(t *testing.T) {
	reg := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{codeblock.New(), check.New()} {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	z, err := newZen(zenOptions{Registry: reg, ConfigDir: t.TempDir(), Dir: tree(t, map[string]string{"README.md": "# Home\n"}), Events: &sink{}, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	for _, layout := range []string{"", "slides", "course", "docs", "post", "landing", "wide", "handout"} {
		rel, err := z.CreatePage("new-"+layout, layout)
		if err != nil {
			t.Fatal(err)
		}
		pg, err := z.Page(rel)
		want := layout
		if want == "" {
			want = "article"
		}
		if err != nil || pg.Layout.Name != want || len(pg.Warnings) != 0 {
			t.Errorf("%q: layout %+v, warnings %v, err %v", layout, pg.Layout, pg.Warnings, err)
		}
	}
}
