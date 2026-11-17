package render

import (
	"encoding/json"
	"html"
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dragosh/zen/internal/core/plugins"
)

// owner owns names and ships assets but has no hooks: zen renders the
// default content and the plugin's frontend takes over.
type owner struct{ m plugins.Manifest }

func (o owner) Manifest() plugins.Manifest { return o.m }
func (o owner) Assets() fs.FS {
	return fstest.MapFS{"o.js": {Data: []byte("1")}, "o.css": {Data: []byte("a{}")}}
}
func (o owner) Entry() plugins.Assets {
	return plugins.Assets{JS: []string{"o.js"}, CSS: []string{"o.css"}}
}

// hooked renders blocks and inlines itself and counts the calls.
type hooked struct {
	m     plugins.Manifest
	calls *int
}

func (h hooked) Manifest() plugins.Manifest { return h.m }
func (h hooked) RenderBlock(_ plugins.Context, b plugins.Block) (string, error) {
	*h.calls++
	return "<em>block " + b.Name + "</em>", nil
}
func (h hooked) RenderInline(_ plugins.Context, i plugins.Inline) (string, error) {
	*h.calls++
	return "<em>inline " + i.Name + "</em>", nil
}

// undeclared has the methods of every hook but declares none (as an external
// plugin without a backend does).
type undeclared struct{ hooked }

func (undeclared) HasHook(string) bool { return false }

func xm(id string, blocks, inlines, _ []string) plugins.Manifest {
	return plugins.Manifest{ID: id, Version: "1.0.0", APIVersion: plugins.APIVersion, Default: true,
		Blocks: blocks, Inlines: inlines}
}

func xrender(t *testing.T, src string, ps ...plugins.Plugin) Page {
	t.Helper()
	reg := plugins.NewRegistry()
	for _, p := range ps {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	pg, err := New(reg, nil, nil).Render("page.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return pg
}

var wrapperRE = regexp.MustCompile(`<(div|span)( data-zen-[a-z]+="[^"]*")+>`)

// wrappers returns every plugin wrapper tag as its attributes (unescaped).
func wrappers(htmlText string) []map[string]string {
	var out []map[string]string
	for _, tag := range wrapperRE.FindAllString(htmlText, -1) {
		attrs := map[string]string{"tag": tag[1:strings.IndexByte(tag, ' ')]}
		for _, m := range regexp.MustCompile(`(data-zen-[a-z]+)="([^"]*)"`).FindAllStringSubmatch(tag, -1) {
			attrs[m[1]] = html.UnescapeString(m[2])
		}
		out = append(out, attrs)
	}
	return out
}

func TestWrapperNameAndAttrs(t *testing.T) {
	n := 0
	h := hooked{xm("quiz", []string{"quiz", "poll"}, []string{"badge"}, nil), &n}
	pg := xrender(t, "```quiz {id=q1 title=\"Two <words>\"}\nx\n```\n\n:::poll\ny\n:::\n\nA :badge[b]{color=gray} here.\n", h)
	ws := wrappers(pg.HTML)
	if len(ws) != 3 {
		t.Fatalf("want 3 wrappers, got %v in %s", ws, pg.HTML)
	}
	byName := map[string]map[string]string{}
	for _, w := range ws {
		byName[w["data-zen-name"]] = w
		if w["data-zen-plugin"] != "quiz" {
			t.Errorf("plugin id: %v", w)
		}
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(byName["quiz"]["data-zen-attrs"]), &attrs); err != nil || attrs["id"] != "q1" || attrs["title"] != "Two <words>" || len(attrs) != 2 {
		t.Errorf("fence attrs %q: %v", byName["quiz"]["data-zen-attrs"], err)
	}
	if byName["quiz"]["tag"] != "div" || byName["poll"]["tag"] != "div" || byName["badge"]["tag"] != "span" {
		t.Errorf("tags: %v", ws)
	}
	if _, has := byName["poll"]["data-zen-attrs"]; has {
		t.Error("data-zen-attrs must be omitted without attributes")
	}
	if byName["badge"]["data-zen-attrs"] != `{"color":"gray"}` {
		t.Errorf("inline attrs: %q", byName["badge"]["data-zen-attrs"])
	}
	if strings.Contains(pg.HTML, "<words>") {
		t.Errorf("attribute value not escaped: %s", pg.HTML)
	}
	if !strings.Contains(pg.HTML, "<em>block quiz</em>") || !strings.Contains(pg.HTML, "<em>inline badge</em>") {
		t.Errorf("hook output missing: %s", pg.HTML)
	}
}

func TestDefaultContent(t *testing.T) {
	n := 0
	o := owner{xm("own", []string{"card", "raw"}, []string{"kbd"}, nil)}
	h := hooked{xm("quiz", []string{"quiz"}, []string{"badge"}, nil), &n}
	src := ":::card{title=T}\nSome **bold** text and :kbd[<Ctrl>] and :badge[x].\n\n" +
		":::quiz\ninner\n:::\n\n:::card\n*nested card*\n:::\n\n```raw\nfence <in> card\n```\n:::\n"
	pg := xrender(t, src, o, h)
	for _, want := range []string{
		"<strong>bold</strong>", // container body is markdown
		"<em>block quiz</em>",   // nested plugin block ran
		"<em>nested card</em>",  // nested default content, emphasis rendered
		"<em>inline badge</em>", // inline plugin inside the body
		`<pre class="zen-plugin-source">fence &lt;in&gt; card` + "\n</pre>", // fence body as text
		"&lt;Ctrl&gt;", // inline text escaped
	} {
		if !strings.Contains(pg.HTML, want) {
			t.Errorf("missing %q in\n%s", want, pg.HTML)
		}
	}
	names := map[string]int{}
	for _, w := range wrappers(pg.HTML) {
		names[w["data-zen-plugin"]+"/"+w["data-zen-name"]]++
	}
	if names["own/card"] != 2 || names["own/raw"] != 1 || names["own/kbd"] != 1 || names["quiz/quiz"] != 1 || names["quiz/badge"] != 1 {
		t.Errorf("wrappers: %v", names)
	}
	if len(pg.Warnings) != 0 {
		t.Errorf("warnings: %v", pg.Warnings)
	}
	// Default content counts as use: the plugin's assets load.
	if strings.Join(pg.Assets.JS, ",") != "/_plugins/own/o.js" || strings.Join(pg.Assets.CSS, ",") != "/_plugins/own/o.css" {
		t.Errorf("assets: %+v", pg.Assets)
	}
	// The outer card is balanced: its closing tag comes after the nested blocks.
	if strings.Count(pg.HTML, "<div") != strings.Count(pg.HTML, "</div>") {
		t.Errorf("unbalanced: %s", pg.HTML)
	}

	// A plugin that has the methods but does not declare the hook gets
	// default content too, and its hook is not called.
	calls := 0
	u := undeclared{hooked{xm("ext", []string{"box"}, []string{"tag"}, nil), &calls}}
	pg = xrender(t, ":::box\n**md**\n:::\n\n:tag[t]\n", u)
	if calls != 0 || !strings.Contains(pg.HTML, "<strong>md</strong>") || strings.Contains(pg.HTML, "<em>") {
		t.Errorf("undeclared hook: calls=%d html=%s", calls, pg.HTML)
	}
}

func TestDefaultContentDepthLimit(t *testing.T) {
	o := owner{xm("own", []string{"box"}, nil, nil)}
	const levels = 12
	src := strings.Repeat(":::box\n", levels) + "core\n" + strings.Repeat(":::\n", levels)
	pg := xrender(t, src, o)
	if got := len(wrappers(pg.HTML)); got != 8 {
		t.Fatalf("want 8 nested wrappers, got %d:\n%s", got, pg.HTML)
	}
	if !strings.Contains(pg.HTML, "core") {
		t.Error("content beyond the limit must stay visible as text")
	}
	found := false
	for _, w := range pg.Warnings {
		found = found || strings.Contains(w, "nested")
	}
	if !found {
		t.Errorf("no warning about nesting: %v", pg.Warnings)
	}
	// Within the limit there is no warning.
	src = strings.Repeat(":::box\n", 8) + "core\n" + strings.Repeat(":::\n", 8)
	if pg := xrender(t, src, o); len(pg.Warnings) != 0 || len(wrappers(pg.HTML)) != 8 {
		t.Errorf("8 levels: %v", pg.Warnings)
	}
}

func TestSanitiserKeepsPluginMarkup(t *testing.T) {
	// Hand-written wrappers go through the same rules.
	pg := xrender(t, `<div data-zen-plugin="a" data-zen-name="ok-name" data-zen-layout="ok" data-zen-attrs="{&quot;a&quot;:&quot;&lt;b&gt;&quot;}">x</div>`+"\n\n"+
		`<div data-zen-name="Bad Name" data-zen-layout="x y" data-zen-attrs="&quot;><script>alert(1)</script>">y</div>`+"\n\n"+
		`<span data-zen-name="n" data-zen-attrs="{}">z</span>`+"\n\n"+
		`<p data-zen-name="n" data-zen-layout="l" data-zen-attrs="{}">p</p>`+"\n\n"+
		`<pre class="zen-plugin-source">s</pre><pre class="zen-plugin-output">o</pre>`+"\n")
	for _, want := range []string{
		`data-zen-name="ok-name"`, `data-zen-layout="ok"`, `<span data-zen-name="n" data-zen-attrs="{}">`,
		`<pre class="zen-plugin-source">s</pre>`, `<pre class="zen-plugin-output">o</pre>`, "<p>p</p>",
	} {
		if !strings.Contains(pg.HTML, want) {
			t.Errorf("missing %q in\n%s", want, pg.HTML)
		}
	}
	for _, bad := range []string{"Bad Name", `"x y"`, "<script"} {
		if strings.Contains(pg.HTML, bad) {
			t.Errorf("%q survived in\n%s", bad, pg.HTML)
		}
	}
	ws := wrappers(pg.HTML)
	if len(ws) < 1 || ws[0]["data-zen-attrs"] != `{"a":"<b>"}` {
		t.Errorf("attrs text must survive, escaped: %v", ws)
	}
}

type loud struct{ m plugins.Manifest }

func (l loud) Manifest() plugins.Manifest { return l.m }
func (l loud) RenderBlock(plugins.Context, plugins.Block) (string, error) {
	return `<div class="c" data-x="1" onclick="steal()" style="color:red">` +
		`<script>alert(1)</script><iframe src="https://example.com"></iframe>` +
		`<progress value="3" max="5" onmouseover="x()"></progress><form action="/x"><input name="a"></form></div>`, nil
}

func TestPluginAllowRulesKeepHardDenials(t *testing.T) {
	m := xm("loud", []string{"loud"}, nil, nil)
	m.Allow = []plugins.AllowRule{
		{Element: "div", Attrs: []string{"data-x", "onclick", "style"}},
		{Element: "progress", Attrs: []string{"value", "max", "onmouseover"}},
		{Element: "script", Attrs: []string{"src"}},
		{Element: "iframe", Attrs: []string{"src", "srcdoc"}},
		{Element: "form", Attrs: []string{"action"}},
	}
	pg := xrender(t, "```loud\n```\n", loud{m})
	for _, want := range []string{`data-x="1"`, `<progress value="3" max="5">`} {
		if !strings.Contains(pg.HTML, want) {
			t.Errorf("allowed markup %q dropped: %s", want, pg.HTML)
		}
	}
	for _, bad := range []string{"<script", "<iframe", "<form", "onclick", "onmouseover", "style=", "alert(1)"} {
		if strings.Contains(pg.HTML, bad) {
			t.Errorf("%q survived: %s", bad, pg.HTML)
		}
	}
}

func TestDeniedPluginRendersPlain(t *testing.T) {
	reg := plugins.NewRegistry()
	reg.Deny(xm("x", []string{"secret"}, []string{"sec"}, nil))
	src := "---\nplugins: { x: true }\n---\n:::secret\nplain **md**\n:::\n\n```secret\ncode\n```\n\nText :sec[t] and :sec[u].\n"
	pg, err := New(reg, nil, nil).Render("p.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	const want = `plugin "x" is not allowed (Settings › Plugins)`
	if len(pg.Warnings) != 1 || pg.Warnings[0] != want {
		t.Fatalf("warnings: %q", pg.Warnings)
	}
	if len(wrappers(pg.HTML)) != 0 {
		t.Errorf("a denied plugin was used: %s", pg.HTML)
	}
	for _, plain := range []string{"<strong>md</strong>", `<code class="language-secret">code`, ":sec[t]"} {
		if !strings.Contains(pg.HTML, plain) {
			t.Errorf("missing plain %q in %s", plain, pg.HTML)
		}
	}
	// Pages that do not use its names carry no warning.
	pg, _ = New(reg, nil, nil).Render("p.md", []byte("# Nothing here\n\n`:sec[x]` in code\n"))
	if len(pg.Warnings) != 0 {
		t.Errorf("warnings on an unrelated page: %q", pg.Warnings)
	}
}

type slowSource struct {
	m     plugins.Manifest
	calls *int
}

func (s slowSource) Manifest() plugins.Manifest { return s.m }
func (s slowSource) External() bool             { return true }
func (s slowSource) Source(_ plugins.Context, src []byte) ([]byte, error) {
	*s.calls++
	return src, nil
}
func (s slowSource) HTML(_ plugins.Context, h string) (string, error) {
	*s.calls++
	return h, nil
}

func TestTitleRunsNoBackends(t *testing.T) {
	blocks, ext := 0, 0
	reg := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{
		hooked{xm("quiz", []string{"quiz"}, []string{"badge"}, nil), &blocks},
		slowSource{xm("ext", nil, nil, nil), &ext},
	} {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	r := New(reg, nil, nil)
	src := []byte(":::quiz\n# Not the title\n:::\n\n```quiz\nx\n```\n\n# Real :badge[title] here\n")
	if got := r.Title("dir/page.md", src); got != "Real title here" {
		t.Fatalf("title %q", got)
	}
	if blocks != 0 || ext != 0 {
		t.Fatalf("Title ran hooks: blocks=%d external=%d", blocks, ext)
	}
	if got := r.Title("dir/page.md", []byte("---\ntitle: From meta\n---\n# H\n")); got != "From meta" {
		t.Fatalf("frontmatter title %q", got)
	}
	if got := r.Title("dir/page.md", []byte("no heading\n")); got != "page" {
		t.Fatalf("fallback title %q", got)
	}
	// Same answer as a full render.
	pg, _ := r.Render("dir/page.md", src)
	if pg.Title != "Real title here" {
		t.Fatalf("render title %q", pg.Title)
	}
}
