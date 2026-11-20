package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/builtins/dialects/myst"
	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/dialects"
	"github.com/zenmark-org/zenmd/engine/plugins"
)

// bare is a plugin without hooks: a container's body is rendered as markdown.
type bare struct{ m plugins.Manifest }

func (b bare) Manifest() plugins.Manifest { return b.m }

func admonitions(id string, names ...string) bare {
	m := man(id, true)
	m.Blocks = names
	return bare{m}
}

func mystReg(t *testing.T) *dialects.Registry {
	t.Helper()
	r, err := dialects.NewRegistry(myst.New())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// folderCfg loads a config whose .zen.yml says yml.
func folderCfg(t *testing.T, yml string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".zen.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	return config.Load("", dir)
}

const mystOn = "markdown:\n  dialects: [myst]\n"

func mystRenderer(t *testing.T, yml string, ps ...plugins.Plugin) *Renderer {
	t.Helper()
	return New(newReg(t, ps...), folderCfg(t, yml), fakeResolver{}).WithDialects(mystReg(t))
}

func TestDialectOff(t *testing.T) {
	note := admonitions("note", "note")
	src := "```{note} Remember\nBody text.\n```\n"
	// the dialect exists in the build but the folder does not list it
	for name, r := range map[string]*Renderer{
		"default":      New(newReg(t, note), config.Default(), fakeResolver{}).WithDialects(mystReg(t)),
		"no .zen.yml":  New(newReg(t, note), folderCfg(t, "version: 1\n"), fakeResolver{}).WithDialects(mystReg(t)),
		"not in build": New(newReg(t, note), folderCfg(t, mystOn), fakeResolver{}),
	} {
		p := render(t, r, src)
		contains(t, p.HTML, "<pre><code", "Body text.")
		lacks(t, p.HTML, "data-zen-plugin")
		if name == "not in build" {
			continue
		}
		if len(p.Warnings) != 0 {
			t.Errorf("%s: warnings %v", name, p.Warnings)
		}
	}
	// front matter cannot turn it on
	r := mystRenderer(t, "version: 1\n", note)
	p := render(t, r, "---\nmarkdown:\n  dialects: [myst]\n---\n"+src)
	lacks(t, p.HTML, "data-zen-plugin")
	if !strings.Contains(strings.Join(p.Warnings, "\n"), "markdown") {
		t.Errorf("no warning for front matter: %v", p.Warnings)
	}
}

func TestDialectOn(t *testing.T) {
	note := admonitions("note", "note")
	r := mystRenderer(t, mystOn, note)
	p := render(t, r, "```{note} Remember\nBody text.\n```\n")
	contains(t, p.HTML, `data-zen-plugin="note"`, `data-zen-name="note"`, "Remember", "<p>Body text.</p>")
	lacks(t, p.HTML, "<pre><code")

	// the same as zen's own syntax
	own := render(t, r, ":::note{title=\"Remember\" arg=\"Remember\"}\nBody text.\n:::\n")
	if p.HTML != own.HTML {
		t.Errorf("MyST and :::\n%s\n%s", p.HTML, own.HTML)
	}
	// colon fence, tilde fence
	if c := render(t, r, ":::{note} Remember\nBody text.\n:::\n"); c.HTML != own.HTML {
		t.Errorf("colon fence:\n%s", c.HTML)
	}
	if c := render(t, r, "~~~{note} Remember\nBody text.\n~~~\n"); c.HTML != own.HTML {
		t.Errorf("tilde fence:\n%s", c.HTML)
	}

	// an id the build does not have: ignored, one warning
	r = mystRenderer(t, "markdown:\n  dialects: [mkdocs, myst]\n", note)
	p = render(t, r, "```{note}\nx\n```\n")
	contains(t, p.HTML, `data-zen-plugin="note"`)
	n := 0
	for _, w := range p.Warnings {
		if strings.Contains(w, "dialect mkdocs is not in this build") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("warnings %v", p.Warnings)
	}
}

func TestMystCode(t *testing.T) {
	r := mystRenderer(t, mystOn)
	for _, name := range []string{"code-block", "code", "code-cell"} {
		p := render(t, r, "```{"+name+"} python\nprint(\"<hi>\")\n```\n")
		contains(t, p.HTML, `<pre><code class="language-python">`, "print(&#34;&lt;hi&gt;&#34;)")
	}
	// a body with backticks cannot end the fence early
	p := render(t, r, "````{code-block} md\n```{note}\nx\n```\n````\n")
	contains(t, p.HTML, `class="language-md"`, "```{note}")
	// without a language
	contains(t, render(t, r, "```{code}\nplain\n```\n").HTML, "<pre><code>plain")
	// highlighted by a plugin that owns the language
	hl := blockPlugin("hl", true, []string{"python"}, nil)
	r = mystRenderer(t, mystOn, hl)
	contains(t, render(t, r, "```{code-block} python\nx = 1\n```\n").HTML, `data-zen-plugin="hl"`, "x = 1")
}

func TestMystUnknown(t *testing.T) {
	r := mystRenderer(t, mystOn, admonitions("note", "note"))
	p := render(t, r, "```{mystery} arg\nx\n```\n\n:::{other}\ny\n:::\n")
	contains(t, p.HTML, "<pre><code", "x")
	lacks(t, p.HTML, "data-zen-plugin")
	if len(p.Warnings) != 0 {
		t.Errorf("warnings %v", p.Warnings)
	}
}

func TestMystDenied(t *testing.T) {
	reg := newReg(t)
	reg.Deny(plugins.Manifest{ID: "note", Blocks: []string{"note"}})
	r := New(reg, folderCfg(t, mystOn), fakeResolver{}).WithDialects(mystReg(t))
	own := render(t, r, ":::note\nx\n:::\n")
	p := render(t, r, "```{note}\nx\n```\n")
	if !strings.Contains(strings.Join(own.Warnings, "|"), `plugin "note" is not allowed`) {
		t.Fatalf("::: warnings %v", own.Warnings)
	}
	if fmt.Sprint(p.Warnings) != fmt.Sprint(own.Warnings) {
		t.Errorf("MyST warnings %v, ::: warnings %v", p.Warnings, own.Warnings)
	}
	lacks(t, p.HTML, "data-zen-plugin")
}

func TestMystAdmonitionNames(t *testing.T) {
	names := []string{"note", "tip", "warning", "important", "caution", "attention", "danger", "error", "hint", "seealso", "admonition"}
	r := mystRenderer(t, mystOn, admonitions("note", names...))
	for _, n := range names {
		p := render(t, r, "```{"+n+"} T\nbody\n```\n")
		contains(t, p.HTML, `data-zen-plugin="note"`, `data-zen-name="`+n+`"`)
	}
}

func TestMystNestingLimit(t *testing.T) {
	r := mystRenderer(t, mystOn, admonitions("note", "note"))
	var open, shut strings.Builder
	for i := 0; i < 12; i++ {
		fence := strings.Repeat("`", 14-i)
		open.WriteString(fence + "{note}\n")
		shut.WriteString(strings.Repeat("`", 3+i) + "\n")
	}
	// the closers must run from the innermost (shortest) outwards
	var closers strings.Builder
	for i := 11; i >= 0; i-- {
		closers.WriteString(strings.Repeat("`", 14-i) + "\n")
	}
	p := render(t, r, open.String()+"deep\n"+closers.String())
	if got := strings.Count(p.HTML, `data-zen-plugin="note"`); got > maxNesting+1 {
		t.Errorf("%d nested containers, limit %d", got, maxNesting)
	}
	contains(t, strings.Join(p.Warnings, "\n"), "nested more than")
}

func TestMystOptionsSanitised(t *testing.T) {
	r := mystRenderer(t, mystOn, admonitions("note", "note"))
	p := render(t, r, "```{note} <script>alert(1)</script>\n:class: \"><img src=x onerror=alert(1)>\n:1bad: x\nbody <b onclick=x>b</b>\n```\n")
	lacks(t, p.HTML, "<script", "<img", "onerror=alert", "onclick")
	contains(t, p.HTML, "&lt;script&gt;")
	contains(t, strings.Join(p.Warnings, "\n"), "1bad")
}

// A MyST fence inside a ::: container and the reverse.
func TestMystMixed(t *testing.T) {
	r := mystRenderer(t, mystOn, admonitions("note", "note", "tip"))
	p := render(t, r, ":::note\nouter\n```{tip} In\ninner\n```\nafter\n:::\n")
	if strings.Count(p.HTML, `data-zen-plugin="note"`) != 2 {
		t.Errorf("myst in :::\n%s", p.HTML)
	}
	p = render(t, r, "````{note}\nouter\n:::tip\ninner\n:::\nafter\n````\n")
	if strings.Count(p.HTML, `data-zen-plugin="note"`) != 2 {
		t.Errorf("::: in myst\n%s", p.HTML)
	}
	// MyST's colon fence inside a ::: container: its closer is not the container's
	p = render(t, r, ":::note\nouter\n:::{tip} In\ninner\n:::\nafter\n:::\ntail\n")
	if strings.Count(p.HTML, `data-zen-plugin="note"`) != 2 || !strings.Contains(p.HTML, "after") || !strings.Contains(p.HTML, "tail") {
		t.Errorf("colon fence in :::\n%s", p.HTML)
	}
	if strings.Contains(p.HTML, ":::") {
		t.Errorf("stray fence left:\n%s", p.HTML)
	}
}

// Rendering is not slowed to a crawl by a page of directives.
func TestMystHugePage(t *testing.T) {
	r := mystRenderer(t, mystOn, admonitions("note", "note"))
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&b, "```{note} n%d\nbody\n```\n\n", i)
	}
	p := render(t, r, b.String())
	if got := strings.Count(p.HTML, `data-zen-plugin="note"`); got != 2000 {
		t.Errorf("%d notes", got)
	}
}
