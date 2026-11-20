package watch

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu      sync.Mutex
	batches [][]Event
	ch      chan struct{}
}

func newRecorder() *recorder { return &recorder{ch: make(chan struct{}, 1000)} }

func (r *recorder) on(ev []Event) {
	r.mu.Lock()
	r.batches = append(r.batches, append([]Event(nil), ev...))
	r.mu.Unlock()
	r.ch <- struct{}{}
}

func (r *recorder) all() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Event
	for _, b := range r.batches {
		out = append(out, b...)
	}
	return out
}

func (r *recorder) reset() {
	r.mu.Lock()
	r.batches = nil
	r.mu.Unlock()
}

// waitFor waits up to 2 s until want is seen.
func (r *recorder) waitFor(t *testing.T, want Event) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		for _, e := range r.all() {
			if e == want {
				return
			}
		}
		select {
		case <-r.ch:
		case <-deadline:
			t.Fatalf("timeout waiting for %+v; got %+v", want, r.all())
		}
	}
}

// settle waits until no batch arrives for 300 ms (bounded by 2 s).
func (r *recorder) settle() {
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-r.ch:
		case <-time.After(300 * time.Millisecond):
			return
		case <-deadline:
			return
		}
	}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func start(t *testing.T, root string) (*Watcher, *recorder) {
	t.Helper()
	r := newRecorder()
	w, err := Start(root, r.on)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	return w, r
}

func TestEvents(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.md"), "a")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	_, r := start(t, root)

	write(t, filepath.Join(root, "b.md"), "b")
	r.waitFor(t, Event{"b.md", "created"})

	write(t, filepath.Join(root, "a.md"), "a2")
	r.waitFor(t, Event{"a.md", "changed"})

	os.Remove(filepath.Join(root, "b.md"))
	r.waitFor(t, Event{"b.md", "removed"})

	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	time.Sleep(50 * time.Millisecond)
	write(t, filepath.Join(root, "sub", "deep", "c.MD"), "c")
	r.waitFor(t, Event{"sub/deep/c.MD", "created"})

	write(t, filepath.Join(root, "img.png"), "x")
	r.waitFor(t, Event{"img.png", "created"})
	write(t, filepath.Join(root, ".zenmd.yml"), "x")
	r.waitFor(t, Event{".zenmd.yml", "created"})
	write(t, filepath.Join(root, ".zen.yml"), "x") // the old name is watched too
	r.waitFor(t, Event{".zen.yml", "created"})

	if err := os.Rename(filepath.Join(root, "a.md"), filepath.Join(root, "z.md")); err != nil {
		t.Fatal(err)
	}
	r.waitFor(t, Event{"a.md", "removed"})
	r.waitFor(t, Event{"z.md", "created"})

	r.settle()
	r.reset()
	write(t, filepath.Join(root, "notes.txt"), "x")
	write(t, filepath.Join(root, ".git", "x.md"), "x")
	write(t, filepath.Join(root, "node_modules", "x.md"), "x")
	write(t, filepath.Join(root, "vendor", "x.md"), "x")
	write(t, filepath.Join(outside, "o.md"), "x")
	write(t, filepath.Join(root, "sub", "deep", "c.MD"), "c2")
	r.waitFor(t, Event{"sub/deep/c.MD", "changed"})
	r.settle()
	for _, e := range r.all() {
		if e.Path != "sub/deep/c.MD" {
			t.Errorf("unexpected event %+v", e)
		}
	}
}

func TestDebounce(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.md"), "a")
	_, r := start(t, root)
	for i := 0; i < 20; i++ {
		write(t, filepath.Join(root, "a.md"), "x")
	}
	r.waitFor(t, Event{"a.md", "changed"})
	r.settle()
	n := 0
	for _, e := range r.all() {
		if e.Path == "a.md" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want 1 event, got %d: %+v", n, r.all())
	}
}

func TestAtomicSave(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.md"), "a")
	_, r := start(t, root)
	for _, tmpName := range []string{".a.md.123.tmp", "a.md.tmp.md"} {
		r.settle()
		r.reset()
		tmp := filepath.Join(root, tmpName)
		write(t, tmp, "new")
		if err := os.Rename(tmp, filepath.Join(root, "a.md")); err != nil {
			t.Fatal(err)
		}
		r.waitFor(t, Event{"a.md", "changed"})
		r.settle()
		if got := r.all(); len(got) != 1 {
			t.Errorf("%s: want single changed, got %+v", tmpName, got)
		}
	}
}

func TestStop(t *testing.T) {
	root := t.TempDir()
	w, r := start(t, root)
	write(t, filepath.Join(root, "a.md"), "a")
	w.Stop()
	w.Stop()
	r.reset()
	write(t, filepath.Join(root, "b.md"), "b")
	time.Sleep(300 * time.Millisecond)
	if got := r.all(); len(got) != 0 {
		t.Errorf("callback after Stop: %+v", got)
	}
	if w.Err() != nil {
		t.Errorf("Err after Stop = %v", w.Err())
	}
}

func TestRootRemoved(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	write(t, filepath.Join(root, "a.md"), "a")
	w, r := start(t, root)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for w.Err() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if w.Err() == nil {
		t.Fatal("Err() = nil after root removed")
	}
	r.reset()
	os.MkdirAll(root, 0o755)
	write(t, filepath.Join(root, "b.md"), "b")
	time.Sleep(300 * time.Millisecond)
	if got := r.all(); len(got) != 0 {
		t.Errorf("callback after failure: %+v", got)
	}
}
