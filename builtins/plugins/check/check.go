// Package check is the :::check block: a checklist you can tick. The block
// wraps its task list; assets/check.js enables the boxes and keeps the ticks
// in ctx.storage, except inside a course, where the course layout
// (builtins/layouts/course) owns the boxes and unlocks the next step.
package check

import (
	"embed"
	"io/fs"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

//go:embed assets/check.js assets/check.css
var assets embed.FS

type plugin struct{}

// New returns the check plugin.
func New() plugins.Plugin { return plugin{} }

func (plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "check",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "A checklist block whose ticks are kept",
		Default:     true,
		Blocks:      []string{"check"},
		Help: plugins.Help{
			Summary: "A check block holds a checklist you can tick, and the ticks are kept for the page. A Reset button appears once they differ from the markdown. In a page with layout: course, ticking every box unlocks the next step.",
			Snippets: []plugins.Snippet{
				{Title: "A checklist", Markdown: ":::check\n- [ ] I opened a folder\n- [ ] I can see its pages\n:::\n"},
			},
		},
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
	return plugins.Assets{CSS: []string{"check.css"}, JS: []string{"check.js"}}
}
