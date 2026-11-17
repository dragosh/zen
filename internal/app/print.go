package app

import (
	"errors"
	"net/url"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// EventPrintPage asks the main window to print the page it shows (File ›
// Print Page…, ⌘P). It is delivered to every window; each one prints only
// when it has the focus.
const EventPrintPage = "menu:print-page"

const (
	printWindowName   = "print"
	printWindowWidth  = 980
	printWindowHeight = 800
)

// PrintPage asks the main window to print the page it shows, through the
// macOS print panel.
func (z *Zen) PrintPage() error {
	if z.s.Workspace() == nil {
		return errNoWorkspace
	}
	if z.events != nil {
		z.events.Emit(EventPrintPage, nil)
	}
	return nil
}

// PrintSite opens the Print Preview window for the whole folder, or focuses
// the one that is open.
func (z *Zen) PrintSite() error {
	ws := z.s.Workspace()
	if ws == nil {
		return errNoWorkspace
	}
	printWins.mu.Lock()
	defer printWins.mu.Unlock()
	if w, ok := printWins.m[z]; ok {
		w.Focus()
		return nil
	}
	current := z.s.Current()
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
