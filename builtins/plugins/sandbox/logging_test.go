package sandbox

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

// logs keeps the records of the default logger as "LEVEL message source=x".
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
func (h *logs) list() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.recs...)
}
func (h *logs) text() string { return strings.Join(h.list(), "\n") }

func capture(t *testing.T) *logs {
	t.Helper()
	h := &logs{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

func want(t *testing.T, h *logs, re string) {
	t.Helper()
	if !regexp.MustCompile(re).MatchString(h.text()) {
		t.Errorf("the log lacks %s:\n%s", re, h.text())
	}
}

const codePayload = `{"op":"run","image":"python","cmd":"python main.py","file":"main.py","code":"print('TOPSECRET-CODE')\n"}`

func TestLogRun(t *testing.T) {
	fake(t, `echo "hello"; echo "oops" >&2; exit 3`)
	h := capture(t)
	if _, err := runStream(t, context.Background(), cfg(nil), codePayload); err != nil {
		t.Fatal(err)
	}
	// the steps, in order, and never the program's output (it is in the block)
	steps := []string{
		`INFO sandbox: run python: python main\.py`,
		`INFO sandbox: checking the container service`,
		`INFO sandbox: the container service is running`,
		`INFO sandbox: wrote main\.py \(\d+ bytes\) to a temporary folder, mounted at /work`,
		`INFO sandbox: starting container zen-[0-9a-f]{12} \(image python\)`,
		`INFO sandbox: the program is running \(first output after \d+\.\d s\)`,
		`INFO sandbox: exit 3 in \d+\.\d s`,
		`INFO sandbox: removed the temporary folder`,
	}
	text := h.text()
	at := 0
	for _, st := range steps {
		loc := regexp.MustCompile(`(?m)^` + st + ` source=plugin$`).FindStringIndex(text[at:])
		if loc == nil {
			t.Fatalf("the log lacks %s after the earlier steps:\n%s", st, text)
		}
		at += loc[1]
	}
	for _, out := range []string{"hello", "oops"} {
		if strings.Contains(text, out) {
			t.Errorf("the program's output %q is logged:\n%s", out, text)
		}
	}
}

func TestLogTimeout(t *testing.T) {
	fake(t, `echo started; exec sleep 30`)
	h := capture(t)
	if _, err := runStream(t, context.Background(), cfg(map[string]any{"timeout": 1}), codePayload); err != nil {
		t.Fatal(err)
	}
	want(t, h, `(?m)^WARN sandbox: exit -1 in \d+\.\d s`)
}

func TestLogStopped(t *testing.T) {
	fake(t, `echo started; exec sleep 30`)
	h := capture(t)
	c, cancel := context.WithCancel(context.Background())
	go func() {
		for i := 0; i < 100 && !strings.Contains(h.text(), "the program is running"); i++ {
			sleepMs(50)
		}
		cancel()
	}()
	if _, err := runStream(t, c, cfg(nil), codePayload); err == nil {
		t.Fatal("cancel returned nil")
	}
	want(t, h, `(?m)^INFO sandbox: stopped source=plugin$`)
}

func TestLogOutputLimit(t *testing.T) {
	fake(t, `echo started; exec sleep 30`)
	h := capture(t)
	c, cancel := context.WithCancelCause(context.Background())
	go func() {
		for i := 0; i < 100 && !strings.Contains(h.text(), "the program is running"); i++ {
			sleepMs(50)
		}
		cancel(plugins.ErrOutputLimit)
	}()
	if _, err := runStream(t, c, cfg(nil), codePayload); err == nil {
		t.Fatal("cancel returned nil")
	}
	want(t, h, `(?m)^WARN sandbox: output over the limit`)
}

func TestLogServiceStopped(t *testing.T) {
	d := fake(t, "exit 0")
	os.WriteFile(filepath.Join(d, "stopped"), nil, 0o644)
	h := capture(t)
	if _, err := runStream(t, context.Background(), cfg(nil), codePayload); err == nil {
		t.Fatal("want an error")
	}
	want(t, h, `(?m)^WARN sandbox: the container service is not running source=plugin$`)
	if strings.Contains(h.text(), "starting container") {
		t.Errorf("a run that never started is logged as started:\n%s", h.text())
	}
}

func TestLogStatusProbe(t *testing.T) {
	fake(t, "")
	h := capture(t)
	if _, err := New().(plugins.ActionHook).Action(cfg(nil), []byte(`{"op":"status"}`)); err != nil {
		t.Fatal(err)
	}
	want(t, h, `(?m)^DEBUG sandbox: service running source=plugin$`)
}

// The code, the image reference and the config never reach the log.
func TestNoSecretsLogged(t *testing.T) {
	fake(t, `echo hi`)
	h := capture(t)
	if _, err := runStream(t, context.Background(), cfg(nil), codePayload); err != nil {
		t.Fatal(err)
	}
	if _, err := New().(plugins.ActionHook).Action(cfg(nil), []byte(`{"op":"status"}`)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"TOPSECRET", "python:3.12-slim", "node:22-alpine", "print("} {
		if strings.Contains(h.text(), bad) {
			t.Errorf("the log has %q:\n%s", bad, h.text())
		}
	}
	for _, r := range h.list() {
		if !strings.HasSuffix(r, " source=plugin") || !strings.Contains(r, " sandbox: ") {
			t.Errorf("entry %q is not source=plugin with the id first", r)
		}
	}
}

func sleepMs(n int) { time.Sleep(time.Duration(n) * time.Millisecond) }
