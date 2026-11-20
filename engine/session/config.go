package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zenmark-org/zenmd/engine/config"
)

// ConfigPrefix marks the two config files as editor "pages":
// zen:config/global (config.yml) and zen:config/workspace (.zen.yml). A colon
// never appears in a workspace page path the editor accepts, so these cannot
// collide with real pages.
const ConfigPrefix = "zen:config/"

// configWhich reports whether path names a config file, and which one.
func configWhich(path string) (string, bool) {
	which, ok := strings.CutPrefix(path, ConfigPrefix)
	return which, ok && (which == "global" || which == "workspace")
}

// Templates written when a config file is opened for the first time. Every
// setting is commented out, so creating the file changes nothing.
const globalTemplate = `# zen global settings (all windows, all folders).
# Docs: docs/PLUGINS.md "Enabling, disabling, configuring".
version: 1

# plugins:
#   codeblock: true          # code colours, Copy and Wrap buttons
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

// ConfigFile returns the file and its first-use template for "global" or
// "workspace".
func (s *Session) ConfigFile(which string) (path, template string, err error) {
	switch which {
	case "global":
		if s.globalPath == "" {
			return "", "", errors.New("no settings folder on this system")
		}
		return s.globalPath, globalTemplate, nil
	case "workspace":
		ws := s.Workspace()
		if ws == nil {
			return "", "", errors.New("no folder is open")
		}
		return filepath.Join(ws.Root, config.WorkspaceFile), workspaceTemplate, nil
	}
	return "", "", errors.New("unknown settings file " + which)
}

// EnsureConfig creates the global config file ("global") or the open folder's
// .zen.yml ("workspace") from a commented template when it does not exist,
// and returns its path.
func (s *Session) EnsureConfig(which string) (string, error) {
	path, template, err := s.ConfigFile(which)
	if err != nil {
		return "", err
	}
	if _, serr := os.Stat(path); serr != nil && s.readOnly { // it would be created
		return "", ErrReadOnly
	}
	if err := ensureFile(path, template); err != nil {
		return "", errors.New("could not create " + filepath.Base(path) + ": " + unwrapAll(err).Error())
	}
	return path, nil
}

// configSource reads a config file for the editor ("" when it does not exist yet).
func (s *Session) configSource(which string) (string, error) {
	path, _, err := s.ConfigFile(which)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("could not read " + filepath.Base(path) + ": " + unwrapAll(err).Error())
	}
	return string(b), nil
}

// saveConfig checks that content is a YAML mapping, writes it atomically and
// applies the new settings to the open folder.
func (s *Session) saveConfig(which, content string) error {
	path, _, err := s.ConfigFile(which)
	if err != nil {
		return err
	}
	name := filepath.Base(path)
	var doc any
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return fmt.Errorf("%s is not valid YAML, not saved: %s", name, strings.TrimPrefix(err.Error(), "yaml: "))
	}
	if _, isMap := doc.(map[string]any); doc != nil && !isMap {
		return fmt.Errorf("%s must be a mapping of settings (key: value), not saved", name)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errors.New("could not save " + name + ": " + unwrapAll(err).Error())
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+name+".*")
	if err != nil {
		return errors.New("could not save " + name + ": " + unwrapAll(err).Error())
	}
	_, werr := tmp.WriteString(content)
	if werr == nil {
		werr = tmp.Chmod(0o644)
	}
	cerr := tmp.Close()
	if werr == nil && cerr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return errors.New("could not save " + name)
	}
	s.reloadConfig()
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
