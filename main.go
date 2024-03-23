package main

import (
	"fmt"

	"os"
	"sort"

	"github.com/dragosh/zen/cmd"
	doc "github.com/dragosh/zen/cmd/doc"
	"github.com/dragosh/zen/pkg/api"
	"github.com/urfave/cli/v2"
)

const (
	Version  = "0.0.0"
	Revision = "000001"
	Name     = "zen"
	Usage    = "Command Description"
)

var Commands []*cli.Command

func main() {

	// EXAMPLE: Override a template
	cli.AppHelpTemplate = `
	ooooooo_________________
	_____oo__ooooo__oo_ooo__
	____oo__oo____o_ooo___o_
	___o____ooooooo_oo____o_
	_oo_____oo______oo____o_
	ooooooo__ooooo__oo____o_
	________________________

NAME:
		{{.Name}} - {{.Usage}}
USAGE:
		{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}
		{{if len .Authors}}
AUTHOR:
		{{range .Authors}}{{ . }}{{end}}
		{{end}}{{if .Commands}}
COMMANDS:
{{range .Commands}}{{if not .HideHelp}}   {{join .Names ", "}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{end}}{{if .VisibleFlags}}
GLOBAL OPTIONS:
		{{range .VisibleFlags}}{{.}}
		{{end}}{{end}}{{if .Copyright }}
COPYRIGHT:
		{{.Copyright}}
		{{end}}{{if .Version}}
VERSION:
		{{.Version}}
		{{end}}
	`

	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("version=%s revision=%s\n", c.App.Version, Revision)
	}

	Commands = append(Commands, cmd.Doc())
	// Commands = append(Commands, doc.Preview())

	app := &cli.App{
		EnableBashCompletion: true,
		Version:              Version,
		Name:                 Name,
		Usage:                Usage,
		Commands:             Commands,
		Action:               doc.PreviewAction,

		Flags: []cli.Flag{
			// &cli.BoolFlag{
			// 	Name:    "no-color",
			// 	Usage:   "disable color output",
			// 	EnvVars: []string{"NO_COLOR"},
			// },
			// &cli.StringFlag{
			// 	Name:    "log-level",
			// 	Usage:   "set log level, use \"DEBUG\" for more informations",
			// 	Value:   api.LOG_LEVEL,
			// 	EnvVars: []string{"LOG_LEVEL"},
			// },
			&cli.StringFlag{
				Name:        "app-layout",
				Value:       "markdown",
				DefaultText: "Static application layout",
				Aliases:     []string{"a"},
				Usage:       "Application layout",
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if err := app.Run(os.Args); err != nil {
		api.Log.Fatal(err)
	}
}
