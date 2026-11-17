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
		Help: plugins.Help{
			Summary: "Shows a page as a step-by-step course. Set layout: course in the frontmatter; each second-level heading is a step, and one step shows at a time. A check block holds a checklist that unlocks the next step. Progress is saved on this Mac.",
			Snippets: []plugins.Snippet{
				{Title: "A course", Markdown: "---\ntitle: Getting started\nlayout: course\nlayoutOptions: { id: getting-started }\n---\n# Getting started\n\nWork through the steps in order.\n\n## Open a folder\n\nChoose File, then Open Folder.\n\n:::check\n- [ ] I opened a folder\n- [ ] I can see its pages\n:::\n\n## Read a page\n\nClick a page in the sidebar.\n"},
			},
		},
		Layouts: []string{"course"},
		Blocks:  []string{"check"},
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
