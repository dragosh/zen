// Package layouts holds the built-in page layouts: article (the default
// reading view), slides and course (moved from the plugins they used to be),
// and the template layouts docs, post, landing, wide and handout. Each is a
// folder with a layout.yml in the same format as a user's own layout (see
// docs/LAYOUTS.md), embedded in zen.
package layouts

import (
	"embed"
	"fmt"
	"io/fs"

	core "github.com/dragosh/zen/internal/core/layouts"
)

//go:embed article slides course docs post landing wide handout
var files embed.FS

// IDs lists the built-in layouts in the order Settings shows them.
var IDs = []string{"article", "docs", "post", "landing", "wide", "handout", "slides", "course"}

// All loads every built-in layout. It panics on a broken built-in, which the
// tests catch.
func All() []*core.Layout {
	out := make([]*core.Layout, 0, len(IDs))
	for _, id := range IDs {
		sub, err := fs.Sub(files, id)
		if err != nil {
			panic(err)
		}
		l, err := core.LoadFS(sub, id)
		if err != nil {
			panic(fmt.Sprintf("built-in layout %s: %v", id, err))
		}
		out = append(out, l)
	}
	return out
}

// Registry returns a registry with the built-in layouts; version is the ?v= of
// their asset URLs (the app version).
func Registry(version string) *core.Registry {
	r := core.NewRegistry()
	r.SetAssetVersion(version)
	for _, l := range All() {
		if err := r.Register(l); err != nil {
			panic(err)
		}
	}
	return r
}
