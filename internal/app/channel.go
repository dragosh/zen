package app

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Channel is the release channel this binary was built for: "appstore"
// (go build -tags appstore: sandboxed, Mac App Store) or "devid" (Developer ID).
func Channel() string {
	if appStore {
		return "appstore"
	}
	return "devid"
}

// Errors of folder access in the sandbox.
var (
	// ErrNoBookmark: zen holds no bookmark for the folder yet.
	ErrNoBookmark = errors.New("no saved access to this folder")
	// ErrStale: the bookmark no longer points at the folder (it was moved or
	// renamed); the user is asked again.
	ErrStale = errors.New("saved access to this folder is out of date")
	// ErrGone: the folder is on a disk that is not mounted.
	ErrGone = errors.New("the folder is not available (is its disk connected?)")
	// ErrDeclined: the user closed the folder panel.
	ErrDeclined = errors.New("no folder chosen")
)

// folderAccess keeps the sandbox's access to the folders the user opened.
type folderAccess interface {
	// Remember stores a security-scoped bookmark for dir.
	Remember(dir string) error
	// Resolve begins access to dir from its bookmark. stop ends it. ErrNoBookmark
	// when there is none, ErrStale when it no longer matches.
	Resolve(dir string) (stop func(), err error)
}

// folderPicker shows the native folder panel, preselected on preselect, with
// a message line. It returns "" when the user cancels.
type folderPicker func(preselect, message string) (string, error)

const (
	folderMessage = "Allow zen to open this folder"
	fileMessage   = "Allow zen to show the pages next to this file"
)

func noop() {}

// volumeMounted is false for a path on /Volumes/<disk> whose disk is gone.
func volumeMounted(dir string) bool {
	const v = "/Volumes/"
	if !strings.HasPrefix(dir, v) {
		return true
	}
	disk, _, _ := strings.Cut(strings.TrimPrefix(dir, v), "/")
	_, err := os.Stat(v + disk)
	return err == nil
}

// openWithAccess begins access to dir: from its bookmark when there is a
// good one, otherwise by asking with the folder panel (preselected on dir).
// The panel's choice replaces the bookmark. A folder on a disk that is not
// mounted is an error, not a question.
func openWithAccess(fa folderAccess, dir string, pick folderPicker) (root string, stop func(), err error) {
	if !volumeMounted(dir) {
		return "", noop, ErrGone
	}
	stop, err = fa.Resolve(dir)
	switch {
	case err == nil:
		return dir, stop, nil
	case !errors.Is(err, ErrNoBookmark) && !errors.Is(err, ErrStale):
		return "", noop, err
	}
	chosen, err := pick(dir, folderMessage)
	if err != nil {
		return "", noop, err
	}
	if chosen == "" {
		return "", noop, ErrDeclined
	}
	if err := fa.Remember(chosen); err != nil {
		return "", noop, err
	}
	if stop, err = fa.Resolve(chosen); err != nil {
		return "", noop, err
	}
	return chosen, stop, nil
}

// openFileWithAccess is for a single file the user opened: the sandbox gave
// zen that file only. zen asks once for the folder next to it, so it can build
// the sidebar. root is "" when the user declines (the file is shown alone).
func openFileWithAccess(fa folderAccess, file string, pick folderPicker) (root string, stop func(), err error) {
	dir := filepath.Dir(file)
	if volumeMounted(dir) {
		if stop, err := fa.Resolve(dir); err == nil {
			return dir, stop, nil
		}
	}
	chosen, err := pick(dir, fileMessage)
	if err != nil {
		return "", noop, err
	}
	if chosen == "" {
		return "", noop, nil
	}
	if err := fa.Remember(chosen); err != nil {
		return "", noop, err
	}
	if stop, err = fa.Resolve(chosen); err != nil {
		return "", noop, err
	}
	return chosen, stop, nil
}

// importMarker records that the one-time offer to import the old config was made.
const importMarker = ".config-import-offered"

const maxImport = 1 << 20

// importConfigOnce offers once (per container) to import the Developer ID
// build's config.yml, which the sandbox does not let zen read silently. The
// offer is remembered even when declined. An existing config.yml is never
// replaced. It reports whether a file was imported.
func importConfigOnce(configDir string, offer func() (path string, ok bool)) (bool, error) {
	marker := filepath.Join(configDir, importMarker)
	if _, err := os.Stat(marker); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return false, err
	}
	if err := os.WriteFile(marker, []byte("offered\n"), 0o600); err != nil {
		return false, err
	}
	dst := filepath.Join(configDir, "config.yml")
	if _, err := os.Stat(dst); err == nil {
		return false, nil
	}
	src, ok := offer()
	if !ok || src == "" {
		return false, nil
	}
	f, err := os.Open(src)
	if err != nil {
		return false, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxImport+1))
	if err != nil {
		return false, err
	}
	if len(b) > maxImport {
		return false, errors.New("the config file is too large to import")
	}
	return true, os.WriteFile(dst, b, 0o600)
}

// bookmarker makes and resolves macOS security-scoped bookmarks.
type bookmarker interface {
	// Create returns bookmark data for path.
	Create(path string) ([]byte, error)
	// Resolve starts access from data. It returns the path the bookmark now
	// points at and whether the data is stale; stop ends the access.
	Resolve(data []byte) (path string, stale bool, stop func(), err error)
}

// storeAccess is a folderAccess that keeps bookmarks in a JSON file (in the
// app container, next to the trust file).
type storeAccess struct {
	file string
	mk   bookmarker
	mu   sync.Mutex
}

func newStoreAccess(file string, mk bookmarker) *storeAccess {
	return &storeAccess{file: file, mk: mk}
}

func (s *storeAccess) load() map[string][]byte {
	m := map[string][]byte{}
	if b, err := os.ReadFile(s.file); err == nil {
		if json.Unmarshal(b, &m) != nil {
			m = map[string][]byte{} // a damaged file is ignored: the user is asked again
		}
	}
	return m
}

func (s *storeAccess) Remember(dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.mk.Create(dir)
	if err != nil {
		return err
	}
	m := s.load()
	m[dir] = data
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.file), 0o700); err != nil {
		return err
	}
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.file)
}

func (s *storeAccess) Resolve(dir string) (func(), error) {
	s.mu.Lock()
	data, ok := s.load()[dir]
	s.mu.Unlock()
	if !ok {
		return nil, ErrNoBookmark
	}
	path, stale, stop, err := s.mk.Resolve(data)
	if err != nil {
		return nil, err
	}
	if stale || filepath.Clean(path) != filepath.Clean(dir) {
		stop()
		return nil, ErrStale
	}
	return stop, nil
}
