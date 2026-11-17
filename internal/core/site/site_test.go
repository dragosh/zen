package site

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func richFake(root string) *fake {
	f := newFake()
	f.root, f.home = root, "README.md"
	f.page("README.md", "Home", `<h1 id="home">Home</h1><p><a data-zen-link="internal" data-path="docs/guide.md" href="#">Guide</a> <img src="/_ws/img/a.png" alt="a"></p>`)
	f.page("docs/guide.md", "Guide", `<p><a data-zen-link="internal" data-path="README.md" data-anchor="home" href="#">Home</a> <a data-zen-link="external" href="https://example.org/" rel="noopener noreferrer">ext</a></p>`)
	f.page("docs/deep/x.md", "X", `<p>x</p>`)
	f.files["img/a.png"] = "PNG"
	f.pages["docs/guide.md"] = func() Page {
		p := f.pages["docs/guide.md"]
		p.CSS = []string{"/_plugins/hello/hello.css?v=abc"}
		p.JS = []string{"/_plugins/hello/hello.js?v=abc"}
		return p
	}()
	f.assets["/_plugins/hello/hello.css"] = ".hello{}"
	f.assets["/_plugins/hello/hello.js"] = "export default { mount() {} }"
	return f
}

func TestDeterministic(t *testing.T) {
	root := t.TempDir()
	var outs []string
	var lists []map[string]string
	for i := 0; i < 2; i++ {
		f := richFake(root)
		_, out := build(t, f, Options{Out: filepath.Join(t.TempDir(), "o"), SiteID: "demo"})
		outs = append(outs, out)
		lists = append(lists, listing(t, out))
	}
	if !reflect.DeepEqual(lists[0], lists[1]) {
		t.Error("two builds of the same input differ")
	}
	if len(lists[0]) < 6 {
		t.Fatalf("too few files: %v", lists[0])
	}
	for name, body := range lists[0] {
		for _, secret := range []string{root, outs[0], outs[1], "/Users/", "/private/", "/var/folders"} {
			if strings.Contains(body, secret) {
				t.Errorf("%s contains %q", name, secret)
			}
		}
	}
	// A portable build is deterministic too.
	var p [2]map[string]string
	for i := range p {
		_, out := build(t, richFake(root), Options{Out: filepath.Join(t.TempDir(), "o"), SiteID: "demo", Portable: true})
		p[i] = listing(t, out)
	}
	if !reflect.DeepEqual(p[0], p[1]) {
		t.Error("two portable builds differ")
	}
}

func TestDrafts(t *testing.T) {
	mk := func() *fake {
		f := newFake()
		f.home = "README.md"
		f.page("README.md", "Home", `<p><a data-zen-link="internal" data-path="wip.md" href="#">WIP</a></p>`)
		f.page("wip.md", "Work in progress", `<p>wip</p>`)
		f.with("wip.md", func(p *Page) { p.Draft = true })
		f.page("pub.md", "Published", `<p>pub</p>`)
		return f
	}
	rep, out := build(t, mk(), Options{})
	if has(out, "wip.html") {
		t.Error("draft was built")
	}
	home := read(t, out, "README.html")
	contains(t, "home", home, `data-zen-link="broken"`)
	lacks(t, "home", home, `wip.html`, `data-path="wip.md"`)
	lacks(t, "sidebar", home, "Work in progress")
	if rep.Pages != 2 {
		t.Errorf("pages = %d, want 2", rep.Pages)
	}

	rep, out = build(t, mk(), Options{Drafts: true})
	if !has(out, "wip.html") || rep.Pages != 3 {
		t.Errorf("--drafts: %d pages, %v", rep.Pages, listing(t, out))
	}
	contains(t, "home", read(t, out, "README.html"), `href="wip.html"`, "Work in progress")
}

func TestOutputPaths(t *testing.T) {
	// .md becomes .html; the home is also index.html at the root.
	f := richFake(t.TempDir())
	rep, out := build(t, f, Options{})
	l := listing(t, out)
	for _, want := range []string{"README.html", "index.html", "docs/guide.html", "docs/deep/x.html", "img/a.png", "_zen/zen.js", "_zen/zen.css", ".zen-build"} {
		if _, ok := l[want]; !ok {
			t.Errorf("%s not written; have %v", want, keys(l))
		}
	}
	if l["index.html"] != l["README.html"] {
		t.Error("index.html is not the home page")
	}
	if rep.Pages != 3 {
		t.Errorf("pages = %d", rep.Pages)
	}
	// 3 workspace/asset files + zen.js + zen.css + plugin css/js = img, hello.css, hello.js, zen.css, zen.js.
	if rep.Files != 5 {
		t.Errorf("files = %d, want 5 (%v)", rep.Files, keys(l))
	}
	// Relative links between folders.
	contains(t, "guide", l["docs/guide.html"], `href="../README.html#home"`)
	contains(t, "home", l["README.html"], `href="docs/guide.html"`, `src="img/a.png"`)

	// A home in a folder: index.html links to it and redirects.
	g := newFake()
	g.home = "docs/README.md"
	g.page("docs/README.md", "Docs", "<p>d</p>").page("docs/a.md", "A", "<p>a</p>")
	_, out = build(t, g, Options{})
	idx := read(t, out, "index.html")
	contains(t, "redirect", idx, `http-equiv="refresh"`, `docs/README.html`, `<a href="docs/README.html"`)

	// A root index.md is the index.
	h := newFake().page("index.md", "Index", "<p>i</p>").page("b.md", "B", "<p>b</p>")
	_, out = build(t, h, Options{})
	contains(t, "index", read(t, out, "index.html"), "<p>i</p>")

	// Empty folder: only index.html, saying so.
	_, out = build(t, newFake(), Options{})
	e := listing(t, out)
	contains(t, "empty index", e["index.html"], "No pages yet")
	for name := range e {
		if strings.HasSuffix(name, ".html") && name != "index.html" {
			t.Errorf("empty folder wrote %s", name) // print.html is not written without pages
		}
	}

	// Collision: the first in sort order wins and a warning says so.
	c := newFake().page("A.md", "Upper", "<p>upper</p>").page("a.md", "Lower", "<p>lower</p>")
	rep, out = build(t, c, Options{})
	var htmls []string
	for name := range listing(t, out) {
		if strings.HasSuffix(name, ".html") && name != "index.html" && name != "print.html" {
			htmls = append(htmls, name)
		}
	}
	if len(htmls) != 1 || htmls[0] != "A.html" || !hasWarn(rep, "a.md", "A.md") || rep.Pages != 1 {
		t.Errorf("collision: %v %v pages=%d", htmls, rep.Warnings, rep.Pages)
	}
	contains(t, "winner", read(t, out, "A.html"), "upper")

	// Spaces, unicode and #.
	s := newFake()
	s.page("my page #1.md", "Mine", `<p>m</p>`).page("ünï/ça.md", "Ca", `<p><a data-zen-link="internal" data-path="my page #1.md" href="#">m</a></p>`)
	_, out = build(t, s, Options{})
	names := keys(listing(t, out))
	if !contains2(names, "my page #1.html") || !contains2(names, "ünï/ça.html") {
		t.Errorf("names: %v", names)
	}
	contains(t, "ça", read(t, out, "ünï/ça.html"), `href="../my%20page%20%231.html"`)
	contains(t, "sidebar", read(t, out, "my page #1.html"), `href="%C3%BCn%C3%AF/%C3%A7a.html"`)

	// _zen is reserved.
	z := newFake().page("_zen/x.md", "Z", "<p>z</p>").page("ok.md", "OK", "<p>ok</p>")
	rep, out = build(t, z, Options{})
	if has(out, "_zen/x.html") || !hasWarn(rep, "_zen/x.md") {
		t.Errorf("_zen page: %v", rep.Warnings)
	}
}

func TestExcludeGlobs(t *testing.T) {
	f := newFake().page("a.md", "A", "<p>a</p>").page("private/x.md", "X", "<p>x</p>").page("scratch.md", "S", "<p>s</p>").page("docs/scratch.md", "D", "<p>d</p>")
	_, out := build(t, f, Options{Exclude: []string{"private/**", "scratch.md"}})
	if has(out, "private/x.html") || has(out, "scratch.html") || !has(out, "docs/scratch.html") || !has(out, "a.html") {
		t.Errorf("excludes: %v", keys(listing(t, out)))
	}
}

func TestRenderErrorIsAWarning(t *testing.T) {
	f := newFake().page("a.md", "A", "<p>a</p>").page("b.md", "B", "<p>b</p>")
	delete(f.pages, "b.md")
	// Pages lists only what the map has; ask for a path Render cannot serve.
	g := &failing{fake: f}
	rep, out := build(t, g, Options{})
	if !has(out, "a.html") || !hasWarn(rep, "bad.md", "boom") {
		t.Errorf("%v", rep.Warnings)
	}
}

type failing struct{ *fake }

func (f *failing) Pages() ([]string, error) {
	ps, _ := f.fake.Pages()
	return append(ps, "bad.md"), nil
}
func (f *failing) Render(p string) (Page, error) {
	if p == "bad.md" {
		return Page{}, errBoom
	}
	return f.fake.Render(p)
}

func TestEditURL(t *testing.T) {
	mk := func() *fake {
		return newFake().page("docs/a.md", "A", `<p><a data-zen-action="edit">Edit this page</a></p>`)
	}
	_, out := build(t, mk(), Options{EditURL: "https://github.com/me/notes/edit/main/{path}"})
	contains(t, "edit", read(t, out, "docs/a.html"), `href="https://github.com/me/notes/edit/main/docs/a.md"`, `Edit this page`)
	_, out = build(t, mk(), Options{})
	lacks(t, "no editURL", read(t, out, "docs/a.html"), "Edit this page", "data-zen-action")
	if _, err := Build(mk(), Options{Out: filepath.Join(t.TempDir(), "o"), SiteID: "s", EditURL: "javascript:alert(1)//{path}"}); err == nil {
		t.Error("a javascript: editURL was accepted")
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains2(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestBuildPrintFile(t *testing.T) {
	f := guideFake()
	f.page("pic.md", "Pic", `<p><img src="/_ws/img/a.png"></p>`)
	f.files["img/a.png"] = "PNG"
	f.tree = append(f.tree, NavNode{Title: "Pic", Path: "pic.md", Kind: "page"})
	rep, out := build(t, f, Options{Portable: true})
	doc := read(t, out, "print.html")
	contains(t, "print.html", doc, `<h1 class="zen-print-title">Welcome</h1>`, `href="_zen/zen.css"`,
		`script-src &#39;self&#39; file: blob:`, `src="img/a.png"`, "4 pages") // the portable CSP
	if rep.Print != "print.html" {
		t.Errorf("Report.Print = %q", rep.Print)
	}
	if !has(out, "img/a.png") || !has(out, "_zen/zen.js") {
		t.Error("what print.html uses is not in the build")
	}
	// No page links to it.
	for name, body := range listing(t, out) {
		if strings.HasSuffix(name, ".html") && name != "print.html" {
			lacks(t, name, body, "print.html")
		}
	}
	// Drafts follow the build: left out unless asked for.
	f.with("pic.md", func(p *Page) { p.Draft = true })
	_, out = build(t, f, Options{})
	lacks(t, "print.html", read(t, out, "print.html"), "Pic<")
	// A hosted build's print.html has the hosted CSP, like its pages.
	_, out = build(t, f, Options{})
	lacks(t, "hosted print.html", read(t, out, "print.html"), "file:", "blob:")
	// Deterministic.
	_, o2 := build(t, guideFake(), Options{})
	_, o3 := build(t, guideFake(), Options{})
	if read(t, o2, "print.html") != read(t, o3, "print.html") {
		t.Error("print.html differs between two builds")
	}
	// NoPrint leaves it out.
	rep, out = build(t, guideFake(), Options{NoPrint: true})
	if has(out, "print.html") || rep.Print != "" {
		t.Error("--no-print still wrote print.html")
	}
	// The count of files does not change by it.
	repA, _ := build(t, guideFake(), Options{})
	repB, _ := build(t, guideFake(), Options{NoPrint: true})
	if repA.Files != repB.Files {
		t.Errorf("print.html counted as a file: %d vs %d", repA.Files, repB.Files)
	}
}
