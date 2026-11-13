package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/watch"
)

type event struct {
	Name string
	Data any
}

type sink struct {
	mu  sync.Mutex
	evs []event
}

func (s *sink) Emit(name string, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, event{name, data})
}

func (s *sink) all() []event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]event(nil), s.evs...)
}

func (s *sink) reset() {
	s.mu.Lock()
	s.evs = nil
	s.mu.Unlock()
}

type fakeDialogs struct {
	folder  string
	confirm bool
	asked   []string
}

func (d *fakeDialogs) PickFolder() (string, error) { return d.folder, nil }
func (d *fakeDialogs) Confirm(msg string) bool {
	d.asked = append(d.asked, msg)
	return d.confirm
}

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for p, c := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newTestZen(t *testing.T, dir, page, mode string) (*Zen, *sink, *fakeDialogs, *[]string) {
	t.Helper()
	ev := &sink{}
	dl := &fakeDialogs{}
	var opened []string
	z, err := newZen(zenOptions{
		Registry: plugins.NewRegistry(),
		Dir:      dir,
		Page:     page,
		Mode:     mode,
		Events:   ev,
		Dialogs:  dl,
		Open:     func(u string) error { opened = append(opened, u); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	return z, ev, dl, &opened
}

var docs = map[string]string{
	"README.md":       "# Home\n\n[Guide](docs/guide.md)\n",
	"docs/guide.md":   "# Guide\n\n[Deep](deep.md) ![i](../img/a.png)\n",
	"docs/deep.md":    "# Deep\n",
	"img/a.png":       "\x89PNG\r\n\x1a\n",
	"img/x.svg":       `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
	"notes/secret.txt": "nope",
}

func TestStartup(t *testing.T) {
	dir := tree(t, docs)
	z, _, _, _ := newTestZen(t, dir, "", "read")
	s := z.Startup()
	if s.Workspace == nil || s.Page != "README.md" || s.Mode != "read" {
		t.Fatalf("startup: %+v", s)
	}
	w := s.Workspace
	if w.Home != "README.md" || w.Name != filepath.Base(dir) || len(w.Tree) != 1 || w.Tree[0].Title != "Home" ||
		len(w.Tree[0].Children) != 1 || w.Tree[0].Children[0].Path != "docs/guide.md" || w.Tree[0].Kind != "page" {
		t.Fatalf("workspace: %+v", w)
	}

	z2, _, _, _ := newTestZen(t, dir, "docs/deep.md", "edit")
	if s := z2.Startup(); s.Page != "docs/deep.md" || s.Mode != "edit" {
		t.Fatalf("explicit page: %+v", s)
	}

	z3, _, _, _ := newTestZen(t, "", "", "")
	if s := z3.Startup(); s.Workspace != nil || s.Page != "" || s.Mode != "read" {
		t.Fatalf("no workspace: %+v", s)
	}
}

func TestSwapWorkspace(t *testing.T) {
	a := tree(t, map[string]string{"README.md": "# A\n"})
	b := tree(t, map[string]string{"README.md": "# B\n", "other.md": "# Other\n"})
	z, ev, dl, _ := newTestZen(t, a, "", "read")

	dl.folder = ""
	if w, err := z.PickFolder(); err != nil || w != nil {
		t.Fatalf("cancel: %v %v", w, err)
	}
	if z.Startup().Workspace.Tree[0].Title != "A" {
		t.Fatal("cancel must keep the workspace")
	}

	dl.folder = b
	w, err := z.PickFolder()
	if err != nil || w == nil || w.Name != filepath.Base(b) || w.Tree[0].Title != "B" {
		t.Fatalf("swap: %+v %v", w, err)
	}
	if p, err := z.Page("README.md"); err != nil || p.Title != "B" {
		t.Fatalf("page after swap: %+v %v", p, err)
	}

	// The old watcher is stopped: changes in a produce nothing.
	time.Sleep(100 * time.Millisecond)
	ev.reset()
	if err := os.WriteFile(filepath.Join(a, "README.md"), []byte("# A2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if got := ev.all(); len(got) != 0 {
		t.Fatalf("old watcher still emits: %+v", got)
	}
	// The new one runs.
	if err := os.WriteFile(filepath.Join(b, "other.md"), []byte("# Other 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		found := false
		for _, e := range ev.all() {
			if e.Name == "page:changed" && reflect.DeepEqual(e.Data, PageChanged{Path: "other.md"}) {
				found = true
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("new watcher silent: %+v", ev.all())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestPageLifecycle(t *testing.T) {
	dir := tree(t, docs)
	z, _, _, _ := newTestZen(t, dir, "", "read")

	p, err := z.Page("docs/deep.md")
	if err != nil {
		t.Fatal(err)
	}
	want := []Crumb{{"Home", "README.md"}, {"Guide", "docs/guide.md"}, {"Deep", "docs/deep.md"}}
	if p.Title != "Deep" || p.Path != "docs/deep.md" || !reflect.DeepEqual(p.Breadcrumb, want) {
		t.Fatalf("page: %+v", p)
	}
	if p.Frontmatter == nil || p.Warnings == nil || p.Assets.CSS == nil || p.Assets.JS == nil {
		t.Fatalf("nil collections would be null in JSON: %+v", p)
	}
	g, _ := z.Page("docs/guide.md")
	if !strings.Contains(g.HTML, `src="/_ws/img/a.png"`) {
		t.Fatalf("image: %s", g.HTML)
	}

	src, err := z.Source("docs/deep.md")
	if err != nil || src != "# Deep\n" {
		t.Fatalf("source: %q %v", src, err)
	}
	pv, err := z.Preview("docs/deep.md", "# Changed\n")
	if err != nil || pv.Title != "Changed" || len(pv.Breadcrumb) != 3 {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	if err := z.Save("docs/deep.md", "# Saved\n"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "docs", "deep.md")); string(b) != "# Saved\n" {
		t.Fatalf("saved: %q", b)
	}
	if src, _ := z.Source("docs/deep.md"); src != "# Saved\n" {
		t.Fatalf("round trip: %q", src)
	}

	for _, bad := range []string{"../outside.md", "/etc/passwd", "missing.md", "docs/../../x.md", "notes/secret.txt"} {
		_, err1 := z.Page(bad)
		_, err2 := z.Source(bad)
		err3 := z.Save(bad, "x")
		for _, err := range []error{err1, err2, err3} {
			if err == nil {
				t.Fatalf("%s: expected error", bad)
			}
			if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), os.TempDir()) {
				t.Fatalf("%s: absolute path leaked: %v", bad, err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "notes", "secret.txt")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "notes", "secret.txt")); string(b) != "nope" {
		t.Fatal("non-markdown overwritten")
	}

	none, _, _, _ := newTestZen(t, "", "", "")
	if _, err := none.Page("README.md"); err == nil {
		t.Fatal("page without workspace must fail")
	}
}

func TestOpenExternal(t *testing.T) {
	z, _, dl, opened := newTestZen(t, "", "", "")
	for _, u := range []string{"file:///etc/passwd", "javascript:alert(1)", "zen://x", "ftp://a", "", "//evil.com", "data:text/html,x", "HTTPS"} {
		if z.OpenExternal(u) {
			t.Fatalf("%q opened", u)
		}
	}
	if len(dl.asked) != 0 || len(*opened) != 0 {
		t.Fatalf("disallowed scheme reached dialog/opener: %v %v", dl.asked, *opened)
	}

	dl.confirm = false
	if z.OpenExternal("https://example.com/a?b=c") || len(*opened) != 0 {
		t.Fatal("declined but opened")
	}
	if len(dl.asked) != 1 || !strings.Contains(dl.asked[0], "https://example.com/a?b=c") {
		t.Fatalf("dialog must show full URL: %v", dl.asked)
	}
	dl.confirm = true
	for _, u := range []string{"https://example.com", "http://example.com/x", "mailto:a@b.c", "HTTPS://EXAMPLE.COM"} {
		if !z.OpenExternal(u) {
			t.Fatalf("%q not opened", u)
		}
	}
	if !reflect.DeepEqual(*opened, []string{"https://example.com", "http://example.com/x", "mailto:a@b.c", "HTTPS://EXAMPLE.COM"}) {
		t.Fatalf("opened: %v", *opened)
	}
}

func TestWatchEvents(t *testing.T) {
	dir := tree(t, docs)
	z, ev, _, _ := newTestZen(t, dir, "", "read")
	z.state.stopWatch() // drive handle() directly, deterministically
	names := func() []string {
		var n []string
		for _, e := range ev.all() {
			n = append(n, e.Name)
		}
		return n
	}
	has := func(name string, data any) bool {
		for _, e := range ev.all() {
			if e.Name == name && (data == nil || reflect.DeepEqual(e.Data, data)) {
				return true
			}
		}
		return false
	}

	// Content change, tree unchanged: only page:changed.
	os.WriteFile(filepath.Join(dir, "docs", "deep.md"), []byte("# Deep 2\n"), 0o644)
	z.state.handle([]watch.Event{{Path: "docs/deep.md", Kind: watch.Changed}})
	if !has("page:changed", PageChanged{"docs/deep.md"}) || has("workspace:changed", nil) {
		t.Fatalf("change: %v", ev.all())
	}

	// Title change alters the tree: workspace:changed too.
	ev.reset()
	os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte("# Guide!\n\n[Deep](deep.md)\n"), 0o644)
	z.state.handle([]watch.Event{{Path: "docs/guide.md", Kind: watch.Changed}})
	if !has("workspace:changed", nil) || !has("page:changed", PageChanged{"docs/guide.md"}) {
		t.Fatalf("tree change: %v", names())
	}
	for _, e := range ev.all() {
		if e.Name == "workspace:changed" {
			if w, ok := e.Data.(*Workspace); !ok || w.Tree[0].Children[0].Title != "Guide!" {
				t.Fatalf("workspace payload: %#v", e.Data)
			}
		}
	}

	// Created file joins the tree.
	ev.reset()
	os.WriteFile(filepath.Join(dir, "new.md"), []byte("# New\n"), 0o644)
	z.state.handle([]watch.Event{{Path: "new.md", Kind: watch.Created}})
	if !has("workspace:changed", nil) {
		t.Fatalf("create: %v", names())
	}
	// Removed.
	ev.reset()
	os.Remove(filepath.Join(dir, "new.md"))
	z.state.handle([]watch.Event{{Path: "new.md", Kind: watch.Removed}})
	if !has("workspace:changed", nil) || !has("page:changed", PageChanged{"new.md"}) {
		t.Fatalf("remove: %v", names())
	}

	// Asset change: the currently served page.
	if _, err := z.Page("docs/guide.md"); err != nil {
		t.Fatal(err)
	}
	ev.reset()
	z.state.handle([]watch.Event{{Path: "img/a.png", Kind: watch.Changed}})
	if !reflect.DeepEqual(ev.all(), []event{{"page:changed", PageChanged{"docs/guide.md"}}}) {
		t.Fatalf("asset: %v", ev.all())
	}

	// .zen.yml: reload config, current page.
	ev.reset()
	os.WriteFile(filepath.Join(dir, ".zen.yml"), []byte("theme: dark\n"), 0o644)
	old := z.state.cfg
	z.state.handle([]watch.Event{{Path: ".zen.yml", Kind: watch.Changed}})
	if z.state.cfg == old || !has("page:changed", PageChanged{"docs/guide.md"}) {
		t.Fatalf("config: %v", ev.all())
	}

	// Several in one batch: page:changed once per file.
	ev.reset()
	z.state.handle([]watch.Event{{Path: "docs/deep.md", Kind: watch.Changed}, {Path: "docs/deep.md", Kind: watch.Changed}, {Path: "README.md", Kind: watch.Changed}})
	n := 0
	for _, e := range ev.all() {
		if e.Name == "page:changed" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("batch: %v", ev.all())
	}
}

