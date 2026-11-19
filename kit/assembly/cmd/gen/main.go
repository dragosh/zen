// Command gen turns an app's assembly into code and build tags:
//
//	go run ./kit/assembly/cmd/gen apps/zenmark              # writes apps/zenmark/assembly_gen.go
//	go run ./kit/assembly/cmd/gen -tags-only apps/zenmark   # prints addon_vale (the Taskfile's tags)
//	go run ./kit/assembly/cmd/gen -ids-only apps/zenmark    # prints vale (the default of ADDONS)
//	go run ./kit/assembly/cmd/gen -tags-only -addons "" apps/zenmark   # an explicit ADDONS wins
//
// The folder holds app-assembly.yml; it is loaded and checked against the
// built-in catalog first, and nothing is written when it does not pass. The
// generated file is in the package of the folder's Go files and is committed;
// a test fails when it no longer matches the YAML (specs/kit/assembly).
package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zenmark-org/zenmd/kit/assembly"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// generatedFile is the name of the generated Go file.
const generatedFile = "assembly_gen.go"

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tagsOnly := fs.Bool("tags-only", false, "print the add-on build tags (addon_<id>, comma-separated) and write nothing")
	idsOnly := fs.Bool("ids-only", false, "print the add-on ids (comma-separated) and write nothing")
	addonsFlag := fs.String("addons", "", "add-ons to use instead of the assembly's (an empty list means none)")
	pkg := fs.String("package", "", "package name of the generated file (default: the package of the folder's Go files)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "gen: one app folder expected, for example: gen apps/zenmark")
		return 2
	}
	dir := fs.Arg(0)
	override := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "addons" {
			override = true
		}
	})

	cat, err := assembly.BuiltinCatalog()
	if err != nil {
		return fail(stderr, err)
	}
	a, err := assembly.Load(filepath.Join(dir, "app-assembly.yml"), cat)
	if err != nil {
		return fail(stderr, err)
	}
	if override {
		a.Addons = nil
		for _, id := range strings.Split(*addonsFlag, ",") {
			if id = strings.TrimSpace(id); id != "" {
				a.Addons = append(a.Addons, id)
			}
		}
		if err := assembly.Check(a, cat); err != nil {
			return fail(stderr, err)
		}
	}

	switch {
	case *tagsOnly || *idsOnly:
		var out []string
		for _, id := range a.Addons {
			if *tagsOnly {
				id = "addon_" + id
			}
			out = append(out, id)
		}
		fmt.Fprintln(stdout, strings.Join(out, ","))
		return 0
	}

	name := *pkg
	if name == "" {
		name = packageOf(dir)
	}
	src, err := assembly.Generate(a, name)
	if err != nil {
		return fail(stderr, err)
	}
	target := filepath.Join(dir, generatedFile)
	if old, err := os.ReadFile(target); err == nil && string(old) == string(src) {
		return 0
	}
	if err := os.WriteFile(target, src, 0o644); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "gen: %v\n", err)
	return 1
}

// packageOf is the package of the Go files in dir (not the generated file or
// external test packages); main when there are none.
func packageOf(dir string) string {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	slices.Sort(files)
	for _, f := range files {
		if filepath.Base(f) == generatedFile {
			continue
		}
		pf, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.PackageClauseOnly)
		if err != nil || strings.HasSuffix(pf.Name.Name, "_test") {
			continue
		}
		return pf.Name.Name
	}
	return "main"
}
