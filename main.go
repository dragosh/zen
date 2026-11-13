package main

import (
	"embed"
	"io/fs"
	"os"

	"github.com/dragosh/zen/internal/app"
	"github.com/dragosh/zen/internal/cli"
)

//go:embed all:frontend/dist
var dist embed.FS

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, launch))
}

func launch(files []string) error {
	assets, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		return err
	}
	return app.Run(app.Options{Assets: assets, Files: files})
}
