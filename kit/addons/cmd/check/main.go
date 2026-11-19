// Command check turns the ADDONS list of a build into Go build tags:
//
//	go run ./kit/addons/cmd/check -known vale "$ADDONS"   # prints addon_vale
//
// The list is comma-separated add-on ids. Each must be a valid id and one of
// the known ones; otherwise nothing is printed and the command fails with the
// list of known add-ons, so a typo stops the build early. An empty list is
// valid and prints nothing.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zenmark-org/zenmd/kit/addons"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	known := fs.String("known", "", "comma-separated ids of the add-ons that exist")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "check: one ADDONS list expected")
		return 2
	}
	list := ""
	if fs.NArg() == 1 {
		list = fs.Arg(0)
	}
	tags, err := Tags(*known, list)
	if err != nil {
		fmt.Fprintf(stderr, "check: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, strings.Join(tags, ","))
	return 0
}

// Tags returns the build tags (addon_<id>) of the comma-separated list, in the
// order given and without duplicates.
func Tags(known, list string) ([]string, error) {
	isKnown := map[string]bool{}
	for _, id := range strings.Split(known, ",") {
		if id = strings.TrimSpace(id); id != "" {
			isKnown[id] = true
		}
	}
	if strings.TrimSpace(list) == "" {
		return nil, nil
	}
	var tags []string
	seen := map[string]bool{}
	for _, id := range strings.Split(list, ",") {
		id = strings.TrimSpace(id)
		if !addons.ValidID(id) || !isKnown[id] {
			return nil, fmt.Errorf("unknown add-on %q; known add-ons: %s", id, strings.Join(knownList(known), ", "))
		}
		if !seen[id] {
			seen[id] = true
			tags = append(tags, "addon_"+id)
		}
	}
	return tags, nil
}

func knownList(known string) []string {
	var out []string
	for _, id := range strings.Split(known, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}
