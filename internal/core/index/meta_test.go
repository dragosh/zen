package index

import (
	"reflect"
	"strings"
	"testing"
)

func metas(m map[string]Meta) func(string) Meta {
	return func(p string) Meta { return m[p] }
}

func ip(n int) *int { return &n }

func hasWarning(ws []string, sub string) bool {
	for _, w := range ws {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

func TestNoMetaUnchanged(t *testing.T) {
	cases := []struct {
		files []string
		home  string
		links map[string][]string
	}{
		{[]string{"README.md", "a.md", "b.md", "c.md", "d.md"}, "README.md", map[string][]string{
			"README.md": {"README.md", "b.md", "a.md", "b.md", "missing.md"},
			"a.md":      {"c.md", "README.md", "d.md"},
			"b.md":      {"c.md", "A.md"},
			"c.md":      {"a.md"},
		}},
		{[]string{"z.md", "b.md", "sub/x.md", "sub/y.md"}, "", map[string][]string{"z.md": {"sub/x.md", "b.md"}, "b.md": {"sub/x.md"}}},
		{[]string{"README.md", "z.md", "a/b/c.md", "a/d.md", "e.md"}, "README.md", nil},
		{[]string{"README.md", "Docs/Guide.md"}, "readme.md", map[string][]string{"README.md": {"docs/guide.MD"}}},
		{nil, "", nil},
	}
	for i, c := range cases {
		a := Build(c.files, c.home, graph(c.links), title)
		for _, m := range []func(string) Meta{nil, metas(nil)} {
			b := BuildWithMeta(c.files, c.home, graph(c.links), title, m)
			if !reflect.DeepEqual(a.Tree(), b.Tree()) {
				t.Fatalf("case %d: tree differs\n%+v\n%+v", i, a.Tree(), b.Tree())
			}
			for _, f := range c.files {
				if !reflect.DeepEqual(a.Breadcrumb(f), b.Breadcrumb(f)) {
					t.Fatalf("case %d: crumb %s differs", i, f)
				}
				if w := b.Warnings(f); len(w) != 0 {
					t.Fatalf("case %d: warnings %v", i, w)
				}
			}
		}
	}
}

func TestNavTitle(t *testing.T) {
	files := []string{"README.md", "a.md", "x/o.md"}
	links := map[string][]string{"README.md": {"a.md"}}
	ix := BuildWithMeta(files, "README.md", graph(links), title, metas(map[string]Meta{
		"a.md":   {Title: "Setup"},
		"x/o.md": {Title: "Orphan"},
	}))
	tr := ix.Tree()
	if tr[0].Children[0].Title != "Setup" {
		t.Fatalf("tree: %+v", tr)
	}
	if got := ix.Breadcrumb("a.md"); !reflect.DeepEqual(got, []Crumb{{"T:README.md", "README.md"}, {"Setup", "a.md"}}) {
		t.Fatalf("crumb: %+v", got)
	}
	if o := tr[1].Children[0].Children[0]; o.Title != "Orphan" || o.Path != "x/o.md" {
		t.Fatalf("other: %+v", tr[1])
	}
	if got := ix.Breadcrumb("x/o.md"); !reflect.DeepEqual(got, []Crumb{{"Orphan", "x/o.md"}}) {
		t.Fatalf("other crumb: %+v", got)
	}
}

func TestOrder(t *testing.T) {
	files := []string{"README.md", "a.md", "b.md", "c.md", "d.md", "e.md"}
	links := map[string][]string{"README.md": {"a.md", "b.md", "c.md", "d.md", "e.md"}}
	ix := BuildWithMeta(files, "README.md", graph(links), title, metas(map[string]Meta{
		"c.md": {Order: ip(2)},
		"e.md": {Order: ip(1)},
		"b.md": {Order: ip(2)},
	}))
	var got []string
	for _, c := range ix.Tree()[0].Children {
		got = append(got, c.Path)
	}
	// Ordered first ascending; tie b/c keeps link order; then a, d in link order.
	if want := []string{"e.md", "b.md", "c.md", "a.md", "d.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}

	// Roots (no home) are ordered too.
	ix = BuildWithMeta([]string{"a.md", "b.md"}, "", graph(nil), title, metas(map[string]Meta{"b.md": {Order: ip(-1)}}))
	if tr := ix.Tree(); tr[0].Path != "b.md" || tr[1].Path != "a.md" {
		t.Fatalf("roots: %+v", tr)
	}
}

func TestParent(t *testing.T) {
	files := []string{"README.md", "guides/index.md", "a.md", "b.md", "orphan.md", "c.md"}
	links := map[string][]string{
		"README.md": {"guides/index.md", "a.md"},
		"a.md":      {"b.md"},
	}
	ix := BuildWithMeta(files, "README.md", graph(links), title, metas(map[string]Meta{
		"a.md":            {Parent: "guides/index.md"}, // moves with its link child b
		"orphan.md":       {Parent: "Guides/Index.md"}, // case-insensitive, leaves Other pages
		"c.md":            {Parent: "missing.md"},
		"README.md":       {Parent: "README.md"}, // self
		"guides/index.md": {Parent: "b.md"},      // cycle: b is under a under guides
	}))
	want := []Node{page("README.md",
		page("guides/index.md", page("a.md", page("b.md")), page("orphan.md")),
	), {Title: "Other pages", Kind: "group", Children: []Node{{Title: "c", Path: "c.md", Kind: "page"}}}}
	if got := ix.Tree(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := ix.Breadcrumb("b.md"); len(got) != 4 || got[1].Path != "guides/index.md" {
		t.Fatalf("crumb: %+v", got)
	}
	if !hasWarning(ix.Warnings("c.md"), "missing.md") {
		t.Fatalf("missing: %v", ix.Warnings("c.md"))
	}
	if len(ix.Warnings("readme.md")) != 1 {
		t.Fatalf("self: %v", ix.Warnings("README.md"))
	}
	if !hasWarning(ix.Warnings("guides/index.md"), "cycle") {
		t.Fatalf("cycle: %v", ix.Warnings("guides/index.md"))
	}
	if len(ix.Warnings("a.md")) != 0 {
		t.Fatalf("a: %v", ix.Warnings("a.md"))
	}

	// Mutual overrides: the first (by path) wins, the second is a cycle.
	ix = BuildWithMeta([]string{"README.md", "x.md", "y.md"}, "README.md", graph(nil), title, metas(map[string]Meta{
		"x.md": {Parent: "y.md"}, "y.md": {Parent: "x.md"},
	}))
	other := ix.Tree()[1]
	if len(other.Children) != 1 || other.Children[0].Path != "y.md" || other.Children[0].Children[0].Path != "x.md" {
		t.Fatalf("mutual: %+v", ix.Tree())
	}
	if !hasWarning(ix.Warnings("y.md"), "cycle") || len(ix.Warnings("x.md")) != 0 {
		t.Fatalf("mutual warnings: %v %v", ix.Warnings("x.md"), ix.Warnings("y.md"))
	}

	// A hidden parent hides the child.
	ix = BuildWithMeta([]string{"README.md", "h.md", "k.md"}, "README.md", graph(map[string][]string{"README.md": {"h.md"}}), title, metas(map[string]Meta{
		"h.md": {Hidden: true}, "k.md": {Parent: "h.md"},
	}))
	if got := ix.Tree(); !reflect.DeepEqual(got, []Node{page("README.md")}) {
		t.Fatalf("hidden parent: %+v", got)
	}
	if got := ix.Breadcrumb("k.md"); len(got) != 3 {
		t.Fatalf("hidden parent crumb: %+v", got)
	}
}

func TestHidden(t *testing.T) {
	files := []string{"README.md", "a.md", "b.md", "c.md", "lost.md"}
	links := map[string][]string{"README.md": {"a.md"}, "a.md": {"b.md", "c.md"}}
	ix := BuildWithMeta(files, "README.md", graph(links), title, metas(map[string]Meta{
		"a.md":    {Hidden: true},
		"c.md":    {Parent: "README.md"}, // its own visible parent
		"lost.md": {Hidden: true},        // would be in Other pages
	}))
	want := []Node{page("README.md", page("c.md"))}
	if got := ix.Tree(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := ix.Breadcrumb("b.md"); !reflect.DeepEqual(got, []Crumb{{"T:README.md", "README.md"}, {"T:a.md", "a.md"}, {"T:b.md", "b.md"}}) {
		t.Fatalf("crumb: %+v", got)
	}
	if got := ix.Breadcrumb("lost.md"); len(got) != 1 {
		t.Fatalf("lost crumb: %+v", got)
	}
	// Hiding changes the tree.
	if ix.Equal(BuildWithMeta(files, "README.md", graph(links), title, nil)) {
		t.Fatal("hidden must change the tree")
	}
}

func TestParseMeta(t *testing.T) {
	m, slug, w := ParseMeta(map[string]any{
		"title": "Page",
		"slug":  "guides/install-2",
		"nav":   map[string]any{"title": " Setup ", "order": 2, "parent": "/guides/index.md", "hidden": true},
	})
	if slug != "guides/install-2" || m.Title != "Setup" || m.Order == nil || *m.Order != 2 ||
		m.Parent != "guides/index.md" || !m.Hidden || len(w) != 0 {
		t.Fatalf("valid: %+v %q %v", m, slug, w)
	}
	if m, slug, w := ParseMeta(nil); !reflect.DeepEqual(m, Meta{}) || slug != "" || len(w) != 0 {
		t.Fatalf("empty: %+v %q %v", m, slug, w)
	}
	if m, _, w := ParseMeta(map[string]any{"nav": map[string]any{"order": float64(3)}}); m.Order == nil || *m.Order != 3 || len(w) != 0 {
		t.Fatalf("float order: %+v %v", m, w)
	}
	if _, slug, w := ParseMeta(map[string]any{"slug": 404}); slug != "404" || len(w) != 0 {
		t.Fatalf("numeric slug: %q %v", slug, w)
	}

	bad := []map[string]any{
		{"nav": "nope"},
		{"nav": map[string]any{"title": 3}},
		{"nav": map[string]any{"title": "  "}},
		{"nav": map[string]any{"order": "2"}},
		{"nav": map[string]any{"order": 1.5}},
		{"nav": map[string]any{"parent": 7}},
		{"nav": map[string]any{"parent": ""}},
		{"nav": map[string]any{"hidden": "yes"}},
		{"nav": map[string]any{"colour": "red"}},
		{"slug": ""},
		{"slug": "../etc"},
		{"slug": "/install"},
		{"slug": "Install"},
		{"slug": "a//b"},
		{"slug": "a/"},
		{"slug": "-a"},
		{"slug": "https://x"},
		{"slug": "a.md"},
		{"slug": strings.Repeat("a", 101)},
		{"slug": []any{"a"}},
	}
	for _, fm := range bad {
		m, slug, w := ParseMeta(fm)
		if !reflect.DeepEqual(m, Meta{}) || slug != "" || len(w) != 1 {
			t.Errorf("%v: %+v %q %v", fm, m, slug, w)
		}
	}
	if _, slug, _ := ParseMeta(map[string]any{"slug": strings.Repeat("a", 100)}); slug == "" {
		t.Fatal("100 chars is allowed")
	}
}
