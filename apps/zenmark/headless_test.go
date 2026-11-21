package app

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zenmark-org/zenmd/internal/cli"
)

// The hidden window and the application options of `zenmd export pdf` (behaviour 11): no window shown,
// no Dock icon, no menu, the print document in export mode.
func TestHeadlessPlan(t *testing.T) {
	p := planHeadless(cli.ExportPDF{Dir: "/x/My Docs", Out: "/x/out.pdf"}, "My Docs")
	if p.Out != "/x/out.pdf" || p.Timeout != 30*time.Second || headlessTimeout != 30*time.Second {
		t.Errorf("plan %+v", p)
	}
	w := p.windowOptions()
	u, err := url.Parse(w.URL)
	if err != nil || u.Path != "/" || u.Host != "" || u.Scheme != "" {
		t.Fatalf("window URL %q (%v): must be a path on the app's own origin", w.URL, err)
	}
	q := u.Query()
	if q.Get("window") != "print" || q.Get("export") != "1" || !q.Has("page") || q.Get("page") != "" {
		t.Errorf("query %v", q)
	}
	if !w.Hidden {
		t.Error("the window must be hidden")
	}
	if w.Name != "print" || !strings.Contains(w.Title, "My Docs") {
		t.Errorf("window %q %q", w.Name, w.Title)
	}
	// A default output: exports/pdf in the folder, named after it.
	if got := planHeadless(cli.ExportPDF{Dir: "/x/My Docs"}, "My Docs").Out; got != "/x/My Docs/exports/pdf/My Docs.pdf" {
		t.Errorf("default out %q", got)
	}

	var svc = []application.Service{}
	o := p.appOptions(application.AssetOptions{}, svc)
	if o.Mac.ActivationPolicy != application.ActivationPolicyAccessory {
		t.Errorf("activation policy %v, want accessory (no Dock icon, no menu bar)", o.Mac.ActivationPolicy)
	}
	if o.Mac.ApplicationShouldTerminateAfterLastWindowClosed {
		t.Error("the app ends by Quit, not by its window closing")
	}
	if o.ShouldQuit != nil {
		t.Error("the headless app has no close handshake")
	}
}

// PrintReady writes the PDF once, prints the summary and ends the wait (behaviour 11).
func TestPrintReadySaves(t *testing.T) {
	var out, errOut bytes.Buffer
	var paths, titles []string
	h := newHeadlessService(func(path, title string) (int, error) {
		paths, titles = append(paths, path), append(titles, title)
		return 7, nil
	}, "/x/out.pdf", "My Docs", &out, &errOut)
	h.PrintReady(nil)
	h.PrintReady(nil) // the preview calls it once; a second call changes nothing
	if err := h.wait(time.Second); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/x/out.pdf" || titles[0] != "My Docs" {
		t.Errorf("saved %v %v", paths, titles)
	}
	if out.String() != "Wrote /x/out.pdf (7 pages)\n" || errOut.Len() != 0 {
		t.Errorf("stdout %q stderr %q", out.String(), errOut.String())
	}
	// One page.
	out.Reset()
	one := newHeadlessService(func(string, string) (int, error) { return 1, nil }, "/x/a.pdf", "t", &out, &errOut)
	one.PrintReady(nil)
	if out.String() != "Wrote /x/a.pdf (1 page)\n" {
		t.Errorf("stdout %q", out.String())
	}
	// A failing save ends the wait with that error and prints no summary.
	out.Reset()
	bad := newHeadlessService(func(string, string) (int, error) { return 0, errors.New("cannot write") }, "/x/b.pdf", "t", &out, &errOut)
	bad.PrintReady(nil)
	if err := bad.wait(time.Second); err == nil || !strings.Contains(err.Error(), "cannot write") || out.Len() != 0 {
		t.Errorf("err %v, stdout %q", err, out.String())
	}
}

// Without PrintReady the export fails by itself (behaviour 12).
func TestHeadlessTimeout(t *testing.T) {
	h := newHeadlessService(func(string, string) (int, error) { t.Error("nothing to save"); return 0, nil }, "/x/a.pdf", "t", io.Discard, io.Discard)
	start := time.Now()
	err := h.wait(30 * time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not finish") || strings.Contains(err.Error(), "\n") {
		t.Fatalf("err %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("wait took too long")
	}
	// A PrintReady that comes after the timeout saves nothing.
	h.PrintReady(nil)
	if e := headlessTimeoutError(30 * time.Second); e.Error() != "the page layout did not finish in 30 s" {
		t.Errorf("message %q", e)
	}
}

// Blocks that were not ready are warnings on stderr; the PDF is still written (behaviour 13).
func TestLateBlocksWarn(t *testing.T) {
	var out, errOut bytes.Buffer
	h := newHeadlessService(func(string, string) (int, error) { return 2, nil }, "/x/a.pdf", "t", &out, &errOut)
	h.PrintReady([]string{"diagram", "quiz"})
	if err := h.wait(time.Second); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(errOut.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "diagram") || !strings.Contains(lines[1], "quiz") || !strings.Contains(lines[0], "warning") {
		t.Errorf("stderr %q", errOut.String())
	}
	if out.String() != "Wrote /x/a.pdf (2 pages)\n" {
		t.Errorf("stdout %q", out.String())
	}
}

// The headless window loads the print document through the same middleware as the app: the same
// content security policy, the same session routes, and the print route only for the preview's script.
func TestHeadlessUsesPrintCSP(t *testing.T) {
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "frontend") })
	a := headlessAssets(z, next)
	if a.Middleware == nil || a.Handler == nil {
		t.Fatal("no middleware or handler")
	}
	h := a.Middleware(a.Handler)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/?window=print&export=1&page=", nil))
	if rec.Body.String() != "frontend" || rec.Header().Get("Content-Security-Policy") != frontendCSP {
		t.Errorf("frontend: %q, CSP %q", rec.Body.String(), rec.Header().Get("Content-Security-Policy"))
	}
	req := httptest.NewRequest("GET", "/_print/site?drafts=0", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("print route without the preview's header: %d", rec.Code)
	}
	req = httptest.NewRequest("GET", "/_print/site?drafts=0", nil)
	req.Header.Set("X-Zen-Print", "1")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "zen-print-doc") {
		t.Errorf("print route: %d %.80s", rec.Code, rec.Body.String())
	}
	// The services of the headless app are the Zen service and the one PrintReady method.
	svcs := headlessServices(z, newHeadlessService(nil, "", "", io.Discard, io.Discard))
	if len(svcs) != 2 {
		t.Errorf("%d services, want Zen and the headless service", len(svcs))
	}
}

// Folder checks before any window: a folder with no pages is an error (behaviour 12).
func TestHeadlessChecksFolder(t *testing.T) {
	var out, errOut bytes.Buffer
	err := ExportPDF(cli.ExportPDF{Dir: t.TempDir() + "/nosuch", Out: ""}, &out, &errOut)
	if err == nil || strings.Contains(err.Error(), "\n") || out.Len() != 0 {
		t.Errorf("missing folder: %v", err)
	}
	empty := t.TempDir()
	err = ExportPDF(cli.ExportPDF{Dir: empty, Out: empty + "/e.pdf"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "no pages") {
		t.Errorf("empty folder: %v", err)
	}
}

// A preview that cannot load ends the export at once with its message (behaviour 12).
func TestPrintFailedEndsExport(t *testing.T) {
	var out bytes.Buffer
	h := newHeadlessService(func(string, string) (int, error) { t.Error("nothing to save"); return 0, nil }, "/x/a.pdf", "t", &out, io.Discard)
	start := time.Now()
	h.PrintFailed("the app answered 500\nsecond line")
	h.PrintReady(nil) // too late: ignored
	err := h.wait(10 * time.Second)
	if err == nil || err.Error() != "export pdf: the app answered 500 second line" || out.Len() != 0 {
		t.Fatalf("err %v, stdout %q", err, out.String())
	}
	if time.Since(start) > time.Second {
		t.Error("the failure must not wait for the timeout")
	}
}

// The default folder is created for the headless export; a chosen -o is left alone.
func TestEnsureDefaultPDFDir(t *testing.T) {
	dir := t.TempDir()
	e := cli.ExportPDF{Dir: dir}
	plan := planHeadless(e, "x")
	if err := ensurePDFDir(e, plan.Out); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "exports", "pdf")); err != nil || !fi.IsDir() {
		t.Errorf("exports/pdf not created: %v", err)
	}
	chosen := cli.ExportPDF{Dir: dir, Out: filepath.Join(dir, "nosuch", "a.pdf")}
	if err := ensurePDFDir(chosen, chosen.Out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nosuch")); err == nil {
		t.Error("-o: its folder is the user's to make")
	}
}
