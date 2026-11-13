package index

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func graph(m map[string][]string) func(string) []string {
	return func(p string) []string { return m[p] }
}

func title(p string) string { return "T:" + p }

func page(p string, children ...Node) Node {
	return Node{Title: "T:" + p, Path: p, Kind: "page", Children: children}
}

func TestTreeFromLinks(t *testing.T) {
	files := []string{"README.md", "a.md", "b.md", "c.md", "d.md"}
	links := map[string][]string{
		"README.md": {"README.md", "b.md", "a.md", "b.md", "missing.md"},
		"a.md":      {"c.md", "README.md", "d.md"},
		"b.md":      {"c.md", "A.md"},
		"c.md":      {"a.md"},
	}
	got := Build(files, "README.md", graph(links), title).Tree()
	want := []Node{page("README.md", page("b.md", page("c.md")), page("a.md", page("d.md")))}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestCaseInsensitive(t *testing.T) {
	files := []string{"README.md", "Docs/Guide.md"}
	got := Build(files, "readme.md", graph(map[string][]string{"README.md": {"docs/guide.MD"}}), title).Tree()
	want := []Node{page("README.md", page("Docs/Guide.md"))}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestEdgeCases(t *testing.T) {
	if tr := Build(nil, "", graph(nil), title).Tree(); len(tr) != 0 {
		t.Fatalf("empty: %+v", tr)
	}
	got := Build([]string{"README.md"}, "README.md", graph(map[string][]string{"README.md": {"README.md"}}), title).Tree()
	if !reflect.DeepEqual(got, []Node{page("README.md")}) {
		t.Fatalf("single: %+v", got)
	}
}

func TestDeepChain(t *testing.T) {
	n := 1000
	files := make([]string, n)
	links := map[string][]string{}
	for i := range files {
		files[i] = fmt.Sprintf("p%04d.md", i)
		if i > 0 {
			links[files[i-1]] = []string{files[i]}
		}
	}
	ix := Build(files, files[0], graph(links), title)
	if bc := ix.Breadcrumb(files[n-1]); len(bc) != n {
		t.Fatalf("breadcrumb len %d", len(bc))
	}
}

func TestNoHome(t *testing.T) {
	files := []string{"z.md", "b.md", "sub/x.md", "sub/y.md"}
	links := map[string][]string{"z.md": {"sub/x.md", "b.md"}, "b.md": {"sub/x.md"}}
	got := Build(files, "", graph(links), title).Tree()
	want := []Node{
		page("b.md", page("sub/x.md")),
		page("z.md"),
		{Title: "Other pages", Kind: "group", Children: []Node{
			{Title: "sub", Path: "sub", Kind: "group", Children: []Node{
				{Title: "y", Path: "sub/y.md", Kind: "page"},
			}},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestOtherPages(t *testing.T) {
	files := []string{"README.md", "z.md", "a/b/c.md", "a/d.md", "e.md"}
	got := Build(files, "README.md", graph(nil), title).Tree()
	want := []Node{
		page("README.md"),
		{Title: "Other pages", Kind: "group", Children: []Node{
			{Title: "a", Path: "a", Kind: "group", Children: []Node{
				{Title: "b", Path: "a/b", Kind: "group", Children: []Node{
					{Title: "c", Path: "a/b/c.md", Kind: "page"},
				}},
				{Title: "d", Path: "a/d.md", Kind: "page"},
			}},
			{Title: "e", Path: "e.md", Kind: "page"},
			{Title: "z", Path: "z.md", Kind: "page"},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestBreadcrumb(t *testing.T) {
	files := []string{"README.md", "a.md", "b.md", "x/o.md"}
	links := map[string][]string{"README.md": {"a.md"}, "a.md": {"b.md"}}
	ix := Build(files, "README.md", graph(links), title)
	want := []Crumb{{"T:README.md", "README.md"}, {"T:a.md", "a.md"}, {"T:b.md", "b.md"}}
	if got := ix.Breadcrumb("B.md"); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if got := ix.Breadcrumb("x/o.md"); !reflect.DeepEqual(got, []Crumb{{"o", "x/o.md"}}) {
		t.Fatalf("other: %+v", got)
	}
	if got := ix.Breadcrumb("nope.md"); len(got) != 0 {
		t.Fatalf("unknown: %+v", got)
	}
}

func TestEqual(t *testing.T) {
	files := []string{"README.md", "a.md"}
	l1 := graph(map[string][]string{"README.md": {"a.md"}})
	a, b := Build(files, "README.md", l1, title), Build(files, "README.md", l1, title)
	if !a.Equal(b) {
		t.Fatal("expected equal")
	}
	c := Build(files, "README.md", graph(nil), title)
	if a.Equal(c) {
		t.Fatal("expected different")
	}
	d := Build(files, "README.md", l1, func(p string) string { return strings.ToUpper(p) })
	if a.Equal(d) {
		t.Fatal("expected different titles")
	}
}

func large() ([]string, func(string) []string) {
	n := 5000
	files := make([]string, n)
	for i := range files {
		files[i] = fmt.Sprintf("dir%02d/page%04d.md", i%50, i)
	}
	files[0] = "README.md"
	links := make(map[string][]string, n)
	for i, f := range files {
		ls := make([]string, 10)
		for j := range ls {
			ls[j] = files[(i*7+j*131+1)%n]
		}
		links[f] = ls
	}
	return files, graph(links)
}

func TestBuildLarge(t *testing.T) {
	files, links := large()
	start := time.Now()
	Build(files, "README.md", links, title)
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("took %v", d)
	}
}

func BenchmarkBuild(b *testing.B) {
	files, links := large()
	for i := 0; i < b.N; i++ {
		Build(files, "README.md", links, title)
	}
}
