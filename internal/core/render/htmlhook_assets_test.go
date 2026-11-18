package render

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dragosh/zenmd/internal/core/plugins"
)

type rewriter struct{ touch bool }

func (r rewriter) Manifest() plugins.Manifest {
	return plugins.Manifest{ID: "rw", Version: "0.1.0", APIVersion: plugins.APIVersion, Default: true}
}
func (r rewriter) HTML(_ plugins.Context, h string) (string, error) {
	if r.touch {
		return strings.ReplaceAll(h, "<p>", `<p class="x">`), nil
	}
	return h, nil
}
func (rewriter) Assets() fs.FS         { return fstest.MapFS{"rw.css": {Data: []byte("p{}")}} }
func (rewriter) Entry() plugins.Assets { return plugins.Assets{CSS: []string{"rw.css"}} }

func TestHTMLHookAssetsOnlyWhenChanged(t *testing.T) {
	for _, touch := range []bool{true, false} {
		reg := plugins.NewRegistry()
		if err := reg.Register(rewriter{touch: touch}); err != nil {
			t.Fatal(err)
		}
		page, err := New(reg, nil, nil).Render("a.md", []byte("hello"))
		if err != nil {
			t.Fatal(err)
		}
		got := len(page.Assets.CSS) == 1 && page.Assets.CSS[0] == "/_plugins/rw/rw.css"
		if got != touch {
			t.Errorf("touch=%v: assets %v", touch, page.Assets)
		}
	}
}
