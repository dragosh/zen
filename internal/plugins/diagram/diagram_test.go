package diagram

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/render"
)

func TestManifest(t *testing.T) {
	m := New().Manifest()
	if m.ID != "diagram" || !m.Default || m.APIVersion != plugins.APIVersion || m.Version == "" {
		t.Fatalf("manifest %+v", m)
	}
	if strings.Join(m.Blocks, ",") != "mermaid,diagram" {
		t.Errorf("blocks %v", m.Blocks)
	}
	o, ok := m.Config["theme"]
	if !ok || o.Type != "string" || o.Default != "neutral" {
		t.Errorf("theme option %+v", o)
	}
	p := New()
	if _, ok := p.(plugins.BlockHook); !ok {
		t.Error("not a BlockHook")
	}
	ap, ok := p.(plugins.AssetProvider)
	if !ok {
		t.Fatal("not an AssetProvider")
	}
	e := ap.Entry()
	if strings.Join(e.JS, ",") != "diagram.js" || strings.Join(e.CSS, ",") != "diagram.css" {
		t.Errorf("entry %+v", e)
	}
	for _, f := range []string{"diagram.js", "diagram.css", "mermaid.min.js"} {
		if _, err := fs.Stat(ap.Assets(), f); err != nil {
			t.Errorf("asset %s: %v", f, err)
		}
	}
}

func TestRenderBlock(t *testing.T) {
	h := New().(plugins.BlockHook)
	cases := []struct {
		cfg  map[string]any
		want string
	}{
		{nil, "neutral"}, {map[string]any{"theme": "dark"}, "dark"},
		{map[string]any{"theme": "default"}, "default"}, {map[string]any{"theme": `x" onload="y`}, "neutral"},
	}
	for _, c := range cases {
		for _, fenced := range []bool{true, false} {
			out, err := h.RenderBlock(plugins.Context{Config: c.cfg}, plugins.Block{Name: "mermaid", Body: "graph TD\nA-->B<script>alert(1)</script>&", Fenced: fenced})
			if err != nil {
				t.Fatal(err)
			}
			want := `<div data-zen-plugin="diagram" class="zen-diagram" data-theme="` + c.want +
				`"><pre>graph TD` + "\n" + `A--&gt;B&lt;script&gt;alert(1)&lt;/script&gt;&amp;</pre></div>`
			if out != want {
				t.Errorf("got  %s\nwant %s", out, want)
			}
		}
	}
}

func newRenderer(t *testing.T) *render.Renderer {
	reg := plugins.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatal(err)
	}
	return render.New(reg, nil, nil)
}

func TestThroughRenderer(t *testing.T) {
	r := newRenderer(t)
	for _, src := range []string{"```mermaid\ngraph TD\nA-->B\n```\n", ":::diagram\ngraph TD\nA-->B\n:::\n"} {
		p, err := r.Render("a.md", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range []string{`class="zen-diagram"`, `data-theme="neutral"`, `data-zen-plugin="diagram"`, "A--&gt;B</pre>"} {
			if !strings.Contains(p.HTML, w) {
				t.Errorf("%q: missing %q in %s", src, w, p.HTML)
			}
		}
	}
}

func TestAssetsOnlyWhenUsed(t *testing.T) {
	r := newRenderer(t)
	p, _ := r.Render("a.md", []byte("# Plain\n\n```go\nx\n```\n"))
	if len(p.Assets.JS)+len(p.Assets.CSS) != 0 {
		t.Errorf("assets on plain page: %+v", p.Assets)
	}
	p, _ = r.Render("a.md", []byte("```mermaid\ngraph TD\n```\n"))
	if strings.Join(p.Assets.JS, ",") != "/_plugins/diagram/diagram.js" || strings.Join(p.Assets.CSS, ",") != "/_plugins/diagram/diagram.css" {
		t.Errorf("assets %+v", p.Assets)
	}
}

func TestVendorChecksum(t *testing.T) {
	doc, err := os.ReadFile("VENDOR.md")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile("`assets/mermaid\\.min\\.js`.*`([0-9a-f]{64})`").FindSubmatch(doc)
	if m == nil {
		t.Fatal("no checksum in VENDOR.md")
	}
	b, err := fs.ReadFile(New().(plugins.AssetProvider).Assets(), "mermaid.min.js")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != string(m[1]) {
		t.Errorf("sha256 %s, VENDOR.md says %s", got, m[1])
	}
}

func TestDiagramJS(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	out, err := exec.Command(node, "--test", "assets/diagram.test.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("node --test failed: %v\n%s", err, out)
	}
}
