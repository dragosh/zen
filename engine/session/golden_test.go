package session

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/demo-build.golden from the current build")

// TestDemoBuildUnchanged builds examples/showcase (portable and hosted) with the
// real runtime and compares the file list and the SHA-256 of every file with
// testdata/demo-build.golden, which was generated before the runtime moved to
// runtime/. Regenerate on purpose with: go test ./engine/session -run
// TestDemoBuildUnchanged -update-golden
func TestDemoBuildUnchanged(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	demo := filepath.Join(filepath.Dir(file), "..", "..", "examples", "showcase")
	var lines []string
	for _, mode := range []string{"portable", "hosted"} {
		out := filepath.Join(t.TempDir(), "site")
		r := runBuild(t, demo, t.TempDir(), SiteOptions{Out: out, Portable: mode == "portable", Hosted: mode == "hosted"}, func(e *SiteEnv) { e.Runtime = nil; e.Version = "0.0.0-golden" }) // nil: the embedded runtime package
		if r.err != nil {
			t.Fatalf("%s: %v\n%s", mode, r.err, r.stderr)
		}
		err := filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if strings.Contains(string(b), demo) {
				t.Errorf("%s: %s mentions the absolute demo path", mode, p)
			}
			rel, _ := filepath.Rel(out, p)
			sum := sha256.Sum256(b)
			lines = append(lines, fmt.Sprintf("%s %s %s", mode, hex.EncodeToString(sum[:]), filepath.ToSlash(rel)))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"
	golden := filepath.Join("testdata", "demo-build.golden")
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
		t.Errorf("the demo build changed:\n--- want\n%s--- got\n%s", want, got)
	}
}
