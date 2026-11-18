package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dragosh/zenmd/internal/core/lint"
)

// fakeLinter records its calls and returns canned alerts.
type fakeLinter struct {
	mu     sync.Mutex
	calls  []lintCall
	alerts []lint.Alert
	err    error
}

type lintCall struct{ root, path, content string }

func (f *fakeLinter) Lint(_ context.Context, root, path string, content []byte) ([]lint.Alert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, lintCall{root, path, string(content)})
	return f.alerts, f.err
}

func (f *fakeLinter) all() []lintCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]lintCall(nil), f.calls...)
}

func withLinter(t *testing.T, l textLinter) {
	t.Helper()
	restore := setLinter(l)
	t.Cleanup(restore)
}

var lintDocs = map[string]string{
	"README.md":       "# Home\n\nTeh text.\n",
	"docs/guide.md":   "# Guide\n",
	"off/page.md":     "---\nplugins:\n  lint: false\n---\n# Off\n",
	"notes/plain.txt": "nope",
}

func TestLintUnavailable(t *testing.T) {
	withLinter(t, nil)
	z, _, _, _ := newTestZen(t, tree(t, lintDocs), "", "read")
	r, err := z.Lint("README.md", "")
	if err != nil || r.Available || r.Alerts == nil || len(r.Alerts) != 0 {
		t.Fatalf("no vale: %+v %v", r, err)
	}

	f := &fakeLinter{err: lint.ErrUnavailable}
	withLinter(t, f)
	if r, err := z.Lint("README.md", ""); err != nil || r.Available {
		t.Fatalf("ErrUnavailable from Lint: %+v %v", r, err)
	}
}

func TestLintDiskAndUnsaved(t *testing.T) {
	f := &fakeLinter{alerts: []lint.Alert{{Line: 3, Col: 1, EndCol: 3, Severity: "error", Check: "Vale.Spelling", Message: "Did you really mean 'Teh'?", Match: "Teh"}}}
	withLinter(t, f)
	dir := tree(t, lintDocs)
	z, _, _, _ := newTestZen(t, dir, "", "read")

	r, err := z.Lint("README.md", "")
	if err != nil || !r.Available || len(r.Alerts) != 1 {
		t.Fatalf("disk: %+v %v", r, err)
	}
	a := r.Alerts[0]
	if a.Line != 3 || a.Col != 1 || a.EndCol != 3 || a.Severity != "error" || a.Check != "Vale.Spelling" || a.Match != "Teh" {
		t.Fatalf("alert: %+v", a)
	}
	calls := f.all()
	root, _ := filepath.EvalSymlinks(dir)
	if len(calls) != 1 || calls[0].path != "README.md" || calls[0].content != lintDocs["README.md"] {
		t.Fatalf("calls: %+v", calls)
	}
	if got, _ := filepath.EvalSymlinks(calls[0].root); got != root {
		t.Fatalf("root %q, want %q", calls[0].root, root)
	}

	if _, err := z.Lint("docs/guide.md", "unsaved words"); err != nil {
		t.Fatal(err)
	}
	if c := f.all()[1]; c.content != "unsaved words" || c.path != "docs/guide.md" {
		t.Fatalf("unsaved: %+v", c)
	}
	// A page that does not exist yet can be linted with its unsaved text.
	if _, err := z.Lint("docs/new.md", "new"); err != nil {
		t.Fatalf("new page: %v", err)
	}

	// JSON shape of the Service API.
	b, _ := json.Marshal(r)
	for _, k := range []string{`"available":true`, `"alerts":[`, `"line":3`, `"col":1`, `"endCol":3`, `"severity":"error"`, `"check":"Vale.Spelling"`, `"message":`, `"match":"Teh"`} {
		if !strings.Contains(string(b), k) {
			t.Fatalf("json %s lacks %s", b, k)
		}
	}
}

func TestLintPathValidation(t *testing.T) {
	f := &fakeLinter{}
	withLinter(t, f)
	dir := tree(t, lintDocs)
	outside := tree(t, map[string]string{"x.md": "# X\n"})
	if err := os.Symlink(filepath.Join(outside, "x.md"), filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	z, _, _, _ := newTestZen(t, dir, "", "read")
	for _, p := range []string{"notes/plain.txt", "../x.md", "/etc/passwd", "link.md", "missing.md"} {
		for _, content := range []string{"", "text"} {
			if p == "missing.md" && content != "" {
				continue // unsaved text for a new page is fine
			}
			if _, err := z.Lint(p, content); err == nil {
				t.Errorf("Lint(%q, %q) must fail", p, content)
			} else if strings.Contains(err.Error(), dir) {
				t.Errorf("error leaks the root: %v", err)
			}
		}
	}
	if n := len(f.all()); n != 0 {
		t.Fatalf("linter ran %d times for invalid paths", n)
	}

	none, _, _, _ := newTestZen(t, "", "", "")
	if _, err := none.Lint("README.md", "x"); err == nil {
		t.Fatal("no workspace must fail")
	}
}

func TestLintConfigSwitch(t *testing.T) {
	f := &fakeLinter{alerts: []lint.Alert{{Line: 1, Col: 1, EndCol: 1, Severity: "warning", Match: "x"}}}
	withLinter(t, f)
	dir := tree(t, lintDocs)
	z, _, _, _ := newTestZen(t, dir, "", "read")

	// Frontmatter switch, on disk and in unsaved text.
	if r, err := z.Lint("off/page.md", ""); err != nil || r.Available || len(r.Alerts) != 0 {
		t.Fatalf("frontmatter off: %+v %v", r, err)
	}
	if r, _ := z.Lint("README.md", "---\nplugins: {lint: false}\n---\nx\n"); r.Available {
		t.Fatalf("unsaved frontmatter off: %+v", r)
	}
	if n := len(f.all()); n != 0 {
		t.Fatalf("linter ran %d times while off", n)
	}

	// Workspace switch; frontmatter turns it back on.
	ws := tree(t, map[string]string{
		".zen.yml":  "plugins:\n  lint: false\n",
		"README.md": "# Home\n",
		"on.md":     "---\nplugins:\n  lint: true\n---\n# On\n",
	})
	z2, _, _, _ := newTestZen(t, ws, "", "read")
	if r, _ := z2.Lint("README.md", ""); r.Available {
		t.Fatalf(".zen.yml off: %+v", r)
	}
	if r, err := z2.Lint("on.md", ""); err != nil || !r.Available || len(r.Alerts) != 1 {
		t.Fatalf("frontmatter on: %+v %v", r, err)
	}

	// Global switch.
	global := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(global, []byte("plugins:\n  lint: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	z3, err := newZen(zenOptions{GlobalPath: global, Dir: dir, Events: &sink{}, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z3.close)
	if r, _ := z3.Lint("README.md", ""); r.Available {
		t.Fatalf("global off: %+v", r)
	}
}

func TestLintErrors(t *testing.T) {
	f := &fakeLinter{}
	withLinter(t, f)
	dir := tree(t, lintDocs)
	z, _, _, _ := newTestZen(t, dir, "", "read")
	root := z.Workspace().Root
	f.err = errors.New("lint: vale: exit status 2: " + root + "/.vale.ini: bad")
	_, err := z.Lint("README.md", "")
	if err == nil || strings.Contains(err.Error(), root) {
		t.Fatalf("error must be returned without the root: %v", err)
	}
	f.err = nil
	f.alerts = nil
	if r, err := z.Lint("README.md", ""); err != nil || !r.Available || r.Alerts == nil {
		t.Fatalf("nil alerts must become []: %+v %v", r, err)
	}
}
