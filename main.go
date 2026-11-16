package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/dragosh/zen/internal/app"
	"github.com/dragosh/zen/internal/cli"
	"github.com/dragosh/zen/internal/core/scriptplugins"
)

//go:embed all:frontend/dist
var dist embed.FS

func main() {
	// The hidden script-helper mode: before the command line is parsed and
	// before anything of the GUI runs.
	if len(os.Args) > 1 && os.Args[1] == scriptplugins.HostArg {
		os.Exit(scriptplugins.HostMain(os.Stdin, os.Stdout, os.Stderr))
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zen: %v\n", err)
		os.Exit(1)
	}
	os.Exit(cli.Run(os.Args[1:], cwd, os.Stdout, os.Stderr, launch))
}

func launch(l cli.Launch) error {
	assets, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		return err
	}
	return app.Run(app.Options{Assets: assets, Dir: l.Dir, Page: l.Page, Mode: l.Mode})
}
