package app

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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

// External plugins serve only the files their manifest lists, and only once
// the plugin is allowed.
func TestExternalPluginAssets(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	cfg := t.TempDir()
	frontend := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	get := func(z *Zen, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		z.state.middleware(http.FileServerFS(frontend)).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}

	blocked, _, _ := openWith(t, dir, cfg, false)
	if r := get(blocked, "/_plugins/hello/hello.js"); r.Code == 200 {
		t.Fatal("asset of a plugin that is not allowed was served")
	}
	if err := blocked.PluginTrust("hello", true); err != nil {
		t.Fatal(err)
	}

	z, _, _ := openWith(t, dir, cfg, false)
	r := get(z, "/_plugins/hello/hello.js")
	if r.Code != 200 || !strings.Contains(r.Body.String(), "zen.register") || !strings.HasPrefix(r.Header().Get("Content-Type"), "text/javascript") || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("js: %d %v %q", r.Code, r.Header(), r.Body.String())
	}
	r = get(z, "/_plugins/hello/hello.css")
	if r.Code != 200 || !strings.HasPrefix(r.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("css: %d %v", r.Code, r.Header())
	}
	for _, bad := range []string{
		"/_plugins/hello/secret.txt", "/_plugins/hello/plugin.yml", "/_plugins/hello/hello.sh", "/_plugins/hello/lib/more.js",
		"/_plugins/hello/", "/_plugins/hello", "/_plugins/hello/../hello/hello.js", "/_plugins/hello/..%2f..%2f..%2fREADME.md",
		"/_plugins/hello/%2e%2e/%2e%2e/%2e%2e/README.md", "/_plugins/hello//etc/hosts", "/_plugins/hello/./hello.js",
		"/_plugins/hello/HELLO.JS/", "/_plugins/nope/hello.js", "/_plugins/../README.md",
	} {
		if r := get(z, bad); r.Code == 200 {
			t.Errorf("%s served: %q", bad, r.Body.String())
		}
	}
	// A listed file replaced by a link out of the plugin folder is not served
	// (and the plugin needs approval again when the folder is reopened).
	js := filepath.Join(dir, ".zen", "plugins", "hello", "hello.js")
	if err := os.Remove(js); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "README.md"), js); err != nil {
		t.Fatal(err)
	}
	if r := get(z, "/_plugins/hello/hello.js"); r.Code == 200 {
		t.Errorf("symlink out of the plugin folder served: %q", r.Body.String())
	}
	// Built-in assets still work next to external ones.
	reg := plugins.NewRegistry()
	if err := reg.Register(assetPlugin{}); err != nil {
		t.Fatal(err)
	}
	zb, err := newZen(zenOptions{Registry: reg, ConfigDir: cfg, Dir: tree(t, docs), Events: &sink{}, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	defer zb.close()
	if r := get(zb, "/_plugins/demo/demo.js"); r.Code != 200 {
		t.Errorf("built-in asset: %d", r.Code)
	}
}
