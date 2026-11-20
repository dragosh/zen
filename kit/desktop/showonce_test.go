package desktop

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestShowWhenReadyShowsOnReady(t *testing.T) {
	g := newReadyGate()
	var shows atomic.Int32
	timeout := make(chan time.Time)
	done := make(chan struct{})
	go func() { showWhenReady(g, timeout, func() { shows.Add(1) }); close(done) }()
	g.Ready()
	<-done
	if n := shows.Load(); n != 1 {
		t.Fatalf("shows = %d, want 1", n)
	}
}

func TestShowWhenReadyShowsOnTimeout(t *testing.T) {
	g := newReadyGate()
	var shows atomic.Int32
	timeout := make(chan time.Time, 1)
	timeout <- time.Time{} // the fake clock fired
	showWhenReady(g, timeout, func() { shows.Add(1) })
	if n := shows.Load(); n != 1 {
		t.Fatalf("shows = %d, want 1", n)
	}
}

func TestShowWhenReadyShowsOnceWhenBothFire(t *testing.T) {
	g := newReadyGate()
	var shows atomic.Int32
	timeout := make(chan time.Time, 1)
	timeout <- time.Time{}
	g.Ready()
	g.Ready() // a reloaded frontend emits it again
	showWhenReady(g, timeout, func() { shows.Add(1) })
	if n := shows.Load(); n != 1 {
		t.Fatalf("shows = %d, want 1", n)
	}
}

func TestShowWhenReadyWaitsUntilOneFires(t *testing.T) {
	g := newReadyGate()
	var shows atomic.Int32
	timeout := make(chan time.Time)
	done := make(chan struct{})
	go func() { showWhenReady(g, timeout, func() { shows.Add(1) }); close(done) }()
	select {
	case <-done:
		t.Fatal("showed before the frontend was ready and before the timeout")
	case <-time.After(20 * time.Millisecond):
	}
	if shows.Load() != 0 {
		t.Fatal("Show was called early")
	}
	timeout <- time.Time{}
	<-done
	if n := shows.Load(); n != 1 {
		t.Fatalf("shows = %d, want 1", n)
	}
}
