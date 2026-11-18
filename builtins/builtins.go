// Package builtins is the default set of zenmd: the built-in plugins, page
// layouts and site templates, ready to hand to a session or a renderer.
//
// The sub-packages hold the pieces (plugins/codeblock, plugins/diagram,
// plugins/check, plugins/quiz, plugins/sandbox, layouts and sitetemplates);
// this package assembles them, so there is one list that the Zenmark app, the
// zenmd command and an embedding program all use:
//
//	reg := builtins.Plugins()
//	lay := builtins.Layouts("1.0.0")
//	tpl := builtins.SiteTemplates()
//
// An embedder that wants fewer built-ins builds its own plugins.Registry and
// registers only the sub-packages it needs.
package builtins

import (
	"io/fs"

	"github.com/zenmark-org/zenmd/builtins/layouts"
	"github.com/zenmark-org/zenmd/builtins/plugins/check"
	"github.com/zenmark-org/zenmd/builtins/plugins/codeblock"
	"github.com/zenmark-org/zenmd/builtins/plugins/diagram"
	"github.com/zenmark-org/zenmd/builtins/plugins/quiz"
	"github.com/zenmark-org/zenmd/builtins/plugins/sandbox"
	"github.com/zenmark-org/zenmd/builtins/sitetemplates"
	enginelayouts "github.com/zenmark-org/zenmd/engine/layouts"
	"github.com/zenmark-org/zenmd/engine/plugins"
)

// Plugins returns a registry with every built-in plugin, in order: codeblock,
// diagram, check, quiz, sandbox. Each call returns a fresh registry.
func Plugins() *plugins.Registry {
	r := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{codeblock.New(), diagram.New(), check.New(), quiz.New(), sandbox.New()} {
		if err := r.Register(p); err != nil {
			panic(err) // the built-ins are fixed: a clash is a programming error
		}
	}
	return r
}

// Layouts returns a registry with the built-in layouts: article, docs, post,
// landing, wide, handout, slides and course. version is the ?v= of their
// asset URLs (the app version).
func Layouts(version string) *enginelayouts.Registry {
	return layouts.Registry(version)
}

// SiteTemplates returns the built-in site templates, one folder per preset:
// techdocs, learning and planner.
func SiteTemplates() fs.FS {
	return sitetemplates.FS
}
