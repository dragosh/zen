package layouts

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fullManifest = `id: post
version: 0.1.0
description: Blog post with cover, date and author
template: layout.html
css: [post.css]
js: [post.js]
fields:
  date:   { type: date, description: Publication date }
  author: { type: string }
  cover:  { type: image, description: Workspace image shown on top }
  tags:   { type: list }
options:
  toc: { type: bool, default: true, description: Show the contents }
  width: { type: string, default: wide }
help:
  summary: A blog post.
  snippets:
    - title: A post
      markdown: "---\nlayout: post\n---\n# Title"
`

func fullFiles() map[string]string {
	return map[string]string{
		"layout.yml":  fullManifest,
		"layout.html": "<article>{{.Content}}</article>",
		"post.css":    "article{}",
		"post.js":     "export default { mount() {} }",
	}
}

func TestLoadFull(t *testing.T) {
	l := mustLoad(t, "post", fullFiles())
	m := l.Manifest()
	if m.ID != "post" || m.Version != "0.1.0" || m.Description == "" || m.Template != "layout.html" {
		t.Fatalf("manifest %+v", m)
	}
	if strings.Join(m.CSS, ",") != "post.css" || strings.Join(m.JS, ",") != "post.js" {
		t.Errorf("assets %+v %+v", m.CSS, m.JS)
	}
	if len(m.Fields) != 4 || m.Fields[0].Name != "author" || m.Fields[1].Name != "cover" || m.Fields[1].Type != "image" || m.Fields[1].Description == "" {
		t.Errorf("fields (sorted by name) %+v", m.Fields)
	}
	if len(m.Options) != 2 || m.Options[0].Name != "toc" || m.Options[0].Type != "bool" || m.Options[0].Default != true || m.Options[1].Default != "wide" {
		t.Errorf("options %+v", m.Options)
	}
	if m.Help.Summary != "A blog post." || len(m.Help.Snippets) != 1 || m.Help.Snippets[0].Title != "A post" {
		t.Errorf("help %+v", m.Help)
	}
	if !l.HasTemplate() || !l.HasScript() || l.Source() != SourceFolder {
		t.Errorf("template %v script %v source %q", l.HasTemplate(), l.HasScript(), l.Source())
	}
	if h, err := l.Hash(); err != nil || len(h) != 64 || len(l.AssetVersion()) != 12 {
		t.Errorf("hash %q %v version %q", h, err, l.AssetVersion())
	}
	if e := l.Entry(); strings.Join(e.CSS, ",") != "post.css" || strings.Join(e.JS, ",") != "post.js" {
		t.Errorf("entry %+v", e)
	}
}

func TestLoadCSSOnlyTemplateOnlyScriptOnly(t *testing.T) {
	css := mustLoad(t, "tidy", map[string]string{"layout.yml": "id: tidy\nversion: 1.0.0\ncss: [a.css]\n", "a.css": "p{}"})
	if css.HasTemplate() || css.HasScript() {
		t.Error("css only")
	}
	tpl := mustLoad(t, "tpl", map[string]string{"layout.yml": "id: tpl\nversion: 1.0.0\ntemplate: t.html\n", "t.html": "{{.Content}}"})
	if !tpl.HasTemplate() || tpl.HasScript() {
		t.Error("template only")
	}
	js := mustLoad(t, "run", map[string]string{"layout.yml": "id: run\nversion: 1.0.0\njs: [a.js]\n", "a.js": "export default {}"})
	if js.HasTemplate() || !js.HasScript() {
		t.Error("script only")
	}
}

func TestLoadRefusals(t *testing.T) {
	base := func(yml string, extra map[string]string) map[string]string {
		m := map[string]string{"layout.yml": yml}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name  string
		id    string
		files map[string]string
		want  string
	}{
		{"unknown key", "x", base("id: x\nversion: 1.0.0\ncss: [a.css]\nblocks: [b]\n", map[string]string{"a.css": ""}), "unknown key"},
		{"id differs from folder", "x", base("id: y\nversion: 1.0.0\ncss: [a.css]\n", map[string]string{"a.css": ""}), "must equal the folder name"},
		{"bad id", "x", base("id: X\nversion: 1.0.0\ncss: [a.css]\n", map[string]string{"a.css": ""}), "invalid id"},
		{"bad version", "x", base("id: x\nversion: one\ncss: [a.css]\n", map[string]string{"a.css": ""}), "SemVer"},
		{"nothing to show", "x", base("id: x\nversion: 1.0.0\n", nil), "template, css or js"},
		{"template missing", "x", base("id: x\nversion: 1.0.0\ntemplate: nope.html\n", nil), "not found"},
		{"css outside", "x", base("id: x\nversion: 1.0.0\ncss: [../a.css]\n", nil), "outside"},
		{"css absolute", "x", base("id: x\nversion: 1.0.0\ncss: [/etc/passwd]\n", nil), "outside"},
		{"js not a list", "x", base("id: x\nversion: 1.0.0\njs: a.js\n", map[string]string{"a.js": ""}), "js"},
		{"template parse error", "x", base("id: x\nversion: 1.0.0\ntemplate: t.html\n", map[string]string{"t.html": "{{.Content"}), "template"},
		{"template unknown function", "x", base("id: x\nversion: 1.0.0\ntemplate: t.html\n", map[string]string{"t.html": "{{readFile \"x\"}}"}), "template"},
		{"bad field type", "x", base("id: x\nversion: 1.0.0\ncss: [a.css]\nfields:\n  d: { type: colour }\n", map[string]string{"a.css": ""}), "field"},
		{"bad field name", "x", base("id: x\nversion: 1.0.0\ncss: [a.css]\nfields:\n  Bad-Name: { type: string }\n", map[string]string{"a.css": ""}), "field"},
		{"bad option type", "x", base("id: x\nversion: 1.0.0\ncss: [a.css]\noptions:\n  o: { type: float }\n", map[string]string{"a.css": ""}), "option"},
		{"bad option default", "x", base("id: x\nversion: 1.0.0\ncss: [a.css]\noptions:\n  o: { type: bool, default: 3 }\n", map[string]string{"a.css": ""}), "option"},
		{"summary too long", "x", base("id: x\nversion: 1.0.0\ncss: [a.css]\nhelp:\n  summary: " + strings.Repeat("a", 501) + "\n", map[string]string{"a.css": ""}), "summary"},
		{"no manifest", "x", map[string]string{"a.css": ""}, "layout.yml not found"},
		{"DS_Store as css", "x", base("id: x\nversion: 1.0.0\ncss: [.DS_Store]\n", map[string]string{".DS_Store": "x"}), "DS_Store"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(write(t, c.id, c.files), SourceFolder)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want %q", err, c.want)
			}
			if strings.Contains(err.Error(), os.TempDir()) {
				t.Errorf("error names an absolute path: %v", err)
			}
		})
	}
}

func TestLoadSymlinkOutsideIsRefused(t *testing.T) {
	dir := write(t, "x", map[string]string{"layout.yml": "id: x\nversion: 1.0.0\ncss: [a.css]\n"})
	outside := filepath.Join(t.TempDir(), "secret.css")
	os.WriteFile(outside, []byte("x"), 0o644)
	if err := os.Symlink(outside, filepath.Join(dir, "a.css")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, SourceFolder); err == nil {
		t.Fatal("a symlink out of the folder was accepted")
	}
}

func TestChangedAfterEditAndServedFiles(t *testing.T) {
	dir := write(t, "post", fullFiles())
	l, err := Load(dir, SourceFolder)
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	l.OnChange(func() { called++ })
	if l.Changed() {
		t.Fatal("changed right after load")
	}
	fsys := l.Assets()
	if _, err := fs.Stat(fsys, "post.css"); err != nil {
		t.Errorf("listed css not served: %v", err)
	}
	for _, name := range []string{"layout.yml", "layout.html", "../layout.yml", ".DS_Store"} {
		if _, err := fsys.Open(name); err == nil {
			t.Errorf("%s is served but not listed", name)
		}
	}
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(dir, "layout.html"), []byte("<article>changed {{.Content}}</article>"), 0o644)
	if !l.Changed() || called != 1 {
		t.Fatalf("edit of layout.html not noticed (called %d)", called)
	}
	if _, err := fsys.Open("post.css"); err == nil {
		t.Error("assets served after the layout changed")
	}
}

func TestScan(t *testing.T) {
	root := t.TempDir()
	mk := func(id, yml string, extra map[string]string) {
		for k, v := range extra {
			p := filepath.Join(root, id, k)
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte(v), 0o644)
		}
		os.MkdirAll(filepath.Join(root, id), 0o755)
		os.WriteFile(filepath.Join(root, id, "layout.yml"), []byte(yml), 0o644)
	}
	mk("b", "id: b\nversion: 1.0.0\ncss: [a.css]\n", map[string]string{"a.css": ""})
	mk("a", "id: a\nversion: 1.0.0\ncss: [a.css]\n", map[string]string{"a.css": ""})
	mk("bad", "id: bad\nversion: nope\n", nil)
	os.MkdirAll(filepath.Join(root, "empty"), 0o755) // no layout.yml: ignored
	found, invalid := Scan(root, SourceUser)
	if len(found) != 2 || found[0].Manifest().ID != "a" || found[1].Source() != SourceUser {
		t.Errorf("found %v", found)
	}
	if len(invalid) != 1 || invalid[0].ID != "bad" || invalid[0].Problem == "" {
		t.Errorf("invalid %+v", invalid)
	}
	if f, i := Scan(filepath.Join(root, "missing"), SourceUser); len(f)+len(i) != 0 {
		t.Error("a missing folder yields layouts")
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	a := mustLoad(t, "a", map[string]string{"layout.yml": "id: a\nversion: 1.0.0\ncss: [a.css]\n", "a.css": ""})
	if err := r.Register(a); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(a); err == nil || !strings.Contains(err.Error(), `"a"`) {
		t.Errorf("duplicate id: %v", err)
	}
	if r.Get("a") != a || r.Get("zzz") != nil || len(r.All()) != 1 {
		t.Error("lookup")
	}
	r.Deny("blocked")
	if !r.Denied("blocked") || r.Denied("a") {
		t.Error("deny")
	}
	if _, ok := r.AssetFS("a"); !ok {
		t.Error("asset fs of a")
	}
	if _, ok := r.AssetFS("zzz"); ok {
		t.Error("asset fs of unknown")
	}
	r.SetAssetVersion("9.9")
	if r.AssetVersion() != "9.9" {
		t.Error("asset version")
	}
}
