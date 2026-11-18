package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// The app serves the session's routes itself and gives everything else to the
// frontend with its content security policy.
func TestMiddlewareFrontendAndRoutes(t *testing.T) {
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "read")
	frontend := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	h := z.middleware(http.FileServerFS(frontend))
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}

	if r := get("/_ws/img/a.png"); r.Code != 200 || r.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("workspace file: %d %v", r.Code, r.Header())
	}
	if r := get("/_ws/notes/secret.txt"); r.Code == 200 {
		t.Fatalf("a non-asset was served: %s", r.Body.String())
	}
	r := get("/")
	if r.Code != 200 || !strings.Contains(r.Body.String(), "app") {
		t.Fatalf("frontend: %d", r.Code)
	}
	csp := r.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self'", "style-src 'self'", "img-src 'self' https:"} {
		if !strings.Contains(csp, want) {
			t.Fatalf("csp %q lacks %q", csp, want)
		}
	}
	if strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("csp too loose: %q", csp)
	}
	// A file of the folder never gets the frontend's policy: it is sandboxed.
	if r := get("/_ws/img/x.svg"); r.Header().Get("Content-Security-Policy") != "sandbox" {
		t.Errorf("svg policy: %v", r.Header())
	}
}
