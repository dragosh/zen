package desktop

import (
	"sync"
	"time"
)

// showTimeout is how long the hidden main window waits for the frontend before
// it is shown anyway, so a frontend that fails to start never leaves no window.
const showTimeout = 2 * time.Second

// readyGate turns the frontend's app:ready event (which a reloaded page may
// send again) into one signal.
type readyGate struct {
	once sync.Once
	ch   chan struct{}
}

func newReadyGate() *readyGate { return &readyGate{ch: make(chan struct{})} }

// Ready records that the frontend drew its first screen; calling it again does nothing.
func (g *readyGate) Ready() { g.once.Do(func() { close(g.ch) }) }

// showWhenReady blocks until the frontend is ready or timeout fires, whichever
// comes first, and calls show exactly once.
func showWhenReady(g *readyGate, timeout <-chan time.Time, show func()) {
	select {
	case <-g.ch:
	case <-timeout:
	}
	show()
}
