package layouts

import (
	"strings"
	"testing"

	"github.com/dragosh/zenmd/internal/core/config"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"blog/**", "blog/a.md", true},
		{"blog/**", "blog/2026/a.md", true},
		{"blog/**", "blogger/a.md", false},
		{"blog/**", "a/blog/a.md", false},
		{"talks/*.md", "talks/a.md", true},
		{"talks/*.md", "talks/x/a.md", false},
		{"*.md", "a.md", true},
		{"*.md", "x/a.md", false},
		{"**/*.md", "a.md", true},
		{"**/*.md", "x/y/a.md", true},
		{"docs/**/guide.md", "docs/guide.md", true},
		{"docs/**/guide.md", "docs/a/b/guide.md", true},
		{"docs/**/guide.md", "docs/a/b/other.md", false},
		{"notes/a.md", "notes/a.md", true},
		{"notes/a.md", "Notes/a.md", false},
		{"a.b/**", "axb/c.md", false}, // a dot is a dot
		{"x(1)/*.md", "x(1)/a.md", true},
		{"", "a.md", false},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v", c.pattern, c.path, got)
		}
	}
}

func defs(def string, rules ...config.LayoutRule) config.LayoutDefaults {
	return config.LayoutDefaults{Default: def, Rules: rules}
}

func TestResolvePrecedence(t *testing.T) {
	d := defs("docs", config.LayoutRule{Pattern: "blog/**", Name: "post"}, config.LayoutRule{Pattern: "**", Name: "wide"})
	cases := []struct {
		name string
		path string
		meta map[string]any
		want string
	}{
		{"frontmatter wins over pattern and default", "blog/a.md", map[string]any{"layout": "slides"}, "slides"},
		{"pattern wins over default", "blog/a.md", nil, "post"},
		{"first matching pattern wins", "x/a.md", nil, "wide"},
		{"default when no pattern matches", "a.md", nil, "wide"}, // ** matches everything
		{"article when nothing is set", "a.md", map[string]any{}, "article"},
	}
	for _, c := range cases {
		dd := d
		if c.name == "article when nothing is set" {
			dd = config.LayoutDefaults{}
		}
		if got := Resolve(c.path, c.meta, dd).Name; got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if got := Resolve("a.md", nil, defs("docs", config.LayoutRule{Pattern: "blog/**", Name: "post"})).Name; got != "docs" {
		t.Errorf("default: %q", got)
	}
	if got := Resolve("a.md", nil, config.LayoutDefaults{}).Name; got != "article" {
		t.Errorf("fallback: %q", got)
	}
}

func TestResolveOptions(t *testing.T) {
	c := Resolve("a.md", map[string]any{"layout": "slides", "layoutOptions": map[string]any{"ratio": "4:3", "n": 3}}, defs(""))
	if c.Name != "slides" || c.Options["ratio"] != "4:3" || c.Options["n"] != 3 || len(c.Warnings) != 0 {
		t.Errorf("layoutOptions: %+v", c)
	}
	// The short form: a mapping with the name and the options.
	c = Resolve("a.md", map[string]any{"layout": map[string]any{"name": "post", "cover": true, "toc": "no"}}, defs(""))
	if c.Name != "post" || c.Options["cover"] != true || c.Options["toc"] != "no" || len(c.Warnings) != 0 {
		t.Errorf("short form: %+v", c)
	}
	// layoutOptions override the short form's options.
	c = Resolve("a.md", map[string]any{"layout": map[string]any{"name": "post", "toc": "a"}, "layoutOptions": map[string]any{"toc": "b"}}, defs(""))
	if c.Options["toc"] != "b" {
		t.Errorf("override: %+v", c)
	}
	// Options of a default layout come from the page only when it names the layout.
	c = Resolve("a.md", map[string]any{"layoutOptions": map[string]any{"x": "y"}}, defs("docs"))
	if c.Name != "docs" || c.Options["x"] != "y" {
		t.Errorf("default with options: %+v", c)
	}
}

func TestResolveWarnings(t *testing.T) {
	for name, meta := range map[string]map[string]any{
		"number":          {"layout": 3},
		"bad name":        {"layout": "Bad Name"},
		"map w/o name":    {"layout": map[string]any{"cover": "x"}},
		"options a list":  {"layout": "post", "layoutOptions": []any{"a"}},
		"nested option":   {"layout": "post", "layoutOptions": map[string]any{"a": map[string]any{"b": "c"}}},
		"map name number": {"layout": map[string]any{"name": 4}},
	} {
		c := Resolve("a.md", meta, defs(""))
		if len(c.Warnings) == 0 {
			t.Errorf("%s: no warning (%+v)", name, c)
		}
		for _, w := range c.Warnings {
			if strings.Contains(w, "\n") {
				t.Errorf("%s: multi-line warning %q", name, w)
			}
		}
	}
	// An invalid frontmatter layout falls back to article.
	if c := Resolve("a.md", map[string]any{"layout": 3}, defs("docs")); c.Name != "article" {
		t.Errorf("invalid frontmatter layout: %+v", c)
	}
}
