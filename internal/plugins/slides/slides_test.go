package slides

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/render"
)

func renderer(t *testing.T) *render.Renderer {
	reg := plugins.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatal(err)
	}
	return render.New(reg, nil, nil)
}

func TestManifest(t *testing.T) {
	m := New().Manifest()
	if m.ID != "slides" || !m.Default || m.APIVersion != plugins.APIVersion || m.Version == "" {
		t.Fatalf("manifest %+v", m)
	}
	if strings.Join(m.Layouts, ",") != "slides" {
		t.Errorf("layouts %v", m.Layouts)
	}
	if strings.Join(m.Blocks, ",") != "" {
		t.Errorf("blocks %v", m.Blocks)
	}
	ap, ok := New().(plugins.AssetProvider)
	if !ok {
		t.Fatal("not an AssetProvider")
	}
	e := ap.Entry()
	if strings.Join(e.JS, ",") != "slides.js" || strings.Join(e.CSS, ",") != "slides.css" {
		t.Errorf("entry %+v", e)
	}
	for _, f := range []string{"slides.js", "slides.css"} {
		if _, err := fs.Stat(ap.Assets(), f); err != nil {
			t.Errorf("asset %s: %v", f, err)
		}
	}
}

func TestLayoutWrapper(t *testing.T) {
	p, err := renderer(t).Render("a.md", []byte("---\nlayout: slides\nlayoutOptions: { id: demo }\n---\n# One\n\n---\n\n## Two\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Layout != "slides" {
		t.Errorf("layout %q", p.Layout)
	}
	for _, w := range []string{`data-zen-plugin="slides"`, `data-zen-layout="slides"`, `&#34;id&#34;:&#34;demo&#34;`, "<h1", "<hr"} {
		if !strings.Contains(p.HTML, w) {
			t.Errorf("missing %q in %s", w, p.HTML)
		}
	}
	if strings.Join(p.Assets.JS, ",") != "/_plugins/slides/slides.js" || strings.Join(p.Assets.CSS, ",") != "/_plugins/slides/slides.css" {
		t.Errorf("assets %+v", p.Assets)
	}
}

func TestAssetsOnlyWhenUsed(t *testing.T) {
	p, _ := renderer(t).Render("a.md", []byte("# Plain\n"))
	if len(p.Assets.JS)+len(p.Assets.CSS) != 0 || p.Layout != "" {
		t.Errorf("plain page: %+v", p)
	}
}
