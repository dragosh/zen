package app

import (
	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/internal/cli"
	"github.com/zenmark-org/zenmd/kit/addons"
)

// AddonInfo is one add-on of this build as the frontend sees it (Startup.Addons).
type AddonInfo = session.AddonInfo

// addonInfos lists the add-ons this build includes, in a fixed order (by id).
func addonInfos() []AddonInfo {
	list := addons.List()
	out := make([]AddonInfo, 0, len(list))
	for _, a := range list {
		out = append(out, AddonInfo{ID: a.ID, Name: a.Name, Summary: a.Summary})
	}
	return out
}

// BuildInfo is what this build is made of (Startup.Build): the assembly of
// apps/zenmark/app-assembly.yml and the add-ons linked in.
type BuildInfo = session.BuildInfo

// buildInfo returns the assembly with the add-ons of this build.
func buildInfo() *BuildInfo {
	b := assemblyInfo
	b.Addons = addonInfos()
	return &b
}

// `zenmd version --verbose` prints the same.
func init() {
	cli.SetBuildInfo(func() cli.BuildInfo {
		b := assemblyInfo
		return cli.BuildInfo{App: b.App, Plugins: b.Plugins, Layouts: b.Layouts, Templates: b.Templates, Theme: b.Theme}
	})
}
