package site

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// fake is an Input over maps.
type fake struct {
	root    string
	pages   map[string]Page
	tree    []NavNode
	home    string
	slugs   map[string]string
	files   map[string]string // workspace files by path
	assets  map[string]string // plugin and layout files by URL without ?v=
	js, css string

	rendered []string // paths Render was asked for
	wsAsked  []string
	assAsked []string
}

func newFake() *fake {
	return &fake{pages: map[string]Page{}, slugs: map[string]string{}, files: map[string]string{}, assets: map[string]string{}, js: "/*zen.js*/", css: "/*zen.css*/"}
}

// page adds a page and a leaf node for it (unless tree was set by hand).
func (f *fake) page(path, title, html string) *fake {
	f.pages[path] = Page{Path: path, Title: title, HTML: html, Layout: "article"}
	return f
}

func (f *fake) with(path string, edit func(*Page)) *fake {
	p := f.pages[path]
	edit(&p)
	f.pages[path] = p
	return f
}

func (f *fake) Pages() ([]string, error) {
	var out []string
	for p := range f.pages {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}
func (f *fake) Home() string { return f.home }
func (f *fake) Root() string { return f.root }
func (f *fake) Tree() []NavNode {
	if f.tree != nil {
		return f.tree
	}
	ps, _ := f.Pages()
	var out []NavNode
	for _, p := range ps {
		out = append(out, NavNode{Title: f.pages[p].Title, Path: p, Kind: "page"})
	}
	return out
}
func (f *fake) Render(path string) (Page, error) {
	f.rendered = append(f.rendered, path)
	p, ok := f.pages[path]
	if !ok {
		return Page{}, errors.New("no such page")
	}
	return p, nil
}
func (f *fake) Slug(s string) (string, bool) { p, ok := f.slugs[s]; return p, ok }
func (f *fake) WorkspaceFile(path string) (fs.File, error) {
	f.wsAsked = append(f.wsAsked, path)
	return open(f.files, path)
}
func (f *fake) Asset(url string) (fs.File, error) {
	f.assAsked = append(f.assAsked, url)
	u, _, _ := strings.Cut(url, "?")
	return open(f.assets, u)
}
func (f *fake) Runtime() (js, css []byte) { return []byte(f.js), []byte(f.css) }

func open(m map[string]string, key string) (fs.File, error) {
	c, ok := m[key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	fsys := fstest.MapFS{"f": {Data: []byte(c)}}
	return fsys.Open("f")
}

// build runs Build into a fresh folder and returns the report and the folder.
func build(t *testing.T, in Input, o Options) (Report, string) {
	t.Helper()
	if o.Out == "" {
		o.Out = filepath.Join(t.TempDir(), "out")
	}
	if o.SiteID == "" {
		o.SiteID = "site"
	}
	rep, err := Build(in, o)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return rep, o.Out
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%v", err)
	}
	return string(b)
}

func has(dir, rel string) bool {
	_, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil
}

// listing returns every file under dir (relative, sorted) with its content.
func listing(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func contains(t *testing.T, what, s string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("%s lacks %q:\n%s", what, w, s)
		}
	}
}

func lacks(t *testing.T, what, s string, bad ...string) {
	t.Helper()
	for _, w := range bad {
		if strings.Contains(s, w) {
			t.Errorf("%s has %q:\n%s", what, w, s)
		}
	}
}

func hasWarn(rep Report, parts ...string) bool {
	for _, w := range rep.Warnings {
		ok := true
		for _, p := range parts {
			ok = ok && strings.Contains(w, p)
		}
		if ok {
			return true
		}
	}
	return false
}

var _ = bytes.Equal

var errBoom = errors.New("boom")
