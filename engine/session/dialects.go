package session

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/zenmark-org/zenmd/engine/config"
)

// DialectInfo is one markdown dialect of the build as Settings shows it, and
// whether the open folder turned it on.
type DialectInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	On      bool   `json:"on"`
}

// Dialects lists the markdown dialects the build has, with the open folder's
// choice (specs/engine/dialects). Without a folder none is on.
func (s *Session) Dialects() []DialectInfo {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()
	on := cfg.Dialects() // a nil config has none
	var out []DialectInfo
	for _, d := range s.dbase.List() {
		out = append(out, DialectInfo{ID: d.ID(), Name: d.Name(), Summary: d.Summary(), On: slices.Contains(on, d.ID())})
	}
	return out
}

// SetDialect turns a dialect of the build on or off for the open folder by
// editing its .zenmd.yml (comments and other keys stay; the file is created when
// there is none) and reloads: the pages render again. It is a write, so it
// goes through the read-only guard.
func (s *Session) SetDialect(id string, on bool) (err error) {
	defer s.logFail("SetDialect", &err)
	if err := s.writable(); err != nil {
		return err
	}
	s.mu.Lock()
	ws, cfg := s.ws, s.cfg
	s.mu.Unlock()
	if ws == nil || cfg == nil {
		return ErrNoWorkspace
	}
	if s.dbase.Get(id) == nil {
		return fmt.Errorf("dialect %s is not in this build", id)
	}
	ids := cfg.Dialects()
	switch {
	case on && !slices.Contains(ids, id):
		ids = append(ids, id)
	case !on && slices.Contains(ids, id):
		ids = slices.DeleteFunc(ids, func(x string) bool { return x == id })
	default:
		return nil // already so
	}
	src, err := s.configSource("workspace")
	if err != nil {
		return err
	}
	out, err := config.SetDialects([]byte(src), ids)
	if err != nil {
		return err
	}
	if err := s.saveConfig("workspace", string(out)); err != nil {
		return err
	}
	logApp(slog.LevelInfo, "dialect switched", "id", id, "on", on)
	return nil
}

// DialectHint is the id of a dialect the open folder looks written for (a
// myst.yml, a Jupyter Book _config.yml, a Sphinx conf.py) that the build has
// and the folder neither lists nor dismissed, or "". A read-only app never
// suggests one. Detection opens a few files in the folder's root and runs
// nothing.
func (s *Session) DialectHint() string {
	if s.readOnly {
		return ""
	}
	s.mu.Lock()
	ws, cfg := s.ws, s.cfg
	s.mu.Unlock()
	if ws == nil || cfg == nil {
		return ""
	}
	root := ws.Root()
	listed := cfg.Dialects()
	for _, d := range s.dbase.List() {
		if slices.Contains(listed, d.ID()) || s.hints.dismissed(s.configDir, root, d.ID()) {
			continue
		}
		if d.Detect(rootFiles{root}) {
			return d.ID()
		}
	}
	return ""
}

// DismissDialectHint remembers that the open folder does not want the hint for
// a dialect. The choice is app state (the settings folder), not the folder's:
// it writes nothing the folder owns, so a read-only app may do it too.
func (s *Session) DismissDialectHint(id string) (err error) {
	defer s.logFail("DismissDialectHint", &err)
	if s.dbase.Get(id) == nil {
		return fmt.Errorf("dialect %s is not in this build", id)
	}
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if ws == nil {
		return ErrNoWorkspace
	}
	return s.hints.dismiss(s.configDir, ws.Root(), id)
}

// rootFiles is the folder's root as a read-only file system for detection:
// regular files only, so a link out of the folder is not followed.
type rootFiles struct{ root string }

func (r rootFiles) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	p := filepath.Join(r.root, filepath.FromSlash(name))
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return os.Open(p)
}

// hintStore keeps the dismissed hints: dialect id -> folders. It lives in
// dialect-hints.yml in the settings folder (memory only without one).
type hintStore struct {
	mu     sync.Mutex
	loaded bool
	m      map[string][]string
}

const hintsFile = "dialect-hints.yml"

type hintsDoc struct {
	Dismissed map[string][]string `yaml:"dismissed"`
}

func (h *hintStore) load(dir string) {
	if h.loaded {
		return
	}
	h.loaded = true
	h.m = map[string][]string{}
	if dir == "" {
		return
	}
	b, err := os.ReadFile(filepath.Join(dir, hintsFile))
	if err != nil || len(b) > 1<<20 {
		return
	}
	var d hintsDoc
	if yaml.Unmarshal(b, &d) == nil && d.Dismissed != nil {
		h.m = d.Dismissed
	}
}

func (h *hintStore) dismissed(dir, root, id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.load(dir)
	return slices.Contains(h.m[id], root)
}

func (h *hintStore) dismiss(dir, root, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.load(dir)
	if slices.Contains(h.m[id], root) {
		return nil
	}
	h.m[id] = append(h.m[id], root)
	if dir == "" {
		return nil
	}
	b, err := yaml.Marshal(hintsDoc{Dismissed: h.m})
	if err == nil {
		err = os.MkdirAll(dir, 0o755)
	}
	if err == nil {
		tmp := filepath.Join(dir, "."+hintsFile+".tmp")
		if err = os.WriteFile(tmp, b, 0o644); err == nil {
			err = os.Rename(tmp, filepath.Join(dir, hintsFile))
		}
	}
	if err != nil {
		return errors.New("could not remember that: " + unwrapAll(err).Error())
	}
	return nil
}
