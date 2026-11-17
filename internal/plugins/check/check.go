// Package check is the :::check block: a checklist that the course layout
// (internal/layouts/course) watches to unlock the next step. The block only
// wraps its task list; course.js does the work in the page.
package check

import "github.com/dragosh/zen/internal/core/plugins"

type plugin struct{}

// New returns the check plugin.
func New() plugins.Plugin { return plugin{} }

func (plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "check",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "A checklist block for the course layout",
		Default:     true,
		Blocks:      []string{"check"},
		Help: plugins.Help{
			Summary: "A check block holds a checklist. In a page with layout: course, ticking every box unlocks the next step; elsewhere it is a plain checklist.",
			Snippets: []plugins.Snippet{
				{Title: "A checklist", Markdown: ":::check\n- [ ] I opened a folder\n- [ ] I can see its pages\n:::\n"},
			},
		},
	}
}
