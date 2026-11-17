// Package sessiontest has the helpers that the tests of engine/session and of
// the apps built on it share: a folder tree, an event sink and a prompter
// that answers as told. It imports nothing from the session, so tests inside
// engine/session can use it.
package sessiontest

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Tree writes files (slash paths to contents) into a new temporary folder and
// returns it.
func Tree(t testing.TB, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for p, c := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Event is one event a Sink received.
type Event struct {
	Name string
	Data any
}

// Sink collects the events of a session; it implements session.Events.
type Sink struct {
	mu  sync.Mutex
	evs []Event
}

// Emit records an event.
func (s *Sink) Emit(name string, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, Event{name, data})
}

// All returns the events received so far.
func (s *Sink) All() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.evs...)
}

// Reset forgets the events received so far.
func (s *Sink) Reset() {
	s.mu.Lock()
	s.evs = nil
	s.mu.Unlock()
}

// Prompter answers every question with Allow and remembers what was asked; it
// implements session.Prompter.
type Prompter struct {
	Allow bool
	Asked []string
}

// Confirm records the question and returns Allow.
func (p *Prompter) Confirm(message string) bool {
	p.Asked = append(p.Asked, message)
	return p.Allow
}
