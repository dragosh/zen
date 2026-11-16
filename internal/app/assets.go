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
		case strings.HasPrefix(p, "/_plugins/"):
			s.servePlugin(w, r, strings.TrimPrefix(p, "/_plugins/"))
		default:
			w.Header().Set("Content-Security-Policy", frontendCSP)
			next.ServeHTTP(w, r)
		}
	})
}

func setFileHeaders(w http.ResponseWriter, name string) {
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
	setFileHeaders(w, filepath.Base(rel))
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

func (s *state) servePlugin(w http.ResponseWriter, r *http.Request, rest string) {
	id, file, ok := strings.Cut(rest, "/")
	if !ok || id == "" || !fs.ValidPath(file) || file == "." {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	reg := s.reg
	s.mu.Unlock()
	fsys, ok := reg.AssetFS(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
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
	setFileHeaders(w, file)
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, "", fi.ModTime(), rs)
		return
	}
	io.Copy(w, f)
}
