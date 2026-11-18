package session_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dragosh/zenmd/engine/plugins"
	"github.com/dragosh/zenmd/engine/session"
	"github.com/dragosh/zenmd/engine/session/sessiontest"
)

// waiter is a built-in plugin whose stream waits until it is cancelled.
type waiter struct{ started chan struct{} }

func (w *waiter) Manifest() plugins.Manifest {
	return plugins.Manifest{ID: "waiter", Version: "0.1.0", APIVersion: plugins.APIVersion, Default: true}
}

func (w *waiter) Stream(c context.Context, _ plugins.Context, _ []byte, _ func([]byte)) error {
	w.started <- struct{}{}
	<-c.Done()
	return c.Err()
}

func TestOpenTwice(t *testing.T) {
	a := sessiontest.Tree(t, map[string]string{"README.md": "# A\n"})
	b := sessiontest.Tree(t, map[string]string{"README.md": "# B\n", "x.md": "# X\n"})
	s := session.New(session.Options{ConfigDir: t.TempDir(), Watch: true})
	t.Cleanup(s.Close)
	if s.Workspace() != nil {
		t.Fatal("a new session has a folder open")
	}
	for i := 0; i < 3; i++ {
		for _, c := range []struct{ dir, home, title string }{{a, "README.md", "A"}, {b, "README.md", "B"}} {
			if err := s.Open(c.dir); err != nil {
				t.Fatal(err)
			}
			ws := s.Workspace()
			if ws == nil || ws.Root != filepath.Clean(ws.Root) || ws.Home != c.home || s.Home() != c.home || ws.Tree[0].Title != c.title {
				t.Fatalf("workspace after opening %s: %+v", c.dir, ws)
			}
			if _, err := s.Page(c.home); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCloseStopsWork(t *testing.T) {
	dir := sessiontest.Tree(t, map[string]string{"README.md": "# A\n"})
	w := &waiter{started: make(chan struct{}, 1)}
	reg := plugins.NewRegistry()
	if err := reg.Register(w); err != nil {
		t.Fatal(err)
	}
	ev := &sessiontest.Sink{}
	s := session.New(session.Options{ConfigDir: t.TempDir(), Registry: reg, Events: ev, Watch: true})
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}
	call, err := s.PluginStream("waiter", "README.md", `"go"`)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.started:
	case <-time.After(3 * time.Second):
		t.Fatal("the stream did not start")
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not return")
	}
	var last *session.PluginStreamEvent
	for _, e := range ev.All() {
		if pe, ok := e.Data.(session.PluginStreamEvent); ok && pe.Call == call {
			last = &pe
		}
	}
	if last == nil || !last.Done || last.Error != "cancelled" {
		t.Errorf("the stream was not ended by Close: %+v", last)
	}
}

// Two sessions in one process share nothing: separate folders, events, builds.
func TestTwoSessions(t *testing.T) {
	runtime := fstest.MapFS{
		"static/zen.js":  {Data: []byte("/* js */")},
		"static/zen.css": {Data: []byte("/* css */")},
	}
	a := sessiontest.Tree(t, map[string]string{"README.md": "# A\n", "a.md": "# AA\n"})
	b := sessiontest.Tree(t, map[string]string{"README.md": "# B\n"})
	evA, evB := &sessiontest.Sink{}, &sessiontest.Sink{}
	sa := session.New(session.Options{ConfigDir: t.TempDir(), Events: evA, Runtime: runtime, Watch: true})
	sb := session.New(session.Options{ConfigDir: t.TempDir(), Events: evB, Runtime: runtime, Watch: true})
	t.Cleanup(sa.Close)
	t.Cleanup(sb.Close)
	if err := sa.Open(a); err != nil {
		t.Fatal(err)
	}
	if err := sb.Open(b); err != nil {
		t.Fatal(err)
	}
	if sa.Workspace().Tree[0].Title != "A" || sb.Workspace().Tree[0].Title != "B" {
		t.Fatal("the sessions share a folder")
	}
	// Both build at once, each into its own output; one's cancel is not the other's.
	var wg sync.WaitGroup
	results := make([]session.BuildResult, 2)
	errs := make([]error, 2)
	for i, s := range []*session.Session{sa, sb} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = s.Build(context.Background(), session.BuildOptions{Out: filepath.Join(t.TempDir(), "site"), Portable: true}, nil)
		}()
	}
	wg.Wait()
	for i := range errs {
		if errs[i] != nil || results[i].Pages == 0 || results[i].Cancelled {
			t.Errorf("build %d: %+v %v", i, results[i], errs[i])
		}
	}
	sa.CancelBuild() // idle: nothing happens, to this or the other session
	// Closing one leaves the other working.
	sa.Close()
	if _, err := sb.Page("README.md"); err != nil {
		t.Errorf("the second session after closing the first: %v", err)
	}
	// Events went to their own sink.
	for _, e := range evB.All() {
		if p, ok := e.Data.(session.OpenProgress); ok && p.Name == filepath.Base(a) {
			t.Errorf("session B received an event of A: %+v", p)
		}
	}
}
