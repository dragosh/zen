package app

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
)

// fakeWindow behaves like a Wails window: Close runs the closing hook and,
// unless it cancels, the window goes away.
type fakeWindow struct {
	name    string
	m       *windowManager
	x, y    int
	focused int
	asked   int // Close calls
	gone    bool
}

func (w *fakeWindow) Focus()               { w.focused++ }
func (w *fakeWindow) Position() (int, int) { return w.x, w.y }
func (w *fakeWindow) Close() {
	w.asked++
	if w.gone {
		return
	}
	if w.m.closing(w.name) {
		w.gone = true
		w.m.closed(w.name)
	}
}

type windowRig struct {
	m       *windowManager
	main    *fakeWindow
	specs   []windowSpec
	wins    map[string]*fakeWindow
	asked   []string // windows sent app:before-close, in order
	timers  []func()
	timerMu sync.Mutex
}

func newWindowRig(t *testing.T) *windowRig {
	t.Helper()
	r := &windowRig{wins: map[string]*fakeWindow{}}
	r.m = newWindowManager(windowManagerOptions{
		New: func(s windowSpec) appWindow {
			r.specs = append(r.specs, s)
			w := &fakeWindow{name: s.Name, m: r.m, x: s.X, y: s.Y}
			r.wins[s.Name] = w
			return w
		},
		Emit: func(name string, data any) {
			if name != eventBeforeClose {
				t.Errorf("unexpected event %s", name)
				return
			}
			r.timerMu.Lock()
			r.asked = append(r.asked, data.(closeRequest).Window)
			r.timerMu.Unlock()
		},
		After: func(_ time.Duration, f func()) {
			r.timerMu.Lock()
			r.timers = append(r.timers, f)
			r.timerMu.Unlock()
		},
		Go: func(f func()) { f() },
	})
	r.main = &fakeWindow{name: mainWindowName, m: r.m, x: 100, y: 50}
	r.m.setMain(r.main)
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
		if err := r.m.openEditor(p, strings.ToUpper(p)); err != nil {
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
	want := windowSpec{
		Name: "editor:README.md", Title: "Home — Edit — zen",
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
		got.Title != "Alpha & <Beta> — Edit — zen" || got.X != 148 || got.Y != 98 {
		t.Fatalf("second spec = %+v", got)
	}

	// No title in the tree: the file name.
	if err := z.OpenEditor("docs/plain.md"); err != nil {
		t.Fatal(err)
	}
	if title := r.specs[2].Title; !strings.HasSuffix(title, " — Edit — zen") || title == " — Edit — zen" {
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

func TestOpenEditorAfterClose(t *testing.T) {
	r := newWindowRig(t)
	r.open(t, "a.md")
	w := r.wins["editor:a.md"]
	w.Close()
	r.m.reply("editor:a.md", replyClose)
	if !w.gone {
		t.Fatal("editor did not close")
	}
	r.open(t, "a.md")
	if len(r.specs) != 2 {
		t.Fatalf("reopening a closed editor: %d windows", len(r.specs))
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

func TestCloseOrder(t *testing.T) {
	t.Run("an editor closes through its own handshake", func(t *testing.T) {
		r := newWindowRig(t)
		r.open(t, "a.md", "b.md")
		a := r.wins["editor:a.md"]
		a.Close()
		if a.gone || !reflect.DeepEqual(r.asked, []string{"editor:a.md"}) {
			t.Fatalf("closed without asking: gone %v, asked %v", a.gone, r.asked)
		}
		r.m.reply("editor:a.md", replyPending)
		r.fireTimers() // the user is being asked: no timeout
		if a.gone {
			t.Fatal("closed while the user was asked")
		}
		r.m.reply("editor:a.md", replyCancel)
		if a.gone {
			t.Fatal("closed after cancel")
		}
		a.Close()
		r.m.reply("editor:a.md", replyClose)
		if !a.gone || r.wins["editor:b.md"].gone || r.main.gone {
			t.Fatal("wrong windows closed")
		}
	})

	t.Run("an editor that never answers closes after the timeout", func(t *testing.T) {
		r := newWindowRig(t)
		r.open(t, "a.md")
		r.wins["editor:a.md"].Close()
		r.fireTimers()
		if !r.wins["editor:a.md"].gone {
			t.Fatal("not closed after timeout")
		}
	})

	t.Run("main closes its editors first, one at a time", func(t *testing.T) {
		r := newWindowRig(t)
		r.open(t, "b.md", "a.md")
		r.main.Close()
		if r.main.gone || !reflect.DeepEqual(r.asked, []string{"editor:a.md"}) {
			t.Fatalf("asked %v, main gone %v", r.asked, r.main.gone)
		}
		r.m.reply("editor:a.md", replyPending)
		r.m.reply("editor:a.md", replyClose)
		if !r.wins["editor:a.md"].gone || !reflect.DeepEqual(r.asked, []string{"editor:a.md", "editor:b.md"}) {
			t.Fatalf("after a: asked %v", r.asked)
		}
		r.m.reply("editor:b.md", replyClose)
		if !reflect.DeepEqual(r.asked, []string{"editor:a.md", "editor:b.md", "main"}) || r.main.gone {
			t.Fatalf("after b: asked %v, main gone %v", r.asked, r.main.gone)
		}
		r.m.reply("main", replyClose)
		if !r.main.gone {
			t.Fatal("main stayed open")
		}
	})

	t.Run("an editor cancelling keeps main open", func(t *testing.T) {
		r := newWindowRig(t)
		r.open(t, "a.md", "b.md")
		r.main.Close()
		r.m.reply("editor:a.md", replyPending)
		r.m.reply("editor:a.md", replyCancel)
		if r.main.gone || r.wins["editor:a.md"].gone || r.wins["editor:b.md"].gone {
			t.Fatal("something closed after cancel")
		}
		if !reflect.DeepEqual(r.asked, []string{"editor:a.md"}) {
			t.Fatalf("asked %v", r.asked)
		}
		// Closing the cancelled editor later does not resume closing main.
		r.wins["editor:a.md"].Close()
		r.m.reply("editor:a.md", replyClose)
		if r.main.gone || r.wins["editor:b.md"].asked != 0 {
			t.Fatal("main close resumed")
		}
		// A second attempt starts again with what is left.
		r.main.Close()
		r.m.reply("editor:b.md", replyClose)
		r.m.reply("main", replyClose)
		if !r.main.gone || !r.wins["editor:b.md"].gone {
			t.Fatal("second attempt failed")
		}
	})

	t.Run("an editor closed by the user meanwhile is skipped", func(t *testing.T) {
		r := newWindowRig(t)
		r.open(t, "a.md", "b.md")
		r.main.Close()
		r.m.reply("editor:a.md", replyPending)
		r.wins["editor:b.md"].Close() // the user closes b from its own window
		r.m.reply("editor:b.md", replyClose)
		r.m.reply("editor:a.md", replyClose)
		r.m.reply("main", replyClose)
		if !r.main.gone {
			t.Fatalf("main stayed open, asked %v", r.asked)
		}
		if n := r.wins["editor:b.md"].asked; n != 1 {
			t.Fatalf("b asked to close %d times", n)
		}
	})

	t.Run("main without editors uses its handshake", func(t *testing.T) {
		r := newWindowRig(t)
		r.main.Close()
		if r.main.gone || !reflect.DeepEqual(r.asked, []string{"main"}) {
			t.Fatalf("asked %v", r.asked)
		}
		r.fireTimers()
		if !r.main.gone {
			t.Fatal("main stayed open after timeout")
		}
	})

	t.Run("quit closes main the same way", func(t *testing.T) {
		r := newWindowRig(t)
		r.open(t, "a.md")
		if r.m.shouldQuit() {
			t.Fatal("quit allowed with an editor open")
		}
		r.m.reply("editor:a.md", replyClose)
		r.m.reply("main", replyClose)
		if !r.main.gone || !r.m.shouldQuit() {
			t.Fatal("quit did not go through")
		}
	})

	t.Run("opening another folder closes the editors, cancellable", func(t *testing.T) {
		r := newWindowRig(t)
		if !r.m.closeEditorsWait() {
			t.Fatal("no editors: refused")
		}
		r.open(t, "a.md", "b.md")
		done := make(chan bool, 1)
		go func() { done <- r.m.closeEditorsWait() }()
		waitAsked(t, r, 1)
		r.m.reply("editor:a.md", replyPending)
		r.m.reply("editor:a.md", replyCancel)
		if ok := <-done; ok || r.wins["editor:b.md"].asked != 0 {
			t.Fatalf("cancelled: ok %v", ok)
		}

		go func() { done <- r.m.closeEditorsWait() }()
		waitAsked(t, r, 2)
		r.m.reply("editor:a.md", replyClose)
		waitAsked(t, r, 3)
		r.m.reply("editor:b.md", replyClose)
		if ok := <-done; !ok || r.main.gone {
			t.Fatalf("closing editors: ok %v, main gone %v", ok, r.main.gone)
		}
		if r.m.editorCount() != 0 {
			t.Fatal("editors left")
		}
	})
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
