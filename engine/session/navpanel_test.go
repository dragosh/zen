package session

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceLogo(t *testing.T) {
	files := map[string]string{
		"README.md":      "# Home\n",
		"img/logo.svg":   `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"img/logo.png":   "\x89PNG\r\n\x1a\n",
		"notes/data.txt": "no",
	}
	cases := []struct{ name, yml, want string }{
		{"none", "", ""},
		{"valid svg", "logo: img/logo.svg\n", "/_ws/img/logo.svg"},
		{"valid png", "logo: img/logo.png\n", "/_ws/img/logo.png"},
		{"missing", "logo: img/nope.svg\n", ""},
		{"outside", "logo: ../logo.svg\n", ""},
		{"absolute", "logo: /etc/passwd.png\n", ""},
		{"not an image", "logo: notes/data.txt\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := tree(t, files)
			if c.yml != "" {
				if err := os.WriteFile(filepath.Join(dir, ".zenmd.yml"), []byte(c.yml), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			z, _, _, _ := newTestZen(t, dir, "", "")
			if got := z.Workspace().Logo; got != c.want {
				t.Fatalf("Logo = %q, want %q", got, c.want)
			}
			if c.want != "" {
				rec := httptest.NewRecorder()
				z.Handler().ServeHTTP(rec, httptest.NewRequest("GET", c.want, nil))
				if rec.Code != 200 || rec.Header().Get("Content-Security-Policy") != "sandbox" {
					t.Fatalf("serve: %d %v", rec.Code, rec.Header())
				}
			}
		})
	}
}

func TestWorkspaceLogoSymlinkOut(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "o.png"), []byte("\x89PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := tree(t, map[string]string{"README.md": "# Home\n"})
	if err := os.Symlink(filepath.Join(outside, "o.png"), filepath.Join(dir, "logo.png")); err != nil {
		t.Skip("no symlinks")
	}
	if err := os.WriteFile(filepath.Join(dir, ".zenmd.yml"), []byte("logo: logo.png\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	z, _, _, _ := newTestZen(t, dir, "", "")
	if got := z.Workspace().Logo; got != "" {
		t.Fatalf("a symlink out of the folder gave %q", got)
	}
}

func TestWorkspaceTitle(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"heading of the home page", map[string]string{"README.md": "# My Course\n\ntext\n"}, "My Course"},
		{"readme.md in lower case", map[string]string{"readme.md": "# Welcome\n"}, "Welcome"},
		{"no home page", map[string]string{"a.md": "# A\n", "b.md": "# B\n"}, ""}, // the folder's name, below
		{"home without a heading", map[string]string{"README.md": "just text\n"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := tree(t, c.files)
			z, _, _, _ := newTestZen(t, dir, "", "")
			ws := z.Workspace()
			want := c.want
			if want == "" {
				want = ws.Name
			}
			if ws.Title != want {
				t.Fatalf("Title = %q, want %q (home %q)", ws.Title, want, ws.Home)
			}
		})
	}
}

func TestWorkspaceTitleLive(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# Old\n"})
	z, ev, _, _ := newTestZen(t, dir, "", "")
	if z.Workspace().Title != "Old" {
		t.Fatalf("start: %q", z.Workspace().Title)
	}
	ev.reset()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the workspace change", func() bool { return changedEvents(ev) > 0 })
	if got := z.Workspace().Title; got != "New" {
		t.Fatalf("after the edit: %q", got)
	}
	var last *Workspace
	for _, e := range ev.all() {
		if e.Name == EventWorkspaceChanged {
			last, _ = e.Data.(*Workspace)
		}
	}
	if last == nil || !strings.Contains(last.Title, "New") {
		t.Fatalf("the event did not carry the title: %+v", last)
	}
}
