package session_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dragosh/zen/engine/session"
	"github.com/dragosh/zen/engine/session/sessiontest"
	"github.com/dragosh/zen/internal/core/plugins"
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
	b := sessiontest.Tree(t, map[string]string{"index.md": "# B\n", "x.md": "# X\n"})
	s := session.New(session.Options{ConfigDir: t.TempDir(), Watch: true})
	t.Cleanup(s.Close)
	if s.Workspace() != nil {
		t.Fatal("a new session has a folder open")
	}
	for i := 0; i < 3; i++ {
		for _, c := range []struct{ dir, home string }{{a, "README.md"}, {b, "index.md"}} {
			if err := s.Open(c.dir); err != nil {
				t.Fatal(err)
			}
			ws := s.Workspace()
			if ws == nil || ws.Root != filepath.Clean(ws.Root) || ws.Home != c.home || s.Home() != c.home {
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
