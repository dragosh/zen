package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
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
		Width:  1100, // the size it returns to when un-maximised
		Height: 720,
		URL:    "/",
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

	a.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		// Fill the screen's usable area (not macOS full-screen mode). Done
		// here because macOS ignores maximising a window that is not shown
		// yet (StartState: WindowStateMaximised had no effect).
		// Editor windows are sized from the main window (90%), so the startup
		// editor waits until maximising has finished (it is asynchronous), or
		// one second at most.
		maximised := make(chan struct{})
		var once sync.Once
		win.OnWindowEvent(events.Common.WindowMaximise, func(*application.WindowEvent) {
			once.Do(func() { close(maximised) })
		})
		win.Maximise()
		go func() {
			select {
			case <-maximised:
			case <-time.After(time.Second):
			}
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
