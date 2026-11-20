// Package desktop is the reusable window shell for a zenmd desktop app (Wails):
// the window manager with its close handshake, events to the frontend, native
// dialogs as a session.Prompter, native print, Finder, the App Store channel
// and a Run helper. It imports the engine and Wails, never an app.
package desktop

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

// Close handshake with the frontend (ui/editor answers through api/wails.ts),
// per window: the payload names the window, replies carry it as the sender.
const (
	eventBeforeClose  = "app:before-close"  // Go -> frontend: may this window close?
	eventClose        = "app:close"         // frontend -> Go: yes
	eventClosePending = "app:close-pending" // frontend -> Go: asking the user
	closeReplyTimeout = 3 * time.Second     // no reply at all: the frontend is gone
)

// eventAppReady: frontend -> Go, once its first screen is drawn (api/wails.ts
// appReady); the hidden main window is shown then (showWhenReady).
const eventAppReady = "app:ready"

// eventAppShown: Go -> frontend, once the main window is on screen; the
// welcome fade starts then (it would finish unseen in the hidden window).
const eventAppShown = "app:shown"

// Window names. The frontend derives the same names from its URL
// (api/wails.ts) to tell which close request is meant for it.
const (
	// MainWindowName is the name of the main window.
	MainWindowName    = "main"
	editorWindowName  = "editor:"          // + workspace-relative page path
	eventCloseCancel  = "app:close-cancel" // frontend -> Go: the user cancelled
	editorWidth       = 1000               // without a main window to size against
	editorHeight      = 720
	editorScale       = 0.9 // editor windows are 90% of the main window
	editorMinWidth    = 640
	editorMinHeight   = 420
	editorCascadeStep = 24
)

// CloseRequest is the payload of app:before-close: which window should answer.
type CloseRequest struct {
	Window string `json:"window"`
}

// Reply is a window's answer to app:before-close.
type Reply int

const (
	// ReplyClose: close now.
	ReplyClose   Reply = iota // close now
	ReplyPending              // the user is being asked
	ReplyCancel               // the user said no
)

// Window is the part of a native window the manager uses.
// *application.WebviewWindow satisfies it.
type Window interface {
	Focus()
	// Close asks the window to close; it runs the closing hook first.
	Close()
	Position() (int, int)
	Size() (int, int)
}

// WindowSpec describes a new window.
type WindowSpec struct {
	Name, Title, URL string
	Width, Height    int
	X, Y             int // absolute, top-left origin
}

type WindowsOptions struct {
	// Title is the product name that ends an editor window's title.
	Title string
	// New creates and shows a window; its closing hook must call closing and
	// closed (see wailsWindowFactory).
	New func(WindowSpec) Window
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

// Windows tracks the main window and one editor window per page, and
// runs the close handshake for each of them (docs: specs/ui/editor-window).
type Windows struct {
	o      WindowsOptions
	openMu sync.Mutex // serialises OpenEditor

	mu       sync.Mutex
	main     Window
	mainGone bool
	editors  map[string]Window // by page path
	allowed  map[string]bool   // window name -> may close without asking
	attempt  map[string]int    // window name -> handshake number
	answered map[string]int    // window name -> last handshake answered
	seq      *closeAll
}

// NewWindows returns the window manager; Run supplies the parts that need the
// application (New, Emit).
func NewWindows(o WindowsOptions) *Windows {
	if o.After == nil {
		o.After = func(d time.Duration, f func()) { time.AfterFunc(d, f) }
	}
	if o.Go == nil {
		o.Go = func(f func()) { go f() }
	}
	return &Windows{
		o:        o,
		editors:  map[string]Window{},
		allowed:  map[string]bool{},
		attempt:  map[string]int{},
		answered: map[string]int{},
	}
}

func (m *Windows) SetMain(w Window) {
	m.mu.Lock()
	m.main = w
	m.mu.Unlock()
}

func EditorName(p string) string { return editorWindowName + p }

// window returns the window with that name, nil when it is gone.
func (m *Windows) windowLocked(name string) Window {
	if name == MainWindowName {
		return m.main
	}
	if p, ok := strings.CutPrefix(name, editorWindowName); ok {
		return m.editors[p]
	}
	return nil
}

func (m *Windows) EditorCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.editors)
}

// OpenEditor opens an editor window for p, or focuses the one already open.
// p must already be validated.
func (m *Windows) OpenEditor(p, title string) error {
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
	w := m.o.New(WindowSpec{
		Name:   EditorName(p),
		Title:  title + " — Edit — " + m.o.Title,
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
func (m *Windows) Closing(name string) bool {
	m.mu.Lock()
	if m.allowed[name] {
		m.mu.Unlock()
		return true
	}
	if name == MainWindowName {
		if m.seq != nil { // already closing the editors
			m.mu.Unlock()
			return false
		}
		if len(m.editors) > 0 {
			m.startCloseAllLocked(func(ok bool) {
				if ok {
					m.mu.Lock()
					m.askLocked(MainWindowName)
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
func (m *Windows) askLocked(name string) {
	m.attempt[name]++
	n := m.attempt[name]
	m.mu.Unlock()
	m.o.Emit(eventBeforeClose, CloseRequest{Window: name})
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
func (m *Windows) allowLocked(name string) {
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
func (m *Windows) Reply(name string, r Reply) {
	m.mu.Lock()
	m.answered[name] = m.attempt[name]
	switch r {
	case ReplyClose:
		m.allowLocked(name)
	case ReplyCancel:
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
func (m *Windows) Closed(name string) {
	m.mu.Lock()
	delete(m.allowed, name)
	if name == MainWindowName {
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
func (m *Windows) startCloseAllLocked(then func(ok bool)) {
	paths := make([]string, 0, len(m.editors))
	for p := range m.editors {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	m.seq = &closeAll{queue: paths, then: then}
	m.nextLocked()
}

// nextLocked asks the next editor of the running closeAll. It unlocks m.mu.
func (m *Windows) nextLocked() {
	for len(m.seq.queue) > 0 {
		p := m.seq.queue[0]
		m.seq.queue = m.seq.queue[1:]
		w := m.editors[p]
		if w == nil {
			continue
		}
		m.seq.current = EditorName(p)
		m.mu.Unlock()
		m.o.Go(w.Close)
		return
	}
	then := m.seq.then
	m.seq = nil
	m.mu.Unlock()
	then(true)
}

// CloseEditorsWait closes every editor window, each with its own prompt,
// and reports whether all closed (false: one was cancelled, or windows are
// already closing).
func (m *Windows) CloseEditorsWait() bool {
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

// ShouldQuit is the application's quit hook (⌘Q): quitting closes the main
// window the usual way, editors first.
func (m *Windows) ShouldQuit() bool {
	m.mu.Lock()
	main := m.main
	if m.mainGone || main == nil || m.allowed[MainWindowName] {
		m.mu.Unlock()
		return true
	}
	m.mu.Unlock()
	m.o.Go(main.Close)
	return false
}

// wailsWindowFactory creates editor windows in the running application and
// wires their close handshake to m.
func wailsWindowFactory(a *application.App, m *Windows) func(WindowSpec) Window {
	return func(s WindowSpec) Window {
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
func wireWindow(w *application.WebviewWindow, name string, m *Windows) {
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !m.Closing(name) {
			e.Cancel()
		}
	})
	// Listeners only run when no hook cancelled: the window is going away.
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		m.Closed(name)
	})
}

// replyHandler turns the frontend's close answers into manager replies.
func replyHandler(m *Windows, r Reply) func(*application.CustomEvent) {
	return func(e *application.CustomEvent) {
		// Sender is set by Wails from the window that emitted the event.
		if e.Sender != "" {
			m.Reply(e.Sender, r)
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
