package app

import (
	"io"
	"io/fs"

	"github.com/dragosh/zenmd/engine/config"
	"github.com/dragosh/zenmd/engine/session"
	"github.com/dragosh/zenmd/internal/cli"
)

// StaticAssets is the built frontend (frontend/dist), set by main.go before
// BuildSite runs. The static runtime of a built site is static/zen.js and
// static/zen.css in it.
var StaticAssets fs.FS

// staticRuntime is StaticAssets read when the session needs it, so a value
// set after the service was made still counts. Without it, opening a file
// fails.
type staticRuntime struct{}

func (staticRuntime) Open(name string) (fs.File, error) {
	if StaticAssets == nil {
		return nil, fs.ErrNotExist
	}
	return StaticAssets.Open(name)
}

// BuildSite is `zenmd build`: it writes the static site of b.Dir and prints the
// summary to stdout and the warnings to stderr. It opens no window and asks
// nothing; trust decisions are only read.
func BuildSite(b cli.Build, stdout, stderr io.Writer) error {
	globalPath, _ := config.GlobalPath()
	return session.WriteSite(session.SiteEnv{GlobalPath: globalPath, Runtime: StaticAssets, Version: cli.Version}, siteOptions(b), stdout, stderr)
}

func siteOptions(b cli.Build) session.SiteOptions {
	return session.SiteOptions{Dir: b.Dir, Out: b.Out, Portable: b.Portable, Hosted: b.Hosted, Drafts: b.Drafts, NoDrafts: b.NoDrafts, Clean: b.Clean, NoPrint: b.NoPrint}
}
