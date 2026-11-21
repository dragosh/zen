package plugins

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

type logRec struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

type logCapture struct {
	mu   sync.Mutex
	recs []logRec
}

func (h *logCapture) Enabled(context.Context, slog.Level) bool { return true }
func (h *logCapture) Handle(_ context.Context, r slog.Record) error {
	rec := logRec{level: r.Level, msg: r.Message, attrs: map[string]string{}}
	r.Attrs(func(a slog.Attr) bool { rec.attrs[a.Key] = a.Value.String(); return true })
	h.mu.Lock()
	h.recs = append(h.recs, rec)
	h.mu.Unlock()
	return nil
}
func (h *logCapture) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *logCapture) WithGroup(string) slog.Handler      { return h }

func TestLog(t *testing.T) {
	h := &logCapture{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	Log("sandbox", slog.LevelInfo, "run python main.py")
	Log("quiz t", slog.LevelWarn, "checked", "n", 3)
	if len(h.recs) != 2 {
		t.Fatalf("records %+v", h.recs)
	}
	r := h.recs[0]
	if r.level != slog.LevelInfo || r.msg != "sandbox: run python main.py" || r.attrs["source"] != "plugin" {
		t.Errorf("record %+v", r)
	}
	r = h.recs[1]
	if r.level != slog.LevelWarn || !strings.HasPrefix(r.msg, "quiz t: ") || r.attrs["source"] != "plugin" || r.attrs["n"] != "3" {
		t.Errorf("record %+v", r)
	}
}
