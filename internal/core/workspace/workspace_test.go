package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fixture creates files (path -> content) below a fresh temp dir and opens it.
func fixture(t *testing.T, files map[string]string) (*Workspace, string) {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%q): %v", dir, err)
	}
	return w, w.Root()
}

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	notes := filepath.Join(dir, "My Notes")
	if err := os.Mkdir(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(notes)
	if err != nil {
		t.Fatal(err)
	}

	w, err := Open(notes)
	if err != nil {
		t.Fatalf("Open dir: %v", err)
	}
	if w.Name() != "My Notes" {
		t.Errorf("Name() = %q", w.Name())
	}
	if w.Root() != resolved {
		t.Errorf("Root() = %q, want %q", w.Root(), resolved)
	}
	if !filepath.IsAbs(w.Root()) {
		t.Errorf("Root() not absolute: %q", w.Root())
	}

	// relative path
	t.Chdir(dir)
	w, err = Open("My Notes")
	if err != nil {
		t.Fatalf("Open relative: %v", err)
	}
	if w.Root() != resolved {
		t.Errorf("relative Root() = %q, want %q", w.Root(), resolved)
	}

	// symlink to the directory is resolved
	link := filepath.Join(dir, "link")
	if err := os.Symlink(notes, link); err != nil {
		t.Fatal(err)
	}
	w, err = Open(link)
	if err != nil {
		t.Fatalf("Open symlink: %v", err)
	}
	if w.Root() != resolved || w.Name() != "My Notes" {
		t.Errorf("symlink: Root() = %q, Name() = %q", w.Root(), w.Name())
	}

	// file path
	file := filepath.Join(dir, "a.md")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(file); err == nil {
		t.Error("Open(file) should fail")
	}
	// missing path
	if _, err := Open(filepath.Join(dir, "missing")); err == nil {
		t.Error("Open(missing) should fail")
	}
}

func TestFiles(t *testing.T) {
	w, root := fixture(t, map[string]string{
		"README.md":              "",
		"b.markdown":             "",
		"a.MD":                   "",
		"notes.txt":              "",
		"image.png":              "",
		"docs/guide.md":          "",
		"docs/deep/x.Markdown":   "",
		"docs/über uns/caf é.md": "",
		".git/config.md":         "",
		".obsidian/x.md":         "",
		"node_modules/pkg/r.md":  "",
		"vendor/lib/r.md":        "",
		"docs/node_modules/r.md": "",
		"docs/.hidden/r.md":      "",
	})

	// symlinks: one leaving the root (file + dir), one staying inside.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "leak.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "leakdir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "docs", "guide.md"), filepath.Join(root, "alias.md")); err != nil {
		t.Fatal(err)
	}

	got, err := w.Files()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"README.md",
		"a.MD",
		"alias.md",
		"b.markdown",
		"docs/deep/x.Markdown",
		"docs/guide.md",
		"docs/über uns/caf é.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Files() =\n %q\nwant\n %q", got, want)
	}

	// empty folder
	empty, _ := fixture(t, nil)
	got, err = empty.Files()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("empty Files() = %q", got)
	}
}

func TestHome(t *testing.T) {
	w, _ := fixture(t, map[string]string{"README.md": "", "a.md": ""})
	if got := w.Home(); got != "README.md" {
		t.Errorf("Home() = %q", got)
	}

	w, _ = fixture(t, map[string]string{"ReadMe.MD": ""})
	if got := w.Home(); got != "ReadMe.MD" {
		t.Errorf("Home() case = %q", got)
	}

	// only the root counts
	w, _ = fixture(t, map[string]string{"docs/README.md": "", "a.md": ""})
	if got := w.Home(); got != "" {
		t.Errorf("Home() nested = %q, want empty", got)
	}

	// a directory named README.md is not a home page
	w, _ = fixture(t, map[string]string{"README.md/x.md": ""})
	if got := w.Home(); got != "" {
		t.Errorf("Home() dir = %q, want empty", got)
	}

	w, _ = fixture(t, nil)
	if got := w.Home(); got != "" {
		t.Errorf("Home() empty = %q", got)
	}
}

func TestResolveRejectsEscapes(t *testing.T) {
	w, root := fixture(t, map[string]string{
		"a.md":           "a",
		"docs/b.md":      "b",
		"my notes/ü.md":  "u",
		"docs/img/p.png": "",
	})
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "leak.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "leakdir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "docs"), filepath.Join(root, "inner")); err != nil {
		t.Fatal(err)
	}
	// sibling directory sharing the root as a string prefix
	sibling := root + "-evil"
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sibling) })
	if err := os.WriteFile(filepath.Join(sibling, "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ok := map[string]string{
		"a.md":                "a.md",
		"docs/b.md":           "docs/b.md",
		"./docs/b.md":         "docs/b.md",
		"docs/../a.md":        "a.md",
		"docs/img/../b.md":    "docs/b.md",
		"my notes/ü.md":       "my notes/ü.md",
		"inner/b.md":          "docs/b.md", // symlink staying inside
		"docs/new.md":         "docs/new.md",
		"nodir/deeper/new.md": "nodir/deeper/new.md",
	}
	for rel, want := range ok {
		got, err := w.Resolve(rel)
		if err != nil {
			t.Errorf("Resolve(%q): %v", rel, err)
			continue
		}
		if exp := filepath.Join(root, filepath.FromSlash(want)); got != exp {
			t.Errorf("Resolve(%q) = %q, want %q", rel, got, exp)
		}
	}

	bad := []string{
		"../x.md",
		"../../etc/passwd",
		"docs/../../etc/passwd",
		"..",
		"/etc/passwd",
		filepath.Join(root, "a.md"), // absolute, even inside the root
		"a.md\x00.png",
		"\x00",
		"leak.md",
		"leakdir/secret.md",
		"leakdir/new.md",
		"../" + filepath.Base(sibling) + "/x.md",
	}
	for _, rel := range bad {
		got, err := w.Resolve(rel)
		if !errors.Is(err, ErrOutside) {
			t.Errorf("Resolve(%q) = %q, %v; want ErrOutside", rel, got, err)
		}
	}

	// the other accessors go through the same check
	if _, err := w.Read("../" + filepath.Base(sibling) + "/x.md"); !errors.Is(err, ErrOutside) {
		t.Errorf("Read escape: %v", err)
	}
	if _, err := w.Read("leak.md"); !errors.Is(err, ErrOutside) {
		t.Errorf("Read symlink escape: %v", err)
	}
	if err := w.Write("leak.md", []byte("pwn")); !errors.Is(err, ErrOutside) {
		t.Errorf("Write symlink escape: %v", err)
	}
	if err := w.Write("leakdir/new.md", []byte("pwn")); !errors.Is(err, ErrOutside) {
		t.Errorf("Write symlinked dir escape: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(outside, "secret.md")); string(b) != "s" {
		t.Errorf("outside file modified: %q", b)
	}
	if _, err := os.Stat(filepath.Join(outside, "new.md")); err == nil {
		t.Error("file created outside the root")
	}
}

func TestRead(t *testing.T) {
	w, root := fixture(t, map[string]string{
		"a.md":          "hello",
		"my notes/ü.md": "unicode",
		"docs/empty.md": "",
	})
	for rel, want := range map[string]string{"a.md": "hello", "my notes/ü.md": "unicode", "./docs/../a.md": "hello", "docs/empty.md": ""} {
		got, err := w.Read(rel)
		if err != nil || string(got) != want {
			t.Errorf("Read(%q) = %q, %v", rel, got, err)
		}
	}
	if _, err := w.Read("missing.md"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Read missing: %v", err)
	}
	if _, err := w.Read("docs"); err == nil {
		t.Error("Read(dir) should fail")
	}

	const limit = 10 << 20
	if err := os.WriteFile(filepath.Join(root, "exact.md"), bytes.Repeat([]byte("x"), limit), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := w.Read("exact.md"); err != nil || len(b) != limit {
		t.Errorf("Read exactly 10 MB: len %d, %v", len(b), err)
	}
	if err := os.WriteFile(filepath.Join(root, "big.md"), bytes.Repeat([]byte("x"), limit+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Read("big.md"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Read >10 MB: %v, want ErrTooLarge", err)
	}
}

func TestWrite(t *testing.T) {
	w, root := fixture(t, map[string]string{
		"a.md":      "old",
		"docs/b.md": "old",
		"notes.txt": "keep",
		"script.sh": "keep",
	})

	// overwrite keeps the mode
	a := filepath.Join(root, "a.md")
	if err := os.Chmod(a, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := w.Write("a.md", []byte("new content")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if b, _ := os.ReadFile(a); string(b) != "new content" {
		t.Errorf("content = %q", b)
	}
	if fi, _ := os.Stat(a); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}

	// shorter content fully replaces longer content
	if err := w.Write("a.md", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(a); string(b) != "x" {
		t.Errorf("content = %q", b)
	}

	// new files, both extensions, any case, unicode + spaces
	for _, rel := range []string{"new.md", "docs/new.markdown", "docs/UP.MD", "docs/mein text ü.md"} {
		if err := w.Write(rel, []byte("n")); err != nil {
			t.Errorf("Write(%q): %v", rel, err)
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(rel))
		if b, _ := os.ReadFile(p); string(b) != "n" {
			t.Errorf("%s content = %q", rel, b)
		}
		if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o644 {
			t.Errorf("%s: new file mode %v, %v", rel, fi.Mode().Perm(), err)
		}
	}

	// atomic: no temp files left behind
	for _, dir := range []string{root, filepath.Join(root, "docs")} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if strings.Contains(e.Name(), "tmp") {
				t.Errorf("temp file left behind: %s", e.Name())
			}
		}
	}

	// atomic: rename replaces the inode, so an already-open handle keeps the old content
	old, err := os.Open(filepath.Join(root, "docs", "b.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := w.Write("docs/b.md", []byte("replaced")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	n, _ := old.Read(buf)
	if string(buf[:n]) != "old" {
		t.Errorf("open handle saw %q; write was not a rename", buf[:n])
	}

	// non-markdown refused, existing or new
	for _, rel := range []string{"notes.txt", "script.sh", "new.html", "noext", "a.md.sh", ".md/x", "docs"} {
		if err := w.Write(rel, []byte("pwn")); !errors.Is(err, ErrNotMarkdown) {
			t.Errorf("Write(%q) = %v, want ErrNotMarkdown", rel, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(root, "notes.txt")); string(b) != "keep" {
		t.Errorf("notes.txt modified: %q", b)
	}

	// a markdown-named symlink to a non-markdown file is refused too
	if err := os.Symlink(filepath.Join(root, "script.sh"), filepath.Join(root, "sneaky.md")); err != nil {
		t.Fatal(err)
	}
	if err := w.Write("sneaky.md", []byte("pwn")); !errors.Is(err, ErrNotMarkdown) {
		t.Errorf("Write via symlink = %v, want ErrNotMarkdown", err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "script.sh")); string(b) != "keep" {
		t.Errorf("script.sh modified: %q", b)
	}

	// outside the root
	if err := w.Write("../escape.md", []byte("pwn")); !errors.Is(err, ErrOutside) {
		t.Errorf("Write escape = %v, want ErrOutside", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape.md")); err == nil {
		t.Error("file created outside root")
	}

	// a directory with a markdown name is not writable
	if err := os.Mkdir(filepath.Join(root, "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := w.Write("dir.md", []byte("x")); err == nil {
		t.Error("Write(directory) should fail")
	}
}

func TestAsset(t *testing.T) {
	files := map[string]string{
		"a.md": "md", "secret.txt": "s", "page.html": "h", "data.json": "{}", "noext": "n",
		"img/my pic ü.png": "png",
	}
	allowed := []string{"p.png", "p.jpg", "p.jpeg", "p.gif", "p.webp", "p.svg", "p.avif", "UP.PNG", "img/my pic ü.png"}
	for _, a := range allowed {
		files[a] = "data:" + a
	}
	w, root := fixture(t, files)

	for _, rel := range allowed {
		f, err := w.Asset(rel)
		if err != nil {
			t.Errorf("Asset(%q): %v", rel, err)
			continue
		}
		b := make([]byte, 64)
		n, _ := f.Read(b)
		f.Close()
		if string(b[:n]) != "data:"+rel {
			t.Errorf("Asset(%q) content = %q", rel, b[:n])
		}
	}

	for _, rel := range []string{"a.md", "secret.txt", "page.html", "data.json", "noext", "p.png.txt", "img"} {
		f, err := w.Asset(rel)
		if !errors.Is(err, ErrNotAsset) {
			t.Errorf("Asset(%q) = %v, want ErrNotAsset", rel, err)
		}
		if f != nil {
			f.Close()
		}
	}

	if _, err := w.Asset("missing.png"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Asset missing: %v", err)
	}
	if _, err := w.Asset("../x.png"); !errors.Is(err, ErrOutside) {
		t.Errorf("Asset escape: %v", err)
	}

	// image-named symlink to a non-asset is refused
	if err := os.Symlink(filepath.Join(root, "secret.txt"), filepath.Join(root, "fake.png")); err != nil {
		t.Fatal(err)
	}
	if f, err := w.Asset("fake.png"); !errors.Is(err, ErrNotAsset) {
		t.Errorf("Asset via symlink = %v, want ErrNotAsset", err)
		if f != nil {
			f.Close()
		}
	}
	// a directory with an image name
	if err := os.Mkdir(filepath.Join(root, "dir.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	if f, err := w.Asset("dir.png"); err == nil {
		f.Close()
		t.Error("Asset(directory) should fail")
	}
}

func TestResolveLink(t *testing.T) {
	w, root := fixture(t, map[string]string{
		"README.md":            "",
		"a.md":                 "",
		"notes.txt":            "",
		"logo.png":             "",
		"docs/README.md":       "",
		"docs/guide.md":        "",
		"docs/other.markdown":  "",
		"docs/img/pic one.PNG": "",
		"docs/sub/deep.md":     "",
		"docs/noreadme/x.md":   "",
		"my notes/über uns.md": "",
		"docs/v1.2/README.md":  "",
		"docs/release-1.0.md":  "",
	})
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "leak.md")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		from, href string
		want       Link
	}{
		// internal
		{"README.md", "a.md", Link{"a.md", "", "internal"}},
		{"README.md", "./a.md", Link{"a.md", "", "internal"}},
		{"README.md", "docs/guide.md", Link{"docs/guide.md", "", "internal"}},
		{"docs/guide.md", "other.markdown", Link{"docs/other.markdown", "", "internal"}},
		{"docs/guide.md", "../a.md", Link{"a.md", "", "internal"}},
		{"docs/sub/deep.md", "../../a.md", Link{"a.md", "", "internal"}},
		{"docs/sub/deep.md", "../guide.md", Link{"docs/guide.md", "", "internal"}},
		{"docs/guide.md", "./sub/../guide.md", Link{"docs/guide.md", "", "internal"}},
		// root-relative
		{"docs/sub/deep.md", "/a.md", Link{"a.md", "", "internal"}},
		{"docs/sub/deep.md", "/docs/guide.md#top", Link{"docs/guide.md", "top", "internal"}},
		// anchors
		{"README.md", "a.md#section-one", Link{"a.md", "section-one", "internal"}},
		{"docs/guide.md", "#intro", Link{"docs/guide.md", "intro", "internal"}},
		{"README.md", "a.md#", Link{"a.md", "", "internal"}},
		{"README.md", "a.md#%C3%BCber", Link{"a.md", "über", "internal"}},
		{"README.md", "a.md?raw=1#sec", Link{"a.md", "sec", "internal"}},
		// directories
		{"README.md", "docs", Link{"docs/README.md", "", "internal"}},
		{"README.md", "docs/", Link{"docs/README.md", "", "internal"}},
		{"README.md", "docs/#setup", Link{"docs/README.md", "setup", "internal"}},
		{"docs/guide.md", "..", Link{"README.md", "", "internal"}},
		{"docs/guide.md", "/", Link{"README.md", "", "internal"}},
		{"docs/guide.md", ".", Link{"docs/README.md", "", "internal"}},
		{"README.md", "docs/v1.2", Link{"docs/v1.2/README.md", "", "internal"}},
		{"README.md", "docs/noreadme", Link{"docs/noreadme", "", "broken"}},
		// extensionless
		{"README.md", "a", Link{"a.md", "", "internal"}},
		{"README.md", "docs/guide#x", Link{"docs/guide.md", "x", "internal"}},
		{"docs/guide.md", "other", Link{"docs/other.markdown", "", "internal"}},
		{"docs/guide.md", "release-1.0", Link{"docs/release-1.0.md", "", "internal"}},
		// url-encoded, unicode, spaces
		{"README.md", "my%20notes/%C3%BCber%20uns.md", Link{"my notes/über uns.md", "", "internal"}},
		{"README.md", "my notes/über uns.md", Link{"my notes/über uns.md", "", "internal"}},
		{"README.md", "my%20notes/%C3%BCber%20uns", Link{"my notes/über uns.md", "", "internal"}},
		{"my notes/über uns.md", "../docs/guide.md", Link{"docs/guide.md", "", "internal"}},
		// assets
		{"README.md", "logo.png", Link{"logo.png", "", "asset"}},
		{"docs/guide.md", "img/pic%20one.PNG", Link{"docs/img/pic one.PNG", "", "asset"}},
		{"docs/guide.md", "../logo.png", Link{"logo.png", "", "asset"}},
		{"README.md", "missing.png", Link{"missing.png", "", "broken"}},
		// external
		{"README.md", "https://example.com/a.md#x", Link{"https://example.com/a.md#x", "", "external"}},
		{"README.md", "http://example.com", Link{"http://example.com", "", "external"}},
		{"README.md", "mailto:me@example.com", Link{"mailto:me@example.com", "", "external"}},
		{"README.md", "//cdn.example.com/x.png", Link{"//cdn.example.com/x.png", "", "external"}},
		{"README.md", "file:///etc/passwd", Link{"file:///etc/passwd", "", "external"}},
		{"README.md", "javascript:alert(1)", Link{"javascript:alert(1)", "", "external"}},
		// broken
		{"README.md", "missing.md", Link{"missing.md", "", "broken"}},
		{"README.md", "missing.md#a", Link{"missing.md", "a", "broken"}},
		{"README.md", "missing", Link{"missing", "", "broken"}},
		{"README.md", "notes.txt", Link{"notes.txt", "", "broken"}},
		{"README.md", "", Link{"", "", "broken"}},
		{"README.md", "a%zz.md", Link{"a%zz.md", "", "broken"}},
		{"README.md", "a.md%00", Link{"", "", "broken"}},
		// escapes are broken, never resolved
		{"README.md", "../a.md", Link{"", "", "broken"}},
		{"README.md", "../../etc/passwd", Link{"", "", "broken"}},
		{"docs/guide.md", "../../etc/passwd", Link{"", "", "broken"}},
		{"README.md", "%2e%2e/%2e%2e/etc/passwd", Link{"", "", "broken"}},
		{"README.md", "/../etc/passwd", Link{"", "", "broken"}},
		{"README.md", "leak.md", Link{"leak.md", "", "broken"}},
	}
	for _, tt := range tests {
		if got := w.ResolveLink(tt.from, tt.href); got != tt.want {
			t.Errorf("ResolveLink(%q, %q) = %+v, want %+v", tt.from, tt.href, got, tt.want)
		}
	}
}
