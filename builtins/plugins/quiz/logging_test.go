package quiz

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

type logs struct {
	mu   sync.Mutex
	recs []string
}

func (h *logs) Enabled(context.Context, slog.Level) bool { return true }
func (h *logs) Handle(_ context.Context, r slog.Record) error {
	s := r.Level.String() + " " + r.Message
	r.Attrs(func(a slog.Attr) bool { s += " " + a.Key + "=" + a.Value.String(); return true })
	h.mu.Lock()
	h.recs = append(h.recs, s)
	h.mu.Unlock()
	return nil
}
func (h *logs) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *logs) WithGroup(string) slog.Handler      { return h }
func (h *logs) text() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.recs, "\n")
}

func capture(t *testing.T) *logs {
	t.Helper()
	h := &logs{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

func TestLogCheckScore(t *testing.T) {
	h := capture(t)
	p := New()
	renderPage(t, p, plugins.Context{})
	ctx := plugins.Context{Path: "quizzes/http.md"}
	const base = `{"op":"check","quiz":"http-basics","page":"quizzes/http.md","answers":`
	act(t, p, ctx, base+`{"0":1,"1":[0,1],"2":"Transport Layer Security"}}`)
	act(t, p, ctx, base+`{"0":0,"1":[0],"2":"SECRETANSWER-text"}}`)
	got := h.text()
	for _, w := range []string{
		"INFO quiz http-basics: checked, 100% (passed) source=plugin",
		"INFO quiz http-basics: checked, 0% (not passed) source=plugin",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("log lacks %q:\n%s", w, got)
		}
	}
}

// Answers, questions, options and explanations never reach the log.
func TestNoSecretsLogged(t *testing.T) {
	h := capture(t)
	p := New()
	renderPage(t, p, plugins.Context{})
	ctx := plugins.Context{Path: "quizzes/http.md"}
	act(t, p, ctx, `{"op":"check","quiz":"http-basics","page":"quizzes/http.md","answers":{"0":1,"2":"SECRETANSWER-text"}}`)
	got := h.text()
	for _, bad := range []string{"SECRETANSWER", "SECRETEXPLAIN", "idempotent", "PUT", "Transport", "2xx"} {
		if strings.Contains(got, bad) {
			t.Errorf("the log has %q:\n%s", bad, got)
		}
	}
	if got == "" {
		t.Error("nothing is logged")
	}
}
