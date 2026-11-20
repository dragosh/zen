package session

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/watch"
)

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
		if bad == "missing.md" { // saving creates a new page
			err3 = errors.New("ok")
			os.Remove(filepath.Join(dir, "missing.md"))
		}
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

func TestSwapWorkspace(t *testing.T) {
	a := tree(t, map[string]string{"README.md": "# A\n"})
	b := tree(t, map[string]string{"README.md": "# B\n", "other.md": "# Other\n"})
	z, ev, _, _ := newTestZen(t, a, "", "read")

	// A cancelled open keeps the folder that is open.
	z.CancelOpen()
	if z.Workspace().Tree[0].Title != "A" {
		t.Fatal("cancel must keep the workspace")
	}

	if err := z.Open(b); err != nil {
		t.Fatal(err)
	}
	w := z.Workspace()
	if w == nil || w.Name != filepath.Base(b) || w.Tree[0].Title != "B" {
		t.Fatalf("swap: %+v", w)
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

func TestWatchEvents(t *testing.T) {
	dir := tree(t, docs)
	z, ev, _, _ := newTestZen(t, dir, "", "read")
	z.stopWatch() // drive handle() directly, deterministically
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
	os.WriteFile(filepath.Join(dir, "docs", "deep.md"), []byte("# Deep\n\nMore text.\n"), 0o644)
	z.handle([]watch.Event{{Path: "docs/deep.md", Kind: watch.Changed}})
	if !has("page:changed", PageChanged{"docs/deep.md"}) || has("workspace:changed", nil) {
		t.Fatalf("change: %v", ev.all())
	}

	// Title change alters the tree: workspace:changed too.
	ev.reset()
	os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte("# Guide!\n\n[Deep](deep.md)\n"), 0o644)
	z.handle([]watch.Event{{Path: "docs/guide.md", Kind: watch.Changed}})
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
	z.handle([]watch.Event{{Path: "new.md", Kind: watch.Created}})
	if !has("workspace:changed", nil) {
		t.Fatalf("create: %v", names())
	}
	// Removed.
	ev.reset()
	os.Remove(filepath.Join(dir, "new.md"))
	z.handle([]watch.Event{{Path: "new.md", Kind: watch.Removed}})
	if !has("workspace:changed", nil) || !has("page:changed", PageChanged{"new.md"}) {
		t.Fatalf("remove: %v", names())
	}

	// Asset change: the currently served page.
	if _, err := z.Page("docs/guide.md"); err != nil {
		t.Fatal(err)
	}
	ev.reset()
	z.handle([]watch.Event{{Path: "img/a.png", Kind: watch.Changed}})
	if !reflect.DeepEqual(ev.all(), []event{{"page:changed", PageChanged{"docs/guide.md"}}}) {
		t.Fatalf("asset: %v", ev.all())
	}

	// .zenmd.yml: reload config, current page.
	ev.reset()
	os.WriteFile(filepath.Join(dir, ".zenmd.yml"), []byte("theme: dark\n"), 0o644)
	old := z.cfg
	z.handle([]watch.Event{{Path: ".zenmd.yml", Kind: watch.Changed}})
	if z.cfg == old || !has("page:changed", PageChanged{"docs/guide.md"}) {
		t.Fatalf("config: %v", ev.all())
	}

	// Several in one batch: page:changed once per file.
	ev.reset()
	z.handle([]watch.Event{{Path: "docs/deep.md", Kind: watch.Changed}, {Path: "docs/deep.md", Kind: watch.Changed}, {Path: "README.md", Kind: watch.Changed}})
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

func TestNavMeta(t *testing.T) {
	dir := tree(t, map[string]string{
		"README.md":       "# Home\n\n[Guides](guides/index.md) [Bad](bad.md)\n",
		"guides/index.md": "---\nslug: guides\n---\n# Guides\n",
		"setup.md":        "---\nnav:\n  title: Setup\n  parent: guides\n---\n# Installation\n",
		"bad.md":          "---\nnav:\n  title: 3\n  parent: nowhere.md\n---\n# Bad\n",
	})
	z, ev, _, _ := newTestZen(t, dir, "", "read")
	z.stopWatch()

	p, err := z.Page("setup.md")
	if err != nil {
		t.Fatal(err)
	}
	want := []Crumb{{"Home", "README.md"}, {"Guides", "guides/index.md"}, {"Setup", "setup.md"}}
	if p.Title != "Installation" || !reflect.DeepEqual(p.Breadcrumb, want) || len(p.Warnings) != 0 {
		t.Fatalf("setup: %+v", p)
	}
	b, _ := z.Page("bad.md")
	if !hasWarn(b.Warnings, "nav.title") || !hasWarn(b.Warnings, "nowhere.md") {
		t.Fatalf("warnings: %v", b.Warnings)
	}

	// Frontmatter change: re-indexed, workspace:changed with the new tree.
	os.WriteFile(filepath.Join(dir, "setup.md"), []byte("---\nnav:\n  title: Install\n  parent: guides\n---\n# Installation\n"), 0o644)
	z.handle([]watch.Event{{Path: "setup.md", Kind: watch.Changed}})
	var w *Workspace
	for _, e := range ev.all() {
		if e.Name == "workspace:changed" {
			w = e.Data.(*Workspace)
		}
	}
	if w == nil || w.Tree[0].Children[0].Children[0].Title != "Install" {
		t.Fatalf("reindex: %+v", ev.all())
	}

	// Hidden: out of the tree, breadcrumb kept.
	ev.reset()
	os.WriteFile(filepath.Join(dir, "setup.md"), []byte("---\nnav:\n  hidden: true\n  parent: guides\n---\n# Installation\n"), 0o644)
	z.handle([]watch.Event{{Path: "setup.md", Kind: watch.Changed}})
	if w := z.Startup().Workspace; len(w.Tree[0].Children[0].Children) != 0 {
		t.Fatalf("hidden: %+v", w.Tree)
	}
	if p, _ := z.Page("setup.md"); len(p.Breadcrumb) != 3 {
		t.Fatalf("hidden crumb: %+v", p.Breadcrumb)
	}
}
