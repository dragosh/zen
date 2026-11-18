package desktop

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeWindow behaves like a Wails window: Close runs the closing hook and,
// unless it cancels, the window goes away.
type fakeWindow struct {
	name    string
	m       *Windows
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
	m       *Windows
	main    *fakeWindow
	specs   []WindowSpec
	wins    map[string]*fakeWindow
	asked   []string // windows sent app:before-close, in order
	timers  []func()
	timerMu sync.Mutex
}

func newWindowRig(t *testing.T) *windowRig {
	t.Helper()
	r := &windowRig{wins: map[string]*fakeWindow{}}
	r.m = NewWindows(WindowsOptions{
		Title: "zen.md",
		New: func(s WindowSpec) Window {
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
			r.asked = append(r.asked, data.(CloseRequest).Window)
			r.timerMu.Unlock()
		},
		After: func(_ time.Duration, f func()) {
			r.timerMu.Lock()
			r.timers = append(r.timers, f)
			r.timerMu.Unlock()
		},
		Go: func(f func()) { f() },
	})
	r.main = &fakeWindow{name: MainWindowName, m: r.m, x: 100, y: 50}
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

func TestOpenEditorAfterClose(t *testing.T) {
	r := newWindowRig(t)
	r.open(t, "a.md")
	w := r.wins["editor:a.md"]
	w.Close()
	r.m.Reply("editor:a.md", ReplyClose)
	if !w.gone {
		t.Fatal("editor did not close")
	}
	r.open(t, "a.md")
	if len(r.specs) != 2 {
		t.Fatalf("reopening a closed editor: %d windows", len(r.specs))
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
		r.m.Reply("editor:a.md", ReplyPending)
		r.fireTimers() // the user is being asked: no timeout
		if a.gone {
			t.Fatal("closed while the user was asked")
		}
		r.m.Reply("editor:a.md", ReplyCancel)
		if a.gone {
			t.Fatal("closed after cancel")
		}
		a.Close()
		r.m.Reply("editor:a.md", ReplyClose)
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
		r.m.Reply("editor:a.md", ReplyPending)
		r.m.Reply("editor:a.md", ReplyClose)
		if !r.wins["editor:a.md"].gone || !reflect.DeepEqual(r.asked, []string{"editor:a.md", "editor:b.md"}) {
			t.Fatalf("after a: asked %v", r.asked)
		}
		r.m.Reply("editor:b.md", ReplyClose)
		if !reflect.DeepEqual(r.asked, []string{"editor:a.md", "editor:b.md", "main"}) || r.main.gone {
			t.Fatalf("after b: asked %v, main gone %v", r.asked, r.main.gone)
		}
		r.m.Reply("main", ReplyClose)
		if !r.main.gone {
			t.Fatal("main stayed open")
		}
	})

	t.Run("an editor cancelling keeps main open", func(t *testing.T) {
		r := newWindowRig(t)
		r.open(t, "a.md", "b.md")
		r.main.Close()
		r.m.Reply("editor:a.md", ReplyPending)
		r.m.Reply("editor:a.md", ReplyCancel)
		if r.main.gone || r.wins["editor:a.md"].gone || r.wins["editor:b.md"].gone {
			t.Fatal("something closed after cancel")
		}
		if !reflect.DeepEqual(r.asked, []string{"editor:a.md"}) {
			t.Fatalf("asked %v", r.asked)
		}
		// Closing the cancelled editor later does not resume closing main.
		r.wins["editor:a.md"].Close()
		r.m.Reply("editor:a.md", ReplyClose)
		if r.main.gone || r.wins["editor:b.md"].asked != 0 {
			t.Fatal("main close resumed")
		}
		// A second attempt starts again with what is left.
		r.main.Close()
		r.m.Reply("editor:b.md", ReplyClose)
		r.m.Reply("main", ReplyClose)
		if !r.main.gone || !r.wins["editor:b.md"].gone {
			t.Fatal("second attempt failed")
		}
	})

	t.Run("an editor closed by the user meanwhile is skipped", func(t *testing.T) {
		r := newWindowRig(t)
		r.open(t, "a.md", "b.md")
		r.main.Close()
		r.m.Reply("editor:a.md", ReplyPending)
		r.wins["editor:b.md"].Close() // the user closes b from its own window
		r.m.Reply("editor:b.md", ReplyClose)
		r.m.Reply("editor:a.md", ReplyClose)
		r.m.Reply("main", ReplyClose)
		if !r.main.gone {
			t.Fatalf("main stayed open, asked %v", r.asked)
		}
		n := 0
		for _, name := range r.asked {
			if name == "editor:b.md" {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("b asked %d times: %v", n, r.asked)
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
		if r.m.ShouldQuit() {
			t.Fatal("quit allowed with an editor open")
		}
		r.m.Reply("editor:a.md", ReplyClose)
		r.m.Reply("main", ReplyClose)
		if !r.main.gone || !r.m.ShouldQuit() {
			t.Fatal("quit did not go through")
		}
	})

	t.Run("opening another folder closes the editors, cancellable", func(t *testing.T) {
		r := newWindowRig(t)
		if !r.m.CloseEditorsWait() {
			t.Fatal("no editors: refused")
		}
		r.open(t, "a.md", "b.md")
		done := make(chan bool, 1)
		go func() { done <- r.m.CloseEditorsWait() }()
		waitAsked(t, r, 1)
		r.m.Reply("editor:a.md", ReplyPending)
		r.m.Reply("editor:a.md", ReplyCancel)
		if ok := <-done; ok || r.wins["editor:b.md"].asked != 0 {
			t.Fatalf("cancelled: ok %v", ok)
		}

		go func() { done <- r.m.CloseEditorsWait() }()
		waitAsked(t, r, 2)
		r.m.Reply("editor:a.md", ReplyClose)
		waitAsked(t, r, 3)
		r.m.Reply("editor:b.md", ReplyClose)
		if ok := <-done; !ok || r.main.gone {
			t.Fatalf("closing editors: ok %v, main gone %v", ok, r.main.gone)
		}
		if r.m.EditorCount() != 0 {
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

func TestEditorWindowIs90PercentOfTheMainWindow(t *testing.T) {
	r := newWindowRig(t)
	r.main.x, r.main.y, r.main.w, r.main.h = 0, 30, 1512, 919
	if err := r.m.OpenEditor("a.md", "A"); err != nil {
		t.Fatal(err)
	}
	if err := r.m.OpenEditor("b.md", "B"); err != nil {
		t.Fatal(err)
	}
	first, second := r.specs[0], r.specs[1]
	if first.Width != 1360 || first.Height != 827 {
		t.Errorf("size = %dx%d, want 90%% of 1512x919 = 1360x827", first.Width, first.Height)
	}
	if first.X != 76 || first.Y != 76 { // centred: (1512-1360)/2, 30+(919-827)/2
		t.Errorf("position = %d,%d, want centred at 76,76", first.X, first.Y)
	}
	if second.X != first.X+editorCascadeStep || second.Y != first.Y+editorCascadeStep || second.Width != first.Width {
		t.Errorf("second editor = %+v, want the same size cascaded by %d", second, editorCascadeStep)
	}

	// A small main window still gives a usable editor.
	r2 := newWindowRig(t)
	r2.main.w, r2.main.h = 500, 300
	if err := r2.m.OpenEditor("c.md", "C"); err != nil {
		t.Fatal(err)
	}
	if s := r2.specs[0]; s.Width != editorMinWidth || s.Height != editorMinHeight {
		t.Errorf("small main: %dx%d, want the %dx%d minimum", s.Width, s.Height, editorMinWidth, editorMinHeight)
	}
}

func TestEditorWindowSpec(t *testing.T) {
	r := newWindowRig(t)
	if err := r.m.OpenEditor("README.md", "Home"); err != nil {
		t.Fatal(err)
	}
	want := WindowSpec{
		Name: "editor:README.md", Title: "Home — Edit — zen.md",
		URL:   "/?window=editor&path=README.md",
		Width: 1000, Height: 720, X: 124, Y: 74,
	}
	if len(r.specs) != 1 || r.specs[0] != want {
		t.Fatalf("spec = %+v, want %+v", r.specs, want)
	}
	// Same path: focus, no second window.
	if err := r.m.OpenEditor("README.md", "Home"); err != nil {
		t.Fatal(err)
	}
	if len(r.specs) != 1 || r.wins["editor:README.md"].focused != 1 {
		t.Fatalf("second open: %d windows, focused %d", len(r.specs), r.wins["editor:README.md"].focused)
	}
	// Another page: cascaded further, path URL-encoded.
	if err := r.m.OpenEditor("docs/a b.md", "Alpha"); err != nil {
		t.Fatal(err)
	}
	got := r.specs[1]
	if got.Name != "editor:docs/a b.md" || got.URL != "/?window=editor&path=docs%2Fa+b.md" || got.Title != "Alpha — Edit — zen.md" || got.X != 148 || got.Y != 98 {
		t.Fatalf("second spec = %+v", got)
	}
}
