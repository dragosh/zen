package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/zenmark-org/zenmd/kit/desktop"
)

// logCapture keeps the records of the default logger, as "LEVEL message key=value ...".
type logCapture struct {
	mu   sync.Mutex
	recs []string
}

func (h *logCapture) Enabled(context.Context, slog.Level) bool { return true }
func (h *logCapture) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Level.String() + " " + r.Message)
	r.Attrs(func(a slog.Attr) bool { b.WriteString(" " + a.Key + "=" + a.Value.String()); return true })
	h.mu.Lock()
	h.recs = append(h.recs, b.String())
	h.mu.Unlock()
	return nil
}
func (h *logCapture) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *logCapture) WithGroup(string) slog.Handler      { return h }
func (h *logCapture) all() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.recs, "\n")
}

func captureLogs(t *testing.T) *logCapture {
	t.Helper()
	h := &logCapture{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

func wantLog(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("log lacks %q:\n%s", w, got)
		}
	}
}

func TestStartupLineLogged(t *testing.T) {
	h := captureLogs(t)
	logStartup()
	wantLog(t, h.all(), "INFO Zenmark started", "version=", "channel=", "features=")
}

// A service method that returns an error to the UI logs it at error level, with its name.
func TestServiceErrorsAreLogged(t *testing.T) {
	h := captureLogs(t)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "read")
	if _, err := z.Page("missing.md"); err == nil {
		t.Fatal("want an error")
	}
	wantLog(t, h.all(), "ERROR service call failed", "method=Page", "missing.md")
	if err := z.PluginTrust("nope", true); err == nil {
		t.Fatal("want an error")
	}
	wantLog(t, h.all(), "method=PluginTrust")
}

func TestExportAndPrintAreLogged(t *testing.T) {
	h := captureLogs(t)
	b := newBuildApp(t)
	out := filepath.Join(b.dir, "_site")
	if _, err := b.z.BuildWebsite(BuildOptions{Out: out, Portable: true}); err != nil {
		t.Fatal(err)
	}
	wantLog(t, h.all(), "INFO export started", "INFO export finished", "pages=5")
	if err := b.z.PrintPage(); err != nil {
		t.Fatal(err)
	}
	wantLog(t, h.all(), "INFO print", "what=page")
}

func TestNoAbsolutePaths(t *testing.T) {
	h := captureLogs(t)
	b := newBuildApp(t)
	if _, err := b.z.Page("README.md"); err != nil {
		t.Fatal(err)
	}
	_, _ = b.z.Page("nope.md")
	_, _ = b.z.Page("../outside.md")
	out := filepath.Join(b.dir, "_site")
	if _, err := b.z.BuildWebsite(BuildOptions{Out: out, Portable: true}); err != nil {
		t.Fatal(err)
	}
	got := h.all()
	if got == "" {
		t.Fatal("nothing logged")
	}
	home, _ := os.UserHomeDir()
	for _, bad := range []string{b.dir, home, os.TempDir()} {
		if bad != "" && bad != "/" && strings.Contains(got, bad) {
			t.Errorf("the log has %q:\n%s", bad, got)
		}
	}
}

// A panic in an HTTP handler is logged at error level, and still goes on up.
func TestHandlerPanicIsLogged(t *testing.T) {
	h := captureLogs(t)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "read")
	mw := z.middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	func() {
		defer func() { _ = recover() }()
		mw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
	}()
	wantLog(t, h.all(), "ERROR panic in handler", "boom")
}

var _ = desktop.LogApp
