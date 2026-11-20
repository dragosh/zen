package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Options configures Run. The app makes Sink, Dialogs and Windows first (its
// session needs them before the application exists) and Run binds them.
type Options struct {
	Name        string // window title and app name
	Description string
	Assets      fs.FS // the app's frontend
	// Middleware wraps the asset server (an app's session routes and its CSP).
	Middleware application.Middleware
	Services   []application.Service
	Sink       *Sink
	Dialogs    *DialogSet
	Windows    *Windows

	// OnCreated runs once the application and the main window exist, before
	// the menu is built and the loop starts.
	OnCreated func(a *application.App, w *application.WebviewWindow)
	// Menu builds the menu bar.
	Menu func(a *application.App, w *application.WebviewWindow) *application.Menu
	// OnStarted runs in its own goroutine once the main window is maximised
	// (or one second later): open what was asked for at launch, show prompts,
	// open the startup editor. A non-nil error is shown in a dialog.
	OnStarted func(a *application.App, w *application.WebviewWindow) error
	// OpenWith handles a file or folder handed over by Finder ("Open With",
	// `open -a`) once editor windows closed; it returns what to emit as
	// OpenedEvent, or nil to emit nothing (cancelled by the user).
	OpenWith    func(path string) (any, error)
	OpenedEvent string
}

// launchArg reports whether an "open file" event only repeats one of the
// process's own arguments, or is not an absolute path (Finder and `open -a`
// always send absolute paths).
func launchArg(path string, args []string) bool {
	if !filepath.IsAbs(path) {
		return true
	}
	for _, a := range args {
		if a == path {
			return true
		}
		if abs, err := filepath.Abs(a); err == nil && abs == path {
			return true
		}
	}
	return false
}

// Run creates the application and its main window and blocks until it exits.
func Run(o Options) error {
	if o.Sink == nil || o.Dialogs == nil || o.Windows == nil {
		return errors.New("desktop: Options needs Sink, Dialogs and Windows")
	}
	windows := o.Windows
	a := application.New(application.Options{
		Name:        o.Name,
		Description: o.Description,
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(o.Assets),
			Middleware: o.Middleware,
		},
		Services: o.Services,
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// Quitting goes through the main window's close: editors first.
		ShouldQuit: func() bool { return windows == nil || windows.ShouldQuit() },
	})
	o.Sink.attach(a)
	o.Dialogs.app = a

	windows.o.Emit = func(name string, data any) { a.Event.Emit(name, data) }
	windows.o.New = wailsWindowFactory(a, windows)

	win := a.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   MainWindowName,
		Title:  o.Name,
		Width:  1100, // replaced by the screen's work area at start (ApplicationStarted)
		Height: 720,
		URL:    "/",
		// Hidden until the frontend drew its first screen (eventAppReady), so
		// the launch shows no blank or half-drawn window.
		Hidden: true,
		// Element full screen, for the slides layout's full page (F).
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{FullscreenEnabled: application.Enabled},
		},
	})
	o.Dialogs.bind(a, win, windows.CloseEditorsWait)
	windows.SetMain(win)
	wireWindow(win, MainWindowName, windows)
	a.Event.On(eventClose, replyHandler(windows, ReplyClose))
	a.Event.On(eventClosePending, replyHandler(windows, ReplyPending))
	a.Event.On(eventCloseCancel, replyHandler(windows, ReplyCancel))
	if o.OnCreated != nil {
		o.OnCreated(a, win)
	}

	ready := newReadyGate()
	a.Event.On(eventAppReady, func(*application.CustomEvent) { ready.Ready() })

	a.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		// Fill the screen's usable area (not macOS full-screen mode). The window
		// is hidden, so it gets the work area's frame now and appears at full
		// size: no Maximise, which macOS ignores on a hidden window
		// (StartState: WindowStateMaximised had no effect) and animates (zoom:)
		// on a shown one. A frame equal to the work area is the zoomed frame.
		// Size first, then position: Wails' SetPosition keeps the current height
		// when it turns the top-left y into macOS's bottom-left origin, so the
		// other order (and SetBounds) leaves the frame off the work area.
		// The window's own screen, from macOS: a.Screen.GetPrimary() is still nil
		// here (Wails fills its screen list lazily).
		if s, _ := win.GetScreen(); s != nil {
			win.SetSize(s.WorkArea.Width, s.WorkArea.Height)
			win.SetPosition(s.WorkArea.X, s.WorkArea.Y)
		}
		go func() {
			// Shown when the frontend is ready, or after showTimeout, so that it
			// never stays hidden (and its error screen is seen). Editor windows
			// are sized from the main window (90%), so the startup editor opens
			// after this, when the frame is final.
			showWhenReady(ready, time.After(showTimeout), func() {
				win.Show()
				a.Event.Emit(eventAppShown, nil)
			})
			if o.OnStarted != nil {
				if err := o.OnStarted(a, win); err != nil {
					a.Dialog.Warning().SetTitle("Could not edit").SetMessage(err.Error()).Show()
				}
			}
		}()
	})

	// Finder "Open With" and `open -a <app> <path>`. When it arrives before
	// the frontend asked for Startup, the frontend simply starts there.
	if o.OpenWith != nil {
		a.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
			path := e.Context().Filename()
			// macOS also reports the process's own command-line arguments as
			// "open file" events (`zenmd edit a.md` -> "edit", "a.md"). Those are
			// already handled by the CLI; reopening them closed the workspace.
			if launchArg(path, os.Args[1:]) {
				return
			}
			go func() {
				if !windows.CloseEditorsWait() {
					return // an editor kept its unsaved changes
				}
				payload, err := o.OpenWith(path)
				if err != nil {
					a.Dialog.Warning().SetTitle("Could not open").SetMessage(err.Error()).Show()
					return
				}
				if payload != nil {
					o.Sink.Emit(o.OpenedEvent, payload)
				}
			}()
		})
	}

	if o.Menu != nil {
		a.Menu.Set(o.Menu(a, win))
	}
	return a.Run()
}
