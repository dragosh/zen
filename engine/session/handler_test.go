package session_test

import (
	"net/http/httptest"
	"testing"

	"github.com/dragosh/zenmd/engine/session"
	"github.com/dragosh/zenmd/engine/session/sessiontest"
)

// Handler serves the four asset routes and nothing else.
func TestHandlerRoutes(t *testing.T) {
	dir := sessiontest.Tree(t, map[string]string{
		"README.md": "# Home\n",
		"img/a.png": "\x89PNG\r\n\x1a\n",
		"notes.txt": "secret",
	})
	s := session.New(session.Options{ConfigDir: t.TempDir()})
	t.Cleanup(s.Close)
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	get := func(target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
		return rec
	}
	for _, c := range []struct {
		target string
		code   int
	}{
		{"/_ws/img/a.png", 200},
		{"/_ws/notes.txt", 404},      // not an asset
		{"/_ws/../notes.txt", 404},   // outside
		{"/_plugins/nope/x.js", 404}, // unknown plugin
		{"/_layouts/nope/x.css", 404},
		{"/_print/site", 403}, // no print header
		{"/", 404},            // the frontend is not the session's
		{"/index.html", 404},
		{"/assets/app.js", 404},
	} {
		if rec := get(c.target); rec.Code != c.code {
			t.Errorf("GET %s: %d, want %d", c.target, rec.Code, c.code)
		}
	}
}
