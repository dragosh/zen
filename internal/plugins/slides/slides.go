// Package slides is the "slides" page layout: a page with `layout: slides` is
// shown one slide at a time. Go only declares the layout and ships the
// assets; assets/slides.js does the work in the page.
package slides

import (
	"embed"
	"io/fs"

	"github.com/dragosh/zen/internal/core/plugins"
)

//go:embed assets/slides.js assets/slides.css
var assets embed.FS

type plugin struct{}

// New returns the slides plugin.
func New() plugins.Plugin { return plugin{} }

func (plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "slides",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "Show a page as slides",
		Default:     true,
		Layouts:     []string{"slides"},
	}
}

func (plugin) Assets() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}

func (plugin) Entry() plugins.Assets {
	return plugins.Assets{CSS: []string{"slides.css"}, JS: []string{"slides.js"}}
}
