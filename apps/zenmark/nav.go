package app

import "github.com/zenmark-org/zenmd/kit/desktop"

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
func (z *Zen) RevealInFinder(path string) error {
	if err := requireFeature("nav"); err != nil {
		return err
	}
	abs, err := z.s.AbsPath(path)
	if err != nil {
		return err
	}
	return revealFile(abs)
}
