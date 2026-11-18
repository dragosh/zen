// Package atomicfile writes small state files (trust decisions, plugin
// storage) so that a crash or a full disk never leaves half a file, and sets
// aside files that cannot be read instead of overwriting them.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Write replaces path with data: a temporary file in the same folder is
// written, synced and renamed over path, and the folder is synced. The file
// is readable by the user only.
func Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	if d, err := os.Open(dir); err == nil { // make the rename durable; best effort
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// SetAside renames a file that could not be parsed to <path>.corrupt-<time>
// and returns the new name, so the next write does not destroy it.
func SetAside(path string) (string, error) {
	to := fmt.Sprintf("%s.corrupt-%s", path, time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.Rename(path, to); err != nil {
		return "", err
	}
	return to, nil
}
