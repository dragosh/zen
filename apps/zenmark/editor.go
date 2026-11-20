package app

import (
	"errors"
	"path"
	"strings"

	"github.com/zenmark-org/zenmd/engine/session"
)

// OpenEditor opens the page in its own editor window, or focuses the one
// already open for it.
func (z *Zen) OpenEditor(p string) (err error) {
	defer z.logCall("OpenEditor", &err)
	if z.ro {
		return session.ErrReadOnly
	}
	if p == "" || p != path.Clean(p) || path.IsAbs(p) || strings.HasPrefix(p, "../") {
		return errors.New(p + ": not a page in this folder")
	}
	if _, err := z.s.PageSource(p); err != nil { // markdown, inside the workspace, readable
		return err
	}
	if z.windows == nil {
		return errors.New("editor windows are not available")
	}
	return z.windows.OpenEditor(p, z.pageTitle(p))
}

// pageTitle is the page's title in the navigation tree, "" when absent.
func (z *Zen) pageTitle(p string) string {
	ws := z.s.Workspace()
	if ws == nil {
		return ""
	}
	var find func([]NavNode) string
	find = func(ns []NavNode) string {
		for _, n := range ns {
			if n.Path == p && n.Kind == "page" {
				return n.Title
			}
			if t := find(n.Children); t != "" {
				return t
			}
		}
		return ""
	}
	return find(ws.Tree)
}

// openStartupEditor opens the editor window `zenmd edit <path>` asked for; the
// main window itself always starts in read mode.
func (z *Zen) openStartupEditor(mode string) error {
	if mode != "edit" || z.ro { // a read-only app opens the page in the reader
		return nil
	}
	p := z.Startup().Page
	if p == "" {
		return nil
	}
	return z.OpenEditor(p)
}
