package extplugins

// The pieces that every kind of folder plugin shares: what trust works on,
// the cheap "did anything change" check, and the hardened hashing and path
// containment. Script plugins (core/scriptplugins) use them as they are, so
// the rules of docs/plugin-review-2026-11.md exist in one place.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/dragosh/zenmd/engine/plugins"
)

// Subject is what the user allows or blocks: a plugin folder or a plugin
// file. Dir is its path (the key of the decision), Hash the SHA-256 taken
// when it was loaded (what the user approves) and Changed the cheap check that
// anything differs from what was loaded.
type Subject interface {
	Dir() string
	Hash() (string, error)
	Changed() bool
}

// Guard remembers a fingerprint taken at load and compares it with the current
// one on demand. Once a difference is seen Changed stays true and the OnChange
// function is called, once.
type Guard struct {
	loaded   string
	current  func() (string, error)
	changed  atomic.Bool
	onChange atomic.Pointer[func()]
}

// NewGuard returns a Guard for the fingerprint loaded; current takes the
// fingerprint of what is on disk now (an error counts as a change).
func NewGuard(loaded string, current func() (string, error)) *Guard {
	return &Guard{loaded: loaded, current: current}
}

// OnChange sets f to be called (in the calling goroutine, once) when a change
// is found.
func (g *Guard) OnChange(f func()) { g.onChange.Store(&f) }

// Changed reports whether anything differs from what was loaded.
func (g *Guard) Changed() bool {
	if g.changed.Load() {
		return true
	}
	if fp, err := g.current(); err == nil && fp == g.loaded {
		return false
	}
	if g.changed.CompareAndSwap(false, true) {
		if f := g.onChange.Load(); f != nil {
			(*f)()
		}
	}
	return true
}

// ScanFolder resolves the plugin folder dir and hashes it with the same rules
// as a program plugin: every file, a symlink by its target's content, nothing
// skipped but Finder's .DS_Store; a symlink out of the folder, a special file,
// more than 500 files or 20 MB reject it. It returns the real path, the hash
// and the fingerprint.
func ScanFolder(dir string) (real, hash, fingerprint string, err error) {
	real, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", "", "", errors.New("plugin folder not readable")
	}
	hash, fingerprint, err = scanFolder(real, true)
	if err != nil {
		return "", "", "", err
	}
	return real, hash, fingerprint, nil
}

// FolderFingerprint is the cheap fingerprint of a plugin folder (paths, modes,
// sizes, modification times; no file is read), as taken by ScanFolder.
func FolderFingerprint(real string) (string, error) {
	_, fp, err := scanFolder(real, false)
	return fp, err
}

func fileFingerprint(fi os.FileInfo) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%o/%d/%d", fi.Mode(), fi.Size(), fi.ModTime().UnixNano())))
	return hex.EncodeToString(sum[:])
}

// FileFingerprint is the cheap fingerprint of a single plugin file (its path
// is not followed if it is a symlink).
func FileFingerprint(path string) (string, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return "", errors.New("plugin file not readable")
	}
	return fileFingerprint(fi), nil
}

// HashFile reads a single-file plugin: a regular file (never a symlink, which
// could be repointed without a trace), at most max bytes. It returns the
// content, the hash of exactly those bytes and the fingerprint taken before
// reading; a file that changed while it was read is refused.
func HashFile(path string, max int64) (data []byte, hash, fingerprint string, err error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, "", "", errors.New("plugin file not found")
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		return nil, "", "", errors.New("a plugin file may not be a symlink")
	case !fi.Mode().IsRegular():
		return nil, "", "", errors.New("the plugin is not a regular file")
	}
	data, err = readRegular(path, max)
	if err != nil {
		return nil, "", "", err
	}
	after, err := os.Lstat(path)
	if err != nil || fileFingerprint(after) != fileFingerprint(fi) {
		return nil, "", "", errors.New("the plugin file changed while it was read")
	}
	sum := sha256.Sum256(append([]byte("zen-script-file\x00"), data...))
	return data, hex.EncodeToString(sum[:]), fileFingerprint(fi), nil
}

// ReadRegular reads a regular file, never more than max bytes ("too large"
// otherwise), without blocking on a pipe.
func ReadRegular(path string, max int64) ([]byte, error) { return readRegular(path, max) }

// ResolveInside resolves a path relative to a plugin folder (symlinks
// included) and requires the result to be inside it. what only names the
// thing in messages.
func ResolveInside(real, rel, what string) (string, error) { return resolveInside(real, rel, what) }

// CheckInside requires name to be a plain relative file path that exists
// inside the plugin folder (and is not a .DS_Store).
func CheckInside(real, name, what string) error { return checkInside(real, name, what) }

// InsideRel reports whether full (already resolved) is inside the folder real.
func InsideRel(real, full string) bool { return insideRel(real, full) }

// ParseOption validates one option declaration (type, default, description,
// scope) as plugin.yml does.
func ParseOption(typ string, def any, description, scope string) (plugins.Option, error) {
	switch scope {
	case "", plugins.ScopeUser:
	default:
		return plugins.Option{}, fmt.Errorf("scope %q is not user", scope)
	}
	d, err := optionDefault(rawOption{Type: typ, Default: def, Description: description, Scope: scope})
	if err != nil {
		return plugins.Option{}, err
	}
	return plugins.Option{Type: typ, Default: d, Description: description, Scope: scope}, nil
}
