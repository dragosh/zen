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
	"fmt"
	"io/fs"
	"slices"
	"strings"

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

// PluginIDs lists the built-in plugins in the order Plugins registers them.
var PluginIDs = []string{"codeblock", "diagram", "check", "quiz", "sandbox"}

func newPlugin(id string) plugins.Plugin {
	switch id {
	case "codeblock":
		return codeblock.New()
	case "diagram":
		return diagram.New()
	case "check":
		return check.New()
	case "quiz":
		return quiz.New()
	case "sandbox":
		return sandbox.New()
	}
	return nil
}

// unknown is the error for an id that is not one of known.
func unknown(kind, id string, known []string) error {
	return fmt.Errorf("unknown %s %q; known %ss: %s", kind, id, kind, strings.Join(known, ", "))
}

// check rejects unknown and repeated ids.
func checkIDs(kind string, ids, known []string) error {
	seen := map[string]bool{}
	for _, id := range ids {
		if !slices.Contains(known, id) {
			return unknown(kind, id, known)
		}
		if seen[id] {
			return fmt.Errorf("%s %q is listed twice", kind, id)
		}
		seen[id] = true
	}
	return nil
}

// PluginsFor returns a registry with only the built-in plugins named, in the
// order given (an empty list gives an empty registry). An unknown or repeated
// id is an error that lists the known ids. Plugins() is PluginsFor with all.
func PluginsFor(ids []string) (*plugins.Registry, error) {
	if err := checkIDs("plugin", ids, PluginIDs); err != nil {
		return nil, err
	}
	r := plugins.NewRegistry()
	for _, id := range ids {
		if err := r.Register(newPlugin(id)); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// LayoutsFor returns a registry with the built-in layouts named, in the order
// given, for the app version. article, the fallback layout, is always there:
// first when ids does not list it.
func LayoutsFor(version string, ids []string) (*enginelayouts.Registry, error) {
	if err := checkIDs("layout", ids, layouts.IDs); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		ids = []string{"article"} // layouts.Registry without ids means all of them
	}
	return layouts.Registry(version, ids...), nil
}

// SiteTemplatesFor returns the built-in site templates named, one folder per
// template, in the order given. An empty list gives an empty file system.
func SiteTemplatesFor(ids []string) (fs.FS, error) {
	return sitetemplates.Sub(ids...)
}
