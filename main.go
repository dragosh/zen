package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/dragosh/zen/internal/app"
)

//go:embed all:frontend/dist
var dist embed.FS

func main() {
	assets, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Run(app.Options{Assets: assets}); err != nil {
		log.Fatal(err)
	}
}
