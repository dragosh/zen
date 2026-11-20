package render

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/layouts"
)

func layoutFS(t *testing.T, id, yml string, files map[string]string) *layouts.Layout {
	t.Helper()
	m := fstest.MapFS{"layout.yml": {Data: []byte(yml)}}
	for k, v := range files {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	l, err := layouts.LoadFS(m, id)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func layoutRenderer(t *testing.T, cfg *config.Config, ls ...*layouts.Layout) *Renderer {
	reg := layouts.NewRegistry()
	reg.SetAssetVersion("9.9")
	for _, l := range ls {
		if err := reg.Register(l); err != nil {
			t.Fatal(err)
		}
	}
	return New(nil, cfg, nil).WithLayouts(reg, nil)
}

func TestLayoutWrapperAndAssets(t *testing.T) {
	deck := layoutFS(t, "deck", "id: deck\nversion: 1.0.0\ncss: [d.css]\njs: [d.js]\n", map[string]string{"d.css": "", "d.js": ""})
	r := layoutRenderer(t, nil, deck)
	pg, err := r.Render("a.md", []byte("---\nlayout: deck\nlayoutOptions: { transition: fade, n: 3 }\n---\n# One\n\n---\n\n# Two\n"))
	if err != nil {
		t.Fatal(err)
	}
	if pg.Layout != (LayoutRef{"deck", "builtin"}) || len(pg.Warnings) != 0 {
		t.Errorf("layout %+v warnings %v", pg.Layout, pg.Warnings)
	}
	if !strings.HasPrefix(pg.HTML, `<div data-zen-layout="deck" data-zen-layout-source="builtin" data-zen-attrs="{&#34;n&#34;:&#34;3&#34;,&#34;transition&#34;:&#34;fade&#34;}">`) ||
		!strings.HasSuffix(strings.TrimSpace(pg.HTML), "</div>") || strings.Count(pg.HTML, "<h1") != 2 {
		t.Errorf("wrapper: %s", pg.HTML)
	}
	if strings.Join(pg.Assets.CSS, ",") != "/_layouts/deck/d.css?v=9.9" || strings.Join(pg.Assets.JS, ",") != "/_layouts/deck/d.js?v=9.9" {
		t.Errorf("assets %+v", pg.Assets)
	}
}

func TestNoLayoutIsArticle(t *testing.T) {
	pg, _ := layoutRenderer(t, nil).Render("a.md", []byte("# Plain\n"))
	if pg.Layout != (LayoutRef{"article", "builtin"}) || strings.Contains(pg.HTML, "data-zen-layout") || len(pg.Assets.JS)+len(pg.Assets.CSS) != 0 {
		t.Errorf("plain page: %+v %s", pg.Layout, pg.HTML)
	}
}

func TestUnknownOrDeniedLayoutFallsBack(t *testing.T) {
	reg := layouts.NewRegistry()
	reg.Deny("secret")
	r := New(nil, nil, nil).WithLayouts(reg, nil)
	for src, want := range map[string]string{
		"---\nlayout: poster\n---\n# T\n": "unknown layout",
		"---\nlayout: secret\n---\n# T\n": "not allowed",
		"---\nlayout: [a]\n---\n# T\n":    "layout must be a name",
	} {
		pg, _ := r.Render("a.md", []byte(src))
		if pg.Layout.Name != "article" || strings.Contains(pg.HTML, "data-zen-layout") || !strings.Contains(pg.HTML, "<h1") {
			t.Errorf("%q: %+v %s", src, pg.Layout, pg.HTML)
		}
		if len(pg.Warnings) != 1 || !strings.Contains(pg.Warnings[0], want) {
			t.Errorf("%q: warnings %v", src, pg.Warnings)
		}
	}
}

func TestTemplateLayoutIsSanitised(t *testing.T) {
	post := layoutFS(t, "post", "id: post\nversion: 1.0.0\ntemplate: t.html\nfields:\n  author: { type: string }\n",
		map[string]string{"t.html": `<header class="top"><h1 class="t">{{.Title}}</h1><p>{{.Field "author"}}</p><a data-zen-action="edit" class="x" onclick="evil()">Edit</a></header><article>{{.Content}}</article><script>alert(1)</script>`})
	r := layoutRenderer(t, nil, post)
	pg, err := r.Render("a.md", []byte("---\nlayout: post\nauthor: \"<img src=x onerror=alert(1)>\"\n---\n# T\n\nbody <script>bad()</script>\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<header class="top">`, `<h1 class="t">`, `data-zen-action="edit"`, "<article>", "&lt;img src=x onerror=alert(1)&gt;"} {
		if !strings.Contains(pg.HTML, want) {
			t.Errorf("missing %q in %s", want, pg.HTML)
		}
	}
	for _, bad := range []string{"<script", "onclick", "onerror=alert(1)>", "bad()"} {
		if strings.Contains(pg.HTML, bad) && bad != "onerror=alert(1)>" {
			t.Errorf("%q survived: %s", bad, pg.HTML)
		}
	}
	if strings.Contains(pg.HTML, "<img") {
		t.Errorf("field became markup: %s", pg.HTML)
	}
}

func TestTemplateFailureFallsBack(t *testing.T) {
	bad := layoutFS(t, "bad", "id: bad\nversion: 1.0.0\ntemplate: t.html\n", map[string]string{"t.html": `{{.Field "nope"}}`})
	pg, _ := layoutRenderer(t, nil, bad).Render("a.md", []byte("---\nlayout: bad\n---\n# T\n"))
	if pg.Layout.Name != "article" || strings.Contains(pg.HTML, "data-zen-layout") || len(pg.Warnings) != 1 || !strings.Contains(pg.Warnings[0], "article") {
		t.Errorf("%+v %v %s", pg.Layout, pg.Warnings, pg.HTML)
	}
}

func TestConfigDefaultsAndPatterns(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/.zenmd.yml"
	if err := writeFile(cfgFile, "version: 1\nlayout: a\nlayouts:\n  \"blog/**\": b\n"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Load("", dir)
	mk := func(id string) *layouts.Layout {
		return layoutFS(t, id, "id: "+id+"\nversion: 1.0.0\ncss: [x.css]\n", map[string]string{"x.css": ""})
	}
	r := layoutRenderer(t, cfg, mk("a"), mk("b"), mk("c"))
	for path, src := range map[string]string{
		"x.md":      "# T\n",
		"blog/p.md": "# T\n",
		"blog/q.md": "---\nlayout: c\n---\n# T\n",
	} {
		pg, _ := r.Render(path, []byte(src))
		want := map[string]string{"x.md": "a", "blog/p.md": "b", "blog/q.md": "c"}[path]
		if pg.Layout.Name != want {
			t.Errorf("%s: layout %q, want %q", path, pg.Layout.Name, want)
		}
	}
}

func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }
