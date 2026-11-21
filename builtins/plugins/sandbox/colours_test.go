package sandbox

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/builtins/plugins/codeblock"
	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/render"
	"github.com/zenmark-org/zenmd/engine/workspace"
)

// The block names its language for the codeblock plugin's hook: lang=, else
// the file's extension; an editable block or an unknown one stays as it was.
func TestRenderBlockLanguage(t *testing.T) {
	h := New().(plugins.BlockHook)
	pre := func(attrs map[string]string) string {
		t.Helper()
		attrs["image"] = "python"
		out, err := h.RenderBlock(cfg(nil), plugins.Block{Name: "sandbox", Fenced: true, Body: "x <1>", Attrs: attrs})
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(`<pre class="(zen-sandbox-code[^"]*)">x &lt;1&gt;</pre>`).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("no code pre in %s", out)
		}
		return m[1]
	}
	for name, c := range map[string]struct {
		attrs map[string]string
		want  string
	}{
		"lang":      {map[string]string{"lang": "ruby", "file": "main.py"}, "zen-sandbox-code language-ruby"},
		"py":        {map[string]string{"file": "main.py"}, "zen-sandbox-code language-python"},
		"sh":        {map[string]string{"file": "run.sh"}, "zen-sandbox-code language-bash"},
		"js":        {map[string]string{"file": "a.js"}, "zen-sandbox-code language-javascript"},
		"go":        {map[string]string{"file": "main.go"}, "zen-sandbox-code language-go"},
		"rb":        {map[string]string{"file": "x.rb"}, "zen-sandbox-code language-ruby"},
		"unknown":   {map[string]string{"file": "notes.xyz"}, "zen-sandbox-code"},
		"no file":   {map[string]string{}, "zen-sandbox-code"},
		"bad lang":  {map[string]string{"lang": `x" onclick="y`, "file": "main.py"}, "zen-sandbox-code language-python"},
		"editable":  {map[string]string{"file": "main.py", "editable": "true"}, "zen-sandbox-code"},
		"editable=": {map[string]string{"file": "main.py", "editable": ""}, "zen-sandbox-code"},
	} {
		if got := pre(c.attrs); got != c.want {
			t.Errorf("%s: class %q, want %q", name, got, c.want)
		}
	}
}

type noLinks struct{}

func (noLinks) ResolveLink(from, href string) workspace.Link { return workspace.Link{} }

func page(t *testing.T, fm string) render.Page {
	t.Helper()
	g := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(g, []byte("plugins:\n  sandbox:\n    images:\n      python: \"python:3.12-slim\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{codeblock.New(), New()} {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	r := render.New(reg, config.Load(g, ""), noLinks{})
	src := fm + "# T\n\n~~~sandbox {image=python cmd=\"python main.py\" file=main.py}\nimport os\nprint(\"a <b> & 'c'\")  # hi\n~~~\n"
	p, err := r.Render("a.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func codeOf(t *testing.T, p render.Page) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<pre class="zen-sandbox-code[^"]*">(.*?)</pre>`).FindStringSubmatch(p.HTML)
	if m == nil {
		t.Fatalf("no code in %s", p.HTML)
	}
	return m[1]
}

func hasCodeblockCSS(p render.Page) bool {
	for _, u := range p.Assets.CSS {
		if strings.Contains(u, "/_plugins/codeblock/codeblock.css") {
			return true
		}
	}
	return false
}

// Codeblock on: the code is coloured with chroma classes, its text is
// unchanged (Run reads textContent) and the page gets codeblock's CSS.
func TestCodeIsColouredWhenCodeblockIsOn(t *testing.T) {
	p := page(t, "")
	code := codeOf(t, p)
	if !strings.Contains(code, `<span class="kn">import</span>`) || !strings.Contains(p.HTML, `class="zen-sandbox-code chroma language-python"`) {
		t.Errorf("not coloured: %s", p.HTML)
	}
	if got := html.UnescapeString(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(code, "")); got != "import os\nprint(\"a <b> & 'c'\")  # hi\n" {
		t.Errorf("the text changed: %q", got)
	}
	if !hasCodeblockCSS(p) {
		t.Errorf("the page lacks codeblock's CSS: %v", p.Assets.CSS)
	}
	if strings.Contains(p.HTML, "style=") || strings.Contains(p.HTML, "<script") {
		t.Errorf("unsafe output: %s", p.HTML)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings: %v", p.Warnings)
	}
}

// Codeblock off for the page: the code stays plain and no codeblock CSS comes.
func TestCodeStaysPlainWhenCodeblockIsOff(t *testing.T) {
	p := page(t, "---\nplugins:\n  codeblock: false\n---\n")
	code := codeOf(t, p)
	if strings.Contains(code, "<span") || strings.Contains(p.HTML, "chroma") {
		t.Errorf("coloured with codeblock off: %s", p.HTML)
	}
	if hasCodeblockCSS(p) {
		t.Errorf("codeblock CSS with codeblock off: %v", p.Assets.CSS)
	}
	if !strings.Contains(code, "import os\nprint(") {
		t.Errorf("code %q", code)
	}
}
