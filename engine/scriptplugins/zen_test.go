package scriptplugins_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dragosh/zenmd/engine/plugins"
	"github.com/dragosh/zenmd/engine/scriptplugins"
)

// expr renders `out = <expr>` in a block hook.
func expr(t *testing.T, s *site, id, e string, rt ...scriptplugins.Runtime) (string, error) {
	t.Helper()
	p := s.plugin(id, withHeader("if hook == \"block\" { out = "+e+" }"), rt...)
	return p.RenderBlock(plugins.Context{Path: "a.md"}, plugins.Block{Name: "x", Body: "body"})
}

func TestHTMLEscapesTextAndAttributes(t *testing.T) {
	s := newSite(t)
	cases := []struct{ name, expr, want string }{
		{"text", `zen.html("p", {}, "<b>t</b> & 'q' \"d\"")`, `<p>&lt;b&gt;t&lt;/b&gt; &amp; &#39;q&#39; &#34;d&#34;</p>`},
		{"attr value", `zen.html("a", {href: "x\" onclick=\"y", title: "<&>'"}, "l")`, `<a href="x&#34; onclick=&#34;y" title="&lt;&amp;&gt;&#39;">l</a>`},
		{"bool attrs", `zen.html("input", {disabled: true, checked: false, value: 3})`, `<input disabled value="3">`},
		{"void", `zen.html("br", {})`, `<br>`},
		{"nested not escaped", `zen.html("div", {}, zen.html("b", {}, "x"), " & ", zen.html("i", {}, "y"))`, `<div><b>x</b> &amp; <i>y</i></div>`},
		{"array children", `zen.html("ul", {}, [zen.html("li", {}, "a"), zen.html("li", {}, "b")])`, `<ul><li>a</li><li>b</li></ul>`},
		{"spread", `zen.html("p", {}, ["a", "<"]...)`, `<p>a&lt;</p>`},
		{"numbers", `zen.html("p", {}, 1, 2.5, true)`, `<p>12.5true</p>`},
		{"no attrs map", `zen.html("p")`, `<p></p>`},
		{"escape", `zen.escape("<a href=\"x\">'&'</a>")`, `&lt;a href=&#34;x&#34;&gt;&#39;&amp;&#39;&lt;/a&gt;`},
		{"escape number", `zen.escape(5)`, `5`},
		{"escape of html result is text", `zen.escape(zen.html("b", {}, "x"))`, `&lt;b&gt;x&lt;/b&gt;`},
		{"a string is not trusted", `zen.html("p", {}, "<b>" + "x" + "</b>")`, `<p>&lt;b&gt;x&lt;/b&gt;</p>`},
	}
	for i, c := range cases {
		got, err := expr(t, s, sprintf("e%d", i), c.expr)
		if err != nil || got != c.want {
			t.Errorf("%s:\n got %q %v\nwant %q", c.name, got, err, c.want)
		}
	}
	for _, bad := range []string{
		`zen.html("a b", {})`, `zen.html("a>", {})`, `zen.html("", {})`, `zen.html(5, {})`,
		`zen.html("p", {"a b": "x"})`, `zen.html("p", {"x=y": "x"})`, `zen.html("p", {"\"": "x"})`,
		`zen.html("br", {}, "child")`, `zen.html("p", "notamap")`, `zen.escape()`, `zen.html()`,
	} {
		if _, err := expr(t, s, "bad", bad); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}

func TestMarkdownCallsTheRenderer(t *testing.T) {
	s := newSite(t)
	var got []string
	rt := scriptplugins.Runtime{}
	p := s.plugin("md", withHeader(`if hook == "block" { out = zen.html("div", {}, zen.markdown("**" + body + "**")) }`), rt)
	ctx := plugins.Context{Path: "a.md", Markdown: func(src string) (string, error) {
		got = append(got, src)
		return "<strong>" + strings.Trim(src, "*") + "</strong>", nil
	}}
	out, err := p.RenderBlock(ctx, plugins.Block{Name: "x", Body: "hi"})
	if err != nil || out != "<div><strong>hi</strong></div>" || len(got) != 1 || got[0] != "**hi**" {
		t.Errorf("markdown: %q %v %v", out, err, got)
	}
	// An error of the renderer (the depth limit) fails the call with its message.
	ctx.Markdown = func(string) (string, error) { return "", errors.New("containers nested too deep") }
	_, err = p.RenderBlock(ctx, plugins.Block{Name: "x"})
	errContains(t, err, "containers nested too deep")
	// Where there is no renderer, it says so.
	_, err = p.RenderBlock(plugins.Context{Path: "a.md"}, plugins.Block{Name: "x"})
	errContains(t, err, "zen.markdown")
}

func TestStorage(t *testing.T) {
	s := newSite(t)
	st := &memStore{}
	p := s.plugin("store", `plugin := {version: "1.0.0", blocks: ["x"], hooks: ["action"]}
if hook == "action" {
    zen.storage.set("count", (zen.storage.get("count") || 0) + 1)
    zen.storage.set("obj", {a: [1, 2], b: "x"})
    zen.storage.set("other", "v")
    out = {count: zen.storage.get("count"), obj: zen.storage.get("obj"), missing: zen.storage.get("nope"), keys: zen.storage.keys(""), some: zen.storage.keys("o")}
}`, scriptplugins.Runtime{Storage: st})
	for i := 0; i < 2; i++ {
		if _, err := p.Action(plugins.Context{}, []byte(`null`)); err != nil {
			t.Fatal(err)
		}
	}
	out, err := p.Action(plugins.Context{}, []byte(`null`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"count":3,"keys":["count","obj","other"],"missing":null,"obj":{"a":[1,2],"b":"x"},"some":["obj","other"]}`
	if string(out) != want {
		t.Errorf("storage:\n got %s\nwant %s", out, want)
	}
	if st.m["obj"] != `{"a":[1,2],"b":"x"}` || st.m["other"] != `"v"` {
		t.Errorf("stored values: %v", st.m)
	}
	// Errors of the store (a bad key, a damaged file) surface in the call.
	st.fail = errors.New("storage keys use letters, digits")
	_, err = p.Action(plugins.Context{}, []byte(`null`))
	errContains(t, err, "storage keys use letters")
	// No store bound: an error, not a crash.
	bare := s.plugin("bare", withHeader(`if hook == "block" { out = zen.storage.get("k") }`), scriptplugins.Runtime{})
	_, err = block(bare, "x", "", nil)
	errContains(t, err, "storage")
	// Values that are not JSON (a function) are refused.
	fn := s.plugin("fn", `plugin := {version: "1.0.0", blocks: ["x"]}
if hook == "block" { zen.storage.set("f", func() {}) }`, scriptplugins.Runtime{Storage: &memStore{}})
	if _, err := block(fn, "x", "", nil); err == nil {
		t.Error("a function was stored")
	}
}

// readSite is a folder with pages, text, binary files, links and a big file.
func readSite(t *testing.T) *site {
	s := newSite(t)
	s.file("README.md", "# Home\n\nhello world\n")
	s.file("docs/a.md", "---\ntitle: The A page\n---\n# A heading\n\nsome words here\n")
	s.file("docs/b.md", "# Page B\n")
	s.file("notes.txt", "plain notes\n")
	s.file("data/list.csv", "a,b\n1,2\n")
	s.file("img.png", "\x89PNG\r\n\x1a\n")
	s.file("bin.md", "ok\x00not text")
	s.file("latin.md", "caf\xe9\n")
	s.file("big.md", strings.Repeat("x", 1<<20))
	s.file("toobig.md", strings.Repeat("x", 1<<20+1))
	secret := filepath.Join(filepath.Dir(s.dir), "secret-"+filepath.Base(s.dir)+".md")
	if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(secret) })
	os.Symlink(secret, filepath.Join(s.dir, "leak.md"))
	os.Symlink("docs/a.md", filepath.Join(s.dir, "inside.md"))
	os.Symlink(filepath.Dir(s.dir), filepath.Join(s.dir, "up"))
	return s
}

const readerSrc = `plugin := {version: "1.0.0", blocks: ["x"], permissions: ["read"]}
if hook == "block" { out = zen.escape(zen.read(body) || "MISSING") }`

func TestReadNeedsThePermission(t *testing.T) {
	s := readSite(t)
	p := s.plugin("noperm", `plugin := {version: "1.0.0", blocks: ["x"]}
if hook == "block" { out = zen.read(body) }`)
	_, err := block(p, "x", "docs/a.md", nil)
	errContains(t, err, "read")
	errContains(t, err, "permission")
	q := s.plugin("nopages", `plugin := {version: "1.0.0", blocks: ["x"]}
if hook == "block" { out = string(len(zen.pages())) }`)
	_, err = block(q, "x", "", nil)
	errContains(t, err, "permission")
	// With the permission both work.
	r := s.plugin("reader", readerSrc)
	if got := mustBlock(t, r, "x", "docs/b.md", nil); got != "# Page B\n" {
		t.Errorf("read: %q", got)
	}
}

func TestReadReadsTextInTheFolderOnly(t *testing.T) {
	s := readSite(t)
	p := s.plugin("reader", readerSrc)
	for path, want := range map[string]string{
		"README.md":         "# Home\n\nhello world\n",
		"notes.txt":         "plain notes\n",
		"data/list.csv":     "a,b\n1,2\n",
		"docs/../notes.txt": "plain notes\n",
		"inside.md":         "---\ntitle: The A page\n---\n# A heading\n\nsome words here\n",
		"nope.md":           "MISSING",
	} {
		got, err := block(p, "x", path, nil)
		got = strings.ReplaceAll(strings.ReplaceAll(got, "&#34;", "\""), "&#39;", "'")
		if err != nil || got != want {
			t.Errorf("read(%q) = %q %v, want %q", path, got, err, want)
		}
	}
	if got := mustBlock(t, p, "x", "big.md", nil); len(got) != 1<<20 {
		t.Errorf("a file of exactly 1 MB: %d bytes", len(got))
	}
	refused := map[string]string{
		"../secret.md":    "outside",
		"docs/../../x.md": "outside",
		"/etc/hosts":      "outside",
		"leak.md":         "outside",
		"toobig.md":       "1 MB",
		"img.png":         "text",
		"bin.md":          "text",
		"latin.md":        "UTF-8",
		"docs":            "file",
		"":                "path",
		"a\x00b.md":       "path",
	}
	for path, want := range refused {
		if want == "" {
			continue
		}
		_, err := block(p, "x", path, nil)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("read(%q): %v, want an error containing %q", path, err, want)
		}
		noPath(t, err, s.dir)
	}
	// The secret never appears whatever the path.
	for _, path := range []string{"leak.md", "../secret-" + filepath.Base(s.dir) + ".md"} {
		if got, _ := block(p, "x", path, nil); strings.Contains(got, "SECRET") {
			t.Errorf("read(%q) leaked the secret", path)
		}
	}
	// Without a workspace there is nothing to read.
	bare := s.plugin("bare", readerSrc, scriptplugins.Runtime{Workspace: "-"})
	if _, err := block(bare, "x", "README.md", nil); err == nil {
		t.Error("read worked without a folder")
	}
}

func TestPagesListsTheFolder(t *testing.T) {
	s := readSite(t)
	p := s.plugin("lister", `plugin := {version: "1.0.0", blocks: ["x"], permissions: ["read"]}
if hook == "block" {
    r := ""
    for p in zen.pages() { r += p.path + "=" + p.title + ";" }
    out = zen.escape(r)
}`)
	got := mustBlock(t, p, "x", "", nil)
	for _, want := range []string{"README.md=Home;", "docs/a.md=The A page;", "docs/b.md=Page B;"} {
		if !strings.Contains(got, want) {
			t.Errorf("pages lack %q: %s", want, got)
		}
	}
	if strings.Contains(got, "notes.txt") || strings.Contains(got, "img.png") || strings.Contains(got, "secret") {
		t.Errorf("pages lists more than the folder's markdown: %s", got)
	}
	if strings.Index(got, "README.md") > strings.Index(got, "docs/a.md") {
		t.Errorf("pages are not sorted by path: %s", got)
	}
}

func TestUnknownImportsAreRefusedAtLoad(t *testing.T) {
	s := newSite(t)
	for _, mod := range []string{"os", "file", "net", "http", "exec", "./other", "../x", "/etc/passwd", "zen"} {
		err := s.loadErr("imp", `m := import("`+mod+`")`+"\n"+header)
		if err == nil {
			continue
		}
		errContains(t, err, mod)
		noPath(t, err, s.dir)
	}
	// Importing a source file of the folder is not available either.
	s.file(".zen/plugins/helper.tengo", `export {x: 1}`)
	err := s.loadErr("imp2", `h := import("helper")`+"\n"+header)
	errContains(t, err, "helper")
}
