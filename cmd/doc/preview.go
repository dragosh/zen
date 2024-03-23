package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dragosh/zen/internal/fs"

	"github.com/dragosh/zen/internal/web"
	"github.com/dragosh/zen/pkg/api"
	"github.com/urfave/cli/v2"
)

const (
	Name                 = "preview"
	Usage                = "Preview usage"
	DefaultEntryFileName = "README.md"
)

func Preview() *cli.Command {

	return &cli.Command{
		Name:    Name,
		Usage:   Usage,
		Aliases: []string{"p"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "app-layout",
				Value:       "markdown",
				DefaultText: "Static application layout",
				Aliases:     []string{"a"},
				Usage:       "Application layout",
			},
		},
		Action: PreviewAction,
	}
}

type PreviewError struct {
	Context string
	Err     error
}

func (e *PreviewError) Error() string {
	return fmt.Sprintf("%s: %v", e.Context, e.Err)
}

func AppError(err error) *PreviewError {
	return &PreviewError{
		Context: Name,
		Err:     err,
	}
}

// FR
// - Check entryfile
// - Load & parse config file
// - Determine Http RootPath relative to config
// - Run Http server(s)
// - Start to Watch config Base dir for changes
// - Start the Webview process

// NFR
// - Dev Mode support (logging/inspect/uncompress)
// - Error handleling

func createEntryFromArg(arg string) (api.DocPreviewEntry, error) {
	var entry api.DocPreviewEntry
	var stat os.FileInfo
	var err error
	entryFile := DefaultEntryFileName
	if arg != "" {
		entryFile = arg
		// is Dir?
		stat, err = os.Stat(entryFile)
		if err != nil {
			return entry, AppError(err)
		}
		if stat.IsDir() {
			entryFile = filepath.Join(arg, DefaultEntryFileName)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return entry, AppError(err)
	}

	stat, err = os.Stat(entryFile)
	if err != nil {
		return entry, AppError(err)
	}
	entry.FileSize = fs.GetFileSize(stat.Size())
	entry.FilePath = filepath.Join(cwd, entryFile)
	entry.FileRoot = filepath.Dir(entry.FilePath)
	entry.FileName = filepath.Base(entry.FilePath)
	entry.FileExt = fs.GetFileExt(entry.FilePath)

	switch {
	case entry.FileExt != ".md":
		// check for contents
		return entry, AppError(errors.New("can't process this type of file"))
	}
	return entry, nil
}

func PreviewAction(cCtx *cli.Context) error {

	var err error
	var entry api.DocPreviewEntry
	entry, err = createEntryFromArg(cCtx.Args().First())

	if cCtx.String("app-layout") != "" {
		entry.StaticAppName = cCtx.String("app-layout")
	}
	// entry.StaticAppName = "markdown"

	api.Log.Infof("Layout: %s", entry.StaticAppName)
	if err != nil {
		return err
	}
	api.Log.Infof("DocPreviewEntry %v", entry)

	err = web.Start(entry)
	if err != nil {
		return err
	}
	return nil
}
