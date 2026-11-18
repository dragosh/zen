package main

import (
	"fmt"
	"os"

	app "github.com/dragosh/zenmd/apps/zenmd-app"
	"github.com/dragosh/zenmd/engine/scriptplugins"
	"github.com/dragosh/zenmd/internal/cli"
)

func main() {
	// The hidden script-helper mode: before the command line is parsed and
	// before anything of the GUI runs.
	if len(os.Args) > 1 && os.Args[1] == scriptplugins.HostArg {
		os.Exit(scriptplugins.HostMain(os.Stdin, os.Stdout, os.Stderr))
	}
	// The alias notice: only for the old name, on stderr, after the helper
	// mode above (which must print nothing).
	cli.Notice(os.Stderr, cli.ProgramName(os.Args[0]))
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zenmd: %v\n", err)
		os.Exit(1)
	}
	os.Exit(cli.Run(os.Args[1:], cwd, os.Stdout, os.Stderr, app.Launch, app.BuildSite, app.InitSite))
}
