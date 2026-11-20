package app

// File › Build Website… (⇧⌘B) and Settings › Website: the parts of the dialog
// that need the GUI (the output folder panel, the browser, Finder). The build
// itself is the session's.

import (
	"errors"
	"net/url"
	"path/filepath"

	"github.com/zenmark-org/zenmd/kit/desktop"
)

// revealPath shows a folder in Finder; tests replace it.
var revealPath = func(p string) error { return desktop.Reveal(p) }

// PickBuildFolder shows the folder panel (New Folder allowed) and returns the
// chosen folder, "" on cancel. It does not open the folder; the build still
// checks it like any other output.
func (z *Zen) PickBuildFolder() (string, error) {
	if err := requireFeature("export"); err != nil {
		return "", err
	}
	if d, ok := z.dialogs.(*desktop.DialogSet); ok && d.Ready() {
		return d.PickFolderTitled("Choose Output Folder")
	}
	if z.dialogs == nil {
		return "", errors.New("no dialogs available")
	}
	return z.dialogs.PickFolder()
}

// OpenBuiltSite opens the index.html of the last build of the open folder in
// the default browser. It opens nothing else.
func (z *Zen) OpenBuiltSite(out string) error {
	if err := requireFeature("export"); err != nil {
		return err
	}
	index, err := z.s.BuiltSite(out)
	if err != nil {
		return err
	}
	if z.open == nil {
		return errors.New("cannot open the browser")
	}
	return z.open(fileURL(index))
}

// RevealBuiltSite shows the output folder of the last build in Finder.
func (z *Zen) RevealBuiltSite(out string) error {
	if err := requireFeature("export"); err != nil {
		return err
	}
	if _, err := z.s.BuiltSite(out); err != nil {
		return err
	}
	return revealPath(out)
}

func fileURL(p string) string { return (&url.URL{Scheme: "file", Path: filepath.ToSlash(p)}).String() }
