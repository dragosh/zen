// Package app wires the Wails application: windows, menus and services.
package app

import (
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Name is the application and binary name.
const Name = "zen"

// Options configures a GUI run.
type Options struct {
	// Assets is the built frontend.
	Assets fs.FS
	// Files are document paths to open at startup.
	Files []string
	// Dir is the absolute workspace directory to open.
	Dir string
	// Page is the workspace-relative page to open, "" for the default.
	Page string
	// Mode is "read" or "edit".
	Mode string
}

// Run starts the GUI and blocks until the application exits.
func Run(opts Options) error {
	a := application.New(application.Options{
		Name:        Name,
		Description: "Markdown reader and editor",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(opts.Assets),
		},
		Services: []application.Service{
			application.NewService(&Documents{files: opts.Files}),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	a.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  Name,
		Width:  1100,
		Height: 720,
		URL:    "/",
	})

	return a.Run()
}
