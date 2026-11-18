package session

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// recHandler keeps the records of the default logger for a test.
type recHandler struct {
	mu   sync.Mutex
	recs []string
}

func (h *recHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Level.String() + " " + r.Message)
	r.Attrs(func(a slog.Attr) bool { b.WriteString(" " + a.Key + "=" + a.Value.String()); return true })
	h.mu.Lock()
	h.recs = append(h.recs, b.String())
	h.mu.Unlock()
	return nil
}
func (h *recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recHandler) WithGroup(string) slog.Handler      { return h }
func (h *recHandler) all() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.recs, "\n")
}

// Plugin failures go to the log with source=plugin (ui/developer-tab); the
// payload (page text) is never logged.
func TestPluginFailuresAreLogged(t *testing.T) {
	h := &recHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := pluginTree(t, pluginDocs())
	z, ev := openActing(t, dir, t.TempDir(), newActor("act"))
	if _, err := z.PluginInvoke("act", "README.md", `"fail"`); err == nil {
		t.Fatal("want an error")
	}
	call, err := z.PluginStream("act", "README.md", `"fail"`)
	if err != nil {
		t.Fatal(err)
	}
	finish(t, ev, call)

	got := h.all()
	for _, want := range []string{
		"WARN plugin act: action failed source=plugin",
		"WARN plugin act: stream failed: it broke source=plugin",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"fail"`) {
		t.Errorf("the payload is logged:\n%s", got)
	}
}
