package course

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
	if m.ID != "course" || !m.Default || m.APIVersion != plugins.APIVersion || m.Version == "" {
		t.Fatalf("manifest %+v", m)
	}
	if strings.Join(m.Layouts, ",") != "course" {
		t.Errorf("layouts %v", m.Layouts)
	}
	if strings.Join(m.Blocks, ",") != "check" {
		t.Errorf("blocks %v", m.Blocks)
	}
	ap, ok := New().(plugins.AssetProvider)
	if !ok {
		t.Fatal("not an AssetProvider")
	}
	e := ap.Entry()
	if strings.Join(e.JS, ",") != "course.js" || strings.Join(e.CSS, ",") != "course.css" {
		t.Errorf("entry %+v", e)
	}
	for _, f := range []string{"course.js", "course.css"} {
		if _, err := fs.Stat(ap.Assets(), f); err != nil {
			t.Errorf("asset %s: %v", f, err)
		}
	}
}

func TestLayoutWrapper(t *testing.T) {
	p, err := renderer(t).Render("a.md", []byte("---\nlayout: course\nlayoutOptions: { id: demo }\n---\n# One\n\n---\n\n## Two\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Layout != "course" {
		t.Errorf("layout %q", p.Layout)
	}
	for _, w := range []string{`data-zen-plugin="course"`, `data-zen-layout="course"`, `&#34;id&#34;:&#34;demo&#34;`, "<h1", "<hr"} {
		if !strings.Contains(p.HTML, w) {
			t.Errorf("missing %q in %s", w, p.HTML)
		}
	}
	if strings.Join(p.Assets.JS, ",") != "/_plugins/course/course.js" || strings.Join(p.Assets.CSS, ",") != "/_plugins/course/course.css" {
		t.Errorf("assets %+v", p.Assets)
	}
}

func TestAssetsOnlyWhenUsed(t *testing.T) {
	p, _ := renderer(t).Render("a.md", []byte("# Plain\n"))
	if len(p.Assets.JS)+len(p.Assets.CSS) != 0 || p.Layout != "" {
		t.Errorf("plain page: %+v", p)
	}
}

func TestCheckBlockKeepsTaskList(t *testing.T) {
	p, err := renderer(t).Render("a.md", []byte("---\nlayout: course\n---\n## Step\n\n:::check\n- [ ] ran it\n- [x] read it\n:::\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{`data-zen-name="check"`, `type="checkbox"`, "ran it", "read it"} {
		if !strings.Contains(p.HTML, w) {
			t.Errorf("missing %q in %s", w, p.HTML)
		}
	}
}
