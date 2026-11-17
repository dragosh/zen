package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dragosh/zen/internal/core/site"
)

func printZen(t *testing.T, files map[string]string) *Zen {
	t.Helper()
	z, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), true)
	return z
}

func printGet(z *Zen, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	z.state.servePrint(rec, req)
	return rec
}

var fromWindow = map[string]string{"X-Zen-Print": "1"}

func TestPrintRoute(t *testing.T) {
	old := printNow
	printNow = func() time.Time { return time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { printNow = old })
	z := printZen(t, map[string]string{
		"README.md":   "# Home\n\nhello\n",
		"guide/a.md":  "# Ay\n\nsee [b](b.md#top)\n",
		"guide/b.md":  "---\ndraft: true\n---\n# Bee\n\n## Top\n",
		"notes/c.txt": "secret",
	})

	rec := printGet(z, "/_print/site", fromWindow)
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	body := rec.Body.String()
	// The same output as PrintDocument over the same pages.
	want, _, err := site.PrintDocument(&buildInput{s: z.state}, site.PrintOptions{Date: "17 November 2026", SiteID: "zen"})
	if err != nil {
		t.Fatal(err)
	}
	if body != want {
		t.Errorf("the route and PrintDocument differ:\n%s\n---\n%s", body, want)
	}
	for _, s := range []string{"17 November 2026", "<h1 ", ">Ay<", "zen-print-page"} {
		if !strings.Contains(body, s) {
			t.Errorf("document lacks %q:\n%s", s, body)
		}
	}
	if strings.Contains(body, "Bee") || strings.Contains(body, "secret") {
		t.Error("a draft or a non-page file is in the document")
	}
	// A link to a draft that is left out prints as text.
	if strings.Contains(body, `b.md`) {
		t.Errorf("a link to a page that is not in the document has a target:\n%s", body)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(rec.Header().Get("Cache-Control"), "no-store") {
		t.Errorf("headers %v", rec.Header())
	}

	// drafts=1 and a section.
	if b := printGet(z, "/_print/site?drafts=1", fromWindow).Body.String(); !strings.Contains(b, "Bee") || !strings.Contains(b, `href="#zen-print-`) {
		t.Errorf("drafts=1:\n%s", b)
	}
	rec = printGet(z, "/_print/site?section="+url.QueryEscape("guide/a.md"), fromWindow)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "hello") {
		t.Errorf("section: %d\n%s", rec.Code, rec.Body.String())
	}

	// Only the preview window's own requests are answered, and nothing else is read.
	if rec := printGet(z, "/_print/site", nil); rec.Code != http.StatusForbidden {
		t.Errorf("a request without the window's header: %d", rec.Code)
	}
	if rec := printGet(z, "/_print/site", map[string]string{"X-Zen-Print": "1", "Sec-Fetch-Site": "cross-site"}); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site: %d", rec.Code)
	}
	for _, bad := range []string{"/_print/", "/_print/site/x", "/_print/other", "/_print/site/../x", "/_print/..%2fsite"} {
		if rec := printGet(z, bad, fromWindow); rec.Code == 200 {
			t.Errorf("%s answered", bad)
		}
	}
	for _, sec := range []string{"../README.md", "notes/c.txt", "/etc/passwd", "nope.md", "guide"} {
		if rec := printGet(z, "/_print/site?section="+url.QueryEscape(sec), fromWindow); rec.Code != http.StatusBadRequest || strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("section %q: %d", sec, rec.Code)
		}
	}
	req := httptest.NewRequest("POST", "/_print/site", nil)
	req.Header.Set("X-Zen-Print", "1")
	rec = httptest.NewRecorder()
	z.state.servePrint(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", rec.Code)
	}
}

func TestPrintRouteLimit(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 501; i++ {
		files["p"+strings.Repeat("x", i%7)+string(rune('a'+i%26))+string(rune('a'+(i/26)%26))+string(rune('a'+i/676))+".md"] = "# P\n"
	}
	z := printZen(t, files)
	rec := printGet(z, "/_print/site", fromWindow)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "print a section") {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestPrintSiteTrust(t *testing.T) {
	files := pluginDocs()
	dir := pluginTree(t, files)
	cfg := t.TempDir()

	// Not decided yet: the blocked plugin's output and scripts stay out of the document.
	z, _, _ := openWith(t, dir, cfg, false)
	body := printGet(z, "/_print/site", fromWindow).Body.String()
	if strings.Contains(body, "hello from the backend") || strings.Contains(body, `data-zen-module="plugin:hello"`) || strings.Contains(body, "zen.register") {
		t.Errorf("a blocked plugin reached the print document:\n%s", body)
	}
	if exists(dir + "/ran.txt") {
		t.Error("an unapproved backend ran")
	}

	// Approved in the app: its output is in the document, as on the page.
	z, _, _ = openWith(t, dir, cfg, true)
	body = printGet(z, "/_print/site", fromWindow).Body.String()
	if !strings.Contains(body, "hello from the backend") {
		t.Errorf("an allowed plugin is missing:\n%s", body)
	}
}

// fakePrintWindow is a preview window that is just counted.
type fakePrintWindow struct {
	focused int
	onClose func()
}

func (w *fakePrintWindow) Focus()               { w.focused++ }
func (w *fakePrintWindow) Close()               { w.onClose() }
func (w *fakePrintWindow) Position() (int, int) { return 0, 0 }
func (w *fakePrintWindow) Size() (int, int)     { return 0, 0 }

func TestPrintWindowAndMenuActions(t *testing.T) {
	var specs []windowSpec
	var wins []*fakePrintWindow
	old := newPrintWindow
	newPrintWindow = func(s windowSpec, onClose func()) appWindow {
		w := &fakePrintWindow{onClose: onClose}
		specs, wins = append(specs, s), append(wins, w)
		return w
	}
	t.Cleanup(func() { newPrintWindow = old })

	files := map[string]string{"README.md": "# Home\n", "docs/a b.md": "# A\n"}
	z, ev, _ := openWith(t, pluginTree(t, files), t.TempDir(), true)
	if _, err := z.Page("docs/a b.md"); err != nil { // the page the main window shows
		t.Fatal(err)
	}

	if err := z.PrintSite(); err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].Name != "print" || !strings.HasSuffix(specs[0].Title, " — Print") ||
		specs[0].URL != "/?window=print&page="+url.QueryEscape("docs/a b.md") {
		t.Fatalf("spec %+v", specs)
	}
	if !strings.HasPrefix(specs[0].Title, z.Startup().Workspace.Name) {
		t.Errorf("title %q does not start with the folder name %q", specs[0].Title, z.Startup().Workspace.Name)
	}
	// Opened twice: the first is focused, no second window.
	if err := z.PrintSite(); err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || wins[0].focused != 1 {
		t.Errorf("windows %d, focused %d", len(specs), wins[0].focused)
	}
	// Closed: the next one opens again.
	wins[0].onClose()
	if err := z.PrintSite(); err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Errorf("no new window after closing: %d", len(specs))
	}

	// Print Page asks the main window.
	ev.reset()
	if err := z.PrintPage(); err != nil {
		t.Fatal(err)
	}
	if !hasEvent(ev, EventPrintPage) || EventPrintPage != "menu:print-page" {
		t.Errorf("events %v", ev.all())
	}
}
