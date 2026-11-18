package app

import (
	"errors"
	"path/filepath"

	"github.com/dragosh/zenmd/engine/session"
	"github.com/dragosh/zenmd/kit/desktop"
)

// openFile opens a file in the system's default text editor. It is only the
// fallback when there are no editor windows (tests, headless use).
var openFile = func(path string) error {
	return desktop.OpenFile("-t", path)
}

// OpenConfig opens the global config file ("global") or the current
// folder's .zen.yml ("workspace") in zen's own editor window, creating it
// from a commented template first when it does not exist.
func (z *Zen) OpenConfig(which string) error {
	path, err := z.s.EnsureConfig(which)
	if err != nil {
		return err
	}
	if z.windows == nil {
		if err := openFile(path); err != nil {
			return errors.New("could not open " + filepath.Base(path))
		}
		return nil
	}
	return z.windows.OpenEditor(session.ConfigPrefix+which, filepath.Base(path))
}
