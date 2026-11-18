package session

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dragosh/zenmd/internal/core/site"
)

func printZen(t *testing.T, files map[string]string) *Session {
	t.Helper()
	z, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), true)
	return z
}

func printGet(z *Session, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	z.servePrint(rec, req)
	return rec
}

var fromWindow = map[string]string{"X-Zen-Print": "1"}

func TestPrintRoute(t *testing.T) {
	z := printZen(t, map[string]string{
		"README.md":   "# Home\n\nhello\n",
		"guide/a.md":  "# Ay\n\nsee [b](b.md#top)\n",
		"guide/b.md":  "---\ndraft: true\n---\n# Bee\n\n## Top\n",
		"notes/c.txt": "secret",
	})
	z.now = func() time.Time { return time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC) }

	rec := printGet(z, "/_print/site", fromWindow)
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	body := rec.Body.String()
	// The same output as PrintDocument over the same pages.
	want, _, err := site.PrintDocument(&buildInput{s: z}, site.PrintOptions{Date: "17 November 2026", SiteID: "zen"})
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
	z.servePrint(rec, req)
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
	z, _, _ = openWith(t, dir, t.TempDir(), true)
	body = printGet(z, "/_print/site", fromWindow).Body.String()
	if !strings.Contains(body, "hello from the backend") {
		t.Errorf("an allowed plugin is missing:\n%s", body)
	}
}
