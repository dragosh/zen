package app

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// frontendCSP is sent with everything that is not a workspace or plugin file.
const frontendCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' https:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'"

// middleware serves /_ws/<path> from the workspace and /_plugins/<id>/<file>
// from plugin asset file systems; everything else goes to next.
func (s *state) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case strings.HasPrefix(p, "/_ws/"):
			s.serveWorkspace(w, r, strings.TrimPrefix(p, "/_ws/"))
		case strings.HasPrefix(p, "/_layouts/"):
			s.serveLayout(w, r, strings.TrimPrefix(p, "/_layouts/"))
		case strings.HasPrefix(p, "/_plugins/"):
			s.servePlugin(w, r, strings.TrimPrefix(p, "/_plugins/"))
		default:
			w.Header().Set("Content-Security-Policy", frontendCSP)
			next.ServeHTTP(w, r)
		}
	})
}

// setFileHeaders sets the headers of a served file. workspace is true for the
// user's files: scripts and documents there are text/plain, so that nothing
// of the workspace can be loaded as a script of the app (script-src 'self').
func setFileHeaders(w http.ResponseWriter, name string, workspace bool) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-cache")
	ct := mime.TypeByExtension(strings.ToLower(path.Ext(name)))
	switch strings.ToLower(path.Ext(name)) {
	case ".svg":
		ct = "image/svg+xml"
	case ".avif":
		ct = "image/avif"
	case ".webp":
		ct = "image/webp"
	case ".js":
		ct = "text/javascript; charset=utf-8"
	case ".css":
		ct = "text/css; charset=utf-8"
	}
	if workspace {
		switch strings.ToLower(path.Ext(name)) {
		case ".js", ".mjs", ".cjs", ".html", ".htm", ".xhtml", ".xml", ".wasm":
			ct = "text/plain; charset=utf-8"
		}
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	h.Set("Content-Type", ct)
	// Files are never documents of the app: no script runs inside them.
	h.Set("Content-Security-Policy", "sandbox")
}

func (s *state) serveWorkspace(w http.ResponseWriter, r *http.Request, rel string) {
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if ws == nil || rel == "" {
		http.NotFound(w, r)
		return
	}
	f, err := ws.Asset(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	setFileHeaders(w, filepath.Base(rel), true)
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

func (s *state) servePlugin(w http.ResponseWriter, r *http.Request, rest string) {
	s.mu.Lock()
	reg := s.reg
	s.mu.Unlock()
	fsys, file, ok := assetOf(reg, rest)
	if !ok {
		http.NotFound(w, r)
		return
	}
	serveFromFS(w, r, fsys, file)
}

// serveLayout serves /_layouts/<id>/<file>: the css and js files a layout
// lists, nothing else.
func (s *state) serveLayout(w http.ResponseWriter, r *http.Request, rest string) {
	s.mu.Lock()
	reg := s.lreg
	s.mu.Unlock()
	if reg == nil {
		http.NotFound(w, r)
		return
	}
	fsys, file, ok := assetOf(reg, rest)
	if !ok {
		http.NotFound(w, r)
		return
	}
	serveFromFS(w, r, fsys, file)
}

// assetFSer is the plugin and the layout registry: both hand out the files a
// plugin or layout lists.
type assetFSer interface {
	AssetFS(id string) (fs.FS, bool)
}

// assetOf resolves "<id>/<file>" (the rest of /_plugins/ and /_layouts/ URLs)
// to the file system of the plugin or layout and a clean file name. The
// static build uses it too, so the app and a built site serve the same files.
func assetOf(reg assetFSer, rest string) (fs.FS, string, bool) {
	id, file, ok := strings.Cut(rest, "/")
	if !ok || id == "" || !fs.ValidPath(file) || file == "." {
		return nil, "", false
	}
	fsys, ok := reg.AssetFS(id)
	if !ok {
		return nil, "", false
	}
	return fsys, file, true
}

func serveFromFS(w http.ResponseWriter, r *http.Request, fsys fs.FS, file string) {
	f, err := fsys.Open(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	setFileHeaders(w, file, false)
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, "", fi.ModTime(), rs)
		return
	}
	io.Copy(w, f)
}
