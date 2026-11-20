package session

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/index"
	"github.com/zenmark-org/zenmd/engine/workspace"
)

// navOrderStep is the gap between the numbers ReorderNav writes, so a page
// added by hand later fits between two.
const navOrderStep = 10

// siblingsOf finds the node list that holds the page at path and the path of
// the node that list belongs to ("" for the top level and for "Other pages").
func siblingsOf(nodes []NavNode, path, parent string) (list []NavNode, owner string, ok bool) {
	for _, n := range nodes {
		if n.Kind == "page" && n.Path == path {
			return nodes, parent, true
		}
	}
	for _, n := range nodes {
		if l, o, ok := siblingsOf(n.Children, path, n.Path); ok {
			return l, o, true
		}
	}
	return nil, "", false
}

// ReorderNav puts the pages under parent in the order of paths: parent is the
// path of the page or folder above them ("" for the top level and for "Other
// pages") and paths holds every page node of that level, once. The pages are
// numbered 10, 20, 30… in nav.order, and only the files whose number changes
// are written, each through its frontmatter alone (config.SetNavKey). All
// edits are prepared before the first write, and a failing write puts the
// earlier files back. The tree is rebuilt at once and workspace:changed sent.
func (s *Session) ReorderNav(parent string, paths []string) (err error) {
	defer s.logFail("ReorderNav", &err)
	if err := s.writable(); err != nil {
		return err
	}
	s.mu.Lock()
	ws, err := s.reorderLocked(parent, paths)
	s.mu.Unlock()
	if ws != nil {
		s.emit(EventWorkspaceChanged, ws)
	}
	return err
}

type navEdit struct{ path, before, after string }

func (s *Session) reorderLocked(parent string, paths []string) (*Workspace, error) {
	if s.ws == nil {
		return nil, errNoWorkspace
	}
	if len(paths) == 0 {
		return nil, errors.New("nothing to reorder")
	}
	list, owner, ok := siblingsOf(s.tree, paths[0], "")
	if !ok {
		return nil, fmt.Errorf("%s: not a page in the sidebar", paths[0])
	}
	if owner != parent {
		return nil, fmt.Errorf("%s: not under %q", paths[0], parent)
	}
	siblings := map[string]bool{}
	for _, n := range list {
		if n.Kind == "page" {
			siblings[n.Path] = true
		}
	}
	seen := map[string]bool{}
	for _, p := range paths {
		switch {
		case seen[p]:
			return nil, fmt.Errorf("%s: listed twice", p)
		case !siblings[p]:
			return nil, fmt.Errorf("%s: not a page next to %s", p, paths[0])
		}
		seen[p] = true
	}
	if len(paths) != len(siblings) {
		return nil, errors.New("give every page of the group, in the new order")
	}

	// Prepare every edit before writing anything.
	var edits []navEdit
	for i, p := range paths {
		want := (i + 1) * navOrderStep
		src, err := s.ws.Read(p)
		if err != nil {
			return nil, s.cleanErr(p, err)
		}
		fm, _, _ := config.SplitFrontmatter(src)
		if m, _, _ := index.ParseMeta(fm); m.Order != nil && *m.Order == want {
			continue
		}
		v := strconv.Itoa(want)
		out, err := config.SetNavKey(src, "order", &v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		edits = append(edits, navEdit{p, string(src), string(out)})
	}
	if len(edits) == 0 {
		return nil, nil
	}
	return s.writeNavLocked(edits)
}

// writeNavLocked writes the edits in order, undoes them when one fails, and
// reindexes; the returned workspace is non-nil when the tree changed.
func (s *Session) writeNavLocked(edits []navEdit) (*Workspace, error) {
	for i, e := range edits {
		if err := s.ws.Write(e.path, []byte(e.after)); err != nil {
			for _, done := range edits[:i] {
				_ = s.ws.Write(done.path, []byte(done.before)) // best effort
			}
			return nil, s.cleanErr(e.path, err)
		}
	}
	if s.reindexLocked() {
		return s.workspaceLocked(), nil
	}
	return nil, nil
}

// SetNavHidden writes `nav: hidden: true` into the page's frontmatter, or with
// hidden false removes the key, and rebuilds the tree. The page may be one the
// sidebar does not show (hidden), so it can be shown again.
func (s *Session) SetNavHidden(path string, hidden bool) (err error) {
	defer s.logFail("SetNavHidden", &err)
	if err := s.writable(); err != nil {
		return err
	}
	s.mu.Lock()
	ws, err := s.hideLocked(path, hidden)
	s.mu.Unlock()
	if ws != nil {
		s.emit(EventWorkspaceChanged, ws)
	}
	return err
}

func (s *Session) hideLocked(path string, hidden bool) (*Workspace, error) {
	if s.ws == nil {
		return nil, errNoWorkspace
	}
	if !isMarkdownPath(path) {
		return nil, s.cleanErr(path, workspace.ErrNotMarkdown)
	}
	src, err := s.ws.Read(path)
	if err != nil {
		return nil, s.cleanErr(path, err)
	}
	var v *string
	if hidden {
		t := "true"
		v = &t
	}
	out, err := config.SetNavKey(src, "hidden", v)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if string(out) == string(src) {
		return nil, nil
	}
	return s.writeNavLocked([]navEdit{{path, string(src), string(out)}})
}

// AbsPath is the absolute path of a file or folder inside the open folder
// ("" is the folder itself); it fails for anything outside it or missing.
func (s *Session) AbsPath(rel string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return "", errNoWorkspace
	}
	p, err := s.ws.Resolve(rel)
	if err != nil {
		return "", s.cleanErr(rel, err)
	}
	if _, err := os.Lstat(p); err != nil {
		return "", s.cleanErr(rel, err)
	}
	return p, nil
}
