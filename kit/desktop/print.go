package desktop

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// NativePrint opens the print panel for w's web view (on macOS the system
// panel: portrait, half-inch margins, fitted to the page width). title
// becomes the job name and the suggested PDF file name.
func NativePrint(w application.Window, title string) error { return nativePrint(w, title) }

// PrintFromContext prints the window that a bound service method was called
// from (its context carries the window); name is the job name when title is blank.
func PrintFromContext(ctx context.Context, title, name string) error {
	w, ok := ctx.Value(application.WindowKey).(application.Window)
	if !ok || w == nil {
		return errors.New("no window to print")
	}
	if title = strings.TrimSpace(title); title == "" {
		title = name
	}
	return NativePrint(w, title)
}

// PrintWindows tracks one preview window per key (an app's service, say):
// opening it again only focuses it, and the windows close with the main window.
type PrintWindows struct {
	mu sync.Mutex
	m  map[any]Window
}

// Open creates the preview window described by s for key, or focuses the one
// that is open. It has no close handshake: nothing in it can be lost.
func (p *PrintWindows) Open(key any, s WindowSpec) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if w, ok := p.m[key]; ok {
		w.Focus()
		return nil
	}
	w := NewPrintWindow(s, func() {
		p.mu.Lock()
		delete(p.m, key)
		p.mu.Unlock()
	}, p.closeAll)
	if w == nil {
		return errors.New("could not open the print preview window")
	}
	if p.m == nil {
		p.m = map[any]Window{}
	}
	p.m[key] = w
	return nil
}

// closeAll closes the preview windows: they go with the main window.
func (p *PrintWindows) closeAll() {
	p.mu.Lock()
	var ws []Window
	for _, w := range p.m {
		ws = append(ws, w)
	}
	p.mu.Unlock()
	for _, w := range ws {
		w.Close()
	}
}

var hookMain sync.Once

// NewPrintWindow creates and shows a preview window; onClose runs when it
// closes, and closeWithMain when the main window does (hooked once). Tests
// replace it.
var NewPrintWindow = func(s WindowSpec, onClose func(), closeWithMain func()) Window {
	a := application.Get()
	if a == nil {
		return nil
	}
	w := a.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: s.Name, Title: s.Title, Width: s.Width, Height: s.Height, URL: s.URL, Hidden: true,
	})
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { onClose() })
	hookMain.Do(func() {
		if m, ok := a.Window.GetByName(MainWindowName); ok {
			if mw, ok := m.(*application.WebviewWindow); ok {
				mw.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { closeWithMain() })
			}
		}
	})
	w.SetPosition(s.X, s.Y)
	w.Show()
	w.Focus()
	return w
}
