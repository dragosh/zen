package site

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func printOf(t *testing.T, in Input, o PrintOptions) (string, Report) {
	t.Helper()
	doc, rep, err := PrintDocument(in, o)
	if err != nil {
		t.Fatalf("PrintDocument: %v", err)
	}
	return doc, rep
}

// guideFake is a folder with a home page and a "Guide" group in a set order.
func guideFake() *fake {
	f := newFake()
	f.page("index.md", "Welcome", "<h1>Welcome</h1><p>hello</p>")
	f.page("guide/b.md", "Bee", "<h1>Bee</h1>")
	f.page("guide/a.md", "Ay", "<h1>Ay</h1>")
	f.home = "index.md"
	f.with("guide/a.md", func(p *Page) {
		p.Breadcrumb = []Crumb{{"Guide", "guide"}, {"Ay", "guide/a.md"}}
	})
	f.tree = []NavNode{
		{Title: "Welcome", Path: "index.md", Kind: "page"},
		{Title: "Guide", Path: "guide", Kind: "group", Children: []NavNode{
			{Title: "Bee", Path: "guide/b.md", Kind: "page"},
			{Title: "Ay", Path: "guide/a.md", Kind: "page"},
		}},
	}
	return f
}

func index(t *testing.T, s, sub string) int {
	t.Helper()
	i := strings.Index(s, sub)
	if i < 0 {
		t.Fatalf("missing %q in:\n%s", sub, s)
	}
	return i
}

func TestPrintDocument(t *testing.T) {
	doc, rep := printOf(t, guideFake(), PrintOptions{Date: "17 November 2026"})
	if rep.Pages != 3 {
		t.Errorf("Pages = %d", rep.Pages)
	}
	contains(t, "document", doc, "<!doctype html>", `<h1 class="zen-print-title">Welcome</h1>`, "17 November 2026", "3 pages",
		`<nav class="zen-print-toc" aria-label="Contents">`, `role="document"`)

	// Sections come in sidebar order, not sorted order (b before a).
	s1, s2, s3 := index(t, doc, `id="zen-print-1"`), index(t, doc, `id="zen-print-2"`), index(t, doc, `id="zen-print-3"`)
	if !(s1 < s2 && s2 < s3) {
		t.Errorf("section order %d %d %d", s1, s2, s3)
	}
	if !strings.Contains(doc[s2:s3], "<h1>Bee</h1>") || !strings.Contains(doc[s3:], "<h1>Ay</h1>") {
		t.Errorf("sections are not in sidebar order:\n%s", doc)
	}
	contains(t, "section", doc, `data-path="guide/b.md"`)

	// The contents: nested like the sidebar, each linking to its section.
	toc := doc[index(t, doc, `class="zen-print-toc"`):s1]
	i1 := index(t, toc, `<a href="#zen-print-1">Welcome</a>`)
	ig := index(t, toc, "Guide")
	i2 := index(t, toc, `<a href="#zen-print-2">Bee</a>`)
	i3 := index(t, toc, `<a href="#zen-print-3">Ay</a>`)
	if !(i1 < ig && ig < i2 && i2 < i3) {
		t.Errorf("contents order:\n%s", toc)
	}
	if strings.Count(toc[:ig], "<ul") != strings.Count(toc[:ig], "</ul>")+1 || strings.Count(toc[ig:i2], "<ul") != 1 {
		t.Errorf("Guide's pages are not nested one level down:\n%s", toc)
	}

	// The running label: the breadcrumb above the title.
	contains(t, "label", doc, `<p class="zen-print-crumb">Guide</p>`)

	// A cover title by option, no date when none is given, and one page.
	f := newFake().page("only.md", "Only", "<p>x</p>")
	doc, _ = printOf(t, f, PrintOptions{Title: "My <book>"})
	contains(t, "single", doc, `<h1 class="zen-print-title">My &lt;book&gt;</h1>`, "1 page<")
	lacks(t, "single", doc, "1 pages")
	// Without a home or title it is the folder's name.
	f.root = "/tmp/notes"
	doc, _ = printOf(t, f, PrintOptions{})
	contains(t, "folder name", doc, `<h1 class="zen-print-title">notes</h1>`)

	// Deterministic: the same input and options give the same bytes.
	a, _ := printOf(t, guideFake(), PrintOptions{})
	b, _ := printOf(t, guideFake(), PrintOptions{})
	if a != b {
		t.Error("two documents differ")
	}
	lacks(t, "no date", a, "zen-print-date")
}

func TestPrintEmptyFolder(t *testing.T) {
	doc, rep := printOf(t, newFake(), PrintOptions{})
	if rep.Pages != 0 {
		t.Errorf("Pages = %d", rep.Pages)
	}
	contains(t, "empty", doc, "zen-print-cover", "No pages")
	lacks(t, "empty", doc, `class="zen-print-page`)
}

func TestPrintLinks(t *testing.T) {
	f := newFake()
	f.page("a.md", "A", `<h2 id="intro">Intro</h2>
<p><a data-zen-link="internal" data-path="b.md" data-anchor="intro" href="/b.md">to b</a>
<a data-zen-link="internal" data-path="a.md" href="/a.md">self</a>
<a data-zen-link="internal" data-path="a.md" data-anchor="intro" href="/a.md#intro">self anchor</a>
<a data-zen-link="internal" data-path="gone.md" href="/gone.md">gone</a>
<a data-zen-link="internal" data-path="the-b" href="/the-b">slug</a>
<a href="#intro">local</a>
<a data-zen-link="external" href="https://example.com/x">ext</a>
<a data-zen-link="external" href="https://example.com/y">https://example.com/y</a>
<a data-zen-link="external" href="javascript:alert(1)">bad</a>
<a data-zen-action="edit" href="/edit">Edit</a>
<sup><a href="#fn:1" id="fnref:1">1</a></sup>
<label for="x">l</label><input id="x" type="radio" name="g">
<img src="/_ws/img/a.png"></p>`)
	f.page("b.md", "B", `<h2 id="intro">Intro</h2><p id="p1">b</p>`)
	f.slugs["the-b"] = "b.md"
	doc, _ := printOf(t, f, PrintOptions{})

	a := doc[index(t, doc, `id="zen-print-1"`):index(t, doc, `id="zen-print-2"`)]
	contains(t, "page a", a,
		`<h2 id="zen-print-1-intro">`,
		`href="#zen-print-2-intro"`, // to b, anchored
		`href="#zen-print-1"`,       // self
		`href="#zen-print-1-intro"`, // self anchor and the plain fragment
		`href="#zen-print-2">slug`,  // by slug
		`href="#zen-print-1-fn:1"`,  // plain fragments get the prefix
		`id="zen-print-1-fnref:1"`,
		`for="zen-print-1-x"`, `id="zen-print-1-x"`, `name="zen-print-1-g"`,
		`data-zen-print="url"`, // text is the URL: no second copy
		`src="/_ws/img/a.png"`) // the app serves images
	// A page that is not in the document prints as text; dangerous links go.
	if m := regexp.MustCompile(`<a[^>]*>gone</a>`).FindString(a); m == "" || strings.Contains(m, "href") {
		t.Errorf("a link to a missing page keeps its href: %q", m)
	}
	lacks(t, "page a", a, "javascript:", "/edit", "gone.md")
	if !regexp.MustCompile(`<a[^>]*href="https://example.com/x"[^>]*>ext</a>`).MatchString(a) ||
		regexp.MustCompile(`href="https://example.com/x"[^>]*data-zen-print`).MatchString(a) {
		t.Errorf("an external link with other text: %s", a)
	}
	// The same heading in page b does not clash.
	b := doc[index(t, doc, `id="zen-print-2"`):]
	contains(t, "page b", b, `<h2 id="zen-print-2-intro">`, `id="zen-print-2-p1"`)
	if strings.Count(doc, ` id="intro"`) != 0 {
		t.Error("an unprefixed id is left")
	}
	// No page can reach another's elements by id: every id carries its own prefix.
	for _, m := range regexp.MustCompile(` id="([^"]+)"`).FindAllStringSubmatch(doc, -1) {
		if !strings.HasPrefix(m[1], "zen-print-") {
			t.Errorf("id %q is not prefixed", m[1])
		}
	}
}

func TestPrintDraftsAndErrors(t *testing.T) {
	f := newFake()
	f.page("a.md", "A", "<p>a</p>")
	f.page("draft.md", "Secret draft", "<p>shh</p>")
	f.with("draft.md", func(p *Page) { p.Draft = true })
	f.page("warn.md", "W", "<p>w</p>")
	f.with("warn.md", func(p *Page) { p.Warnings = []string{"plugin \"x\" is blocked"} })
	in := &brokenPages{fake: f, bad: map[string]bool{"broken.md": true}}

	doc, rep := printOf(t, in, PrintOptions{})
	lacks(t, "no drafts", doc, "Secret draft", "shh")
	if rep.Pages != 3 { // a, warn and the broken one as an error section
		t.Errorf("Pages = %d", rep.Pages)
	}
	// A page that fails to render is a short error section and the document goes on.
	contains(t, "error", doc, `zen-print-error`, "broken.md", "could not be rendered")
	if !hasWarn(rep, "broken.md", "boom") || !hasWarn(rep, "warn.md", "blocked") {
		t.Errorf("warnings: %v", rep.Warnings)
	}
	lacks(t, "error", doc, "<script>alert")

	doc, rep = printOf(t, in, PrintOptions{Drafts: true})
	contains(t, "drafts", doc, "<p>shh</p>")
	if rep.Pages != 4 {
		t.Errorf("Pages with drafts = %d", rep.Pages)
	}
}

// brokenPages is a fake in which some pages are listed but fail to render.
type brokenPages struct {
	*fake
	bad map[string]bool
}

func (f *brokenPages) Pages() ([]string, error) {
	ps, _ := f.fake.Pages()
	for p := range f.bad {
		ps = append(ps, p)
	}
	return ps, nil
}
func (f *brokenPages) Render(p string) (Page, error) {
	if f.bad[p] {
		return Page{}, fmt.Errorf("%w <script>alert(1)</script>", errBoom)
	}
	return f.fake.Render(p)
}

func TestPrintLimitAndSection(t *testing.T) {
	f := newFake()
	for i := 0; i < 5; i++ {
		f.page(fmt.Sprintf("p%d.md", i), fmt.Sprintf("P%d", i), "<p>x</p>")
	}
	if _, _, err := PrintDocument(f, PrintOptions{Max: 4}); !errors.Is(err, ErrTooManyPages) {
		t.Errorf("err = %v", err)
	}
	if _, rep, err := PrintDocument(f, PrintOptions{Max: 5}); err != nil || rep.Pages != 5 {
		t.Errorf("at the limit: %v %d", err, rep.Pages)
	}
	// Drafts do not count against the limit.
	f.with("p0.md", func(p *Page) { p.Draft = true })
	if _, rep, err := PrintDocument(f, PrintOptions{Max: 4}); err != nil || rep.Pages != 4 {
		t.Errorf("drafts counted: %v %d", err, rep.Pages)
	}

	// The default is 500.
	big := newFake()
	for i := 0; i < 501; i++ {
		big.page(fmt.Sprintf("p%03d.md", i), "P", "<p>x</p>")
	}
	if _, _, err := PrintDocument(big, PrintOptions{}); !errors.Is(err, ErrTooManyPages) {
		t.Errorf("501 pages: %v", err)
	}

	// A section is a page and the pages under it.
	g := guideFake()
	g.tree = []NavNode{
		{Title: "Welcome", Path: "index.md", Kind: "page", Children: nil},
		{Title: "Guide", Path: "guide/a.md", Kind: "page", Children: []NavNode{{Title: "Bee", Path: "guide/b.md", Kind: "page"}}},
	}
	doc, rep := printOf(t, g, PrintOptions{Section: "guide/a.md"})
	if rep.Pages != 2 {
		t.Errorf("section pages = %d", rep.Pages)
	}
	contains(t, "section", doc, "<h1>Ay</h1>", "<h1>Bee</h1>")
	lacks(t, "section", doc, "<h1>Welcome</h1>")
	// Not a page in the tree: refused, nothing is read.
	if _, _, err := PrintDocument(g, PrintOptions{Section: "../etc/passwd"}); err == nil {
		t.Error("a section outside the tree was accepted")
	}
}

func TestPrintLayoutsCSS(t *testing.T) {
	f := newFake()
	f.page("a.md", "A", `<div data-zen-layout="handout">x</div>`)
	f.page("b.md", "B", `<div data-zen-layout="handout">y</div>`)
	f.page("c.md", "C", `<div data-zen-layout="slides">s</div>`)
	f.page("d.md", "D", "<p>plain</p>")
	for p, l := range map[string]string{"a.md": "handout", "b.md": "handout", "c.md": "slides"} {
		l := l
		f.with(p, func(pg *Page) {
			pg.Layout = l
			pg.CSS = []string{"/_layouts/" + l + "/" + l + ".css?v=1"}
			pg.JS = []string{"/_layouts/" + l + "/" + l + ".js?v=1"}
		})
	}
	f.assets["/_layouts/handout/handout.css"] = "@media print{.zl-sheet{border:0}} /* </style> */"
	f.assets["/_layouts/slides/slides.css"] = "@media print{.slides-slide{break-after:page}}"
	f.assets["/_layouts/handout/handout.js"] = "export default { mount(el) {} }"
	f.assets["/_layouts/slides/slides.js"] = "export default { mount(el) {} }"
	doc, rep := printOf(t, f, PrintOptions{SiteID: "mysite"})
	// Each layout's CSS once, inline, and it cannot close its own tag.
	if n := strings.Count(doc, ".zl-sheet{border:0}"); n != 1 {
		t.Errorf("handout css %d times", n)
	}
	contains(t, "css", doc, ".slides-slide{break-after:page}", "<style")
	lacks(t, "css", doc, "/* </style> */")
	// Sections carry their layout.
	contains(t, "sections", doc, `zen-layout-handout" data-layout="handout"`, `zen-layout-slides" data-layout="slides"`)
	// Scripts as in portable mode: modules inline, the runtime from _zen, the page data lists them all.
	contains(t, "scripts", doc, `data-zen-module="layout:handout"`, `data-zen-module="layout:slides"`, `"portable":true`,
		`"siteId":"mysite"`, `<script src="_zen/zen.js" defer></script>`, `href="_zen/zen.css"`, `"/_layouts/handout/handout.js"`)
	lacks(t, "scripts", doc, `src="/_layouts`)
	if strings.Count(doc, `data-zen-module="layout:handout"`) != 1 {
		t.Error("a module is inlined twice")
	}
	if len(rep.Warnings) != 0 {
		t.Errorf("warnings %v", rep.Warnings)
	}
	// The CSP is the portable one.
	contains(t, "csp", doc, `script-src &#39;self&#39; file: blob:`)
	// A page with no scripts and a document of plain pages carries no runtime.
	doc, _ = printOf(t, newFake().page("p.md", "P", "<p>x</p>"), PrintOptions{})
	lacks(t, "plain", doc, "zen.js", "data-zen-module")
}

func TestPrintScriptsNeedingImportsAreLeftOut(t *testing.T) {
	f := newFake().page("a.md", "A", `<div data-zen-plugin="p">x</div>`)
	f.with("a.md", func(p *Page) { p.JS = []string{"/_plugins/p/p.js?v=1"} })
	f.assets["/_plugins/p/p.js"] = "import x from './y.js'; export default {mount(){}}"
	doc, rep := printOf(t, f, PrintOptions{})
	lacks(t, "doc", doc, "data-zen-module")
	if !hasWarn(rep, "a.md", "imports") {
		t.Errorf("warnings %v", rep.Warnings)
	}
}
