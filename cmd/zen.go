package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/urfave/cli/v2"
)

var (
	Revision = "000001"
)

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("version=%s revision=%s\n", c.App.Version, Revision)
	}
	app := &cli.App{
		Version: "v0.0.0",
		Commands: []*cli.Command{
			{
				Name:    "preview",
				Aliases: []string{"p"},
				Usage:   "Usage",
				Action: func(c *cli.Context) error {
					arg1 := c.Args().Get(0)
					fmt.Println("Preview", arg1)
					return nil
				},
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	err := app.Run(os.Args)

	if err != nil {
		log.Fatal(err)
	}
}
