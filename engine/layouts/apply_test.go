package layouts

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSite struct {
	crumbs     []Crumb
	prev, next *Link
	children   []Child
	calls      int
}

func (f *fakeSite) Breadcrumb(string) []Crumb { return f.crumbs }
func (f *fakeSite) Neighbours(string) (*Link, *Link) {
	return f.prev, f.next
}
func (f *fakeSite) Children(string) []Child { f.calls++; return f.children }

func apply(t *testing.T, tpl string, fields, opts string, in Input) (Result, error) {
	t.Helper()
	yml := "id: lay\nversion: 1.0.0\ntemplate: t.html\n" + fields + opts
	l := mustLoad(t, "lay", map[string]string{"layout.yml": yml, "t.html": tpl})
	return l.Apply(in)
}

func TestApplyWrapper(t *testing.T) {
	res, err := apply(t, "<p>{{.Title}}</p>{{.Content}}", "", "options:\n  toc: { type: bool, default: true }\n", Input{
		Title: "Hello", Path: "a.md", Content: "<h2 id=\"x\">X</h2>", Options: map[string]any{"width": "wide"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.HTML, `<div data-zen-layout="lay" data-zen-layout-source="folder" data-zen-attrs="`) || !strings.HasSuffix(res.HTML, "</div>") {
		t.Errorf("wrapper: %s", res.HTML)
	}
	if !strings.Contains(res.HTML, "<p>Hello</p><h2 id=\"x\">X</h2>") {
		t.Errorf("body: %s", res.HTML)
	}
	// Declared options get their defaults; undeclared options are not passed when options are declared.
	if res.Attrs["toc"] != "true" {
		t.Errorf("attrs %v", res.Attrs)
	}
	if _, ok := res.Attrs["width"]; ok {
		t.Errorf("an undeclared option reached the attributes: %v", res.Attrs)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "width") {
		t.Errorf("warnings %v", res.Warnings)
	}
}

func TestApplyEscapesFieldsAndTitle(t *testing.T) {
	res, err := apply(t, `<h1>{{.Title}}</h1><p>{{.Field "author"}}</p><a title="{{.Field "author"}}">x</a>`,
		"fields:\n  author: { type: string }\n", "", Input{
			Title: "<script>alert(1)</script>", Content: "", Frontmatter: map[string]any{"author": `"><script>bad()</script>`},
		})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.HTML, "<script>") || strings.Contains(res.HTML, `"><script`) {
		t.Errorf("not escaped: %s", res.HTML)
	}
	if !strings.Contains(res.HTML, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("title: %s", res.HTML)
	}
}

func TestApplyDataModel(t *testing.T) {
	site := &fakeSite{
		crumbs:   []Crumb{{"Home", "README.md"}, {"Post", "blog/p.md"}},
		prev:     &Link{"Before", "blog/a.md"},
		children: []Child{{"One", "blog/one.md", "First."}, {"Two", "blog/two.md", "Second."}},
	}
	tpl := `P={{.Path}}|
{{range .Breadcrumb}}[{{.Title}}:{{.Path}}]{{end}}|
{{range .TOC}}[{{.Level}}:{{.ID}}:{{.Text}}]{{end}}|
{{with .Prev}}prev={{.Title}}@{{.Path}}{{end}}{{if .Next}}next{{else}}nonext{{end}}|
{{range .Children}}({{.Title}}/{{.Path}}/{{.Summary}}){{end}}|
w={{.Words}} rt={{.ReadingTime}}|
{{.Field "author"}}/{{date (.Field "date")}}/{{join (.Field "tags") ", "}}/{{upper (.Field "author")}}/{{lower "ABC"}}|
img={{.Field "cover"}}|
{{markdown (.Field "intro")}}|
{{if .Option "toc"}}toc{{end}} {{.Option "width"}} {{.Option "n"}}`
	fields := "fields:\n  author: { type: string }\n  date: { type: date }\n  tags: { type: list }\n  cover: { type: image }\n  intro: { type: string }\n"
	opts := "options:\n  toc: { type: bool, default: false }\n  width: { type: string, default: narrow }\n  n: { type: int, default: 2 }\n"
	content := `<h1 id="t">T</h1><h2 id="a">First <em>part</em></h2><p>` + strings.Repeat("word ", 394) + `</p><h3 id="b">Second</h3><h4 id="c">Third</h4><h5 id="d">Fifth</h5>`
	res, err := apply(t, tpl, fields, opts, Input{
		Title: "T", Path: "blog/p.md", Content: content, Site: site,
		Frontmatter: map[string]any{
			"author": "Ada", "date": time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC), "tags": []any{"a", "b"},
			"cover": "img/c.png", "intro": "*hi*",
		},
		Options:  map[string]any{"toc": "true", "n": 5},
		Markdown: func(s string) (string, error) { return "<p><em>rendered:" + strings.Trim(s, "*") + "</em></p>", nil },
		Image:    func(s string) (string, bool) { return "/_ws/" + s, true },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"P=blog/p.md|", "[Home:README.md][Post:blog/p.md]|",
		"[2:a:First part][3:b:Second][4:c:Third]|", // h2-h4 only, text without tags
		"prev=Before@blog/a.md", "nonext|",
		"(One/blog/one.md/First.)(Two/blog/two.md/Second.)|",
		"w=400 rt=2|",
		"Ada/17 November 2026/a, b/ADA/abc|",
		"img=/_ws/img/c.png|",
		"<p><em>rendered:hi</em></p>|",
		"toc narrow 5",
	} {
		if !strings.Contains(res.HTML, want) {
			t.Errorf("missing %q in\n%s", want, res.HTML)
		}
	}
	if strings.Contains(res.HTML, "Fifth") && strings.Contains(res.HTML, ":d:") {
		t.Error("h5 in the TOC")
	}
	if res.Attrs["toc"] != "true" || res.Attrs["width"] != "narrow" || res.Attrs["n"] != "5" {
		t.Errorf("attrs %v", res.Attrs)
	}
}

func TestChildrenAreLazy(t *testing.T) {
	site := &fakeSite{}
	if _, err := apply(t, "{{.Content}}", "", "", Input{Content: "x", Site: site}); err != nil {
		t.Fatal(err)
	}
	if site.calls != 0 {
		t.Errorf("Children computed %d times for a template that does not use them", site.calls)
	}
}

func TestApplyNilSiteAndMissingValues(t *testing.T) {
	res, err := apply(t, `{{if .Prev}}p{{end}}{{if .Next}}n{{end}}{{len .Children}}{{len .Breadcrumb}}[{{.Field "a"}}][{{date (.Field "d")}}][{{len (.Field "l")}}]{{if .Field "d"}}D{{end}}`,
		"fields:\n  a: { type: string }\n  d: { type: date }\n  l: { type: list }\n", "", Input{Content: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if res.HTML == "" || !strings.Contains(res.HTML, "00[][][0]") || strings.HasSuffix(strings.TrimSuffix(res.HTML, "</div>"), "D") {
		t.Errorf("missing values: %s", res.HTML)
	}
}

func TestFieldTypes(t *testing.T) {
	fields := "fields:\n  d: { type: date }\n  n: { type: int }\n  b: { type: bool }\n  s: { type: string }\n  i: { type: image }\n"
	res, err := apply(t, `{{date (.Field "d")}}|{{.Field "n"}}|{{.Field "b"}}|{{.Field "s"}}|[{{.Field "i"}}]`, fields, "", Input{
		Frontmatter: map[string]any{"d": "2026-03-04", "n": 7, "b": true, "s": 12, "i": "missing.png"},
		Image:       func(string) (string, bool) { return "", false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.HTML, "4 March 2026|7|true|12|[]") {
		t.Errorf("typed fields: %s", res.HTML)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], `"i"`) {
		t.Errorf("an unresolvable image warns once: %v", res.Warnings)
	}
	res, err = apply(t, `[{{date (.Field "d")}}]{{.Field "n"}}`, "fields:\n  d: { type: date }\n  n: { type: int }\n", "", Input{
		Frontmatter: map[string]any{"d": "yesterday", "n": "x"},
	})
	if err != nil || !strings.Contains(res.HTML, "[]0") || len(res.Warnings) != 2 {
		t.Errorf("wrong types: %v %q %v", err, res.HTML, res.Warnings)
	}
}

func TestMarkdownFuncOnlyOnStrings(t *testing.T) {
	res, err := apply(t, `{{markdown "**x**"}}|`, "", "", Input{
		Markdown: func(s string) (string, error) { return "<b>" + s + "</b>", nil },
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(res.HTML, "<b>**x**</b>|") {
		t.Errorf("markdown: %s", res.HTML)
	}
	// Without a markdown renderer the text is escaped, not trusted.
	res, err = apply(t, `{{markdown "<i>x</i>"}}`, "", "", Input{})
	if err != nil || strings.Contains(res.HTML, "<i>") {
		t.Errorf("no renderer: %v %s", err, res.HTML)
	}
	res, err = apply(t, `{{markdown "x"}}`, "", "", Input{Markdown: func(string) (string, error) { return "", errors.New("boom") }})
	if err == nil {
		t.Errorf("a failing markdown renderer is a template error: %s", res.HTML)
	}
}

func TestUndeclaredFieldOrOptionIsAnError(t *testing.T) {
	for _, tpl := range []string{`{{.Field "nope"}}`, `{{.Option "nope"}}`} {
		if _, err := apply(t, tpl, "fields:\n  a: { type: string }\n", "", Input{}); err == nil || !strings.Contains(err.Error(), "nope") {
			t.Errorf("%s: %v", tpl, err)
		}
	}
}

func TestTemplateCannotReachTheOutsideWorld(t *testing.T) {
	// Only date, upper, lower, join and markdown exist beyond the builtins, and
	// the data model has no methods that touch files.
	for _, tpl := range []string{`{{template "x"}}`, `{{call .Field}}`, `{{.Content.Foo}}`, `{{index .Path 99}}`} {
		l, err := Load(write(t, "lay", map[string]string{"layout.yml": "id: lay\nversion: 1.0.0\ntemplate: t.html\n", "t.html": tpl}), SourceFolder)
		if err != nil {
			continue
		}
		if _, err := l.Apply(Input{Path: "a.md"}); err == nil {
			t.Errorf("%s ran", tpl)
		}
	}
}

func TestCSSOnlyLayoutWrapsContent(t *testing.T) {
	l := mustLoad(t, "tidy", map[string]string{"layout.yml": "id: tidy\nversion: 1.0.0\ncss: [a.css]\n", "a.css": "p{}"})
	res, err := l.Apply(Input{Content: "<p>x</p>", Options: map[string]any{"k": "v"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.HTML, `<div data-zen-layout="tidy" data-zen-layout-source="folder" data-zen-attrs="`) ||
		!strings.Contains(res.HTML, "<p>x</p></div>") {
		t.Errorf("css-only wrapper: %s", res.HTML)
	}
	// A layout without declared options passes every option through.
	if res.Attrs["k"] != "v" {
		t.Errorf("attrs %v", res.Attrs)
	}
}

func TestAttrsAreEscapedJSON(t *testing.T) {
	l := mustLoad(t, "tidy", map[string]string{"layout.yml": "id: tidy\nversion: 1.0.0\ncss: [a.css]\n", "a.css": ""})
	res, _ := l.Apply(Input{Content: "x", Options: map[string]any{"id": `a"><script>`}})
	if strings.Contains(res.HTML, "<script>") || strings.Contains(res.HTML, `a">`) {
		t.Errorf("attribute not escaped: %s", res.HTML)
	}
}
