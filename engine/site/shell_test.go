package site

import (
	"regexp"
	"strings"
	"testing"
)

func sidebarFake() *fake {
	f := newFake()
	f.home = "README.md"
	f.page("README.md", "Home", "<p>home</p>")
	f.page("guide/a.md", "Alpha", "<p>a</p>")
	f.page("guide/b.md", "Beta", "<p>b</p>")
	f.page("other/c.md", "Gamma", "<p>c</p>")
	f.page("z.md", "Zed", "<p>z</p>")
	f.tree = []NavNode{
		{Title: "Home", Path: "README.md", Kind: "page", Children: []NavNode{
			{Title: "guide", Path: "guide", Kind: "group", Children: []NavNode{
				{Title: "Alpha", Path: "guide/a.md", Kind: "page"},
				{Title: "Beta", Path: "guide/b.md", Kind: "page"},
			}},
			{Title: "Zed", Path: "z.md", Kind: "page"},
		}},
		{Title: "other", Path: "other", Kind: "group", Children: []NavNode{{Title: "Gamma", Path: "other/c.md", Kind: "page"}}},
	}
	return f
}

func TestSidebar(t *testing.T) {
	_, out := build(t, sidebarFake(), Options{})
	a := read(t, out, "guide/a.html")
	// Order of the tree.
	last := -1
	for _, title := range []string{">Home<", ">guide<", ">Alpha<", ">Beta<", ">Zed<", ">other<", ">Gamma<"} {
		i := strings.Index(a, title)
		if j := strings.Index(a, `<nav aria-label="Pages"`); j > 0 && i >= 0 {
			i = strings.Index(a[j:], title) + j
		}
		if i < 0 || i < last {
			t.Fatalf("%s missing or out of order in:\n%s", title, a)
		}
		last = i
	}
	contains(t, "nav", a, `<nav aria-label="Pages"`, `href="b.html"`, `href="../README.html"`, `href="../other/c.html"`, `href="../z.html"`)
	if n := strings.Count(a, `aria-current="page"`); n != 1 {
		t.Errorf("aria-current %d times", n)
	}
	if !regexp.MustCompile(`<a[^>]*href="a.html"[^>]*aria-current="page"|<a[^>]*aria-current="page"[^>]*href="a.html"`).MatchString(a) {
		t.Errorf("current page not marked:\n%s", a)
	}
	// Open along the current path only.
	if !strings.Contains(a, `<details class="zen-site-group" open><summary>guide`) {
		t.Errorf("group of the current page not open:\n%s", a)
	}
	if strings.Contains(a, `<details class="zen-site-group" open><summary>other`) || !strings.Contains(a, `<details class="zen-site-group"><summary>other`) {
		t.Errorf("other group should be closed:\n%s", a)
	}
	// The page with children (Home) is open along the path too.
	if !strings.Contains(a, `<details class="zen-site-group" open><summary><a href="../README.html"`) {
		t.Errorf("home not open:\n%s", a)
	}
	// The page works without JS: the script is only for the top bar's buttons, so a page without plugins has no module.
	lacks(t, "page", a, "data-zen-module")
	// On another page another path is open.
	c := read(t, out, "other/c.html")
	if !strings.Contains(c, `<details class="zen-site-group" open><summary>other`) || strings.Contains(c, `<details class="zen-site-group" open><summary>guide`) {
		t.Errorf("open groups on other/c:\n%s", c)
	}
	// The narrow-screen toggle is a <details> labelled Contents.
	contains(t, "toggle", a, `<details class="zen-site-menu"><summary>Contents</summary>`)
}

func TestShellContents(t *testing.T) {
	f := newFake()
	f.home = "README.md"
	f.page("README.md", "Home", "<p>home</p>")
	f.page("docs/a.md", `A <b>&"Title"`, `<p>body & <em>text</em></p>`)
	f.with("docs/a.md", func(p *Page) {
		p.Layout = "docs"
		p.Lang = "fr"
		p.Breadcrumb = []Crumb{{Title: "Home", Path: "README.md"}, {Title: "A", Path: "docs/a.md"}}
		p.Warnings = []string{"secret warning text"}
	})
	rep, out := build(t, f, Options{SiteID: "my-notes"})
	a := read(t, out, "docs/a.html")
	contains(t, "a.html", a,
		`<!doctype html>`, `<html lang="fr">`, `<meta charset="utf-8">`, `<meta name="viewport"`,
		`<title>A &lt;b&gt;&amp;&#34;Title&#34;`, `<main id="main">`, `<a class="zen-skip" href="#main">Skip to content</a>`,
		`<nav aria-label="Breadcrumb"`, `<nav aria-label="Pages"`, `href="../README.html">Home</a>`,
		`<article class="zen-page zen-layout-docs" data-layout="docs"><p>body & <em>text</em></p></article>`)
	if !strings.Contains(read(t, out, "README.html"), `<html lang="en">`) {
		t.Error("default lang is not en")
	}
	// No app chrome, no warnings of the page in the output.
	lacks(t, "a.html", a, "secret warning", "Settings", "lint", "Lint", "<textarea", "contenteditable", "Dismiss", "/_ws/")
	if !hasWarn(rep, "docs/a.md", "secret warning text") {
		t.Errorf("page warning not reported: %v", rep.Warnings)
	}
	if strings.Count(read(t, out, "README.html"), `<nav aria-label="Breadcrumb"`) != 0 {
		t.Error("breadcrumb on a page without one")
	}
}

func TestShellCSP(t *testing.T) {
	for _, portable := range []bool{false, true} {
		f := richFake(t.TempDir())
		g := newFake()
		g.home = "docs/README.md"
		g.page("docs/README.md", "D", "<p>d</p>")
		for _, in := range []*fake{f, g, newFake()} {
			_, out := build(t, in, Options{Portable: portable})
			n := 0
			for name, body := range listing(t, out) {
				if !strings.HasSuffix(name, ".html") {
					continue
				}
				n++
				m := regexp.MustCompile(`<meta http-equiv="Content-Security-Policy" content="([^"]*)">`).FindStringSubmatch(body)
				if m == nil {
					t.Errorf("%s has no CSP meta", name)
					continue
				}
				csp := strings.ReplaceAll(m[1], "&#39;", "'")
				for _, want := range []string{"default-src 'self'", "object-src 'none'", "base-uri 'none'"} {
					if !strings.Contains(csp, want) {
						t.Errorf("%s: CSP lacks %s: %s", name, want, csp)
					}
				}
				wantScript := "script-src 'self'"
				if portable {
					wantScript = "script-src 'self' file: blob:"
				}
				if !portable && strings.Contains(csp, "file:") {
					t.Errorf("%s: hosted pages must not allow file: %s", name, csp)
				}
				if !regexp.MustCompile(`(^|; )` + regexp.QuoteMeta(wantScript) + `(;|$)`).MatchString(csp) {
					t.Errorf("%s: want %q in %s", name, wantScript, csp)
				}
				if strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
					t.Errorf("%s: %s", name, csp)
				}
			}
			if n == 0 {
				t.Error("no pages")
			}
		}
	}
}

// ui/site-topbar: the fixed top bar of the page shell.
func TestTopBar(t *testing.T) {
	f := sidebarFake()
	f.with("guide/a.md", func(p *Page) {
		p.Breadcrumb = []Crumb{{Title: "Home", Path: "README.md"}, {Title: "Alpha", Path: "guide/a.md"}}
	})
	_, out := build(t, f, Options{SiteID: "s"})
	a := read(t, out, "guide/a.html")
	bar := regexp.MustCompile(`(?s)<header class="zen-top zen-site-top">.*?</header>`).FindString(a)
	if bar == "" {
		t.Fatalf("no top bar:\n%s", a)
	}
	// Order: sidebar button, back, forward, then the breadcrumb.
	last := -1
	for _, want := range []string{`zen-site-sidebar-toggle`, `zen-site-back`, `zen-site-forward`, `class="zen-site-crumbs"`} {
		i := strings.Index(bar, want)
		if i < 0 || i < last {
			t.Fatalf("%s missing or out of order in:\n%s", want, bar)
		}
		last = i
	}
	contains(t, "bar", bar,
		`aria-label="Hide sidebar"`, `aria-expanded="true"`, `aria-controls="zen-site-side"`, `title="Hide sidebar (⌘\)"`,
		`aria-label="Back"`, `title="Back (⌘[)"`, `aria-label="Forward"`, `title="Forward (⌘])"`, `>‹</button>`, `>›</button>`,
		`<nav aria-label="Breadcrumb"`, `href="../README.html">Home</a>`)
	// The buttons need scripts: hidden until zen.js shows them. Back and Forward start disabled.
	for _, cls := range []string{"zen-site-sidebar-toggle", "zen-site-back", "zen-site-forward"} {
		if !regexp.MustCompile(`<button[^>]*class="[^"]*` + cls + `[^"]*"[^>]*\bhidden\b`).MatchString(bar) {
			t.Errorf("%s is not hidden without scripts:\n%s", cls, bar)
		}
	}
	// The bar sits first in the content column, before main; the sidebar carries the id the toggle controls.
	contains(t, "page", a, `<aside class="zen-site-side" id="zen-site-side">`, `<script src="../_zen/zen.js" defer></script>`, `"siteId":"s"`)
	if strings.Index(a, `class="zen-top zen-site-top"`) > strings.Index(a, `<main id="main">`) {
		t.Error("the bar is after main")
	}
	// The home page has no breadcrumb but still the bar with its buttons.
	h := read(t, out, "README.html")
	contains(t, "home", h, `class="zen-top zen-site-top"`, `zen-site-back`)
	lacks(t, "home", h, `<nav aria-label="Breadcrumb"`)
}

// A site of one page has no sidebar, so no sidebar button; Back and Forward stay.
func TestTopBarNoSidebar(t *testing.T) {
	f := newFake()
	f.home = "README.md"
	f.page("README.md", "Home", "<p>home</p>")
	f.tree = []NavNode{}
	_, out := build(t, f, Options{})
	h := read(t, out, "README.html")
	contains(t, "home", h, `zen-site-back`, `zen-site-forward`)
	lacks(t, "home", h, "zen-site-sidebar-toggle", "<aside")
}
