package session

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var navDocs = map[string]string{
	"README.md":      "# Home\n\n[A](a.md) [B](b.md) [C](c.md) [S](sub/s.md) [T](sub/t.md)\n",
	"a.md":           "# A\n\nalpha\n",
	"b.md":           "---\r\ntitle: B\r\nnav:\r\n  title: Bee # label\r\n---\r\n# B\r\n",
	"c.md":           "# C\n",
	"sub/s.md":       "# S\n",
	"sub/t.md":       "# T\n",
	"loose/x.md":     "# X\n",
	"loose/y.md":     "# Y\n",
	"secret.md":      "---\nnav:\n  hidden: true\n---\n# Secret\n",
	"notes/data.txt": "no",
}

func titles(ns []NavNode) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Path)
	}
	return out
}

func childrenOf(t *testing.T, s *Session, path string) []string {
	t.Helper()
	var find func([]NavNode) *NavNode
	find = func(ns []NavNode) *NavNode {
		for i := range ns {
			if ns[i].Path == path {
				return &ns[i]
			}
			if r := find(ns[i].Children); r != nil {
				return r
			}
		}
		return nil
	}
	n := find(s.Workspace().Tree)
	if n == nil {
		t.Fatalf("%s is not in the tree", path)
	}
	return titles(n.Children)
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func changedEvents(ev *sink) int {
	n := 0
	for _, e := range ev.all() {
		if e.name == EventWorkspaceChanged {
			n++
		}
	}
	return n
}

func TestReorderNav(t *testing.T) {
	dir := tree(t, navDocs)
	z, ev, _, _ := newTestZen(t, dir, "", "")
	if got := childrenOf(t, z, "README.md"); !reflect.DeepEqual(got, []string{"a.md", "b.md", "c.md", "sub/s.md", "sub/t.md"}) {
		t.Fatalf("start: %v", got)
	}
	ev.reset()

	if err := z.ReorderNav("README.md", []string{"c.md", "a.md", "sub/t.md", "b.md", "sub/s.md"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"c.md", "a.md", "sub/t.md", "b.md", "sub/s.md"}
	if got := childrenOf(t, z, "README.md"); !reflect.DeepEqual(got, want) {
		t.Fatalf("tree: %v", got)
	}
	// Numbered 10, 20, 30, ...; the rest of each file is as it was.
	if got := read(t, dir, "c.md"); got != "---\nnav:\n  order: 10\n---\n# C\n" {
		t.Errorf("c.md: %q", got)
	}
	if got := read(t, dir, "a.md"); got != "---\nnav:\n  order: 20\n---\n# A\n\nalpha\n" {
		t.Errorf("a.md: %q", got)
	}
	if got := read(t, dir, "b.md"); got != "---\r\ntitle: B\r\nnav:\r\n  title: Bee # label\r\n  order: 40\r\n---\r\n# B\r\n" {
		t.Errorf("b.md (CRLF, comment kept): %q", got)
	}
	if changedEvents(ev) != 1 {
		t.Errorf("workspace:changed sent %d times, want 1 (without waiting for the watcher)", changedEvents(ev))
	}

	// Only pages whose number changes are written.
	before := map[string]string{}
	for _, p := range want {
		before[p] = read(t, dir, p)
	}
	if err := z.ReorderNav("README.md", []string{"c.md", "a.md", "sub/s.md", "sub/t.md", "b.md"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"c.md", "a.md"} {
		if read(t, dir, p) != before[p] {
			t.Errorf("%s was rewritten but its number did not change", p)
		}
	}
	if got := childrenOf(t, z, "README.md"); !reflect.DeepEqual(got, []string{"c.md", "a.md", "sub/s.md", "sub/t.md", "b.md"}) {
		t.Errorf("tree: %v", got)
	}
	// The same order again writes nothing and sends nothing.
	ev.reset()
	if err := z.ReorderNav("README.md", []string{"c.md", "a.md", "sub/s.md", "sub/t.md", "b.md"}); err != nil || changedEvents(ev) != 0 {
		t.Errorf("same order: %v, %d events", err, changedEvents(ev))
	}
}

func TestReorderNavTopLevelAndFolderGroups(t *testing.T) {
	dir := tree(t, map[string]string{"one.md": "# One\n", "two.md": "# Two\n", "three.md": "# Three\n", "dir/p.md": "# P\n", "dir/q.md": "# Q\n"})
	z, _, _, _ := newTestZen(t, dir, "", "")
	if got := titles(z.Workspace().Tree); !reflect.DeepEqual(got, []string{"one.md", "three.md", "two.md", ""}) {
		t.Fatalf("start: %v", got)
	}
	if err := z.ReorderNav("", []string{"two.md", "three.md", "one.md"}); err != nil {
		t.Fatal(err)
	}
	if got := titles(z.Workspace().Tree)[:3]; !reflect.DeepEqual(got, []string{"two.md", "three.md", "one.md"}) {
		t.Errorf("top level: %v", got)
	}
	// A folder inside "Other pages": its parent is the group's path.
	if err := z.ReorderNav("dir", []string{"dir/q.md", "dir/p.md"}); err != nil {
		t.Fatal(err)
	}
	other := z.Workspace().Tree[3]
	if got := titles(other.Children[0].Children); !reflect.DeepEqual(got, []string{"dir/q.md", "dir/p.md"}) {
		t.Errorf("group: %v", got)
	}
}

func TestReorderNavRejects(t *testing.T) {
	dir := tree(t, navDocs)
	z, ev, _, _ := newTestZen(t, dir, "", "")
	ev.reset()
	snapshot := func() map[string]string {
		m := map[string]string{}
		for p := range navDocs {
			m[p] = read(t, dir, p)
		}
		return m
	}
	before := snapshot()
	all := []string{"a.md", "b.md", "c.md", "sub/s.md", "sub/t.md"}
	for name, c := range map[string]struct {
		parent string
		paths  []string
	}{
		"empty":              {"README.md", nil},
		"missing a sibling":  {"README.md", []string{"a.md", "b.md"}},
		"duplicate":          {"README.md", []string{"a.md", "a.md", "b.md", "c.md", "sub/s.md"}},
		"not in the tree":    {"README.md", []string{"a.md", "b.md", "c.md", "sub/s.md", "nope.md"}},
		"hidden page":        {"README.md", []string{"a.md", "b.md", "c.md", "sub/s.md", "secret.md"}},
		"another parent":     {"README.md", []string{"a.md", "b.md", "c.md", "sub/s.md", "loose/x.md"}},
		"wrong parent name":  {"a.md", all},
		"parent is a folder": {"sub", all},
		"outside":            {"README.md", []string{"../a.md", "b.md", "c.md", "sub/s.md", "sub/t.md"}},
		"absolute":           {"README.md", []string{"/etc/passwd", "b.md", "c.md", "sub/s.md", "sub/t.md"}},
		"a group":            {"", []string{"loose", "README.md"}},
		"not markdown":       {"README.md", []string{"notes/data.txt", "b.md", "c.md", "sub/s.md", "sub/t.md"}},
		"dot dot":            {"README.md", []string{"sub/../a.md", "b.md", "c.md", "sub/s.md", "sub/t.md"}},
	} {
		if err := z.ReorderNav(c.parent, c.paths); err == nil {
			t.Errorf("%s: no error", name)
		} else if strings.Contains(err.Error(), dir) {
			t.Errorf("%s: the error names the folder path: %v", name, err)
		}
	}
	if !reflect.DeepEqual(snapshot(), before) {
		t.Error("a rejected call changed a file")
	}
	if changedEvents(ev) != 0 {
		t.Error("a rejected call sent an event")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "a.md")); err == nil {
		t.Error("wrote outside the folder")
	}
}

func TestReorderNavNoFolder(t *testing.T) {
	s := New(Options{})
	t.Cleanup(s.Close)
	if err := s.ReorderNav("", []string{"a.md"}); err == nil {
		t.Error("ReorderNav without a folder")
	}
	if err := s.SetNavHidden("a.md", true); err == nil {
		t.Error("SetNavHidden without a folder")
	}
	if _, err := s.AbsPath("a.md"); err == nil {
		t.Error("AbsPath without a folder")
	}
}

func TestReorderNavRollsBack(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := tree(t, navDocs)
	z, ev, _, _ := newTestZen(t, dir, "", "")
	ev.reset()
	locked := filepath.Join(dir, "sub")
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	// c.md is written first (10), then sub/s.md (20) fails: c.md goes back.
	err := z.ReorderNav("README.md", []string{"c.md", "sub/s.md", "a.md", "b.md", "sub/t.md"})
	if err == nil {
		t.Fatal("no error although sub/ cannot be written")
	}
	for p, want := range navDocs {
		if got := read(t, dir, p); got != want {
			t.Errorf("%s was left changed: %q", p, got)
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left: %s", e.Name())
		}
	}
}

func TestReorderNavKeepsBytesAndRefusesOdd(t *testing.T) {
	dir := tree(t, map[string]string{
		"README.md": "# Home\n\n[A](a.md) [B](b.md)\n",
		"a.md":      "\xEF\xBB\xBF---\ntitle: A\nnav: 5\n---\n# A\n",
		"b.md":      "# B\n",
	})
	z, _, _, _ := newTestZen(t, dir, "", "")
	err := z.ReorderNav("README.md", []string{"b.md", "a.md"})
	if err == nil || !strings.Contains(err.Error(), "a.md") {
		t.Fatalf("a page with a nav that cannot be edited: %v", err)
	}
	if read(t, dir, "b.md") != "# B\n" || !strings.HasPrefix(read(t, dir, "a.md"), "\xEF\xBB\xBF---\ntitle: A\nnav: 5") {
		t.Error("files changed although the call failed")
	}
}

func TestSetNavHidden(t *testing.T) {
	dir := tree(t, navDocs)
	z, ev, _, _ := newTestZen(t, dir, "", "")
	ev.reset()
	if err := z.SetNavHidden("c.md", true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "c.md"); got != "---\nnav:\n  hidden: true\n---\n# C\n" {
		t.Errorf("c.md: %q", got)
	}
	if got := childrenOf(t, z, "README.md"); !reflect.DeepEqual(got, []string{"a.md", "b.md", "sub/s.md", "sub/t.md"}) {
		t.Errorf("tree: %v", got)
	}
	if changedEvents(ev) != 1 {
		t.Errorf("workspace:changed %d times", changedEvents(ev))
	}
	// A hidden page is no longer in the sidebar: it can be shown again by path (the file is the truth).
	if err := z.SetNavHidden("secret.md", false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "secret.md"); got != "# Secret\n" {
		t.Errorf("secret.md: %q", got)
	}
	// Setting it again changes nothing.
	if err := z.SetNavHidden("c.md", true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "c.md"); got != "---\nnav:\n  hidden: true\n---\n# C\n" {
		t.Errorf("c.md twice: %q", got)
	}
	for _, bad := range []string{"", "../a.md", "/etc/passwd", "notes/data.txt", "nope.md", "sub", "x\x00.md"} {
		if err := z.SetNavHidden(bad, true); err == nil {
			t.Errorf("SetNavHidden(%q) did not fail", bad)
		}
	}
}

func TestAbsPath(t *testing.T) {
	dir := tree(t, navDocs)
	z, _, _, _ := newTestZen(t, dir, "", "")
	real, _ := filepath.EvalSymlinks(dir)
	for rel, want := range map[string]string{"a.md": filepath.Join(real, "a.md"), "sub": filepath.Join(real, "sub"), "": real} {
		got, err := z.AbsPath(rel)
		if err != nil || got != want {
			t.Errorf("AbsPath(%q) = %q, %v; want %q", rel, got, err, want)
		}
	}
	for _, bad := range []string{"../x", "/etc", "sub/../../x", "missing.md", "x\x00"} {
		if got, err := z.AbsPath(bad); err == nil {
			t.Errorf("AbsPath(%q) = %q, want an error", bad, got)
		}
	}
}
