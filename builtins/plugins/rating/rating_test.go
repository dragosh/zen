package rating

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/render"
)

func newRenderer(t *testing.T) *render.Renderer {
	t.Helper()
	reg := plugins.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatal(err)
	}
	return render.New(reg, nil, nil)
}

func page(t *testing.T, src string) render.Page {
	t.Helper()
	p, err := newRenderer(t).Render("courses/a.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestManifest(t *testing.T) {
	p := New()
	m := p.Manifest()
	if m.ID != "rating" || !m.Default || m.APIVersion != plugins.APIVersion || m.Version == "" {
		t.Fatalf("manifest %+v", m)
	}
	if strings.Join(m.Blocks, ",") != "rating,rating-results" {
		t.Errorf("blocks %v", m.Blocks)
	}
	if len(m.Help.Snippets) != 3 {
		t.Errorf("want three snippets, got %d", len(m.Help.Snippets))
	}
	if _, ok := p.(plugins.BlockHook); !ok {
		t.Error("not a BlockHook")
	}
	ap, ok := p.(plugins.AssetProvider)
	if !ok {
		t.Fatal("not an AssetProvider")
	}
	e := ap.Entry()
	if strings.Join(e.JS, ",") != "rating.js" || strings.Join(e.CSS, ",") != "rating.css" {
		t.Errorf("entry %+v", e)
	}
	for _, f := range []string{"rating.js", "rating.css"} {
		if _, err := fs.Stat(ap.Assets(), f); err != nil {
			t.Error(err)
		}
	}
}

func TestDefaults(t *testing.T) {
	p := page(t, ":::rating{id=course}\n:::\n")
	for _, want := range []string{
		`data-zen-plugin="rating"`, `data-zen-name="rating"`, `class="zr-rating"`, `class="zr-legend"`, "Rate this page",
		"1 of 5", "5 of 5", `class="zr-status"`, "Not rated yet",
	} {
		if !strings.Contains(p.HTML, want) {
			t.Errorf("missing %q in\n%s", want, p.HTML)
		}
	}
	if n := strings.Count(p.HTML, `class="zr-star"`); n != 5 {
		t.Errorf("%d radios, want 5", n)
	}
	if strings.Join(p.Assets.JS, ",") != "/_plugins/rating/rating.js" {
		t.Errorf("assets %+v", p.Assets)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings %v", p.Warnings)
	}
}

func TestTitleMaxLabelsAndBody(t *testing.T) {
	p := page(t, ":::rating{id=r1 title=\"How was this course?\" max=3 labels=\"Poor|Fair|Good\"}\nTell us **honestly**.\n:::\n")
	if n := strings.Count(p.HTML, `class="zr-star"`); n != 3 {
		t.Errorf("%d radios, want 3", n)
	}
	for _, want := range []string{"How was this course?", "Poor", "Fair", "Good", "<strong>honestly</strong>"} {
		if !strings.Contains(p.HTML, want) {
			t.Errorf("missing %q in\n%s", want, p.HTML)
		}
	}
	if strings.Contains(p.HTML, "1 of 3") {
		t.Errorf("a labelled star should use its label:\n%s", p.HTML)
	}
	// Without labels the hidden label is "n of max".
	p = page(t, ":::rating{id=r2 max=10}\n:::\n")
	if n := strings.Count(p.HTML, `class="zr-star"`); n != 10 || !strings.Contains(p.HTML, "10 of 10") {
		t.Errorf("max=10:\n%s", p.HTML)
	}
}

func TestBadAttributes(t *testing.T) {
	cases := map[string]string{
		"missing id":     ":::rating\n:::\n",
		"empty id":       ":::rating{id=\"\"}\n:::\n",
		"bad id":         ":::rating{id=\"bad id!\"}\n:::\n",
		"long id":        ":::rating{id=" + strings.Repeat("a", 58) + "}\n:::\n",
		"max too small":  ":::rating{id=x max=2}\n:::\n",
		"max too big":    ":::rating{id=x max=11}\n:::\n",
		"max not number": ":::rating{id=x max=many}\n:::\n",
		"few labels":     ":::rating{id=x max=3 labels=\"a|b\"}\n:::\n",
		"many labels":    ":::rating{id=x max=3 labels=\"a|b|c|d\"}\n:::\n",
	}
	for name, src := range cases {
		p := page(t, src)
		if !strings.Contains(p.HTML, "zen-plugin-error") {
			t.Errorf("%s: no error box in\n%s", name, p.HTML)
		}
		if strings.Contains(p.HTML, `class="zr-star"`) {
			t.Errorf("%s: rendered stars", name)
		}
	}
	// 57 characters is the longest id that fits the storage key "rating-<id>".
	p := page(t, ":::rating{id="+strings.Repeat("a", 57)+"}\n:::\n")
	if strings.Contains(p.HTML, "zen-plugin-error") {
		t.Errorf("a 57 character id should work:\n%s", p.HTML)
	}
}

func TestHostileTextStaysText(t *testing.T) {
	p := page(t, ":::rating{id=h title=\"<script>alert(1)</script><img src=x onerror=alert(2)>\" max=3 labels=\"<b>x</b>|<i onclick=z>y</i>|\\\"q\\\"\"}\n:::\n")
	html := regexp.MustCompile(`data-zen-attrs="[^"]*"`).ReplaceAllString(p.HTML, "") // the attribute is inert text
	for _, bad := range []string{"<script", "<img", "<b>x", "<i onclick"} {
		if strings.Contains(html, bad) {
			t.Errorf("hostile text leaked as markup %q in\n%s", bad, html)
		}
	}
	if !strings.Contains(p.HTML, "&lt;script&gt;") {
		t.Errorf("title should be escaped text:\n%s", p.HTML)
	}
}

func TestResultsBlock(t *testing.T) {
	p := page(t, ":::rating-results\n:::\n")
	if !strings.Contains(p.HTML, `data-zen-name="rating-results"`) || !strings.Contains(p.HTML, `class="zr-results"`) {
		t.Errorf("%s", p.HTML)
	}
}
