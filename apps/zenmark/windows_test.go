package app

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// fakeWindow behaves like a Wails window: Close runs the closing hook and,
// unless it cancels, the window goes away.
type fakeWindow struct {
	name    string
	m       *desktop.Windows
	x, y    int
	w, h    int
	focused int
	asked   int // Close calls
	gone    bool
}

func (w *fakeWindow) Focus()               { w.focused++ }
func (w *fakeWindow) Position() (int, int) { return w.x, w.y }
func (w *fakeWindow) Size() (int, int)     { return w.w, w.h }
func (w *fakeWindow) Close() {
	w.asked++
	if w.gone {
		return
	}
	if w.m.Closing(w.name) {
		w.gone = true
		w.m.Closed(w.name)
	}
}

type windowRig struct {
	m       *desktop.Windows
	main    *fakeWindow
	specs   []desktop.WindowSpec
	wins    map[string]*fakeWindow
	asked   []string // windows sent app:before-close, in order
	timers  []func()
	timerMu sync.Mutex
}

func newWindowRig(t *testing.T) *windowRig {
	t.Helper()
	r := &windowRig{wins: map[string]*fakeWindow{}}
	r.m = desktop.NewWindows(desktop.WindowsOptions{
		Title: Name,
		New: func(s desktop.WindowSpec) desktop.Window {
			r.specs = append(r.specs, s)
			w := &fakeWindow{name: s.Name, m: r.m, x: s.X, y: s.Y}
			r.wins[s.Name] = w
			return w
		},
		Emit: func(name string, data any) {
			if name != "app:before-close" {
				t.Errorf("unexpected event %s", name)
				return
			}
			r.timerMu.Lock()
			r.asked = append(r.asked, data.(desktop.CloseRequest).Window)
			r.timerMu.Unlock()
		},
		After: func(_ time.Duration, f func()) {
			r.timerMu.Lock()
			r.timers = append(r.timers, f)
			r.timerMu.Unlock()
		},
		Go: func(f func()) { f() },
	})
	r.main = &fakeWindow{name: desktop.MainWindowName, m: r.m, x: 100, y: 50}
	r.m.SetMain(r.main)
	return r
}

// fireTimers runs every pending reply timeout, as if no window answered.
func (r *windowRig) fireTimers() {
	r.timerMu.Lock()
	ts := r.timers
	r.timers = nil
	r.timerMu.Unlock()
	for _, f := range ts {
		f()
	}
}

func (r *windowRig) open(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := r.m.OpenEditor(p, strings.ToUpper(p)); err != nil {
			t.Fatal(err)
		}
	}
}

func windowsZen(t *testing.T, r *windowRig, files map[string]string) *Zen {
	t.Helper()
	z, err := newZen(zenOptions{
		Registry: plugins.NewRegistry(), Dir: tree(t, files),
		Events: &sink{}, Dialogs: &fakeDialogs{}, Open: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	z.windows = r.m
	return z
}

func TestOpenEditor(t *testing.T) {
	r := newWindowRig(t)
	z := windowsZen(t, r, map[string]string{
		"README.md":       "# Home\n",
		"docs/a b.md":     "# Alpha & <Beta>\n",
		"notes.txt":       "x",
		"docs/plain.md":   "no heading\n",
		"docs/image.png":  "png",
		"docs/other.mdx":  "# mdx\n",
		"docs/upper.MD":   "# Upper\n",
		"docs/deep/c.md":  "# C\n",
		"docs/deep/d.md":  "# D\n",
		"docs/deep/e.txt": "e",
	})

	if err := z.OpenEditor("README.md"); err != nil {
		t.Fatal(err)
	}
	want := desktop.WindowSpec{
		Name: "editor:README.md", Title: "Home — Edit — Zenmark",
		URL:   "/?window=editor&path=README.md",
		Width: 1000, Height: 720, X: 124, Y: 74,
	}
	if len(r.specs) != 1 || r.specs[0] != want {
		t.Fatalf("spec = %+v, want %+v", r.specs, want)
	}

	// Same path: focus, no second window.
	if err := z.OpenEditor("README.md"); err != nil {
		t.Fatal(err)
	}
	if len(r.specs) != 1 || r.wins["editor:README.md"].focused != 1 {
		t.Fatalf("second open: %d windows, focused %d", len(r.specs), r.wins["editor:README.md"].focused)
	}

	// Another page: its own window, cascaded further, path URL-encoded.
	if err := z.OpenEditor("docs/a b.md"); err != nil {
		t.Fatal(err)
	}
	got := r.specs[1]
	if got.Name != "editor:docs/a b.md" || got.URL != "/?window=editor&path=docs%2Fa+b.md" ||
		z.pageTitle("docs/a b.md") == "" || got.Title != z.pageTitle("docs/a b.md")+" — Edit — Zenmark" || got.X != 148 || got.Y != 98 {
		t.Fatalf("second spec = %+v", got)
	}

	// No title in the tree: the file name.
	if err := z.OpenEditor("docs/plain.md"); err != nil {
		t.Fatal(err)
	}
	if title := r.specs[2].Title; !strings.HasSuffix(title, " — Edit — Zenmark") || title == " — Edit — Zenmark" {
		t.Fatalf("plain title = %q", title)
	}

	// Rejected: not markdown, outside the workspace, absolute, missing.
	for _, p := range []string{"notes.txt", "docs/image.png", "docs/other.mdx", "../x.md", "docs/../../x.md",
		"/etc/passwd.md", "missing.md", "", "docs"} {
		if err := z.OpenEditor(p); err == nil {
			t.Errorf("OpenEditor(%q) = nil, want error", p)
		}
	}
	if len(r.specs) != 3 {
		t.Fatalf("rejected paths opened windows: %+v", r.specs[3:])
	}

	// No window manager (tests, CLI): an error, not a panic.
	z.windows = nil
	if err := z.OpenEditor("README.md"); err == nil {
		t.Fatal("OpenEditor without windows = nil")
	}
}

func TestStartupEditOpensWindow(t *testing.T) {
	r := newWindowRig(t)
	z := windowsZen(t, r, map[string]string{"README.md": "# Home\n", "guide.md": "# Guide\n"})
	z.page = "guide.md"

	if err := z.openStartupEditor("read"); err != nil || len(r.specs) != 0 {
		t.Fatalf("read mode: err %v, %d windows", err, len(r.specs))
	}
	if err := z.openStartupEditor("edit"); err != nil {
		t.Fatal(err)
	}
	if len(r.specs) != 1 || r.specs[0].Name != "editor:guide.md" {
		t.Fatalf("edit mode opened %+v", r.specs)
	}
	// The main window still starts in read mode.
	if st := z.Startup(); st.Mode != "read" || st.Page != "guide.md" {
		t.Fatalf("startup = %+v", st)
	}

	// No page given: the home page.
	r2 := newWindowRig(t)
	z2 := windowsZen(t, r2, map[string]string{"README.md": "# Home\n"})
	if err := z2.openStartupEditor("edit"); err != nil || len(r2.specs) != 1 || r2.specs[0].Name != "editor:README.md" {
		t.Fatalf("edit without page: err %v, %+v", err, r2.specs)
	}
}

func waitAsked(t *testing.T, r *windowRig, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		r.timerMu.Lock()
		got := len(r.asked)
		r.timerMu.Unlock()
		if got >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("asked %d windows, want %d", got, n)
		}
		time.Sleep(time.Millisecond)
	}
}
