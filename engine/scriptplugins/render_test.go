package scriptplugins_test

import (
	"strings"
	"testing"

	"github.com/dragosh/zenmd/engine/plugins"
	"github.com/dragosh/zenmd/engine/render"
	"github.com/dragosh/zenmd/engine/scriptplugins"
)

// renderPage renders src with the given script plugins registered.
func renderPage(t *testing.T, ps []*scriptplugins.Plugin, src string) render.Page {
	t.Helper()
	reg := plugins.NewRegistry()
	for _, p := range ps {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	pg, err := render.New(reg, nil, nil).Render("a.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return pg
}

func TestMarkdownRendersThroughThePipeline(t *testing.T) {
	s := newSite(t)
	note := s.plugin("note", `plugin := {version: "1.0.0", blocks: ["note"], inlines: ["badge"], css: "div.note{color:red}"}
if hook == "block" { out = zen.html("div", {class: "note"}, zen.markdown(body)) }
if hook == "inline" { out = zen.html("em", {}, body) }`)
	chart := s.plugin("chart", `plugin := {version: "1.0.0", blocks: ["chart"], css: "b{color:blue}"}
if hook == "block" { out = zen.html("div", {class: "chart"}, body) }`)
	pg := renderPage(t, []*scriptplugins.Plugin{note, chart}, "# T\n\n:::note\nSome **bold** text :badge[hi]\n\n:::chart\nC\n:::\n\n- one\n- two\n:::\n")
	for _, want := range []string{
		`data-zen-plugin="note"`, `<div class="note">`, `<strong>bold</strong>`, `<em>hi</em>`, `<li>one</li>`,
		`data-zen-plugin="chart"`, `<div class="chart">C`,
	} {
		if !strings.Contains(pg.HTML, want) {
			t.Errorf("page lacks %q:\n%s", want, pg.HTML)
		}
	}
	if len(pg.Warnings) != 0 {
		t.Errorf("warnings: %v", pg.Warnings)
	}
	// The nested plugin's CSS is needed on the page too.
	if !strings.Contains(strings.Join(pg.Assets.CSS, ","), "/_plugins/chart/inline.css") || !strings.Contains(strings.Join(pg.Assets.CSS, ","), "/_plugins/note/inline.css") {
		t.Errorf("assets: %+v", pg.Assets)
	}
}

func TestScriptOutputIsSanitised(t *testing.T) {
	s := newSite(t)
	p := s.plugin("evil", `plugin := {version: "1.0.0", blocks: ["evil"]}
if hook == "block" { out = "<script>alert(1)</script><p onclick='x()' style='color:red'>hi</p><iframe src='https://e.example'></iframe><a href='javascript:alert(1)'>l</a>" }`)
	pg := renderPage(t, []*scriptplugins.Plugin{p}, ":::evil\nx\n:::\n")
	for _, bad := range []string{"<script", "onclick", "style=", "<iframe", "javascript:"} {
		if strings.Contains(pg.HTML, bad) {
			t.Errorf("page keeps %q: %s", bad, pg.HTML)
		}
	}
	if !strings.Contains(pg.HTML, "<p>hi</p>") {
		t.Errorf("the harmless part is gone: %s", pg.HTML)
	}
}

func TestMarkdownNestingIsLimited(t *testing.T) {
	s := newSite(t)
	loop := s.plugin("loop", `plugin := {version: "1.0.0", blocks: ["loop"]}
if hook == "block" { out = zen.html("div", {class: "loop"}, zen.markdown(":::loop\n" + body + "\n:::")) }`)
	pg := renderPage(t, []*scriptplugins.Plugin{loop}, ":::loop\nx\n:::\n")
	if len(pg.Warnings) == 0 {
		t.Fatalf("an endless markdown recursion gave no warning:\n%s", pg.HTML)
	}
	if !strings.Contains(strings.Join(pg.Warnings, " "), "nested") || !strings.Contains(pg.HTML, "zen-plugin-error") {
		t.Errorf("warnings %v\n%s", pg.Warnings, pg.HTML)
	}
	depth := strings.Count(pg.HTML, `<div class="loop">`)
	if depth < 2 || depth > 20 {
		t.Errorf("recursion went %d levels deep", depth)
	}
	// A legitimate, shallow nesting is fine.
	two := s.plugin("two", `plugin := {version: "1.0.0", blocks: ["two"]}
if hook == "block" { out = zen.html("div", {class: "two"}, zen.markdown(body)) }`)
	pg = renderPage(t, []*scriptplugins.Plugin{two}, ":::two\n:::two\nin\n:::\n:::\n")
	if len(pg.Warnings) != 0 || strings.Count(pg.HTML, `<div class="two">`) != 2 {
		t.Errorf("two levels: %v\n%s", pg.Warnings, pg.HTML)
	}
}

func TestMarkdownInASourceHookTitleOnlyAndErrors(t *testing.T) {
	s := newSite(t)
	// A script that fails shows the error box and a warning; the page still renders.
	bad := s.plugin("bad", `plugin := {version: "1.0.0", blocks: ["bad"]}
if hook == "block" { out = zen.read("x.md") }`)
	pg := renderPage(t, []*scriptplugins.Plugin{bad}, "# Title\n\n:::bad\nx\n:::\n\nafter\n")
	if !strings.Contains(pg.HTML, "zen-plugin-error") || !strings.Contains(pg.HTML, "after") || pg.Title != "Title" {
		t.Errorf("failed script: %s", pg.HTML)
	}
	if len(pg.Warnings) != 1 || !strings.Contains(pg.Warnings[0], `plugin "bad"`) || !strings.Contains(pg.Warnings[0], "permission") {
		t.Errorf("warnings: %v", pg.Warnings)
	}
}
