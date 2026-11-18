package session

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

// bigTree is a folder with n pages (plus a README).
func bigTree(t *testing.T, n int) string {
	t.Helper()
	files := map[string]string{"README.md": "# Big\n"}
	for i := 0; i < n; i++ {
		files[fmt.Sprintf("p/%04d.md", i)] = fmt.Sprintf("# Page %d\n\n[home](../README.md)\n", i)
	}
	return tree(t, files)
}

func openings(ev *sink) []OpenProgress {
	var out []OpenProgress
	for _, e := range ev.all() {
		if e.Name == EventWorkspaceOpening {
			out = append(out, e.Data.(OpenProgress))
		}
	}
	return out
}

func TestOpenReportsProgress(t *testing.T) {
	z, ev, _, _ := newTestZen(t, "", "", "read")
	if err := z.Open(bigTree(t, 600)); err != nil {
		t.Fatal(err)
	}
	got := openings(ev)
	phases := map[string]bool{}
	for _, p := range got {
		phases[p.Phase] = true
		if p.Name == "" {
			t.Errorf("no name: %+v", p)
		}
	}
	if !phases["reading"] || !phases["indexing"] || got[len(got)-1].Phase != "done" {
		t.Errorf("events: %+v", got)
	}
	for _, p := range got {
		if p.Phase == "indexing" && (p.Total != 601 || p.Done < 1) {
			t.Errorf("indexing counts: %+v", p)
		}
	}
}

// cancelSink cancels the open as soon as indexing starts.
type cancelSink struct {
	sink
	z    *Session
	once sync.Once
}

func (s *cancelSink) Emit(name string, data any) {
	s.sink.Emit(name, data)
	if p, ok := data.(OpenProgress); ok && s.z != nil && name == EventWorkspaceOpening && p.Phase == "indexing" {
		s.once.Do(s.z.CancelOpen)
	}
}

func TestCancelOpenKeepsTheFolderThatWasOpen(t *testing.T) {
	first := tree(t, docs)
	ev := &cancelSink{}
	dl := &fakeDialogs{}
	z, err := newZen(zenOptions{Registry: plugins.NewRegistry(), Dir: first, Events: ev, Dialogs: dl})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	ev.z = z
	big := bigTree(t, 300)
	if err := z.Open(big); !errors.Is(err, ErrOpenCancelled) {
		t.Fatalf("a cancelled open reports it: %v", err)
	}
	if got := z.Workspace().Root; !strings.HasSuffix(got, strings.TrimPrefix(first, "/private")) {
		t.Fatalf("the first folder should still be open, got %s", got)
	}
	if p, err := z.Page("docs/guide.md"); err != nil || p.Title != "Guide" {
		t.Fatalf("the first folder still renders: %v %v", p.Title, err)
	}
	if last := openings(&ev.sink); last[len(last)-1].Phase != "failed" {
		t.Errorf("last event: %+v", last[len(last)-1])
	}
	ev.z = nil
	if err := z.Open(big); err != nil {
		t.Fatalf("a later open works: %v", err)
	}
}

func TestOpenTimeout(t *testing.T) {
	first := tree(t, docs)
	z, _, _, _ := newTestZen(t, first, "", "read")
	z.openTimeout = time.Nanosecond
	err := z.Open(bigTree(t, 300))
	var te *OpenTimeoutError
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "limits.openTimeout") || strings.Contains(err.Error(), "/") {
		t.Fatalf("err = %v", err)
	}
	if p, err := z.Page("docs/guide.md"); err != nil || p.Title != "Guide" {
		t.Fatalf("the folder that was open stays: %v %v", p.Title, err)
	}
}

func TestWatcherStartsInTheBackgroundAndStopsCleanly(t *testing.T) {
	dir := tree(t, docs)
	s := New(Options{Registry: plugins.NewRegistry(), Watch: true})
	s.startWatch(dir, true)
	s.stopWatch() // before it is ready: the late watcher is dropped
	time.Sleep(200 * time.Millisecond)
	s.watchMu.Lock()
	if s.w != nil {
		t.Error("a stopped watcher came back")
	}
	s.watchMu.Unlock()

	s.startWatch(dir, true)
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.watchMu.Lock()
		w := s.w
		s.watchMu.Unlock()
		if w != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the background watcher never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.stopWatch()
}
