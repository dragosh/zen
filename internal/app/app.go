// Package app wires the Wails application: window, menu and the Zen service.
// It is the only package importing Wails.
package app

import (
	"io/fs"
	"sync/atomic"
	"time"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/plugins/diagram"
	"github.com/dragosh/zen/internal/plugins/highlight"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Name is the application and binary name.
const Name = "zen"

// Close handshake with the frontend (ui/editor answers through api/wails.ts),
// per window: the payload names the window, replies carry it as the sender.
// See windows.go.
const (
	eventBeforeClose  = "app:before-close"  // Go -> frontend: may this window close?
	eventClose        = "app:close"         // frontend -> Go: yes
	eventClosePending = "app:close-pending" // frontend -> Go: asking the user
	closeReplyTimeout = 3 * time.Second     // no reply at all: the frontend is gone
)

// Options configures a GUI run.
type Options struct {
	// Assets is the built frontend.
	Assets fs.FS
	// Dir is the absolute workspace directory to open, "" for none.
	Dir string
	// Page is the workspace-relative page to open, "" for the default.
	Page string
	// Mode is "read" or "edit".
	Mode string
}

// registerPlugins registers the built-in plugins in order: highlight, diagram.
func registerPlugins(r *plugins.Registry) error {
	for _, p := range []plugins.Plugin{highlight.New(), diagram.New()} {
		if err := r.Register(p); err != nil {
			return err
		}
	}
	return nil
}

// appSink emits to the running application; events before start are dropped.
type appSink struct {
	app atomic.Pointer[application.App]
}

func (s *appSink) Emit(name string, data any) {
	if a := s.app.Load(); a != nil {
		a.Event.Emit(name, data)
	}
}

// wailsDialogs are the native dialogs, attached to the main window.
type wailsDialogs struct {
	app *application.App
	win *application.WebviewWindow
	// beforeOpen runs once a folder was chosen; false cancels opening it
	// (an editor window kept its unsaved changes).
	beforeOpen func() bool
}

func (d *wailsDialogs) PickFolder() (string, error) {
	dir, err := d.app.Dialog.OpenFile().
		SetTitle("Open Folder").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		AttachToWindow(d.win).
		PromptForSingleSelection()
	if err != nil || dir == "" {
		return dir, err
	}
	if d.beforeOpen != nil && !d.beforeOpen() {
		return "", nil
	}
	return dir, nil
}

func (d *wailsDialogs) Confirm(message string) bool {
	answer := make(chan bool, 2)
	dlg := d.app.Dialog.Question().SetTitle("Open Link").SetMessage(message)
	open := dlg.AddButton("Open").OnClick(func() { answer <- true })
	cancel := dlg.AddButton("Cancel").OnClick(func() { answer <- false })
	dlg.SetDefaultButton(open).SetCancelButton(cancel).AttachToWindow(d.win)
	dlg.Show()
	return <-answer
}

// Run starts the GUI and blocks until the application exits.
func Run(opts Options) error {
	reg := plugins.NewRegistry()
	if err := registerPlugins(reg); err != nil {
		return err
	}
	globalPath, _ := config.GlobalPath()
	sink := &appSink{}
	dialogs := &wailsDialogs{}
	// The main window always reads; `zen edit` opens an editor window once
	// the application runs.
	z, err := newZen(zenOptions{
		Registry: reg, GlobalPath: globalPath,
		Dir: opts.Dir, Page: opts.Page, Mode: "read",
		Events: sink, Dialogs: dialogs,
	})
	if err != nil {
		return err
	}
	defer z.close()

	var windows *windowManager
	a := application.New(application.Options{
		Name:        Name,
		Description: "Markdown reader and editor",
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(opts.Assets),
			Middleware: z.state.middleware,
		},
		Services: []application.Service{application.NewService(z)},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// Quitting goes through the main window's close: editors first.
		ShouldQuit: func() bool { return windows == nil || windows.shouldQuit() },
	})
	z.open = a.Browser.OpenURL
	sink.app.Store(a)
	dialogs.app = a

	windows = newWindowManager(windowManagerOptions{
		Emit: func(name string, data any) { a.Event.Emit(name, data) },
	})
	windows.o.New = wailsWindowFactory(a, windows)
	z.windows = windows
	dialogs.beforeOpen = windows.closeEditorsWait

	win := a.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   mainWindowName,
		Title:  Name,
		Width:  1100,
		Height: 720,
		URL:    "/",
	})
	dialogs.win = win
	windows.setMain(win)
	wireWindow(win, mainWindowName, windows)
	a.Event.On(eventClose, replyHandler(windows, replyClose))
	a.Event.On(eventClosePending, replyHandler(windows, replyPending))
	a.Event.On(eventCloseCancel, replyHandler(windows, replyCancel))

	a.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			if err := z.openStartupEditor(opts.Mode); err != nil {
				a.Dialog.Warning().SetTitle("Could not edit").SetMessage(err.Error()).Show()
			}
		}()
	})

	// Finder "Open With" and `open -a Zen <path>`. When it arrives before the
	// frontend asked for Startup, the frontend simply starts in that workspace.
	a.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		path := e.Context().Filename()
		go func() {
			if !windows.closeEditorsWait() {
				return // an editor kept its unsaved changes
			}
			st, err := z.openPath(path)
			if err != nil {
				a.Dialog.Warning().SetTitle("Could not open").SetMessage(err.Error()).Show()
				return
			}
			sink.Emit(EventWorkspaceOpened, st)
		}()
	})

	// Menu: File > Open Folder (Cmd+O).
	menu := a.Menu.New()
	menu.AddRole(application.AppMenu)
	file := menu.AddSubmenu("File")
	file.Add("Open Folder…").SetAccelerator("CmdOrCtrl+O").OnClick(func(*application.Context) {
		go func() {
			if ws, err := z.PickFolder(); err == nil && ws != nil {
				sink.Emit(EventWorkspaceOpened, z.Startup())
			}
		}()
	})
	file.AddSeparator()
	file.AddRole(application.CloseWindow)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)
	menu.AddRole(application.WindowMenu)
	a.Menu.Set(menu)

	return a.Run()
}
