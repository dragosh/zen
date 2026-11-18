package app

import (
	"io"

	"github.com/dragosh/zenmd/engine/config"
	"github.com/dragosh/zenmd/engine/session"
	"github.com/dragosh/zenmd/internal/cli"
)

// BuildSite is `zenmd build`: it writes the static site of b.Dir and prints the
// summary to stdout and the warnings to stderr. It opens no window and asks
// nothing; trust decisions are only read.
func BuildSite(b cli.Build, stdout, stderr io.Writer) error {
	globalPath, _ := config.GlobalPath()
	return session.WriteSite(session.SiteEnv{GlobalPath: globalPath, Version: cli.Version}, siteOptions(b), stdout, stderr)
}

func siteOptions(b cli.Build) session.SiteOptions {
	return session.SiteOptions{Dir: b.Dir, Out: b.Out, Portable: b.Portable, Hosted: b.Hosted, Drafts: b.Drafts, NoDrafts: b.NoDrafts, Clean: b.Clean, NoPrint: b.NoPrint}
}
