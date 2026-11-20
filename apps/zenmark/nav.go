package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// The sidebar's context menu and reordering (ui/sidebar-menu). The edits are
// the session's; only Finder is the app's.

// revealFile shows a file or folder selected in Finder (`open -R`); tests replace it.
var revealFile = func(abs string) error { return desktop.OpenFile("-R", abs) }

// ReorderNav puts the pages under parent in the order of paths and saves it in
// their frontmatter (nav.order).
func (z *Zen) ReorderNav(parent string, paths []string) error {
	if err := requireFeature("nav"); err != nil {
		return err
	}
	return z.s.ReorderNav(parent, paths)
}

// SetNavHidden hides a page from the sidebar (nav.hidden: true), or shows it again.
func (z *Zen) SetNavHidden(path string, hidden bool) error {
	if err := requireFeature("nav"); err != nil {
		return err
	}
	return z.s.SetNavHidden(path, hidden)
}

// RevealInFinder shows a page or folder of the open folder in Finder; anything
// outside the folder is refused.
func (z *Zen) RevealInFinder(path string) (err error) {
	defer z.logCall("RevealInFinder", &err)
	if err := requireFeature("nav"); err != nil {
		return err
	}
	// The two settings files (Settings › Configuration) are named zen:config/global and zen:config/workspace.
	if which, ok := strings.CutPrefix(path, session.ConfigPrefix); ok {
		file, _, err := z.s.ConfigFile(which)
		if err != nil {
			return err
		}
		if _, err := os.Stat(file); err != nil {
			return errors.New(filepath.Base(file) + " does not exist yet")
		}
		return revealFile(file)
	}
	abs, err := z.s.AbsPath(path)
	if err != nil {
		return err
	}
	return revealFile(abs)
}
