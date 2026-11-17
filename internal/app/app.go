// Package app wires the Wails application: window, menu and the Zen service.
// It is the only package importing Wails.
package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/plugins/check"
	"github.com/dragosh/zen/internal/plugins/codeblock"
	"github.com/dragosh/zen/internal/plugins/diagram"
	"github.com/dragosh/zen/internal/plugins/quiz"
	"github.com/dragosh/zen/internal/plugins/sandbox"
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

// registerPlugins registers the built-in plugins in order: codeblock, diagram.
func registerPlugins(r *plugins.Registry) error {
	for _, p := range []plugins.Plugin{codeblock.New(), diagram.New(), check.New(), quiz.New(), sandbox.New()} {
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

// Ready reports whether the dialogs can be shown (the window exists).
func (d *wailsDialogs) Ready() bool { return d.app != nil && d.win != nil }

// ConfirmTrust is the prompt before plugins of a folder are used.
func (d *wailsDialogs) ConfirmTrust(message string) bool {
	answer := make(chan bool, 2)
	dlg := d.app.Dialog.Question().SetTitle("Allow?").SetMessage(message)
	allow := dlg.AddButton("Allow").OnClick(func() { answer <- true })
	block := dlg.AddButton("Block").OnClick(func() { answer <- false })
	dlg.SetDefaultButton(block).SetCancelButton(block).AttachToWindow(d.win)
	_ = allow
	dlg.Show()
	return <-answer
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

// Run starts the GUI and blocks until the application exits.
func Run(opts Options) error {
	reg := plugins.NewRegistry()
	if err := registerPlugins(reg); err != nil {
		return err
	}
	globalPath, _ := config.GlobalPath()
	configDir := ""
	if globalPath != "" {
		configDir = filepath.Dir(globalPath)
	}
	sink := &appSink{}
	dialogs := &wailsDialogs{}
	// The main window always reads; `zen edit` opens an editor window once
	// the application runs.
	z, err := newZen(zenOptions{
		Registry: reg, GlobalPath: globalPath,
		Dir: opts.Dir, Page: opts.Page, Mode: "read",
		ConfigDir: configDir,
		Events:    sink, Dialogs: dialogs,
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
		Width:  1100, // the size it returns to when un-maximised
		Height: 720,
		URL:    "/",
		// Element full screen, for the slides layout's full page (F).
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{FullscreenEnabled: application.Enabled},
		},
	})
	dialogs.win = win
	windows.setMain(win)
	wireWindow(win, mainWindowName, windows)
	a.Event.On(eventClose, replyHandler(windows, replyClose))
	a.Event.On(eventClosePending, replyHandler(windows, replyPending))
	a.Event.On(eventCloseCancel, replyHandler(windows, replyCancel))

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
			z.state.promptPending() // plugins of the folder given on the command line
			z.state.promptPendingLayouts()
			if err := z.openStartupEditor(opts.Mode); err != nil {
				a.Dialog.Warning().SetTitle("Could not edit").SetMessage(err.Error()).Show()
			}
		}()
	})

	// Finder "Open With" and `open -a Zen <path>`. When it arrives before the
	// frontend asked for Startup, the frontend simply starts in that workspace.
	a.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		path := e.Context().Filename()
		// macOS also reports the process's own command-line arguments as
		// "open file" events (`zen edit a.md` -> "edit", "a.md"). Those are
		// already handled by the CLI; reopening them closed the workspace.
		if launchArg(path, os.Args[1:]) {
			return
		}
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

	// Menu: File > New Page (Cmd+N), Open Folder (Cmd+O).
	menu := a.Menu.New()
	menu.AddRole(application.AppMenu)
	file := menu.AddSubmenu("File")
	// The main window asks for the name and layout, then calls CreatePage.
	file.Add("New Page…").SetAccelerator("CmdOrCtrl+N").OnClick(func(*application.Context) {
		win.Show()
		win.Focus()
		sink.Emit(EventNewPage, nil)
	})
	file.Add("Open Folder…").SetAccelerator("CmdOrCtrl+O").OnClick(func(*application.Context) {
		go func() {
			if ws, err := z.PickFolder(); err == nil && ws != nil {
				sink.Emit(EventWorkspaceOpened, z.Startup())
			}
		}()
	})
	// File > Print (Cmd+P): the main window's page prints through the web
	// view's own window.print(); page.css has the @media print rules.
	file.Add("Print…").SetAccelerator("CmdOrCtrl+P").OnClick(func(*application.Context) {
		win.ExecJS("window.print()")
	})
	file.AddSeparator()
	file.AddRole(application.CloseWindow)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)
	menu.AddRole(application.WindowMenu)
	a.Menu.Set(menu)

	return a.Run()
}
