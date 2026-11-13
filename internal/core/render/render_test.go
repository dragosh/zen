package render

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/workspace"
)

// fakeResolver resolves by a fixed table; unknown hrefs are broken.
type fakeResolver map[string]workspace.Link

func (f fakeResolver) ResolveLink(from, href string) workspace.Link {
	if l, ok := f[href]; ok {
		return l
	}
	if strings.Contains(href, ":") {
		return workspace.Link{Path: href, Kind: "external"}
	}
	return workspace.Link{Path: href, Kind: "broken"}
}

// fakePlugin implements every hook through optional funcs.
type fakePlugin struct {
	m      plugins.Manifest
	block  func(plugins.Context, plugins.Block) (string, error)
	inline func(plugins.Context, plugins.Inline) (string, error)
}

func (p *fakePlugin) Manifest() plugins.Manifest { return p.m }
func (p *fakePlugin) RenderBlock(c plugins.Context, b plugins.Block) (string, error) {
	if p.block == nil {
		return "", errors.New("no block")
	}
	return p.block(c, b)
}
func (p *fakePlugin) RenderInline(c plugins.Context, i plugins.Inline) (string, error) {
	if p.inline == nil {
		return "", errors.New("no inline")
	}
	return p.inline(c, i)
}

type sourcePlugin struct {
	m plugins.Manifest
	f func([]byte) ([]byte, error)
}

func (p *sourcePlugin) Manifest() plugins.Manifest                         { return p.m }
func (p *sourcePlugin) Source(_ plugins.Context, b []byte) ([]byte, error) { return p.f(b) }

type htmlPlugin struct {
	m plugins.Manifest
	f func(string) (string, error)
}

func (p *htmlPlugin) Manifest() plugins.Manifest                       { return p.m }
func (p *htmlPlugin) HTML(_ plugins.Context, s string) (string, error) { return p.f(s) }

type assetPlugin struct {
	fakePlugin
}

func (p *assetPlugin) Assets() fs.FS { return fstest.MapFS{} }
func (p *assetPlugin) Entry() plugins.Assets {
	return plugins.Assets{CSS: []string{"a.css"}, JS: []string{"a.js"}}
}

func man(id string, def bool) plugins.Manifest {
	return plugins.Manifest{ID: id, Version: "1.0.0", APIVersion: plugins.APIVersion, Default: def}
}

func newReg(t *testing.T, ps ...plugins.Plugin) *plugins.Registry {
	t.Helper()
	reg := plugins.NewRegistry()
	for _, p := range ps {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	return reg
}

func render(t *testing.T, r *Renderer, src string) Page {
	t.Helper()
	p, err := r.Render("docs/page.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func contains(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in %q", w, got)
		}
	}
}

func lacks(t *testing.T, got string, deny ...string) {
	t.Helper()
	for _, d := range deny {
		if strings.Contains(got, d) {
			t.Errorf("unexpected %q in %q", d, got)
		}
	}
}

func plain() *Renderer { return New(plugins.NewRegistry(), config.Default(), fakeResolver{}) }

func TestTitleAndFrontmatter(t *testing.T) {
	r := plain()
	p := render(t, r, "---\ntitle: From FM\ntags: [a]\n---\n# Heading\n")
	if p.Title != "From FM" {
		t.Errorf("title %q", p.Title)
	}
	if p.Frontmatter["title"] != "From FM" {
		t.Errorf("frontmatter %v", p.Frontmatter)
	}
	lacks(t, p.HTML, "tags", "---")
	if p.Path != "docs/page.md" {
		t.Errorf("path %q", p.Path)
	}

	p = render(t, r, "text\n\n# The *Big* One\n\n# Second\n")
	if p.Title != "The Big One" {
		t.Errorf("h1 title %q", p.Title)
	}
	p = render(t, r, "## only h2\n")
	if p.Title != "page" {
		t.Errorf("file title %q", p.Title)
	}
	p = render(t, r, "")
	if p.Title != "page" || p.Frontmatter == nil {
		t.Errorf("empty: %+v", p)
	}
	p = render(t, r, "---\ntitle: Only\n---\n")
	if p.Title != "Only" || strings.TrimSpace(p.HTML) != "" {
		t.Errorf("only frontmatter: %+v", p)
	}
}

func TestMarkdown(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
	}{
		{"heading", "# Title", []string{`<h1 id="title">Title</h1>`}},
		{"bold", "**b**", []string{"<strong>b</strong>"}},
		{"table", "|a|b|\n|-|-|\n|1|2|", []string{"<table>", "<td>1</td>"}},
		{"tasks", "- [x] done", []string{`type="checkbox"`, "checked"}},
		{"strike", "~~x~~", []string{"<del>x</del>"}},
		{"autolink", "see https://example.com now", []string{`href="https://example.com"`}},
		{"footnote", "a[^1]\n\n[^1]: note", []string{`class="footnotes"`, "note"}},
		{"code class", "```go\nx\n```", []string{`class="language-go"`}},
		{"dup ids", "# A\n# A", []string{`id="a"`, `id="a-1"`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			contains(t, render(t, plain(), c.in).HTML, c.want...)
		})
	}
}

func blockPlugin(id string, def bool, blocks, inlines []string) *fakePlugin {
	m := man(id, def)
	m.Blocks, m.Inlines = blocks, inlines
	return &fakePlugin{
		m: m,
		block: func(c plugins.Context, b plugins.Block) (string, error) {
			return fmt.Sprintf(`<b class="blk">%s|%v|%s|%s</b>`, b.Name, b.Fenced, b.Attrs["k"], strings.TrimSpace(b.Body)), nil
		},
		inline: func(c plugins.Context, i plugins.Inline) (string, error) {
			return fmt.Sprintf(`<i class="inl">%s|%s|%s</i>`, i.Name, i.Text, i.Attrs["k"]), nil
		},
	}
}

func TestPluginBlocks(t *testing.T) {
	reg := newReg(t,
		blockPlugin("demo", true, []string{"demo", "box"}, []string{"badge"}),
		blockPlugin("off", false, []string{"offb"}, []string{"offi"}),
	)
	r := New(reg, config.Default(), fakeResolver{})

	p := render(t, r, "```demo {k=v}\nraw *text*\n```\n")
	contains(t, p.HTML, `data-zen-plugin="demo"`, `demo|true|v|raw *text*`)

	p = render(t, r, "intro\n:::box{k=c}\nhello\n:::\nafter\n")
	contains(t, p.HTML, `data-zen-plugin="demo"`, `box|false|c|hello`, "after")

	p = render(t, r, "Status: :badge[stable]{k=g} ok\n")
	contains(t, p.HTML, `<span data-zen-plugin="demo">`, `badge|stable|g`, "Status:", "ok")

	// disabled plugin: plain code block and literal text
	p = render(t, r, "```offb\nx\n```\n\n:offi[t] and :::offb\n")
	contains(t, p.HTML, "<pre><code", ":offi[t]")
	lacks(t, p.HTML, "data-zen-plugin")

	// enabled via frontmatter
	p = render(t, r, "---\nplugins:\n  off: true\n---\n:offi[t]\n")
	contains(t, p.HTML, `data-zen-plugin="off"`)

	// nested containers: outer owned container receives nested markdown
	p = render(t, r, ":::box\nouter\n:::box\ninner\n:::\n:::\n")
	if strings.Count(p.HTML, `data-zen-plugin="demo"`) != 1 {
		t.Errorf("nested: %q", p.HTML)
	}
	contains(t, p.HTML, "inner")

	// directive inside code block stays literal
	p = render(t, r, "```\n:badge[x]\n:::box\ny\n:::\n```\n\n`:badge[z]`\n")
	lacks(t, p.HTML, "data-zen-plugin")
	contains(t, p.HTML, ":badge[x]", ":badge[z]")
}

func TestPluginFailureIsContained(t *testing.T) {
	m := man("bad", true)
	m.Blocks, m.Inlines = []string{"boom", "fail"}, []string{"oops"}
	bad := &fakePlugin{m: m,
		block: func(_ plugins.Context, b plugins.Block) (string, error) {
			if b.Name == "boom" {
				panic("kaboom")
			}
			return "", errors.New("it <failed>")
		},
		inline: func(plugins.Context, plugins.Inline) (string, error) { panic("inline") },
	}
	r := New(newReg(t, bad), config.Default(), fakeResolver{})
	p := render(t, r, "```fail\nx\n```\n\n```boom\nx\n```\n\nhi :oops[x]\n\n# Still here\n")
	if n := strings.Count(p.HTML, `class="zen-plugin-error"`); n != 3 {
		t.Errorf("want 3 error boxes, got %d: %q", n, p.HTML)
	}
	contains(t, p.HTML, "it &lt;failed&gt;", "Still here")
	if len(p.Warnings) == 0 {
		t.Error("expected warnings for failing hooks")
	}
}

func TestHookOrder(t *testing.T) {
	var log []string
	var mu sync.Mutex
	add := func(s string) { mu.Lock(); log = append(log, s); mu.Unlock() }
	reg := newReg(t,
		&sourcePlugin{man("s1", true), func(b []byte) ([]byte, error) { add("s1"); return append(b, []byte(" one")...), nil }},
		&htmlPlugin{man("h1", true), func(s string) (string, error) { add("h1"); return s + "<p>h1</p>", nil }},
		&sourcePlugin{man("s2", true), func(b []byte) ([]byte, error) { add("s2"); return append(b, []byte(" two")...), nil }},
		&htmlPlugin{man("h2", true), func(s string) (string, error) {
			add("h2")
			if !strings.Contains(s, "<p>h1</p>") {
				t.Error("h2 ran before h1")
			}
			return s + `<p>h2<script>x()</script></p>`, nil
		}},
		&sourcePlugin{man("s3", true), func(b []byte) ([]byte, error) { add("s3"); return nil, errors.New("bad") }},
	)
	p := render(t, New(reg, config.Default(), fakeResolver{}), "start")
	if got := strings.Join(log, ","); got != "s1,s2,s3,h1,h2" {
		t.Errorf("order %s", got)
	}
	contains(t, p.HTML, "start one two", "<p>h1</p>", "h2")
	lacks(t, p.HTML, "<script")
}

func TestLinks(t *testing.T) {
	res := fakeResolver{
		"a.md#intro":   {Path: "docs/a.md", Anchor: "intro", Kind: "internal"},
		"https://x.io": {Path: "https://x.io", Kind: "external"},
		"img.png":      {Path: "docs/img.png", Kind: "asset"},
	}
	r := New(plugins.NewRegistry(), config.Default(), res)
	p := render(t, r, "[in](a.md#intro) [ex](https://x.io) [no](missing.md) [here](#top) [js](javascript:alert(1)) <https://x.io>")
	contains(t, p.HTML,
		`href="#" data-zen-link="internal" data-path="docs/a.md" data-anchor="intro">in</a>`,
		`href="https://x.io" data-zen-link="external" rel="noopener noreferrer">ex</a>`,
		`<a data-zen-link="broken">no</a>`,
		`<a href="#top">here</a>`,
	)
	lacks(t, p.HTML, "javascript:", "nofollow")

	links := r.Links("docs/page.md", []byte("---\ntitle: x\n---\n[in](a.md#intro) ![i](img.png) [no](missing.md)"))
	if len(links) != 3 || links[0].Path != "docs/a.md" || links[1].Kind != "asset" || links[2].Kind != "broken" {
		t.Errorf("links %+v", links)
	}
}

func TestImages(t *testing.T) {
	res := fakeResolver{
		"pics/a b.png": {Path: "docs/pics/a b.png", Kind: "asset"},
	}
	r := New(plugins.NewRegistry(), config.Default(), res)
	p := render(t, r, "![one](pics/a%20b.png) ![two](https://e.com/t.png) ![three](http://e.com/t.png) ![four](file:///etc/passwd) ![five](data:image/png;base64,AAAA) ![six](nothere.png)")
	contains(t, p.HTML, `src="/_ws/docs/pics/a%20b.png"`, `alt="one"`, `src="https://e.com/t.png"`, "three", "four", "five", "six")
	lacks(t, p.HTML, "http://e.com", "file:", "data:", "nothere")
}

func TestSanitise(t *testing.T) {
	m := man("evil", true)
	m.Blocks = []string{"evil"}
	evil := &fakePlugin{m: m, block: func(plugins.Context, plugins.Block) (string, error) {
		return `<script>alert(1)</script><img src=x onerror=alert(2)><a href="javascript:alert(3)">x</a>`, nil
	}}
	r := New(newReg(t, evil), config.Default(), fakeResolver{})
	cases := []struct{ name, in string }{
		{"script", "<script>alert(1)</script>ok"},
		{"handler", `<img src="x" onerror="alert(1)">`},
		{"js link", "[x](javascript:alert(1))"},
		{"raw js link", `<a href="javascript:alert(1)">x</a>`},
		{"iframe", `<iframe src="https://e.com"></iframe>`},
		{"object", `<object data="x"></object>`},
		{"style", "<style>body{}</style><p style=\"color:red\">x</p>"},
		{"svg script", `<svg><script>alert(1)</script></svg>`},
		{"data url", `<a href="data:text/html,<script>alert(1)</script>">x</a><img src="data:image/svg+xml,x">`},
		{"plugin", "```evil\n```"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := render(t, r, c.in).HTML
			lacks(t, got, "<script", "onerror", "javascript:", "<iframe", "<object", "<style", "style=", "data:")
		})
	}
}

func TestAllowRules(t *testing.T) {
	mk := func(def bool) *Renderer {
		m := man("canvas", def)
		m.Blocks = []string{"canvas"}
		m.Allow = []plugins.AllowRule{{Element: "canvas", Attrs: []string{"width"}}}
		p := &fakePlugin{m: m, block: func(plugins.Context, plugins.Block) (string, error) {
			return `<canvas width="10"></canvas>`, nil
		}}
		return New(newReg(t, p), config.Default(), fakeResolver{})
	}
	contains(t, render(t, mk(true), "```canvas\n```").HTML, `<canvas width="10">`)
	lacks(t, render(t, mk(false), `<canvas width="10"></canvas>`).HTML, "<canvas")
	lacks(t, render(t, mk(true), "---\nplugins: {canvas: false}\n---\n<canvas width=\"10\"></canvas>").HTML, "<canvas")
}

func TestAssetsAndWarnings(t *testing.T) {
	ap := &assetPlugin{*blockPlugin("pretty", true, []string{"pretty"}, nil)}
	r := New(newReg(t, ap), config.Default(), fakeResolver{})
	p := render(t, r, "no plugin here")
	if len(p.Assets.CSS)+len(p.Assets.JS) != 0 {
		t.Errorf("unused assets: %+v", p.Assets)
	}
	p = render(t, r, "```pretty\nx\n```")
	if len(p.Assets.CSS) != 1 || p.Assets.CSS[0] != "/_plugins/pretty/a.css" || len(p.Assets.JS) != 1 {
		t.Errorf("assets: %+v", p.Assets)
	}
	p = render(t, r, "---\nplugins:\n  nope: true\n  pretty: {zzz: 1}\ntheme: 5\n---\nx")
	w := strings.Join(p.Warnings, "\n")
	contains(t, w, `"nope"`, `"zzz"`, "theme")
}

func TestConcurrent(t *testing.T) {
	r := New(newReg(t, blockPlugin("demo", true, []string{"demo"}, []string{"badge"})), config.Default(), fakeResolver{})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			src := fmt.Sprintf("# T%d\n\n```demo\n%d\n```\n\n:badge[%d]\n", i, i, i)
			p, err := r.Render("p.md", []byte(src))
			if err != nil || p.Title != fmt.Sprintf("T%d", i) || !strings.Contains(p.HTML, fmt.Sprintf("demo|true||%d", i)) {
				t.Errorf("render %d: %v %+v", i, err, p)
			}
		}(i)
	}
	wg.Wait()
}

func TestLargeFile(t *testing.T) {
	var b strings.Builder
	for b.Len() < 5<<20 {
		b.WriteString("## Section\n\nSome *text* with a [link](a.md) and `code`.\n\n- item\n- item\n\n")
	}
	r := plain()
	start := time.Now()
	render(t, r, b.String())
	if d := time.Since(start); d > 500*time.Millisecond && !testing.Short() {
		t.Logf("5 MB render took %v (limit 500ms; race detector inflates this)", d)
		if !raceEnabled && d > 500*time.Millisecond {
			t.Errorf("5 MB render took %v", d)
		}
	}
}

func TestHTMLHelper(t *testing.T) {
	got, err := HTML([]byte("# Hi\n<script>x</script>"))
	if err != nil {
		t.Fatal(err)
	}
	contains(t, got, `<h1 id="hi">Hi</h1>`)
	lacks(t, got, "<script")
}
