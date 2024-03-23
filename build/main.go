package main

import (
	"fmt"
	"log"
	"os"

	"github.com/dragosh/zen/internal/watcher"
	esbuild "github.com/evanw/esbuild/pkg/api"
)

const (
	DEV = false
)

func Run() {
	bundleWeb := Bundle()
	if len(bundleWeb.Errors) > 0 {
		fmt.Println(bundleWeb.Errors)
		os.Exit(1)
	}
	if len(bundleWeb.OutputFiles) > 0 {
		for _, files := range bundleWeb.OutputFiles {
			fmt.Println("Created: ", files.Path)
		}
	}

}
func Bundle() esbuild.BuildResult {
	return esbuild.Build(esbuild.BuildOptions{
		EntryPoints:       []string{"internal/web/src/main.ts"},
		Sourcemap:         esbuild.SourceMapLinked,
		Bundle:            true,
		MinifyWhitespace:  !DEV,
		MinifyIdentifiers: !DEV,
		MinifySyntax:      !DEV,
		// Platform:          api.PlatformNode,
		Outfile: "internal/web/apps/markdown/main.js",
		Write:   true,
	})
}

func main() {
	Run()
	// var WATCH_FILES = false
	WATCH_FILES, isSet := os.LookupEnv("WATCH_FILES")
	if isSet {
		fmt.Println("WATCH_FILES mode running is set to: ", WATCH_FILES)
		watcher.Start("./internal/web/src", "^*.(mjs|ts|js)$", func(path string) {
			log.Println(path)
			Run()
		})
	}
}
