// Package course is the "course" page layout and the :::check block: a page with `layout: course` is
// shown as steps with progress. Go only declares the layout and ships the
// assets; assets/course.js does the work in the page.
package course

import (
	"embed"
	"io/fs"

	"github.com/dragosh/zen/internal/core/plugins"
)

//go:embed assets/course.js assets/course.css
var assets embed.FS

type plugin struct{}

// New returns the course plugin.
func New() plugins.Plugin { return plugin{} }

func (plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "course",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "Show a page as a step-by-step course",
		Default:     true,
		Layouts:     []string{"course"},
		Blocks:      []string{"check"},
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
	return plugins.Assets{CSS: []string{"course.css"}, JS: []string{"course.js"}}
}
