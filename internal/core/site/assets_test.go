package site

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestWorkspaceFiles(t *testing.T) {
	base := t.TempDir()
	f := newFake()
	f.page("README.md", "Home", `<p><img src="/_ws/img/a.png" alt="a"></p><p><img src="/_ws/img/a.png" alt="again"></p>`)
	f.page("docs/x.md", "X", `<p><img src="/_ws/img/a.png" alt="x"><img src="/_ws/img/my%20pic.png" alt="p"></p><p><img src="/_ws/missing.png" alt="m"></p>`+
		`<p><img src="/_ws/../secret.png"><img src="/_ws/%2e%2e/secret.png"><img src="/_ws//etc/passwd"><img src="/_ws/a/../../b.png"></p>`)
	f.files["img/a.png"] = "PNG-A"
	f.files["img/my pic.png"] = "PNG-P"
	f.files["../secret.png"] = "SECRET"
	f.files["/etc/passwd"] = "SECRET"
	f.files["../b.png"] = "SECRET"
	out := filepath.Join(base, "out")
	rep, _ := build(t, f, Options{Out: out})

	if read(t, out, "img/a.png") != "PNG-A" || read(t, out, "img/my pic.png") != "PNG-P" {
		t.Error("files not copied")
	}
	n := 0
	for _, p := range f.wsAsked {
		if p == "img/a.png" {
			n++
		}
		if strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
			t.Errorf("asked for %q", p)
		}
	}
	if n != 1 {
		t.Errorf("img/a.png copied %d times, want once", n)
	}
	x := read(t, out, "docs/x.html")
	contains(t, "x.html", x, `src="../img/a.png"`, `src="../img/my%20pic.png"`, `alt="m"`)
	lacks(t, "x.html", x, "/_ws/", "secret")
	contains(t, "home", read(t, out, "README.html"), `src="img/a.png"`)
	if !hasWarn(rep, "docs/x.md", "missing.png") {
		t.Errorf("no warning for the missing image: %v", rep.Warnings)
	}
	// Nothing was written next to the output folder.
	for _, name := range keys(listing(t, base)) {
		if !strings.HasPrefix(name, "out/") {
			t.Errorf("wrote %s outside the output", name)
		}
	}
	if has(base, "secret.png") || has(base, "b.png") {
		t.Error("a file outside the workspace was copied")
	}
}

func TestPluginAssets(t *testing.T) {
	f := newFake()
	f.page("p1.md", "P1", `<div data-zen-plugin="hello">x</div>`)
	f.page("sub/p2.md", "P2", `<p>none</p>`)
	f.page("sub/p3.md", "P3", `<div data-zen-layout="memo">x</div>`)
	f.with("p1.md", func(p *Page) {
		p.CSS = []string{"/_plugins/hello/hello.css?v=abc"}
		p.JS = []string{"/_plugins/hello/hello.js?v=abc"}
	})
	f.with("sub/p3.md", func(p *Page) { p.Layout = "memo"; p.CSS = []string{"/_layouts/memo/memo.css?v=1"} })
	f.assets["/_plugins/hello/hello.css"] = ".hello{}"
	f.assets["/_plugins/hello/hello.js"] = "export default {}"
	f.assets["/_plugins/hello/secret.txt"] = "not listed"
	f.assets["/_layouts/memo/memo.css"] = ".memo{}"
	f.assets["/_layouts/memo/layout.html"] = "<template>"
	rep, out := build(t, f, Options{})

	for rel, want := range map[string]string{
		"_zen/plugins/hello/hello.css": ".hello{}", "_zen/plugins/hello/hello.js": "export default {}",
		"_zen/layouts/memo/memo.css": ".memo{}", "_zen/zen.css": "/*zen.css*/", "_zen/zen.js": "/*zen.js*/",
	} {
		if read(t, out, rel) != want {
			t.Errorf("%s = %q", rel, read(t, out, rel))
		}
	}
	for _, rel := range []string{"_zen/plugins/hello/secret.txt", "_zen/layouts/memo/layout.html"} {
		if has(out, rel) {
			t.Errorf("%s was copied", rel)
		}
	}
	p1, p2, p3 := read(t, out, "p1.html"), read(t, out, "sub/p2.html"), read(t, out, "sub/p3.html")
	contains(t, "p1", p1, `<link rel="stylesheet" href="_zen/zen.css">`, `href="_zen/plugins/hello/hello.css"`, `<script src="_zen/zen.js" defer></script>`,
		`<script type="application/json" id="zen-page">`, `"path":"p1.md"`, `"siteId":"site"`, `"root":""`, `/_plugins/hello/hello.js`)
	lacks(t, "p1", p1, "?v=")
	lacks(t, "p2", p2, "hello", "memo", "<script")
	contains(t, "p2", p2, `href="../_zen/zen.css"`)
	contains(t, "p3", p3, `href="../_zen/layouts/memo/memo.css"`)
	lacks(t, "p3", p3, "hello.css")
	if len(rep.Warnings) != 0 {
		t.Errorf("warnings: %v", rep.Warnings)
	}
	for _, u := range f.assAsked {
		if strings.Contains(u, "secret") || strings.Contains(u, "layout.html") {
			t.Errorf("asked for %s", u)
		}
	}

	// Bad URLs and missing files are warnings, never a path outside _zen.
	g := newFake().page("a.md", "A", "<p>a</p>").with("a.md", func(p *Page) {
		p.CSS = []string{"/_plugins/../x.css", "/_plugins/p/../../y.css", "/_plugins/p/missing.css", "/elsewhere/z.css"}
	})
	rep, out = build(t, g, Options{})
	if len(rep.Warnings) < 3 {
		t.Errorf("warnings: %v", rep.Warnings)
	}
	for name := range listing(t, out) {
		if strings.Contains(name, "..") || strings.HasSuffix(name, ".css") && !strings.HasPrefix(name, "_zen/") {
			t.Errorf("stray file %s", name)
		}
	}
}

// listingFake also lists every file it serves for a plugin or layout, as the
// app does for built-ins.
type listingFake struct{ *fake }

func (l listingFake) AssetFiles(kind, id string) []string {
	var out []string
	prefix := "/_" + kind + "/" + id + "/"
	for u := range l.assets {
		if strings.HasPrefix(u, prefix) {
			out = append(out, strings.TrimPrefix(u, prefix))
		}
	}
	sort.Strings(out)
	return out
}

func TestPluginExtraFiles(t *testing.T) {
	for _, portable := range []bool{false, true} {
		f := newFake()
		f.page("d.md", "D", `<div data-zen-plugin="diagram">x</div>`)
		f.page("plain.md", "P", `<p>none</p>`)
		f.with("d.md", func(p *Page) {
			p.CSS = []string{"/_plugins/diagram/diagram.css?v=1"}
			p.JS = []string{"/_plugins/diagram/diagram.js?v=1"}
		})
		f.assets["/_plugins/diagram/diagram.css"] = ".d{}"
		f.assets["/_plugins/diagram/diagram.js"] = "export default {}"
		f.assets["/_plugins/diagram/mermaid.min.js"] = "var mermaid"
		f.assets["/_plugins/unused/x.js"] = "unused"
		_, out := build(t, listingFake{f}, Options{Portable: portable})
		if read(t, out, "_zen/plugins/diagram/mermaid.min.js") != "var mermaid" {
			t.Errorf("portable=%v: the unlisted file the plugin serves was not copied", portable)
		}
		if has(out, "_zen/plugins/unused/x.js") {
			t.Errorf("portable=%v: a plugin no page uses was copied", portable)
		}
		if got := has(out, "_zen/plugins/diagram/diagram.js"); got == portable {
			t.Errorf("portable=%v: page script copied = %v", portable, got)
		}
	}
}
