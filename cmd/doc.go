package cmd

import (
	doc "github.com/dragosh/zen/cmd/doc"

	"github.com/urfave/cli/v2"
)

var SubCommands []*cli.Command

func Doc() *cli.Command {

	SubCommands = append(SubCommands, doc.Preview())

	return &cli.Command{
		Name:        "doc",
		Subcommands: SubCommands,
	}
}
