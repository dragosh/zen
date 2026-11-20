package desktop

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogRingSequenceAndBound(t *testing.T) {
	r := NewLogRing(3)
	for i := 1; i <= 5; i++ {
		e := r.Add("info", "app", strings.Repeat("x", i))
		if e.Seq != i {
			t.Fatalf("seq = %d, want %d", e.Seq, i)
		}
	}
	got := r.Since(0)
	if len(got) != 3 || got[0].Seq != 3 || got[2].Seq != 5 {
		t.Fatalf("Since(0) = %+v, want seq 3..5", got)
	}
	if got := r.Since(4); len(got) != 1 || got[0].Seq != 5 {
		t.Fatalf("Since(4) = %+v", got)
	}
	if got := r.Since(5); len(got) != 0 {
		t.Fatalf("Since(5) = %+v, want none", got)
	}
	if got := r.Since(99); len(got) != 0 {
		t.Fatalf("Since(99) = %+v, want none", got)
	}
	if got := r.Since(-3); len(got) != 3 {
		t.Fatalf("Since(-3) = %d entries", len(got))
	}
}

func TestLogRingTruncatesMessages(t *testing.T) {
	r := NewLogRing(2)
	e := r.Add("warn", "plugin", strings.Repeat("é", 5000))
	if len(e.Message) > MaxLogMessage+len("…") {
		t.Errorf("message is %d bytes", len(e.Message))
	}
	if !strings.HasSuffix(e.Message, "…") {
		t.Error("a truncated message ends with an ellipsis")
	}
	if e := r.Add("nonsense", "", "m"); e.Level != "info" || e.Source != "app" {
		t.Errorf("defaults: %+v", e)
	}
}

func TestLogRingConcurrent(t *testing.T) {
	r := NewLogRing(50)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				r.Add("debug", "app", "m")
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = r.Since(i)
			}
		}()
	}
	wg.Wait()
	got := r.Since(0)
	if len(got) != 50 {
		t.Fatalf("kept %d, want 50", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Seq != got[i-1].Seq+1 {
			t.Fatalf("sequence gap at %d", i)
		}
	}
	if got[len(got)-1].Seq != 8*500 {
		t.Errorf("last seq = %d", got[len(got)-1].Seq)
	}
}

func TestLogHandlerTeesAndClassifies(t *testing.T) {
	var buf bytes.Buffer
	r := NewLogRing(10)
	l := slog.New(NewLogHandler(r, slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	l.Info("opened", "folder", "docs")
	l.Warn("plugin x failed", "source", "plugin")
	l.With("source", "plugin").Error("boom", "code", 2)
	l.Debug("quiet")
	got := r.Since(0)
	if len(got) != 4 {
		t.Fatalf("ring has %d entries, want 4 (debug is kept)", len(got))
	}
	if got[0].Level != "info" || got[0].Source != "app" || got[0].Message != "opened folder=docs" {
		t.Errorf("entry 0 = %+v", got[0])
	}
	if got[1].Source != "plugin" || got[1].Level != "warn" || got[1].Message != "plugin x failed" {
		t.Errorf("entry 1 = %+v", got[1])
	}
	if got[2].Source != "plugin" || got[2].Message != "boom code=2" {
		t.Errorf("entry 2 = %+v", got[2])
	}
	if got[3].Level != "debug" {
		t.Errorf("entry 3 = %+v", got[3])
	}
	out := buf.String()
	if !strings.Contains(out, "opened") || !strings.Contains(out, "boom") || strings.Contains(out, "quiet") {
		t.Errorf("stderr tee = %q (info and up only)", out)
	}
}

type recorder struct {
	mu     sync.Mutex
	events [][]LogEntry
}

func (r *recorder) Emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "log:entry" {
		r.events = append(r.events, data.([]LogEntry))
	}
}

func (r *recorder) snapshot() [][]LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]LogEntry(nil), r.events...)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestStreamBatchesAndRateLimits(t *testing.T) {
	r := NewLogRing(1000)
	rec := &recorder{}
	stop := r.Stream(rec, "log:entry", 20*time.Millisecond, 10)
	defer stop()
	for i := 0; i < 35; i++ {
		r.Add("info", "plugin", "chatty")
	}
	waitFor(t, func() bool {
		n := 0
		for _, b := range rec.snapshot() {
			n += len(b)
		}
		return n == 35
	})
	events := rec.snapshot()
	if len(events) < 4 {
		t.Errorf("%d events for 35 entries with batch 10, want at least 4", len(events))
	}
	seq := 0
	for _, b := range events {
		if len(b) > 10 {
			t.Errorf("batch of %d, max 10", len(b))
		}
		for _, e := range b {
			if e.Seq != seq+1 {
				t.Fatalf("seq %d after %d", e.Seq, seq)
			}
			seq = e.Seq
		}
	}
}

func TestStreamStops(t *testing.T) {
	r := NewLogRing(10)
	rec := &recorder{}
	stop := r.Stream(rec, "log:entry", 5*time.Millisecond, 10)
	stop()
	stop() // twice is fine
	r.Add("info", "app", "after")
	time.Sleep(40 * time.Millisecond)
	if n := len(rec.snapshot()); n != 0 {
		t.Errorf("%d events after stop", n)
	}
}

func TestLogAppWritesAppEntries(t *testing.T) {
	ring := NewLogRing(10)
	prev := slog.Default()
	slog.SetDefault(slog.New(NewLogHandler(ring, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	LogApp(slog.LevelInfo, "started", "version", "1.2")
	LogApp(slog.LevelWarn, "careful")
	LogApp(slog.LevelError, "broke", "method", "Page")
	LogApp(slog.LevelDebug, "detail")
	got := ring.Since(0)
	want := []LogEntry{
		{Level: "info", Source: "app", Message: "started version=1.2"},
		{Level: "warn", Source: "app", Message: "careful"},
		{Level: "error", Source: "app", Message: "broke method=Page"},
		{Level: "debug", Source: "app", Message: "detail"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %+v", got)
	}
	for i, w := range want {
		if got[i].Level != w.Level || got[i].Source != w.Source || got[i].Message != w.Message {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], w)
		}
	}
	if got[0].Seq != 1 {
		t.Errorf("the first line has seq %d", got[0].Seq)
	}
}

// Debug entries are limited per kind to 20 a second, then one "… N more" entry.
func TestDebugRateLimit(t *testing.T) {
	ring := NewLogRing(500)
	log := slog.New(NewLogHandler(ring, nil))
	for i := 0; i < 100; i++ {
		log.Debug("file event", "n", i)
	}
	log.Debug("page rendered", "path", "a.md") // another kind has its own budget
	log.Info("info is never limited")
	for i := 0; i < 50; i++ {
		log.Info("info", "n", i)
	}
	count := func(prefix string) (n int) {
		for _, e := range ring.Since(0) {
			if strings.HasPrefix(e.Message, prefix) {
				n++
			}
		}
		return
	}
	if n := count("file event"); n != 20 {
		t.Errorf("%d debug entries of one kind in a second, want 20", n)
	}
	if count("page rendered") != 1 || count("info") != 51 {
		t.Errorf("other kinds were limited: %+v", ring.Since(0))
	}
	waitFor(t, func() bool { return count("… 80 more") == 1 })
	var more LogEntry
	for _, e := range ring.Since(0) {
		if strings.HasPrefix(e.Message, "… 80 more") {
			more = e
		}
	}
	if more.Level != "debug" {
		t.Errorf("the summary is %+v", more)
	}
}
