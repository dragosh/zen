package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var infoDocs = map[string]string{
	"README.md":       "# Home\n",
	"docs/guide é.md": "# Guide\n\nSome text.\n",
	"notes/plain.txt": "nope",
}

func TestPageInfo(t *testing.T) {
	dir := tree(t, infoDocs)
	mod := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(dir, "docs", "guide é.md"), mod, mod); err != nil {
		t.Fatal(err)
	}
	z, _, _, _ := newTestZen(t, dir, "", "read")

	got, err := z.PageInfo("docs/guide é.md")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(dir, "docs", "guide é.md"))
	if got.Path != "docs/guide é.md" || got.AbsPath != want {
		t.Fatalf("paths: %+v, want abs %q", got, want)
	}
	if got.Size != int64(len(infoDocs["docs/guide é.md"])) {
		t.Fatalf("size %d", got.Size)
	}
	if got.ModTime != mod.UnixMilli() {
		t.Fatalf("modTime %d, want %d", got.ModTime, mod.UnixMilli())
	}
	if got.BirthTime < 0 || got.BirthTime > time.Now().Add(time.Minute).UnixMilli() {
		t.Fatalf("birthTime %d", got.BirthTime)
	}
	if runtime.GOOS == "darwin" && got.BirthTime == 0 {
		t.Fatalf("birth time expected on this platform: %+v", got)
	}
}

func TestPageInfoValidation(t *testing.T) {
	dir := tree(t, infoDocs)
	outside := tree(t, map[string]string{"x.md": "# X\n"})
	if err := os.Symlink(filepath.Join(outside, "x.md"), filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	z, _, _, _ := newTestZen(t, dir, "", "read")
	for _, p := range []string{"notes/plain.txt", "../x.md", "/etc/passwd", "link.md", "missing.md", "a\x00.md", "docs", "zen:config/global", ""} {
		_, err := z.PageInfo(p)
		if err == nil {
			t.Errorf("PageInfo(%q) succeeded", p)
			continue
		}
		if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), outside) {
			t.Errorf("PageInfo(%q) leaks a path: %v", p, err)
		}
	}

	none := New(Options{})
	defer none.Close()
	if _, err := none.PageInfo("README.md"); err == nil {
		t.Error("PageInfo without a folder succeeded")
	}
}

func TestPageInfoJSON(t *testing.T) {
	z, _, _, _ := newTestZen(t, tree(t, infoDocs), "", "read")
	info, err := z.PageInfo("README.md")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(info)
	for _, k := range []string{`"path":"README.md"`, `"absPath":`, `"size":`, `"modTime":`, `"birthTime":`} {
		if !strings.Contains(string(b), k) {
			t.Fatalf("json %s lacks %s", b, k)
		}
	}
}
