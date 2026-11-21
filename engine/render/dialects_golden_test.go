package render_test

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/builtins"
	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/dialects"
	"github.com/zenmark-org/zenmd/engine/render"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/demo-render.golden")

// demoHashes renders every page of examples/showcase and hashes the result.
func demoHashes(t *testing.T, r *render.Renderer) string {
	t.Helper()
	demo := filepath.Join("..", "..", "examples", "showcase")
	var lines []string
	err := filepath.WalkDir(demo, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(demo, p)
		pg, err := r.Render(filepath.ToSlash(rel), src)
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(pg.Title + "\x00" + pg.HTML + "\x00" + strings.Join(pg.Warnings, "\n")))
		lines = append(lines, hex.EncodeToString(sum[:])+" "+filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

// TestNoDialectsUnchanged: with no dialect enabled, rendering is what it was
// before dialects existed, byte for byte, for every page of the demo (the
// golden was generated before the render changed), also with the dialects in
// the build.
func TestNoDialectsUnchanged(t *testing.T) {
	cfg := config.Default()
	plain := render.New(builtins.Plugins(), cfg, nil)
	got := demoHashes(t, plain)
	if again := demoHashes(t, render.New(builtins.Plugins(), cfg, nil)); again != got {
		t.Fatal("rendering is not deterministic; the golden cannot work")
	}
	golden := filepath.Join("testdata", "demo-render.golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Errorf("rendering changed without dialects:\n--- want\n%s--- got\n%s", want, got)
	}
	var reg *dialects.Registry = builtins.Dialects()
	if withBuild := demoHashes(t, render.New(builtins.Plugins(), cfg, nil).WithDialects(reg)); withBuild != got {
		t.Errorf("a dialect in the build but not in the folder changed rendering:\n%s", fmt.Sprint(withBuild))
	}
}
