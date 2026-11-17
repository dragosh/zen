package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckOut(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "notes")
	if err := os.MkdirAll(filepath.Join(root, ".zen"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, out string
		bad       bool
	}{
		{"the workspace root", root, true},
		{"a parent of the root", base, true},
		{"the grandparent", filepath.Dir(base), true},
		{"inside .zen", filepath.Join(root, ".zen", "site"), true},
		{".zen itself", filepath.Join(root, ".zen"), true},
		{"a symlink", link, true},
		{"inside the workspace", filepath.Join(root, "_site"), false},
		{"a sibling", filepath.Join(base, "site"), false},
		{"not yet existing, deeper", filepath.Join(base, "a", "b", "c"), false},
	} {
		err := CheckOut(root, c.out)
		if c.bad && err == nil {
			t.Errorf("%s: no error", c.name)
		}
		if !c.bad && err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if err != nil && strings.Contains(err.Error(), "\n") {
			t.Errorf("%s: multi-line error %q", c.name, err)
		}
	}
	// A parent reached through a symlink still counts as the root.
	if err := CheckOut(root, link); err == nil {
		t.Error("symlink to the root accepted")
	}
}

func TestOutInsideWorkspaceExcluded(t *testing.T) {
	root := t.TempDir()
	f := newFake().page("README.md", "Home", "<p>home</p>").page("_site/old.md", "Old", "<p>old</p>")
	f.root, f.home = root, "README.md"
	f.files["_site/pic.png"] = "x"
	f.pages["README.md"] = Page{Path: "README.md", Title: "Home", Layout: "article", HTML: `<p><img src="/_ws/_site/pic.png" alt="a"></p>`}
	out := filepath.Join(root, "_site")
	rep, _ := build(t, f, Options{Out: out})
	for _, p := range f.rendered {
		if strings.HasPrefix(p, "_site/") {
			t.Errorf("output folder read as input: %s", p)
		}
	}
	if has(out, "_site/old.html") || has(out, "old.html") {
		t.Error("a page of the output folder was built")
	}
	if len(f.wsAsked) != 0 {
		t.Errorf("a file of the output folder was read: %v", f.wsAsked)
	}
	if !hasWarn(rep, "inside", "left out") {
		t.Errorf("no warning about the output inside the folder: %v", rep.Warnings)
	}
	if rep.Pages != 1 {
		t.Errorf("pages = %d", rep.Pages)
	}
}

func TestCleanNeedsMarker(t *testing.T) {
	f := newFake().page("a.md", "A", "<p>a</p>")
	out := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "mine.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Build(f, Options{Out: out, SiteID: "s", Clean: true})
	if err == nil || !strings.Contains(err.Error(), out) {
		t.Fatalf("clean of a foreign folder: %v", err)
	}
	if !has(out, "mine.txt") || has(out, "a.html") {
		t.Error("the foreign folder was touched")
	}

	// Without --clean the file stays, the build adds to it, and no marker is left
	// in a folder zen did not create.
	if _, err := Build(f, Options{Out: out, SiteID: "s"}); err != nil {
		t.Fatal(err)
	}
	if !has(out, "mine.txt") || !has(out, "a.html") || has(out, ".zen-build") {
		t.Errorf("plain build into a foreign folder: %v", listing(t, out))
	}
	if _, err := Build(f, Options{Out: out, SiteID: "s", Clean: true}); err == nil {
		t.Error("marker-less folder cleaned after a plain build")
	}

	// An empty or new folder gets the marker; --clean then removes stale files.
	fresh := filepath.Join(t.TempDir(), "fresh")
	if _, err := Build(f, Options{Out: fresh, SiteID: "s"}); err != nil {
		t.Fatal(err)
	}
	if !has(fresh, ".zen-build") {
		t.Fatal("no marker in a new output folder")
	}
	if err := os.WriteFile(filepath.Join(fresh, "stale.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fresh, "old", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(f, Options{Out: fresh, SiteID: "s", Clean: true}); err != nil {
		t.Fatal(err)
	}
	if has(fresh, "stale.html") || has(fresh, "old") || !has(fresh, "a.html") || !has(fresh, ".zen-build") {
		t.Errorf("after clean: %v", listing(t, fresh))
	}
	// Without --clean, files no longer produced stay.
	if err := os.WriteFile(filepath.Join(fresh, "stale.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(f, Options{Out: fresh, SiteID: "s"}); err != nil || !has(fresh, "stale.html") {
		t.Errorf("plain rebuild removed a file: %v", err)
	}
	// An empty existing folder may be cleaned.
	empty := t.TempDir()
	if _, err := Build(f, Options{Out: empty, SiteID: "s", Clean: true}); err != nil {
		t.Errorf("clean of an empty folder: %v", err)
	}
}

func TestOutputSymlinks(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	f := newFake().page("a.md", "A", "<p>a</p>")
	if _, err := Build(f, Options{Out: link, SiteID: "s"}); err == nil {
		t.Error("an output folder that is a symlink was accepted")
	}
	if has(real, "a.html") {
		t.Error("wrote through the symlinked output folder")
	}

	// A file in the output that is a symlink is replaced, not followed; so is a
	// folder that is a symlink.
	out := filepath.Join(base, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(base, "victim.txt")
	if err := os.WriteFile(victim, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(out, "a.html")); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(base, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(out, "sub")); err != nil {
		t.Fatal(err)
	}
	f.page("sub/b.md", "B", "<p>b</p>")
	if _, err := Build(f, Options{Out: out, SiteID: "s"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "precious" {
		t.Errorf("the symlink target was overwritten: %q", b)
	}
	if fi, err := os.Lstat(filepath.Join(out, "a.html")); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("a.html is still a symlink: %v", err)
	}
	if has(elsewhere, "b.html") {
		t.Error("wrote through a symlinked folder of the output")
	}
	if !has(out, "sub/b.html") {
		t.Error("sub/b.html missing")
	}
}
