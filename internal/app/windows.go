package app

import (
	"errors"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Window names. The frontend derives the same names from its URL
// (api/wails.ts) to tell which close request is meant for it.
const (
	mainWindowName    = "main"
	editorWindowName  = "editor:"          // + workspace-relative page path
	eventCloseCancel  = "app:close-cancel" // frontend -> Go: the user cancelled
	editorWidth       = 1000               // without a main window to size against
	editorHeight      = 720
	editorScale       = 0.9 // editor windows are 90% of the main window
	editorMinWidth    = 640
	editorMinHeight   = 420
	editorCascadeStep = 24
)

// closeRequest is the payload of app:before-close: which window should answer.
type closeRequest struct {
	Window string `json:"window"`
}

// closeReply is a window's answer to app:before-close.
type closeReply int

const (
	replyClose   closeReply = iota // close now
	replyPending                   // the user is being asked
	replyCancel                    // the user said no
)

// appWindow is the part of a native window the manager uses.
// *application.WebviewWindow satisfies it.
type appWindow interface {
	Focus()
	// Close asks the window to close; it runs the closing hook first.
	Close()
	Position() (int, int)
	Size() (int, int)
}

// windowSpec describes a new editor window.
type windowSpec struct {
	Name, Title, URL string
	Width, Height    int
	X, Y             int // absolute, top-left origin
}

type windowManagerOptions struct {
	// New creates and shows a window; its closing hook must call closing and
	// closed (see wailsWindowFactory).
	New func(windowSpec) appWindow
	// Emit sends an event to every window.
	Emit func(name string, data any)
	// After runs f once after d (time.AfterFunc).
	After func(d time.Duration, f func())
	// Go runs f without blocking the caller (a goroutine).
	Go func(f func())
}

// closeAll closes the editors one after another, then calls then: true when
// all closed, false when one was cancelled.
type closeAll struct {
	queue   []string // editor paths still to close
	current string   // window name being asked
	then    func(ok bool)
}

// windowManager tracks the main window and one editor window per page, and
// runs the close handshake for each of them (docs: specs/ui/editor-window).
type windowManager struct {
	o      windowManagerOptions
	openMu sync.Mutex // serialises openEditor

	mu       sync.Mutex
	main     appWindow
	mainGone bool
	editors  map[string]appWindow // by page path
	allowed  map[string]bool      // window name -> may close without asking
	attempt  map[string]int       // window name -> handshake number
	answered map[string]int       // window name -> last handshake answered
	seq      *closeAll
}

func newWindowManager(o windowManagerOptions) *windowManager {
	if o.After == nil {
		o.After = func(d time.Duration, f func()) { time.AfterFunc(d, f) }
	}
	if o.Go == nil {
		o.Go = func(f func()) { go f() }
	}
	return &windowManager{
		o:        o,
		editors:  map[string]appWindow{},
		allowed:  map[string]bool{},
		attempt:  map[string]int{},
		answered: map[string]int{},
	}
}

func (m *windowManager) setMain(w appWindow) {
	m.mu.Lock()
	m.main = w
	m.mu.Unlock()
}

func editorName(p string) string { return editorWindowName + p }

// window returns the window with that name, nil when it is gone.
func (m *windowManager) windowLocked(name string) appWindow {
	if name == mainWindowName {
		return m.main
	}
	if p, ok := strings.CutPrefix(name, editorWindowName); ok {
		return m.editors[p]
	}
	return nil
}

func (m *windowManager) editorCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.editors)
}

// openEditor opens an editor window for p, or focuses the one already open.
// p must already be validated.
func (m *windowManager) openEditor(p, title string) error {
	m.openMu.Lock()
	defer m.openMu.Unlock()

	m.mu.Lock()
	if w, ok := m.editors[p]; ok {
		m.mu.Unlock()
		w.Focus()
		return nil
	}
	if m.seq != nil {
		m.mu.Unlock()
		return errors.New("windows are closing")
	}
	main, step := m.main, len(m.editors)*editorCascadeStep
	m.mu.Unlock()

	x, y, width, height := editorCascadeStep, editorCascadeStep, editorWidth, editorHeight
	if main != nil {
		mx, my := main.Position()
		x, y, width, height = editorFrame(mx, my)(main.Size())
	}
	if title == "" {
		title = path.Base(p)
	}
	w := m.o.New(windowSpec{
		Name:   editorName(p),
		Title:  title + " — Edit — " + Name,
		URL:    "/?window=editor&path=" + url.QueryEscape(p),
		Width:  width,
		Height: height,
		X:      x + step,
		Y:      y + step,
	})
	if w == nil {
		return errors.New("could not open the editor window")
	}
	m.mu.Lock()
	m.editors[p] = w
	m.mu.Unlock()
	return nil
}

// closing is the window's closing hook: true lets it close now, false
// cancels and starts asking (the window is closed later through Close).
func (m *windowManager) closing(name string) bool {
	m.mu.Lock()
	if m.allowed[name] {
		m.mu.Unlock()
		return true
	}
	if name == mainWindowName {
		if m.seq != nil { // already closing the editors
			m.mu.Unlock()
			return false
		}
		if len(m.editors) > 0 {
			m.startCloseAllLocked(func(ok bool) {
				if ok {
					m.mu.Lock()
					m.askLocked(mainWindowName)
				}
			})
			return false
		}
	}
	m.askLocked(name)
	return false
}

// askLocked sends app:before-close to one window and closes it anyway when
// it does not answer in time. It unlocks m.mu.
func (m *windowManager) askLocked(name string) {
	m.attempt[name]++
	n := m.attempt[name]
	m.mu.Unlock()
	m.o.Emit(eventBeforeClose, closeRequest{Window: name})
	m.o.After(closeReplyTimeout, func() {
		m.mu.Lock()
		if m.answered[name] >= n || m.attempt[name] != n {
			m.mu.Unlock()
			return
		}
		m.allowLocked(name)
	})
}

// allowLocked lets the window close and closes it. It unlocks m.mu.
func (m *windowManager) allowLocked(name string) {
	w := m.windowLocked(name)
	if w == nil {
		m.mu.Unlock()
		return
	}
	m.allowed[name] = true
	m.mu.Unlock()
	m.o.Go(w.Close)
}

// reply handles a window's answer to app:before-close.
func (m *windowManager) reply(name string, r closeReply) {
	m.mu.Lock()
	m.answered[name] = m.attempt[name]
	switch r {
	case replyClose:
		m.allowLocked(name)
	case replyCancel:
		if m.seq != nil && m.seq.current == name {
			then := m.seq.then
			m.seq = nil
			m.mu.Unlock()
			then(false)
			return
		}
		m.mu.Unlock()
	default:
		m.mu.Unlock()
	}
}

// closed is called once a window has really closed.
func (m *windowManager) closed(name string) {
	m.mu.Lock()
	delete(m.allowed, name)
	if name == mainWindowName {
		m.main = nil
		m.mainGone = true
		m.mu.Unlock()
		return
	}
	p := strings.TrimPrefix(name, editorWindowName)
	delete(m.editors, p)
	if m.seq == nil {
		m.mu.Unlock()
		return
	}
	m.seq.queue = removeString(m.seq.queue, p)
	if m.seq.current == name {
		m.nextLocked()
		return
	}
	m.mu.Unlock()
}

func removeString(list []string, s string) []string {
	out := list[:0]
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

// startCloseAllLocked closes every editor in path order. It unlocks m.mu.
func (m *windowManager) startCloseAllLocked(then func(ok bool)) {
	paths := make([]string, 0, len(m.editors))
	for p := range m.editors {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	m.seq = &closeAll{queue: paths, then: then}
	m.nextLocked()
}

// nextLocked asks the next editor of the running closeAll. It unlocks m.mu.
func (m *windowManager) nextLocked() {
	for len(m.seq.queue) > 0 {
		p := m.seq.queue[0]
		m.seq.queue = m.seq.queue[1:]
		w := m.editors[p]
		if w == nil {
			continue
		}
		m.seq.current = editorName(p)
		m.mu.Unlock()
		m.o.Go(w.Close)
		return
	}
	then := m.seq.then
	m.seq = nil
	m.mu.Unlock()
	then(true)
}

// closeEditorsWait closes every editor window, each with its own prompt,
// and reports whether all closed (false: one was cancelled, or windows are
// already closing).
func (m *windowManager) closeEditorsWait() bool {
	m.mu.Lock()
	if m.seq != nil {
		m.mu.Unlock()
		return false
	}
	if len(m.editors) == 0 {
		m.mu.Unlock()
		return true
	}
	done := make(chan bool, 1)
	m.startCloseAllLocked(func(ok bool) { done <- ok })
	return <-done
}

// shouldQuit is the application's quit hook (⌘Q): quitting closes the main
// window the usual way, editors first.
func (m *windowManager) shouldQuit() bool {
	m.mu.Lock()
	main := m.main
	if m.mainGone || main == nil || m.allowed[mainWindowName] {
		m.mu.Unlock()
		return true
	}
	m.mu.Unlock()
	m.o.Go(main.Close)
	return false
}

// OpenEditor opens the page in its own editor window, or focuses the one
// already open for it.
func (z *Zen) OpenEditor(p string) error {
	if p == "" || p != path.Clean(p) || path.IsAbs(p) || strings.HasPrefix(p, "../") {
		return errors.New(p + ": not a page in this folder")
	}
	if _, err := z.s.PageSource(p); err != nil { // markdown, inside the workspace, readable
		return err
	}
	if z.windows == nil {
		return errors.New("editor windows are not available")
	}
	return z.windows.openEditor(p, z.pageTitle(p))
}

// pageTitle is the page's title in the navigation tree, "" when absent.
func (z *Zen) pageTitle(p string) string {
	ws := z.s.Workspace()
	if ws == nil {
		return ""
	}
	var find func([]NavNode) string
	find = func(ns []NavNode) string {
		for _, n := range ns {
			if n.Path == p && n.Kind == "page" {
				return n.Title
			}
			if t := find(n.Children); t != "" {
				return t
			}
		}
		return ""
	}
	return find(ws.Tree)
}

// openStartupEditor opens the editor window `zen edit <path>` asked for; the
// main window itself always starts in read mode.
func (z *Zen) openStartupEditor(mode string) error {
	if mode != "edit" {
		return nil
	}
	p := z.Startup().Page
	if p == "" {
		return nil
	}
	return z.OpenEditor(p)
}

// wailsWindowFactory creates editor windows in the running application and
// wires their close handshake to m.
func wailsWindowFactory(a *application.App, m *windowManager) func(windowSpec) appWindow {
	return func(s windowSpec) appWindow {
		w := a.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:   s.Name,
			Title:  s.Title,
			Width:  s.Width,
			Height: s.Height,
			URL:    s.URL,
			Hidden: true,
		})
		wireWindow(w, s.Name, m)
		w.SetPosition(s.X, s.Y)
		w.Show()
		w.Focus()
		return w
	}
}

// wireWindow routes a window's closing through the manager.
func wireWindow(w *application.WebviewWindow, name string, m *windowManager) {
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !m.closing(name) {
			e.Cancel()
		}
	})
	// Listeners only run when no hook cancelled: the window is going away.
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		m.closed(name)
	})
}

// replyHandler turns the frontend's close answers into manager replies.
func replyHandler(m *windowManager, r closeReply) func(*application.CustomEvent) {
	return func(e *application.CustomEvent) {
		// Sender is set by Wails from the window that emitted the event.
		if e.Sender != "" {
			m.reply(e.Sender, r)
		}
	}
}

// editorFrame places an editor window at 90% of the main window's size,
// centred over it (callers add the cascade step for further editors).
func editorFrame(mx, my int) func(mw, mh int) (x, y, w, h int) {
	return func(mw, mh int) (int, int, int, int) {
		if mw <= 0 || mh <= 0 {
			return mx + editorCascadeStep, my + editorCascadeStep, editorWidth, editorHeight
		}
		w := max(editorMinWidth, int(float64(mw)*editorScale))
		h := max(editorMinHeight, int(float64(mh)*editorScale))
		return mx + (mw-w)/2, my + (mh-h)/2, w, h
	}
}
