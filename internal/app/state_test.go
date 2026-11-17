package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

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

