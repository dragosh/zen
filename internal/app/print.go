package app

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/dragosh/zen/internal/core/site"
)

// EventPrintPage asks the main window to print the page it shows (File ›
// Print Page…, ⌘P). It is delivered to every window; each one prints only
// when it has the focus.
const EventPrintPage = "menu:print-page"

const (
	printWindowName   = "print"
	printRoute        = "/_print/site"
	printHeader       = "X-Zen-Print" // only the preview window's script sends it
	printWindowWidth  = 980
	printWindowHeight = 800
)

// printNow is the clock of the cover's date.
var printNow = time.Now

// PrintPage asks the main window to print the page it shows, through the
// macOS print panel.
func (z *Zen) PrintPage() error {
	if z.state.workspace() == nil {
		return errNoWorkspace
	}
	z.state.emit(EventPrintPage, nil)
	return nil
}

// PrintSite opens the Print Preview window for the whole folder, or focuses
// the one that is open.
func (z *Zen) PrintSite() error {
	ws := z.state.workspace()
	if ws == nil {
		return errNoWorkspace
	}
	printWins.mu.Lock()
	defer printWins.mu.Unlock()
	if w, ok := printWins.m[z]; ok {
		w.Focus()
		return nil
	}
	z.state.mu.Lock()
	current := z.state.current
	z.state.mu.Unlock()
	u := "/?window=" + printWindowName
	if current != "" {
		u += "&page=" + url.QueryEscape(current)
	}
	name := ws.Name
	if name == "" {
		name = Name
	}
	w := newPrintWindow(windowSpec{
		Name: printWindowName, Title: name + " — Print", URL: u,
		Width: printWindowWidth, Height: printWindowHeight, X: 60, Y: 60,
	}, func() {
		printWins.mu.Lock()
		delete(printWins.m, z)
		printWins.mu.Unlock()
	})
	if w == nil {
		return errors.New("could not open the print preview window")
	}
	printWins.m[z] = w
	return nil
}

// printWins tracks the open preview window of each running service.
var printWins = struct {
	mu sync.Mutex
	m  map[*Zen]appWindow
}{m: map[*Zen]appWindow{}}

// closePrintWindows closes the preview windows: they go with the main window.
func closePrintWindows() {
	printWins.mu.Lock()
	var ws []appWindow
	for _, w := range printWins.m {
		ws = append(ws, w)
	}
	printWins.mu.Unlock()
	for _, w := range ws {
		w.Close()
	}
}

var hookMain sync.Once

// newPrintWindow creates and shows the preview window; onClose runs when it
// closes. It has no close handshake: nothing in it can be lost. Tests replace it.
var newPrintWindow = func(s windowSpec, onClose func()) appWindow {
	a := application.Get()
	if a == nil {
		return nil
	}
	w := a.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: s.Name, Title: s.Title, Width: s.Width, Height: s.Height, URL: s.URL, Hidden: true,
	})
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { onClose() })
	hookMain.Do(func() {
		if m, ok := a.Window.GetByName(mainWindowName); ok {
			if mw, ok := m.(*application.WebviewWindow); ok {
				mw.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { closePrintWindows() })
			}
		}
	})
	w.SetPosition(s.X, s.Y)
	w.Show()
	w.Focus()
	return w
}

// servePrint answers GET /_print/site?drafts=0|1&section=<page> for the
// preview window: the rendered document of the open folder, nothing else.
// It takes no path: the only parameter that names anything is section, which
// must be a page in the sidebar tree.
func (s *state) servePrint(w http.ResponseWriter, r *http.Request) {
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
	opts := site.PrintOptions{
		Drafts:  q.Get("drafts") == "1",
		Section: q.Get("section"),
		Date:    printNow().Format("2 January 2006"),
		SiteID:  "zen",
	}
	// The live state, rendered like the app renders: unlike a build, the reader's own options apply.
	doc, _, err := site.PrintDocument(&buildInput{s: s}, opts)
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
