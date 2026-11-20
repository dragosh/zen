// Package app is Zenmark itself: its menu, the Zen service bound to the
// frontend, its dialogs and its settings. The window shell (windows, the close
// handshake, dialogs, print, Finder, the App Store channel) is kit/desktop.
package app

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// Name is the product's name as people see it: window titles, the app menu.
// The command and executable stay `zen`.
const Name = "Zenmark"

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

// Run starts the GUI and blocks until the application exits.
func Run(opts Options) error {
	reg := builtinPlugins()
	globalPath, _ := config.GlobalPath()
	configDir := ""
	if globalPath != "" {
		configDir = filepath.Dir(globalPath)
	}
	sink := desktop.NewSink()
	logs, stopLogs := installLogging(sink)
	defer stopLogs()
	dialogs := desktop.Dialogs(nil) // bound to the application by desktop.Run
	windows := desktop.NewWindows(desktop.WindowsOptions{Title: Name})
	// The main window always reads; `zenmd edit` opens an editor window once
	// the application runs.
	z, err := newZen(zenOptions{
		Registry: reg, GlobalPath: globalPath,
		Dir: opts.Dir, Page: opts.Page, Mode: "read",
		ConfigDir: configDir,
		Events:    sink, Dialogs: dialogs, Logs: logs,
		DeferOpen: true, // opened once the window can show progress
	})
	if err != nil {
		return err
	}
	defer z.close()
	// Folder-only menu items start enabled only when a folder was given.
	z.menu = newMenuState(opts.Dir != "")
	z.windows = windows

	return desktop.Run(desktop.Options{
		Name:        Name,
		Description: "Markdown reader and editor",
		Assets:      opts.Assets,
		Middleware:  z.middleware,
		Services:    []application.Service{application.NewService(z)},
		Sink:        sink,
		Dialogs:     dialogs,
		Windows:     windows,
		OnCreated: func(a *application.App, _ *application.WebviewWindow) {
			z.open = a.Browser.OpenURL
		},
		OnStarted: func(*application.App, *application.WebviewWindow) error {
			z.openInitial()     // the folder given on the command line (Startup may have done it)
			z.s.PromptPending() // plugins of the folder given on the command line
			z.s.PromptPendingLayouts()
			return z.openStartupEditor(opts.Mode)
		},
		OpenWith: func(path string) (any, error) {
			st, err := z.openPath(path)
			if errors.Is(err, ErrOpenCancelled) {
				return nil, nil // stopped from the loader
			}
			if err != nil {
				return nil, err
			}
			return st, nil
		},
		OpenedEvent: EventWorkspaceOpened,
		// Menu: see menu.go.
		Menu: func(a *application.App, win *application.WebviewWindow) *application.Menu {
			return buildMenu(a, menuActions{
				NewSite:      showAndEmit(win, sink, EventNewSite),
				BuildWebsite: showAndEmit(win, sink, EventBuildWebsite),
				// Print Page goes to the window that has the focus (main or editor).
				PrintPage: func() { _ = z.PrintPage() },
				PrintSite: func() { _ = z.PrintSite() },
				NewPage:   newPageAction(win, sink),
				// Find goes to the window in front: every window gets the event, the focused one acts.
				Find:         emitAction(sink, EventFind),
				FindNext:     emitAction(sink, EventFindNext),
				FindPrevious: emitAction(sink, EventFindPrevious),
				OpenFolder: func() {
					go func() {
						ws, err := z.PickFolder()
						if err != nil {
							a.Dialog.Warning().SetTitle("Could not open the folder").SetMessage(err.Error()).Show()
							return
						}
						if ws != nil {
							sink.Emit(EventWorkspaceOpened, z.Startup())
						}
					}()
				},
				Settings:    settingsAction(win, sink),
				CloseFolder: showAndEmit(win, sink, EventCloseFolder),
			}, z.menu, desktop.DevBuild)
		},
	})
}
