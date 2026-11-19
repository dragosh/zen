package layouts

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/builtins/plugins/check"
	"github.com/zenmark-org/zenmd/engine/config"
	core "github.com/zenmark-org/zenmd/engine/layouts"
	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/render"
)

func renderer(t *testing.T, site core.Site, cfg *config.Config) *render.Renderer {
	reg := plugins.NewRegistry()
	if err := reg.Register(check.New()); err != nil {
		t.Fatal(err)
	}
	return render.New(reg, cfg, nil).WithLayouts(Registry("0.0-test"), site)
}

func TestBuiltInsLoad(t *testing.T) {
	ls := All()
	if len(ls) != len(IDs) {
		t.Fatalf("%d layouts", len(ls))
	}
	for _, l := range ls {
		m := l.Manifest()
		if l.Source() != core.SourceBuiltin || m.Description == "" || m.Version == "" || m.Help.Summary == "" || len(m.Help.Snippets) == 0 {
			t.Errorf("%s: incomplete %+v", m.ID, m)
		}
		if l.Dir() != "" || l.Changed() {
			t.Errorf("%s: a built-in has no folder", m.ID)
		}
		for _, f := range append(append([]string{}, m.CSS...), m.JS...) {
			if fsys := l.Assets(); fsys == nil {
				t.Errorf("%s: no assets", m.ID)
			} else if _, err := fs.Stat(fsys, f); err != nil {
				t.Errorf("%s: asset %s: %v", m.ID, f, err)
			}
		}
	}
	has := func(id string, tpl, script bool) {
		for _, l := range ls {
			if l.Manifest().ID == id && (l.HasTemplate() != tpl || l.HasScript() != script) {
				t.Errorf("%s: template %v script %v", id, l.HasTemplate(), l.HasScript())
			}
		}
	}
	has("article", false, false)
	has("slides", false, true)
	has("course", false, true)
	for _, id := range []string{"docs", "post", "landing", "wide", "handout"} {
		has(id, true, false)
	}
}

// ---- slides and course behave as before (they were plugins)

func TestSlidesWrapper(t *testing.T) {
	p, err := renderer(t, nil, nil).Render("a.md", []byte("---\nlayout: slides\nlayoutOptions: { id: demo }\n---\n# One\n\n---\n\n## Two\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Layout.Name != "slides" || p.Layout.Source != "builtin" || len(p.Warnings) != 0 {
		t.Errorf("layout %+v %v", p.Layout, p.Warnings)
	}
	for _, w := range []string{`data-zen-layout="slides"`, `data-zen-layout-source="builtin"`, `&#34;id&#34;:&#34;demo&#34;`, "<h1", "<hr"} {
		if !strings.Contains(p.HTML, w) {
			t.Errorf("missing %q in %s", w, p.HTML)
		}
	}
	if strings.Join(p.Assets.JS, ",") != "/_layouts/slides/slides.js?v=0.0-test" || strings.Join(p.Assets.CSS, ",") != "/_layouts/slides/slides.css?v=0.0-test" {
		t.Errorf("assets %+v", p.Assets)
	}
}

func TestCourseWrapper(t *testing.T) {
	p, err := renderer(t, nil, nil).Render("a.md", []byte("---\nlayout: course\nlayoutOptions: { id: demo }\n---\n# One\n\n---\n\n## Two\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{`data-zen-layout="course"`, `&#34;id&#34;:&#34;demo&#34;`, "<h1", "<hr"} {
		if !strings.Contains(p.HTML, w) {
			t.Errorf("missing %q in %s", w, p.HTML)
		}
	}
	if strings.Join(p.Assets.JS, ",") != "/_layouts/course/course.js?v=0.0-test" || strings.Join(p.Assets.CSS, ",") != "/_layouts/course/course.css?v=0.0-test" {
		t.Errorf("assets %+v", p.Assets)
	}
}

func TestAssetsOnlyWhenUsed(t *testing.T) {
	p, _ := renderer(t, nil, nil).Render("a.md", []byte("# Plain\n"))
	if len(p.Assets.JS)+len(p.Assets.CSS) != 0 || p.Layout.Name != "article" || strings.Contains(p.HTML, "data-zen-layout") {
		t.Errorf("plain page: %+v", p)
	}
	p, _ = renderer(t, nil, nil).Render("a.md", []byte("---\nlayout: article\n---\n# Plain\n"))
	if len(p.Assets.JS)+len(p.Assets.CSS) != 0 || strings.Contains(p.HTML, "data-zen-layout") {
		t.Errorf("explicit article adds nothing: %+v", p)
	}
}

func TestCheckBlockKeepsTaskList(t *testing.T) {
	p, err := renderer(t, nil, nil).Render("a.md", []byte("---\nlayout: course\n---\n## Step\n\n:::check\n- [ ] ran it\n- [x] read it\n:::\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{`data-zen-name="check"`, `type="checkbox"`, "ran it", "read it"} {
		if !strings.Contains(p.HTML, w) {
			t.Errorf("missing %q in %s", w, p.HTML)
		}
	}
}

// ---- the template layouts

type site struct {
	prev, next *core.Link
	children   []core.Child
}

func (s site) Breadcrumb(string) []core.Crumb             { return nil }
func (s site) Neighbours(string) (*core.Link, *core.Link) { return s.prev, s.next }
func (s site) Children(string) []core.Child               { return s.children }

func page(t *testing.T, s core.Site, src string) render.Page {
	t.Helper()
	p, err := renderer(t, s, nil).Render("blog/p.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings %v", p.Warnings)
	}
	return p
}

func mustHave(t *testing.T, html string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("missing %q in\n%s", w, html)
		}
	}
}

func TestDocs(t *testing.T) {
	s := site{prev: &core.Link{Title: "Before", Path: "a.md"}, next: &core.Link{Title: "After <b>", Path: "c.md"}}
	p := page(t, s, "---\nlayout: docs\n---\n# Title\n\n## One\n\n### Sub\n\n#### Deep\n\ntext\n")
	mustHave(t, p.HTML, `data-zen-layout="docs"`, `class="zl-docs-toc"`, `href="#one"`, `href="#sub"`, `data-zen-action="edit"`,
		`data-path="a.md"`, `data-path="c.md"`, "Before", "After &lt;b&gt;", `<h2 id="one">`)
	if strings.Contains(p.HTML, `href="#deep"`) {
		t.Error("h4 in the docs rail (h2 and h3 only)")
	}
	if strings.Join(p.Assets.CSS, ",") != "/_layouts/docs/docs.css?v=0.0-test" {
		t.Errorf("assets %+v", p.Assets)
	}
	p = page(t, nil, "---\nlayout: { name: docs, toc: false, edit: false }\n---\n# T\n\n## One\n")
	if strings.Contains(p.HTML, "zl-docs-toc") || strings.Contains(p.HTML, "data-zen-action") || strings.Contains(p.HTML, "zl-docs-pager") {
		t.Errorf("options ignored: %s", p.HTML)
	}
}

func TestPost(t *testing.T) {
	p, err := renderer(t, site{next: &core.Link{Title: "Newer", Path: "n.md"}}, nil).Render("blog/p.md", []byte(
		"---\ntitle: Hello\nlayout: post\ndate: 2026-11-17\nauthor: Ada\ntags: [a, b]\n---\n# Hello\n\n"+strings.Repeat("word ", 450)+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings %v", p.Warnings)
	}
	mustHave(t, p.HTML, `<time datetime="2026-11-17">17 November 2026</time>`, "<span>Ada</span>", "<span>3 min read</span>",
		`<ul class="zl-post-tags"><li>a</li><li>b</li></ul>`, `data-path="n.md"`, `class="zl-post-text"`)
	if strings.Contains(p.HTML, "zl-post-cover") {
		t.Error("cover without a cover field")
	}
	p, _ = renderer(t, nil, nil).Render("blog/p.md", []byte("---\ntitle: T\nlayout: { name: post, reading: false }\ncover: https://example.com/c.png\n---\n# T\n"))
	if strings.Contains(p.HTML, "min read") || !strings.Contains(p.HTML, `<img class="zl-post-cover" src="https://example.com/c.png"`) {
		t.Errorf("reading/cover: %s", p.HTML)
	}
}

func TestLanding(t *testing.T) {
	s := site{children: []core.Child{{Title: "Install", Path: "install.md", Summary: "How to <install>."}, {Title: "Use", Path: "use.md"}}}
	p := page(t, s, "---\ntitle: Guide\nlayout: landing\ntagline: Read & write\n---\nWelcome.\n")
	mustHave(t, p.HTML, `<h1>Guide</h1>`, `<p class="zl-tagline">Read &amp; write</p>`, `class="zl-card"`, `data-path="install.md"`,
		"<strong>Install</strong>", "How to &lt;install&gt;.", `data-path="use.md"`, "Welcome.")
	if strings.Count(p.HTML, `class="zl-card"`) != 2 || strings.Count(p.HTML, `<a href="#" data-zen-link="internal"`) != 2 {
		t.Errorf("cards: %s", p.HTML)
	}
	p = page(t, s, "---\nlayout: { name: landing, cards: false }\n---\nWelcome.\n")
	if strings.Contains(p.HTML, `class="zl-card"`) {
		t.Error("cards: false ignored")
	}
}

func TestWide(t *testing.T) {
	p := page(t, nil, "---\nlayout: wide\n---\n# T\n\n| a | b |\n|---|---|\n| 1 | 2 |\n")
	mustHave(t, p.HTML, `class="zl-wide"`, "<table>")
}

func TestHandout(t *testing.T) {
	p := page(t, nil, "---\ntitle: Sheet\nlayout: handout\ndate: 2026-03-04\nauthor: Ada\n---\n# Sheet\n\ntext\n")
	mustHave(t, p.HTML, `class="zl-handout zl-paper-a4"`, `class="zl-sheet-head"`, "4 March 2026", "Ada", `class="zl-sheet-foot"`, "<span>Sheet</span>")
	p = page(t, nil, "---\ntitle: Sheet\nlayout: { name: handout, paper: letter }\n---\n# Sheet\n")
	mustHave(t, p.HTML, "zl-paper-letter")
}

func TestPatternAppliesLayout(t *testing.T) {
	dir := t.TempDir()
	if err := writeConfig(dir, "version: 1\nlayouts:\n  \"blog/**\": post\n"); err != nil {
		t.Fatal(err)
	}
	p, _ := renderer(t, nil, config.Load("", dir)).Render("blog/x.md", []byte("# X\n"))
	if p.Layout.Name != "post" || !strings.Contains(p.HTML, `data-zen-layout="post"`) {
		t.Errorf("pattern: %+v", p.Layout)
	}
}

// ---- requirements (kit/assembly)

var blockRE = regexp.MustCompile(`data-zen-name="([a-z][a-z0-9-]*)"|:::([a-z][a-z0-9-]*)`)

func TestLayoutNeeds(t *testing.T) {
	want := map[string][]string{"course": {"check"}}
	plugin := map[string]string{"check": "check", "quiz": "quiz", "quiz-results": "quiz", "mermaid": "diagram", "diagram": "diagram", "sandbox": "sandbox"}
	for _, l := range All() {
		m := l.Manifest()
		if strings.Join(m.Requires, ",") != strings.Join(want[m.ID], ",") {
			t.Errorf("%s: requires %v, want %v", m.ID, m.Requires, want[m.ID])
		}
		declared := map[string]bool{}
		for _, r := range m.Requires {
			declared[r] = true
		}
		// What the layout's own files mention of a plugin's block must be declared.
		dir, _ := fs.Sub(files, m.ID)
		names, _ := fs.ReadDir(dir, ".")
		for _, f := range names {
			if f.IsDir() {
				continue
			}
			data, _ := fs.ReadFile(dir, f.Name())
			for _, mm := range blockRE.FindAllStringSubmatch(string(data), -1) {
				name := mm[1] + mm[2]
				if p := plugin[name]; p != "" && !declared[p] {
					t.Errorf("%s: %s mentions the %s block but does not require plugin %s", m.ID, f.Name(), name, p)
				}
			}
		}
	}
}
