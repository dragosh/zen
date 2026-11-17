package app

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
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
	z, ev, dl, _ := newTestZen(t, "", "", "read")
	dl.folder = bigTree(t, 600)
	if ws, err := z.PickFolder(); err != nil || ws == nil {
		t.Fatal(ws, err)
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
	z    *Zen
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
	dl.folder = bigTree(t, 300)
	ws, err := z.PickFolder()
	if ws != nil || err != nil {
		t.Fatalf("a cancelled open is like a cancelled dialog: %v %v", ws, err)
	}
	s := z.Startup()
	if s.Workspace == nil || s.Workspace.Root != z.s.Workspace().Root || s.Workspace.Name != z.s.Workspace().Name {
		t.Fatalf("startup: %+v", s)
	}
	if got := z.s.Workspace().Root; !strings.HasSuffix(got, strings.TrimPrefix(first, "/private")) {
		t.Fatalf("the first folder should still be open, got %s", got)
	}
	if p, err := z.Page("docs/guide.md"); err != nil || p.Title != "Guide" {
		t.Fatalf("the first folder still renders: %v %v", p.Title, err)
	}
	if last := openings(&ev.sink); last[len(last)-1].Phase != "failed" {
		t.Errorf("last event: %+v", last[len(last)-1])
	}
	if err := z.s.Open(dl.folder); err != nil {
		t.Fatalf("a later open works: %v", err)
	}
}

func TestOpenTimeout(t *testing.T) {
	first := tree(t, docs)
	z, _, dl, _ := newTestZen(t, first, "", "read")
	z.s.SetOpenTimeout(time.Nanosecond)
	dl.folder = bigTree(t, 300)
	_, err := z.PickFolder()
	var te *OpenTimeoutError
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "limits.openTimeout") || strings.Contains(err.Error(), "/") {
		t.Fatalf("err = %v", err)
	}
	if p, err := z.Page("docs/guide.md"); err != nil || p.Title != "Guide" {
		t.Fatalf("the folder that was open stays: %v %v", p.Title, err)
	}
}

func TestDeferredOpenAtLaunch(t *testing.T) {
	dir := tree(t, docs)
	ev := &sink{}
	z, err := newZen(zenOptions{Registry: plugins.NewRegistry(), Dir: dir, Events: ev, Dialogs: &fakeDialogs{}, DeferOpen: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	if z.s.Workspace() != nil {
		t.Fatal("DeferOpen opened the folder in newZen")
	}
	s := z.Startup()
	if s.Workspace == nil || s.Page != "README.md" || s.Error != "" {
		t.Fatalf("startup: %+v", s)
	}
	if got := openings(ev); len(got) == 0 || got[len(got)-1].Phase != "done" {
		t.Errorf("events: %+v", got)
	}

	// A launch folder that does not open: the welcome screen with why, until a folder opens.
	big := bigTree(t, 300)
	dl := &fakeDialogs{folder: dir}
	z2, err := newZen(zenOptions{Registry: plugins.NewRegistry(), Dir: big, Events: &sink{}, Dialogs: dl, DeferOpen: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z2.close)
	z2.s.SetOpenTimeout(time.Nanosecond)
	s = z2.Startup()
	if s.Workspace != nil || !strings.Contains(s.Error, "took longer than") {
		t.Fatalf("startup: %+v", s)
	}
	z2.s.SetOpenTimeout(0)
	if ws, err := z2.PickFolder(); err != nil || ws == nil {
		t.Fatal(ws, err)
	}
	if s := z2.Startup(); s.Workspace == nil || s.Error != "" {
		t.Fatalf("after Open Folder: %+v", s)
	}
}
