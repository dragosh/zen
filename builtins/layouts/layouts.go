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
	"slices"

	core "github.com/zenmark-org/zenmd/engine/layouts"
)

//go:embed article slides course docs post landing wide handout
var files embed.FS

// IDs lists the built-in layouts in the order Settings shows them.
var IDs = []string{"article", "docs", "post", "landing", "wide", "handout", "slides", "course"}

// load loads the built-in layout id. It panics on a broken built-in, which
// the tests catch.
func load(id string) *core.Layout {
	sub, err := fs.Sub(files, id)
	if err != nil {
		panic(err)
	}
	l, err := core.LoadFS(sub, id)
	if err != nil {
		panic(fmt.Sprintf("built-in layout %s: %v", id, err))
	}
	return l
}

// All loads every built-in layout.
func All() []*core.Layout {
	out := make([]*core.Layout, 0, len(IDs))
	for _, id := range IDs {
		out = append(out, load(id))
	}
	return out
}

// Registry returns a registry with the built-in layouts; version is the ?v= of
// their asset URLs (the app version). With ids it holds only those, in the
// order given, and article (the fallback) first when it is not among them;
// ids must be built-in ids (builtins.LayoutsFor checks them).
func Registry(version string, ids ...string) *core.Registry {
	r := core.NewRegistry()
	r.SetAssetVersion(version)
	if len(ids) == 0 {
		for _, l := range All() {
			if err := r.Register(l); err != nil {
				panic(err)
			}
		}
		return r
	}
	if !slices.Contains(ids, "article") {
		ids = append([]string{"article"}, ids...)
	}
	for _, id := range ids {
		if err := r.Register(load(id)); err != nil {
			panic(err)
		}
	}
	return r
}
