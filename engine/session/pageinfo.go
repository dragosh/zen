package session

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/zenmark-org/zenmd/engine/workspace"
)

// PageInfo describes the file behind a page, for the Inspector.
type PageInfo struct {
	Path      string `json:"path"`      // as asked, slash separated, relative to the folder
	AbsPath   string `json:"absPath"`   // the file's real location, symlinks resolved
	Size      int64  `json:"size"`      // bytes
	ModTime   int64  `json:"modTime"`   // Unix milliseconds
	BirthTime int64  `json:"birthTime"` // Unix milliseconds, 0 when the system does not know it
}

// PageInfo returns the size, times and location of a markdown file inside the
// open folder. Like Page and Source it accepts nothing that resolves outside
// the folder, and its errors carry no absolute path.
func (s *Session) PageInfo(path string) (r0 PageInfo, err error) {
	defer s.logFail("PageInfo", &err)
	if !isMarkdownPath(path) {
		s.mu.Lock()
		err := s.cleanErr(path, workspace.ErrNotMarkdown)
		s.mu.Unlock()
		return PageInfo{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return PageInfo{}, errNoWorkspace
	}
	abs, err := s.ws.Resolve(path)
	if err != nil {
		return PageInfo{}, s.cleanErr(path, err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return PageInfo{}, s.cleanErr(path, err)
	}
	if !fi.Mode().IsRegular() {
		return PageInfo{}, errors.New(path + ": not a file")
	}
	return PageInfo{
		Path:      filepath.ToSlash(path),
		AbsPath:   abs,
		Size:      fi.Size(),
		ModTime:   fi.ModTime().UnixMilli(),
		BirthTime: birthTime(fi),
	}, nil
}
