package session_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dragosh/zenmd/engine/session"
	"github.com/dragosh/zenmd/engine/session/sessiontest"
)

func names(evs []sessiontest.Event) map[string]int {
	m := map[string]int{}
	for _, e := range evs {
		m[e.Name]++
	}
	return m
}

// A session without an event sink works, also with the watcher running.
func TestNilEvents(t *testing.T) {
	dir := sessiontest.Tree(t, map[string]string{"README.md": "# Home\n"})
	s := session.New(session.Options{ConfigDir: t.TempDir(), Watch: true})
	t.Cleanup(s.Close)
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Page("README.md"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.md"), []byte("# New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The watcher reports the new page; with no sink nothing may panic.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ws := s.Workspace(); ws != nil && len(ws.Tree) > 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the new page did not appear in the tree")
}

func TestEventsForwarded(t *testing.T) {
	dir := sessiontest.Tree(t, map[string]string{"README.md": "# Home\n"})
	ev := &sessiontest.Sink{}
	s := session.New(session.Options{ConfigDir: t.TempDir(), Events: ev, Watch: true})
	t.Cleanup(s.Close)
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}
	got := names(ev.All())
	for _, want := range []string{session.EventWorkspaceOpening, session.EventPluginsChanged, session.EventLayoutsChanged} {
		if got[want] == 0 {
			t.Errorf("no %s event; got %v", want, got)
		}
	}
	ev.Reset()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Home\n\nmore\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range ev.All() {
			if e.Name == session.EventPageChanged {
				if pc, ok := e.Data.(session.PageChanged); !ok || pc.Path != "README.md" {
					t.Fatalf("payload %#v", e.Data)
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no page:changed event after a save")
}
