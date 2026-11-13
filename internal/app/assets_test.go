package app

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dragosh/zen/internal/core/plugins"
)

type assetPlugin struct{}

func (assetPlugin) Manifest() plugins.Manifest {
	return plugins.Manifest{ID: "demo", Version: "1.0.0", APIVersion: plugins.APIVersion, Default: true}
}
func (assetPlugin) Assets() fs.FS {
	return fstest.MapFS{"demo.js": {Data: []byte("console.log(1)")}, "demo.css": {Data: []byte("a{}")}}
}
func (assetPlugin) Entry() plugins.Assets { return plugins.Assets{JS: []string{"demo.js"}} }

func TestAssetMiddleware(t *testing.T) {
	dir := tree(t, docs)
	reg := plugins.NewRegistry()
	if err := reg.Register(assetPlugin{}); err != nil {
		t.Fatal(err)
	}
	z, err := newZen(zenOptions{Registry: reg, Dir: dir, Events: &sink{}, Dialogs: &fakeDialogs{}, Open: func(string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer z.close()
	frontend := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	h := z.state.middleware(http.FileServerFS(frontend))

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}

	r := get("/_ws/img/a.png")
	if r.Code != 200 || r.Header().Get("Content-Type") != "image/png" || r.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(r.Body.String(), "\x89PNG") {
		t.Fatalf("png: %d %v", r.Code, r.Header())
	}
	r = get("/_ws/img/x.svg")
	if r.Code != 200 || r.Header().Get("Content-Type") != "image/svg+xml" || r.Header().Get("Content-Security-Policy") != "sandbox" || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("svg: %d %v", r.Code, r.Header())
	}
	for _, bad := range []string{"/_ws/../go.mod", "/_ws/%2e%2e/%2e%2e/etc/passwd", "/_ws/README.md", "/_ws/notes/secret.txt", "/_ws/img/missing.png", "/_ws/", "/_ws//etc/hosts.png", "/_plugins/demo/../x", "/_plugins/nope/demo.js", "/_plugins/demo/missing.js", "/_plugins/demo/"} {
		if r := get(bad); r.Code == 200 {
			t.Fatalf("%s served: %s", bad, r.Body.String())
		}
	}
	if r := get("/_ws/notes/secret.txt"); strings.Contains(r.Body.String(), "nope") || strings.Contains(r.Body.String(), dir) {
		t.Fatalf("leak: %s", r.Body.String())
	}
	r = get("/_plugins/demo/demo.js")
	if r.Code != 200 || r.Body.String() != "console.log(1)" || !strings.Contains(r.Header().Get("Content-Type"), "javascript") || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("plugin js: %d %v", r.Code, r.Header())
	}
	r = get("/")
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

	// Without a workspace /_ws is 404.
	z2, _ := newZen(zenOptions{Registry: reg, Events: &sink{}, Dialogs: &fakeDialogs{}, Open: func(string) error { return nil }})
	defer z2.close()
	rec := httptest.NewRecorder()
	z2.state.middleware(http.FileServerFS(frontend)).ServeHTTP(rec, httptest.NewRequest("GET", "/_ws/img/a.png", nil))
	if rec.Code != 404 {
		t.Fatalf("no workspace: %d", rec.Code)
	}
}
