// Package app wires the Wails application: window, menu and the Zen service.
// It is the only package importing Wails.
package app

import (
	"io/fs"
	"sync"
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

// Close handshake with the frontend (ui/editor answers through api/wails.ts).
const (
	eventBeforeClose  = "app:before-close"  // Go -> frontend: may the window close?
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
}

func (d *wailsDialogs) PickFolder() (string, error) {
	return d.app.Dialog.OpenFile().
		SetTitle("Open Folder").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		AttachToWindow(d.win).
		PromptForSingleSelection()
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
	z, err := newZen(zenOptions{
		Registry: reg, GlobalPath: globalPath,
		Dir: opts.Dir, Page: opts.Page, Mode: opts.Mode,
		Events: sink, Dialogs: dialogs,
	})
	if err != nil {
		return err
	}
	defer z.close()

	var (
		allowClose atomic.Bool
		win        *application.WebviewWindow
	)
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
		// Quitting goes through the window close handshake.
		ShouldQuit: func() bool {
			if allowClose.Load() || win == nil {
				return true
			}
			go win.Close()
			return false
		},
	})
	z.open = a.Browser.OpenURL
	sink.app.Store(a)
	dialogs.app = a

	win = a.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  Name,
		Width:  1100,
		Height: 720,
		URL:    "/",
	})
	dialogs.win = win

	// Closing: ask the frontend, close on its answer (or if it never answers).
	var (
		mu      sync.Mutex
		attempt int
		replied = map[int]bool{}
	)
	finish := func() {
		allowClose.Store(true)
		go win.Close()
	}
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if allowClose.Load() {
			return
		}
		e.Cancel()
		mu.Lock()
		attempt++
		n := attempt
		mu.Unlock()
		a.Event.Emit(eventBeforeClose)
		time.AfterFunc(closeReplyTimeout, func() {
			mu.Lock()
			defer mu.Unlock()
			if !replied[n] && n == attempt {
				finish()
			}
		})
	})
	reply := func(close bool) func(*application.CustomEvent) {
		return func(*application.CustomEvent) {
			mu.Lock()
			replied[attempt] = true
			mu.Unlock()
			if close {
				finish()
			}
		}
	}
	a.Event.On(eventClose, reply(true))
	a.Event.On(eventClosePending, reply(false))

	// Finder "Open With" and `open -a Zen <path>`. When it arrives before the
	// frontend asked for Startup, the frontend simply starts in that workspace.
	a.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		path := e.Context().Filename()
		go func() {
			if st, err := z.openPath(path); err == nil {
				sink.Emit(EventWorkspaceOpened, st)
			}
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
