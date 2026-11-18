package session

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dragosh/zenmd/internal/core/site"
)

const (
	printRoute  = "/_print/site"
	printHeader = "X-Zen-Print" // only the preview window's script sends it
)

// PrintOptions select what PrintDocument returns.
type PrintOptions struct {
	Drafts  bool   // include draft pages
	Section string // a page of the sidebar tree: print only its subtree; "" for all
}

// PrintDocument is the whole open folder (or one section of it) as a single
// print document, rendered like the app renders: unlike a build, the reader's
// own plugin options apply. Errors are site.ErrTooManyPages and
// site.ErrNotASection, or a render failure.
func (s *Session) PrintDocument(o PrintOptions) (string, error) {
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if ws == nil {
		return "", errNoWorkspace
	}
	doc, _, err := site.PrintDocument(&buildInput{s: s}, site.PrintOptions{
		Drafts:  o.Drafts,
		Section: o.Section,
		Date:    s.now().Format("2 January 2006"),
		SiteID:  "zen",
	})
	return doc, err
}

// servePrint answers GET /_print/site?drafts=0|1&section=<page> for the
// preview window: the rendered document of the open folder, nothing else.
// It takes no path: the only parameter that names anything is section, which
// must be a page in the sidebar tree.
func (s *Session) servePrint(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != printRoute {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// A page or another site cannot ask for it: a script of the window adds a header
	// that a cross-origin request cannot carry, and fetch metadata, when sent, must be same-origin.
	if r.Header.Get(printHeader) != "1" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if ws == nil {
		http.Error(w, errNoWorkspace.Error(), http.StatusConflict)
		return
	}
	q := r.URL.Query()
	doc, err := s.PrintDocument(PrintOptions{Drafts: q.Get("drafts") == "1", Section: q.Get("section")})
	switch {
	case errors.Is(err, site.ErrTooManyPages):
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	case errors.Is(err, site.ErrNotASection):
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	case err != nil:
		http.Error(w, strings.TrimSpace(err.Error()), http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	_, _ = w.Write([]byte(doc))
}
