package extplugins

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrChanged is the error of a call on a plugin whose folder changed after it
// was loaded and approved.
var ErrChanged = errors.New("plugin changed; allow it again")

// scanFolder reads the plugin folder once. With read set it returns the
// SHA-256 over the sorted relative paths, kinds and contents of everything in
// it (nothing is skipped; a symlink is hashed by the content of its target);
// without it no file is opened. Both give a fingerprint (paths, modes, sizes
// and modification times) for the cheap "did anything change" check.
//
// The folder is rejected when it has more than maxFiles entries or maxBytes of
// content, a special file (pipe, socket, device: never opened), a symlink
// that does not resolve to a regular file inside the folder, or a file that
// cannot be read. Messages name paths relative to the folder only.
func scanFolder(real string, read bool) (hash, fingerprint string, err error) {
	h := sha256.New()
	fp := sha256.New()
	put := func(w interface{ Write([]byte) (int, error) }, s string) {
		w.Write([]byte(strconv.Itoa(len(s))))
		w.Write([]byte{':'})
		w.Write([]byte(s))
	}
	files, total := 0, int64(0)
	err = filepath.WalkDir(real, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.New("plugin folder not readable")
		}
		if path == real {
			return nil
		}
		rel, _ := filepath.Rel(real, path)
		rel = filepath.ToSlash(rel)
		files++
		if files > maxFiles {
			return fmt.Errorf("the plugin folder has more than %d files", maxFiles)
		}
		fi, err := d.Info() // from Lstat: a symlink is not followed
		if err != nil {
			return errors.New("plugin folder not readable")
		}
		put(h, rel)
		put(fp, rel)
		put(fp, fmt.Sprintf("%o/%d/%d", fi.Mode(), fi.Size(), fi.ModTime().UnixNano()))
		mode := fi.Mode()
		switch {
		case mode.IsDir():
			put(h, "d")
		case mode&fs.ModeSymlink != 0:
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("symlink %s does not resolve", rel)
			}
			if back, err := filepath.Rel(real, target); err != nil || back == ".." || strings.HasPrefix(back, ".."+string(filepath.Separator)) {
				return fmt.Errorf("symlink %s points outside the plugin folder", rel)
			}
			ti, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("symlink %s does not resolve", rel)
			}
			switch {
			case ti.IsDir():
				return fmt.Errorf("symlink %s points to a folder (only files may be linked)", rel)
			case !ti.Mode().IsRegular():
				return fmt.Errorf("%s is a special file (pipe, socket or device), which a plugin folder cannot have", rel)
			}
			put(fp, fmt.Sprintf("->%o/%d/%d", ti.Mode(), ti.Size(), ti.ModTime().UnixNano()))
			if total += ti.Size(); total > maxBytes {
				return errors.New("the plugin folder is larger than 20 MB")
			}
			put(h, "l")
			if read {
				if err := hashFile(h, put, target, ti, maxBytes-(total-ti.Size())); err != nil {
					return fmt.Errorf("%s: %v", rel, err)
				}
			}
		case mode.IsRegular():
			if total += fi.Size(); total > maxBytes {
				return errors.New("the plugin folder is larger than 20 MB")
			}
			if read {
				if err := hashFile(h, put, path, fi, maxBytes-(total-fi.Size())); err != nil {
					return fmt.Errorf("%s: %v", rel, err)
				}
			}
		default:
			return fmt.Errorf("%s is a special file (pipe, socket or device), which a plugin folder cannot have", rel)
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	if read {
		hash = hex.EncodeToString(h.Sum(nil))
	}
	return hash, hex.EncodeToString(fp.Sum(nil)), nil
}

// hashFile adds a regular file's executable bit and content to h; room is how
// many bytes may still be read.
func hashFile(h interface{ Write([]byte) (int, error) }, put func(interface{ Write([]byte) (int, error) }, string), path string, fi fs.FileInfo, room int64) error {
	data, err := readRegular(path, room)
	if err != nil {
		return err
	}
	if fi.Mode().Perm()&0o111 != 0 {
		put(h, "x")
	} else {
		put(h, "f")
	}
	put(h, string(data))
	return nil
}

// readRegular reads a file that must be a regular file, never more than max
// bytes. It is opened without blocking, so a pipe swapped in after a check
// cannot hang it, and the opened file is checked again.
func readRegular(path string, max int64) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("not readable")
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if max < 0 {
		max = 0
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, errors.New("not readable")
	}
	if int64(len(data)) > max {
		return nil, errors.New("too large")
	}
	return data, nil
}

// insideRel reports whether full (already resolved) is inside the folder real.
func insideRel(real, full string) bool {
	back, err := filepath.Rel(real, full)
	return err == nil && back != ".." && !strings.HasPrefix(back, ".."+string(filepath.Separator))
}

// checkArgs refuses a command with an argument that names an existing file
// (relative to the plugin folder, which is the working directory, or
// absolute) that does not resolve, symlinks followed, inside the folder. It
// runs at load and again before every spawn, so a file that appears later
// is caught too.
func checkArgs(real string, command []string) error {
	for i, a := range command {
		if i == 0 || a == "" || strings.ContainsRune(a, 0) {
			continue
		}
		path := a
		if !filepath.IsAbs(path) {
			path = filepath.Join(real, path)
		}
		if _, err := os.Lstat(path); err != nil {
			continue // not a file: a word, an option or a value
		}
		full, err := filepath.EvalSymlinks(path)
		if err != nil || !insideRel(real, full) {
			return fmt.Errorf("backend.command argument %s names a file outside the plugin folder", a)
		}
	}
	return nil
}
