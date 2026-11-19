//go:build addon_vale

// Package vale is the Vale linting add-on: spelling and style checks for pages
// by the Vale prose linter. Importing it (apps/<app>/addons_vale.go, under the
// addon_vale tag) registers the add-on and gives the session its linter; the
// vale binary is found as before (bundled helper, $ZEN_VALE, the dev helper,
// PATH). Without the tag none of this is linked and Lint is unavailable.
package vale

import (
	"github.com/zenmark-org/zenmd/engine/lint"
	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/kit/addons"
)

func init() {
	addons.Register(addons.Addon{
		ID:      "vale",
		Name:    "Vale linting",
		Summary: "Spelling and style checks for your pages, by the Vale prose linter.",
	})
	session.SetLinter(func() session.TextLinter {
		p, err := lint.Find()
		if err != nil {
			return nil
		}
		return lint.New(p)
	})
}
