package app

import (
	"embed"
	"io/fs"

	"github.com/zenmark-org/zenmd/internal/cli"
)

// The built frontend (apps/zenmark/frontend, built by `task build`).
//
//go:embed all:frontend/dist
var dist embed.FS

// Launch is the GUI side of the zenmd command: it opens the app on what the
// command line asked for.
func Launch(l cli.Launch) error {
	assets, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		return err
	}
	return Run(Options{Assets: assets, Dir: l.Dir, Page: l.Page, Mode: l.Mode})
}
