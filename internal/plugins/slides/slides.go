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
		Help: plugins.Help{
			Summary: "Shows a page as slides. Set layout: slides in the frontmatter. A line with three dashes starts a new slide (without any, each top heading does). A paragraph that starts with Note: is a speaker note. Arrow keys move, O shows all slides, F shows the deck full page.",
			Snippets: []plugins.Snippet{
				{Title: "A slide deck", Markdown: "---\ntitle: My talk\nlayout: slides\n---\n# First slide\n\nA short opening.\n\nNote: Welcome everyone.\n\n---\n\n## Second slide\n\n- One point\n- Another point\n"},
				{Title: "Ratio and transition", Markdown: "---\ntitle: My talk\nlayout: slides\nlayoutOptions: { ratio: \"4:3\", transition: fade }\n---\n# A slide\n\nratio is width:height (default 16:9); transition may be fade.\n"},
			},
		},
		Layouts: []string{"slides"},
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
