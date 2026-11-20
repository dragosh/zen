package sitetemplates

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlaceholders(t *testing.T) {
	fixedDate(t)
	dir := t.TempDir()
	tp := tmpl(t, dir, "demo", "id: demo\nname: Demo\nask:\n  title: { label: Title, default: \"{{folder}}\" }\n  author: { label: Author }\n  team: { label: Team, default: Docs }\n",
		map[string]string{
			"README.md":               "# {{title}}\nBy {{author}} on {{date}} in {{folder}} for {{team}}. {{unknown}} {{ title }} {{TITLE}}\n",
			"weeks/{{date}}.md":       "week of {{date}}\n",
			"notes/{{title}}.txt":     "{{title}}",
			"style.css":               "/* {{title}} */",
			"cfg.yaml":                "name: {{title}}\n",
			"data.json":               `{"t": "{{title}}"}`,
			"logo.png":                "\x89PNG{{title}}\x00\x01",
			"{{title}}-{{author}}.md": "x",
			"loop.md":                 "{{title}}",
		})
	target := filepath.Join(t.TempDir(), "My Site")
	res, err := Create(tp, target, map[string]string{"title": "Käse {{date}} {{author}}", "author": "Ana", "extra": "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	// One pass: a value that contains a placeholder is not expanded again. Unknown names stay.
	if got, want := read(t, target, "README.md"), "# Käse {{date}} {{author}}\nBy Ana on 2026-11-17 in My Site for Docs. {{unknown}} {{ title }} {{TITLE}}\n"; got != want {
		t.Errorf("README:\n%s\nwant\n%s", got, want)
	}
	if read(t, target, "weeks/2026-11-17.md") != "week of 2026-11-17\n" {
		t.Error("date in file name")
	}
	if read(t, target, "notes/Käse {{date}} {{author}}.txt") != "Käse {{date}} {{author}}" {
		t.Error("title in file name")
	}
	if read(t, target, "style.css") != "/* Käse {{date}} {{author}} */" || read(t, target, "cfg.yaml") != "name: Käse {{date}} {{author}}\n" {
		t.Error("css and yaml")
	}
	// Other files are copied byte for byte.
	if read(t, target, "data.json") != `{"t": "{{title}}"}` || read(t, target, "logo.png") != "\x89PNG{{title}}\x00\x01" {
		t.Error("non-text files must not change")
	}
	if _, err := os.Stat(filepath.Join(target, "template.yml")); err == nil {
		t.Error("template.yml was copied")
	}
	// Result lists the files in creation order (the template's own order), target-relative.
	wantFiles := []string{"README.md", "cfg.yaml", "data.json", "logo.png", "loop.md", "notes/Käse {{date}} {{author}}.txt", "style.css", "weeks/2026-11-17.md", "Käse {{date}} {{author}}-Ana.md"}
	if !reflect.DeepEqual(res.Files, wantFiles) {
		t.Errorf("files = %q", res.Files)
	}

	// Defaults: the folder name for title (placeholder in a default is expanded), the ask default otherwise.
	target2 := filepath.Join(t.TempDir(), "Über docs")
	if _, err := Create(tp, target2, nil); err != nil {
		t.Fatal(err)
	}
	if got := read(t, target2, "README.md"); !strings.HasPrefix(got, "# Über docs\nBy  on 2026-11-17 in Über docs for Docs.") {
		t.Errorf("defaults: %s", got)
	}
	// An empty value takes the default.
	target3 := filepath.Join(t.TempDir(), "f")
	if _, err := Create(tp, target3, map[string]string{"title": "", "team": ""}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, target3, "README.md"); !strings.HasPrefix(got, "# f\n") || !strings.Contains(got, "for Docs.") {
		t.Errorf("empty values: %s", got)
	}
	// NUL bytes in a value are refused.
	if _, err := Create(tp, filepath.Join(t.TempDir(), "z"), map[string]string{"title": "a\x00b"}); err == nil {
		t.Error("NUL accepted")
	}
	// Nothing is executed: a template with a "script" or hook file just copies it.
	tp2 := tmpl(t, dir, "hooks", "id: hooks\nname: Hooks\n", map[string]string{"hook.sh": "#!/bin/sh\ntouch {{title}}\n", "{{title}}.md": "x"})
	t4 := filepath.Join(t.TempDir(), "h")
	if _, err := Create(tp2, t4, map[string]string{"title": "$(touch pwned)"}); err != nil {
		t.Fatal(err)
	}
	if read(t, t4, "hook.sh") != "#!/bin/sh\ntouch {{title}}\n" {
		t.Error("shell file changed")
	}
	if fi, _ := os.Stat(filepath.Join(t4, "hook.sh")); fi.Mode().Perm() != 0o644 {
		t.Errorf("hook.sh perm %v", fi.Mode())
	}
}

func TestIgnoreAndEmptyAsk(t *testing.T) {
	dir := t.TempDir()
	tp := tmpl(t, dir, "demo", "id: demo\nname: Demo\nask: {}\nignore: [\"*.bak\", drafts]\n", map[string]string{
		"a.md": "a", "a.md.bak": "x", ".DS_Store": "x", "drafts/b.md": "b", "docs/.DS_Store": "x", "docs/c.md": "c",
	})
	target := t.TempDir()
	res, err := Create(tp, target, nil)
	if err != nil || !reflect.DeepEqual(res.Files, []string{"a.md", "docs/c.md"}) {
		t.Errorf("%+v %v", res, err)
	}
}

func TestCreateClashes(t *testing.T) {
	dir := t.TempDir()
	tp := tmpl(t, dir, "demo", basicManifest, map[string]string{"README.md": "# new", "a/one.md": "1", "a/two.md": "2", ".zenmd.yml": "layout: docs\n"})

	// A new folder, with spaces and unicode, is created (also several levels deep).
	base := t.TempDir()
	target := filepath.Join(base, "nested", "Dokümentation ✨")
	if _, err := Create(tp, target, nil); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(target, "a")); err != nil || fi.Mode().Perm() != 0o755 {
		t.Errorf("folder mode: %v %v", fi, err)
	}
	if fi, _ := os.Stat(filepath.Join(target, "README.md")); fi.Mode().Perm() != 0o644 {
		t.Errorf("file mode %v", fi.Mode())
	}

	// A non-empty folder without clashes is fine; existing files stay as they are.
	busy := t.TempDir()
	put(t, busy, map[string]string{"mine.md": "keep", "a/other.md": "keep"})
	if _, err := Create(tp, busy, nil); err != nil {
		t.Fatal(err)
	}
	if read(t, busy, "mine.md") != "keep" || read(t, busy, "a/other.md") != "keep" || read(t, busy, "a/one.md") != "1" {
		t.Error("merge into a busy folder")
	}

	// Any clash: nothing is written, the error lists the clashing files and wraps ErrClash.
	clash := t.TempDir()
	put(t, clash, map[string]string{"README.md": "mine", "a/two.md": "mine"})
	_, err := Create(tp, clash, nil)
	if !errors.Is(err, ErrClash) || !strings.Contains(err.Error(), "README.md") || !strings.Contains(err.Error(), "a/two.md") || strings.Contains(err.Error(), "one.md") {
		t.Fatalf("err = %v", err)
	}
	if got := walk(t, clash); !reflect.DeepEqual(got, []string{"README.md", "a/two.md"}) {
		t.Errorf("clash wrote files: %v", got)
	}
	if read(t, clash, "README.md") != "mine" {
		t.Error("overwritten")
	}

	// The list is capped at 10.
	files := map[string]string{}
	existing := map[string]string{}
	for i := 0; i < 15; i++ {
		n := string(rune('a'+i)) + ".md"
		files[n], existing[n] = "x", "mine"
	}
	big := tmpl(t, dir, "big", "id: big\nname: Big\n", files)
	many := t.TempDir()
	put(t, many, existing)
	_, err = Create(big, many, nil)
	if !errors.Is(err, ErrClash) || strings.Count(err.Error(), ".md") != 10 || !strings.Contains(err.Error(), "5 more") {
		t.Errorf("cap: %v", err)
	}

	// A clash with a directory, and with a path whose parent is a file, count too.
	odd := t.TempDir()
	put(t, odd, map[string]string{"README.md/x": "dir where a file goes", "a": "file where a folder goes"})
	if _, err := Create(tp, odd, nil); !errors.Is(err, ErrClash) {
		t.Errorf("odd: %v", err)
	}
	// A symlinked folder inside the target is never written through.
	elsewhere := t.TempDir()
	linked := t.TempDir()
	if err := os.Symlink(elsewhere, filepath.Join(linked, "a")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(tp, linked, nil); err == nil || len(walk(t, elsewhere)) != 0 {
		t.Errorf("write through symlink: %v %v", err, walk(t, elsewhere))
	}
	// The target must not be a file.
	f := filepath.Join(t.TempDir(), "file")
	put(t, filepath.Dir(f), map[string]string{"file": "x"})
	if _, err := Create(tp, f, nil); err == nil {
		t.Error("target is a file")
	}
	// The templates dir itself, and anything inside it, is refused.
	for _, bad := range []string{dir, filepath.Join(dir, "sites"), filepath.Join(dir, "sites", "demo"), filepath.Join(dir, "sites", "new-one")} {
		if _, err := Create(tp, bad, nil); err == nil || !strings.Contains(err.Error(), "templates folder") {
			t.Errorf("target %s: %v", bad, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "sites", "new-one")); err == nil {
		t.Error("created inside the templates dir")
	}
	// Two template files that end up with the same name are refused.
	dup := tmpl(t, dir, "dup", "id: dup\nname: Dup\nask: { title: {} }\n", map[string]string{"{{title}}.md": "1", "x.md": "2"})
	if _, err := Create(dup, filepath.Join(t.TempDir(), "d"), map[string]string{"title": "x"}); err == nil || !strings.Contains(err.Error(), "x.md") {
		t.Errorf("duplicate names: %v", err)
	}
	// Creating into a folder that is open in zen is just a busy folder: nothing special.
	if _, err := Create(tp, busy, nil); !errors.Is(err, ErrClash) {
		t.Errorf("second run into the same folder must clash: %v", err)
	}
}

func TestCreateContainment(t *testing.T) {
	dir := t.TempDir()
	// A symlink inside a template is refused, whatever it points at.
	tp := tmpl(t, dir, "demo", basicManifest, map[string]string{"a.md": "a"})
	outside := t.TempDir()
	put(t, outside, map[string]string{"secret.md": "s"})
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(tp.Dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "t")
	if _, err := Create(tp, target, nil); err == nil || !strings.Contains(err.Error(), "link.md") || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("symlink file: %v", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("something was written before the symlink was found")
	}
	os.Remove(filepath.Join(tp.Dir, "link.md"))
	if err := os.Symlink(outside, filepath.Join(tp.Dir, "linkdir")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(tp, target, nil); err == nil || !strings.Contains(err.Error(), "linkdir") {
		t.Errorf("symlink dir: %v", err)
	}
	os.Remove(filepath.Join(tp.Dir, "linkdir"))

	// ".." and absolute names from placeholders are refused, with nothing written.
	esc := tmpl(t, dir, "esc", "id: esc\nname: Esc\nask: { title: {} }\n", map[string]string{"{{title}}/x.md": "x", "ok.md": "ok"})
	for _, title := range []string{"..", "../..", "../../out", "a/../..", "/abs"} {
		tgt := filepath.Join(t.TempDir(), "site")
		_, err := Create(esc, tgt, map[string]string{"title": title})
		if err == nil || !strings.Contains(err.Error(), "outside") {
			t.Errorf("title %q: %v", title, err)
		}
		if _, e := os.Stat(tgt); e == nil {
			t.Errorf("title %q: target was created", title)
		}
		if _, e := os.Stat(filepath.Join(filepath.Dir(tgt), "out")); e == nil {
			t.Errorf("title %q: escaped", title)
		}
	}
	// A value with a slash that stays inside makes a subfolder, which is allowed.
	tgt := filepath.Join(t.TempDir(), "site")
	if _, err := Create(esc, tgt, map[string]string{"title": "sub/dir"}); err != nil {
		t.Errorf("inside: %v", err)
	}
	// A backslash is an ordinary character on macOS but never a separator for us.
	// Special files (fifo) are refused.
	fifoT := tmpl(t, dir, "fifo", "id: fifo\nname: Fifo\n", map[string]string{"a.md": "a"})
	if err := mkfifo(filepath.Join(fifoT.Dir, "pipe")); err == nil {
		if _, err := Create(fifoT, filepath.Join(t.TempDir(), "f"), nil); err == nil || !strings.Contains(err.Error(), "pipe") {
			t.Errorf("fifo: %v", err)
		}
	}
}

func TestLimits(t *testing.T) {
	oldF, oldB := MaxFiles, MaxBytes
	t.Cleanup(func() { MaxFiles, MaxBytes = oldF, oldB })
	dir := t.TempDir()
	tp := tmpl(t, dir, "demo", basicManifest, map[string]string{"a.md": "12345", "b.md": "12345", "c.md": "12345"})

	MaxFiles = 2
	target := filepath.Join(t.TempDir(), "t")
	if _, err := Create(tp, target, nil); err == nil || !strings.Contains(err.Error(), "more than 2 files") {
		t.Errorf("count: %v", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("written despite the count limit")
	}
	MaxFiles = 10
	MaxBytes = 14
	if _, err := Create(tp, target, nil); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("size: %v", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("written despite the size limit")
	}
	MaxBytes = 15
	if _, err := Create(tp, target, nil); err != nil {
		t.Errorf("at the limit: %v", err)
	}
	// The limits are not hit by the manifest, and the real defaults are as specified.
	if oldF != 2000 || oldB != 50<<20 {
		t.Errorf("defaults %d %d", oldF, oldB)
	}
}

func TestCreateRollsBackOnFailure(t *testing.T) {
	dir := t.TempDir()
	tp := tmpl(t, dir, "demo", basicManifest, map[string]string{"a.md": "a", "z/b.md": "b"})
	// Make a file unreadable after listing: the copy fails halfway and removes what it wrote.
	if err := os.Chmod(filepath.Join(tp.Dir, "z", "b.md"), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(tp.Dir, "z", "b.md"), 0o644)
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	target := filepath.Join(t.TempDir(), "t")
	if _, err := Create(tp, target, nil); err == nil {
		t.Fatal("want an error")
	}
	if _, err := os.Stat(target); err == nil {
		t.Errorf("partial site left behind: %v", walk(t, target))
	}
}
