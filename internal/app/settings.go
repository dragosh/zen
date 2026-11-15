package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dragosh/zen/internal/core/config"
)

// Templates written when a config file is opened for the first time. Every
// setting is commented out, so creating the file changes nothing.
const globalTemplate = `# zen global settings (all windows, all folders).
# Docs: docs/PLUGINS.md "Enabling, disabling, configuring".
version: 1

# plugins:
#   highlight: true          # syntax colours in code blocks
#   diagram: true            # mermaid diagrams
#   lint: true               # Vale spelling and style flags

# limits:                    # folders larger than this are refused
#   files: 2000              # markdown files (max 100000)
#   entries: 50000           # files and folders scanned (max 5000000)
`

const workspaceTemplate = `# zen settings for this folder (override the global config.yml;
# a page's frontmatter overrides these). Docs: docs/PLUGINS.md.
version: 1

# plugins:
#   lint: false              # turn off Vale for this folder
#   diagram: { theme: neutral }
`

// openFile opens a file in the system's default text editor. Tests replace it.
var openFile = func(path string) error {
	return exec.Command("open", "-t", path).Run()
}

// OpenConfig opens the global config file ("global") or the current
// folder's .zen.yml ("workspace") in the default text editor, creating it
// from a commented template first when it does not exist.
func (z *Zen) OpenConfig(which string) error {
	var path, template string
	switch which {
	case "global":
		if z.state.globalPath == "" {
			return errors.New("no settings folder on this system")
		}
		path, template = z.state.globalPath, globalTemplate
	case "workspace":
		ws := z.state.workspace()
		if ws == nil {
			return errors.New("no folder is open")
		}
		path, template = filepath.Join(ws.Root, config.WorkspaceFile), workspaceTemplate
	default:
		return errors.New("unknown settings file " + which)
	}
	if err := ensureFile(path, template); err != nil {
		return errors.New("could not create " + filepath.Base(path) + ": " + errors.Unwrap(err).Error())
	}
	if err := openFile(path); err != nil {
		return errors.New("could not open " + filepath.Base(path))
	}
	return nil
}

// ensureFile writes template to path unless a file is already there.
func ensureFile(path, template string) error {
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return &os.PathError{Op: "stat", Path: path, Err: err}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return &os.PathError{Op: "mkdir", Path: path, Err: unwrapAll(err)}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return &os.PathError{Op: "create", Path: path, Err: unwrapAll(err)}
	}
	defer f.Close()
	if _, err := f.WriteString(template); err != nil {
		return &os.PathError{Op: "write", Path: path, Err: unwrapAll(err)}
	}
	return nil
}

func unwrapAll(err error) error {
	for {
		u := errors.Unwrap(err)
		if u == nil {
			return err
		}
		err = u
	}
}
