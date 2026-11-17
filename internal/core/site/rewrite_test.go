package site

import (
	"errors"
	"strings"
	"testing"
)

// rw is a rewriter for page docs/a.md that knows these pages and files.
func rw(edit string, pages ...string) (*rewriter, *[]string) {
	known := map[string]bool{"docs/a.md": true}
	for _, p := range pages {
		known[p] = true
	}
	var copied []string
	return &rewriter{
		from: "docs/a.md", fromOut: "docs/a.html", edit: edit,
		page: func(p string) (string, bool) {
			if !known[p] {
				return "", false
			}
			return outPath(p), true
		},
		slug: func(s string) (string, bool) {
			if s == "install" {
				return "docs/b.md", true
			}
			return "", false
		},
		file: func(p string) (string, error) {
			if strings.Contains(p, "missing") {
				return "", errors.New("not found")
			}
			copied = append(copied, p)
			return p, nil
		},
	}, &copied
}

func TestRewriteLinks(t *testing.T) {
	pages := []string{"docs/b.md", "guide/x.md", "a\"b.md", "README.md"}
	for _, c := range []struct {
		name, in string
		edit     string
		has      []string
		not      []string
	}{
		{"internal", `<a data-zen-link="internal" data-path="docs/b.md" href="#">B</a>`, "", []string{`href="b.html"`, ">B</a>"}, []string{`href="#"`}},
		{"internal in another folder with an anchor", `<a data-zen-link="internal" data-path="guide/x.md" data-anchor="intro" href="#">X</a>`, "", []string{`href="../guide/x.html#intro"`}, nil},
		{"root page", `<a data-zen-link="internal" data-path="README.md" href="#">H</a>`, "", []string{`href="../README.html"`}, nil},
		{"self link", `<a data-zen-link="internal" data-path="docs/a.md" data-anchor="s" href="#">me</a>`, "", []string{`href="a.html#s"`}, nil},
		{"slug", `<a data-zen-link="internal" data-path="install" href="#">I</a>`, "", []string{`href="b.html"`}, nil},
		{"anchor only", `<a href="#top">up</a>`, "", []string{`<a href="#top">up</a>`}, nil},
		{"external", `<a data-zen-link="external" href="https://x.org/a?b=1&amp;c=2" rel="noopener noreferrer">e</a>`, "", []string{`href="https://x.org/a?b=1&amp;c=2"`, `target="_blank"`, `rel="noopener noreferrer"`}, nil},
		{"external without rel", `<a data-zen-link="external" href="mailto:a@b.c">m</a>`, "", []string{`rel="noopener noreferrer"`, `target="_blank"`}, nil},
		{"broken", `<a data-zen-link="broken">gone</a>`, "", []string{`data-zen-link="broken"`, ">gone</a>"}, []string{`href=`}},
		{"internal to a missing page is broken", `<a data-zen-link="internal" data-path="nope.md" href="#">n</a>`, "", []string{`data-zen-link="broken"`}, []string{`href=`, `nope`}},
		{"edit", `<a data-zen-action="edit">Edit this page</a>`, "https://github.com/me/notes/edit/main/{path}", []string{`href="https://github.com/me/notes/edit/main/docs/a.md"`, `target="_blank"`, "Edit this page"}, nil},
		{"edit without editURL", `<p>x <a data-zen-action="edit">Edit this page</a> y</p>`, "", []string{"<p>x  y</p>"}, []string{"Edit this", "data-zen-action"}},
		{"plain markup is untouched", `<p class="x">hi &amp; <b>there</b> &lt;3</p>`, "", []string{`<p class="x">hi &amp; <b>there</b> &lt;3</p>`}, nil},
		{"link with a quote in the page name", `<a data-zen-link="internal" data-path="a&#34;b.md" href="#">q</a>`, "", []string{`href="a%22b.html"`}, []string{`"b.html"`}},
		// Hostile input: nothing becomes markup or script.
		{"hostile data-path", `<a data-zen-link="internal" data-path="x&#34; onclick=&#34;alert(1)" href="#">h</a>`, "", nil, []string{"onclick", "alert"}},
		{"hostile data-anchor", `<a data-zen-link="internal" data-path="docs/b.md" data-anchor="&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;" href="#">h</a>`, "", nil, []string{"<script", `onclick`, `"><`}},
		{"javascript href", `<a href="javascript:alert(1)">j</a>`, "", []string{">j</a>"}, []string{"javascript", "alert"}},
		{"javascript href disguised", `<a href="  Java&#10;Script:alert(1)">j</a>`, "", nil, []string{"alert"}},
		{"javascript external", `<a data-zen-link="external" href="javascript:alert(1)">j</a>`, "", nil, []string{"alert", "href=", `data-zen-link="external"`}},
		{"data href", `<a href="data:text/html,<script>alert(1)</script>">d</a>`, "", nil, []string{"alert", "data:"}},
		{"hostile edit url", `<a data-zen-action="edit">e</a>`, `https://e.org/?q="><script>alert(1)</script>&p={path}`, []string{`href="https://e.org/?q=&#34;&gt;&lt;script&gt;`}, []string{`"><script>`}},
		{"edit path with quotes", `<a data-zen-action="edit">e</a>`, `https://e.org/{path}`, nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, _ := rw(c.edit, pages...)
			got := r.html(c.in)
			contains(t, "output", got, c.has...)
			lacks(t, "output", got, c.not...)
		})
	}

	// Page names that need escaping in the edit URL.
	r, _ := rw("https://e.org/{path}")
	r.from = `we ird/"q".md`
	got := r.html(`<a data-zen-action="edit">e</a>`)
	contains(t, "edit url", got, `https://e.org/we%20ird/%22q%22.md`)
}

func TestRewriteWorkspaceURLs(t *testing.T) {
	r, copied := rw("")
	got := r.html(`<img src="/_ws/img/a.png" alt="a"><img src="/_ws/img/my%20pic.png"><img src="/_ws/missing.png" alt="m"><img src="/_ws/../secret.png"><img src="/_ws/%2e%2e/secret.png"><a href="/_ws/docs/f.svg#x">f</a>`)
	contains(t, "html", got, `src="../img/a.png"`, `src="../img/my%20pic.png"`, `href="../docs/f.svg#x"`, `alt="m"`)
	lacks(t, "html", got, "/_ws/", "secret")
	if strings.Join(*copied, ",") != "img/a.png,img/my pic.png,docs/f.svg" {
		t.Errorf("copied %v", *copied)
	}
	if len(r.warns) != 3 { // missing, and the two attempts to leave the folder
		t.Errorf("warnings %v", r.warns)
	}
}
